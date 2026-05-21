package provider

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure RecordResource satisfies the resource.Resource interface.
var _ resource.Resource = &RecordResource{}
var _ resource.ResourceWithImportState = &RecordResource{}

// RecordResource defines the resource implementation.
type RecordResource struct {
	client *opusdns.Client
}

// RecordResourceModel describes the resource data model.
// Each resource represents a full RRSet (same name + type, potentially multiple rdata values).
type RecordResourceModel struct {
	ID       types.String `tfsdk:"id"`
	ZoneName types.String `tfsdk:"zone_name"`
	Name     types.String `tfsdk:"name"`
	Type     types.String `tfsdk:"type"`
	TTL      types.Int64  `tfsdk:"ttl"`
	Records  types.List   `tfsdk:"records"`
}

// NewRecordResource returns a new RecordResource.
func NewRecordResource() resource.Resource {
	return &RecordResource{}
}

func (r *RecordResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_record"
}

func (r *RecordResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a DNS record set (RRSet) in an OpusDNS zone. Each resource represents all records for a given name and type within a zone.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier in the format `zone_name/name/type`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"zone_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The name of the DNS zone that contains the record.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The record name relative to the zone (e.g., `www` or `@` for apex).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The DNS record type (e.g., `A`, `AAAA`, `CNAME`, `MX`, `TXT`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"ttl": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(60),
				MarkdownDescription: "The time-to-live in seconds. Defaults to `60`.",
			},
			"records": schema.ListAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "The list of record data values (e.g., IP addresses for A records, hostnames for CNAME).",
			},
		},
	}
}

func (r *RecordResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*opusdns.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *opusdns.Client, got: %T.", req.ProviderData),
		)
		return
	}
	r.client = client
}

func (r *RecordResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RecordResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rrset := buildRRSet(ctx, data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DNS.PatchRRSets(ctx, data.ZoneName.ValueString(), []models.RRSetPatchOp{
		{Op: models.RecordOpUpsert, RRSet: rrset},
	})
	if err != nil {
		resp.Diagnostics.AddError("Error creating DNS record", formatAPIError(err))
		return
	}

	data.ID = types.StringValue(recordID(data.ZoneName.ValueString(), data.Name.ValueString(), data.Type.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RecordResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RecordResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneName := data.ZoneName.ValueString()
	recordName := data.Name.ValueString()
	recordType := data.Type.ValueString()
	if zoneName == "" || recordName == "" || recordType == "" {
		resp.Diagnostics.AddError(
			"Invalid DNS record state",
			"The opusdns_record resource has an empty `zone_name`, `name`, or `type` in state, which prevents reading it from the API. "+
				"Remove the resource from state with `terraform state rm` and re-import or recreate it.",
		)
		return
	}

	zone, err := r.client.DNS.GetZone(ctx, zoneName)
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading DNS zone", formatAPIError(err))
		return
	}

	rrset := findRRSet(zone.RRSets, recordName, models.RRSetType(recordType), zoneName)
	if rrset == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	rdatas := normalizedRecordData(string(rrset.Type), rrset.Records)

	recordList, diags := types.ListValueFrom(ctx, types.StringType, rdatas)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.TTL = types.Int64Value(int64(rrset.TTL))
	data.Records = recordList
	data.ID = types.StringValue(recordID(zoneName, recordName, recordType))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RecordResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RecordResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	rrset := buildRRSet(ctx, data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DNS.PatchRRSets(ctx, data.ZoneName.ValueString(), []models.RRSetPatchOp{
		{Op: models.RecordOpUpsert, RRSet: rrset},
	})
	if err != nil {
		resp.Diagnostics.AddError("Error updating DNS record", formatAPIError(err))
		return
	}

	data.ID = types.StringValue(recordID(data.ZoneName.ValueString(), data.Name.ValueString(), data.Type.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RecordResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RecordResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneName := data.ZoneName.ValueString()
	recordName := data.Name.ValueString()
	recordType := data.Type.ValueString()
	if zoneName == "" || recordName == "" || recordType == "" {
		resp.Diagnostics.AddError(
			"Invalid DNS record state",
			"The opusdns_record resource has an empty `zone_name`, `name`, or `type` in state, which prevents deletion via the API. "+
				"Remove the resource from state with `terraform state rm` and, if the record still exists at OpusDNS, delete it manually or re-import then destroy.",
		)
		return
	}

	err := r.client.DNS.PatchRRSets(ctx, zoneName, []models.RRSetPatchOp{
		{
			Op: models.RecordOpRemove,
			RRSet: models.RRSetPatch{
				Name: recordName,
				Type: models.RRSetType(recordType),
				// The API rejects a null `records` field (422
				// list_type validation error) even on remove ops,
				// because the SDK's RRSetPatch.Records is not
				// `omitempty`. Send an explicit empty list so the
				// JSON wire form is `"records": []`.
				Records: []models.RecordCreate{},
			},
		},
	})
	if err != nil {
		if !isNotFound(err) {
			resp.Diagnostics.AddError("Error deleting DNS record", formatAPIError(err))
		}
	}
}

func (r *RecordResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import format: zone_name/record_name/type
	parts := strings.SplitN(req.ID, "/", 3)
	if len(parts) != 3 {
		resp.Diagnostics.AddError(
			"Invalid Import ID",
			"Import ID must be in the format zone_name/record_name/type (e.g., example.com/www/A).",
		)
		return
	}

	zoneName, name, rtype := parts[0], parts[1], parts[2]

	zone, err := r.client.DNS.GetZone(ctx, zoneName)
	if err != nil {
		resp.Diagnostics.AddError("Error reading DNS zone during import", formatAPIError(err))
		return
	}

	rrset := findRRSet(zone.RRSets, name, models.RRSetType(rtype), zoneName)
	if rrset == nil {
		resp.Diagnostics.AddError("Record not found", fmt.Sprintf("No %s record named %q in zone %q.", rtype, name, zoneName))
		return
	}

	rdatas := normalizedRecordData(string(rrset.Type), rrset.Records)

	recordList, diags := types.ListValueFrom(ctx, types.StringType, rdatas)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data := RecordResourceModel{
		ID:       types.StringValue(req.ID),
		ZoneName: types.StringValue(zoneName),
		Name:     types.StringValue(name),
		Type:     types.StringValue(rtype),
		TTL:      types.Int64Value(int64(rrset.TTL)),
		Records:  recordList,
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// buildRRSet constructs an RRSetPatch from the resource model.
func buildRRSet(ctx context.Context, data RecordResourceModel, diagnostics *diag.Diagnostics) models.RRSetPatch {
	var rdatas []string
	diagnostics.Append(data.Records.ElementsAs(ctx, &rdatas, false)...)
	if diagnostics.HasError() {
		return models.RRSetPatch{}
	}

	records := make([]models.RecordCreate, len(rdatas))
	for i, rdata := range rdatas {
		records[i] = models.RecordCreate{RData: rdata}
	}

	return models.RRSetPatch{
		Name:    data.Name.ValueString(),
		Type:    models.RRSetType(data.Type.ValueString()),
		TTL:     int(data.TTL.ValueInt64()),
		Records: records,
	}
}

func normalizedRecordData(rrsetType string, records []models.RecordData) []string {
	rdatas := make([]string, len(records))
	for i, rec := range records {
		rdatas[i] = normalizeRData(rrsetType, rec.RData)
	}
	sort.Strings(rdatas)
	return rdatas
}

// findRRSet finds an RRSet by name and type within a slice.
//
// The OpusDNS API returns RRSet names in fully-qualified form
// (e.g. `example.com.` for the apex, `www.example.com.` for a subname), while
// users typically write `@` or relative names like `www` in their configuration.
// To make lookups work consistently, both sides are normalised to relative
// form via `relativeRRSetName` before comparison.
func findRRSet(rrsets []models.RRSet, name string, rtype models.RRSetType, zoneName string) *models.RRSet {
	target := relativeRRSetName(name, zoneName)
	for i := range rrsets {
		if relativeRRSetName(rrsets[i].Name, zoneName) == target && rrsets[i].Type == rtype {
			return &rrsets[i]
		}
	}
	return nil
}

// relativeRRSetName converts an RRSet name into its relative-to-zone form.
//
// Inputs accepted (with example zoneName `example.com`):
//
//	"@"                  -> "@"  (apex, already relative)
//	""                   -> "@"  (apex shorthand)
//	"example.com"        -> "@"
//	"example.com."       -> "@"
//	"www"                -> "www"
//	"www.example.com"    -> "www"
//	"www.example.com."   -> "www"
//
// zoneName itself may be supplied with or without a trailing dot; both forms
// are handled. Names that don't end in the zone are returned with any
// trailing dot stripped so two callers normalising independently agree.
func relativeRRSetName(name, zoneName string) string {
	name = strings.TrimSuffix(name, ".")
	zone := strings.TrimSuffix(zoneName, ".")
	if name == "" || name == "@" {
		return "@"
	}
	if name == zone {
		return "@"
	}
	if zone != "" && strings.HasSuffix(name, "."+zone) {
		return strings.TrimSuffix(name, "."+zone)
	}
	return name
}

// recordID returns the composite ID for a DNS record resource.
func recordID(zoneName, name, rtype string) string {
	return zoneName + "/" + name + "/" + rtype
}
