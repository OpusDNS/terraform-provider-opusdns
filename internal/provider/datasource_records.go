package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure DNSRecordsDataSource satisfies the data source interface.
var _ datasource.DataSource = &DNSRecordsDataSource{}

// DNSRecordsDataSource lists all RRSets in a zone, optionally filtered by
// record name and/or one or more types. The API has no filtering for this
// endpoint; filtering is done client-side after a single DNS.GetZone call.
type DNSRecordsDataSource struct {
	client *opusdns.Client
}

// DNSRecordsDataSourceModel describes the data source model. `records` is a
// list of RRSet-shaped objects.
type DNSRecordsDataSourceModel struct {
	ID         types.String `tfsdk:"id"`
	ZoneName   types.String `tfsdk:"zone_name"`
	NameFilter types.String `tfsdk:"name"`
	TypeFilter types.String `tfsdk:"type"`
	TypesIn    types.List   `tfsdk:"types_in"`
	Records    types.List   `tfsdk:"records"`
}

// recordsListObjectAttrTypes is the attribute schema of each entry in the
// `records` list.
var recordsListObjectAttrTypes = map[string]attr.Type{
	"name":             types.StringType,
	"type":             types.StringType,
	"ttl":              types.Int64Type,
	"records":          types.ListType{ElemType: types.StringType},
	"protected":        types.BoolType,
	"protected_reason": types.StringType,
}

var recordsListObjectType = types.ObjectType{AttrTypes: recordsListObjectAttrTypes}

// NewDNSRecordsDataSource returns a new DNSRecordsDataSource.
func NewDNSRecordsDataSource() datasource.DataSource {
	return &DNSRecordsDataSource{}
}

func (d *DNSRecordsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_records"
}

func (d *DNSRecordsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists DNS record sets (RRSets) for a zone. The OpusDNS API does not support server-side filtering of RRSets, so filtering by `name`, `type`, or `types_in` is performed client-side after retrieving the full zone.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Synthetic identifier (the zone name).",
			},
			"zone_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the zone whose RRSets to list (e.g. `example.com`).",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional exact-match filter on record name (relative to the zone).",
			},
			"type": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional exact-match filter on record type (e.g. `A`, `MX`). Mutually exclusive with `types_in`.",
			},
			"types_in": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Optional set of record types to include. Mutually exclusive with `type`. When set, only RRSets whose type is in the list are returned.",
			},
			"records": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Matching RRSets.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name":             schema.StringAttribute{Computed: true, MarkdownDescription: "Record name relative to the zone."},
						"type":             schema.StringAttribute{Computed: true, MarkdownDescription: "DNS record type."},
						"ttl":              schema.Int64Attribute{Computed: true, MarkdownDescription: "Time-to-live in seconds."},
						"records":          schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Record data values."},
						"protected":        schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the RRSet is protected from modification."},
						"protected_reason": schema.StringAttribute{Computed: true, MarkdownDescription: "Reason the RRSet is protected, when applicable."},
					},
				},
			},
		},
	}
}

func (d *DNSRecordsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*opusdns.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *opusdns.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	d.client = client
}

func (d *DNSRecordsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DNSRecordsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hasType := !data.TypeFilter.IsNull() && !data.TypeFilter.IsUnknown() && data.TypeFilter.ValueString() != ""
	hasTypesIn := !data.TypesIn.IsNull() && !data.TypesIn.IsUnknown() && len(data.TypesIn.Elements()) > 0
	if hasType && hasTypesIn {
		resp.Diagnostics.AddError(
			"Conflicting filters",
			"`type` and `types_in` are mutually exclusive; set at most one.",
		)
		return
	}

	zoneName := data.ZoneName.ValueString()
	zone, err := d.client.DNS.GetZone(ctx, zoneName)
	if err != nil {
		resp.Diagnostics.AddError("Error reading DNS zone", formatAPIError(err))
		return
	}

	// Normalise the user-supplied name filter to relative form so apex
	// shorthand like `@` and `""` match the API's fully-qualified
	// `<zone>.` representation.
	nameFilter := ""
	if !data.NameFilter.IsNull() && !data.NameFilter.IsUnknown() {
		nameFilter = relativeRRSetName(data.NameFilter.ValueString(), zoneName)
	}
	typeFilter := ""
	if hasType {
		typeFilter = data.TypeFilter.ValueString()
	}
	var typesIn map[string]struct{}
	if hasTypesIn {
		var raw []string
		resp.Diagnostics.Append(data.TypesIn.ElementsAs(ctx, &raw, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		typesIn = make(map[string]struct{}, len(raw))
		for _, t := range raw {
			typesIn[t] = struct{}{}
		}
	}

	values := make([]attr.Value, 0, len(zone.RRSets))
	for _, rr := range zone.RRSets {
		relName := relativeRRSetName(rr.Name, zoneName)
		if nameFilter != "" && relName != nameFilter {
			continue
		}
		if typeFilter != "" && string(rr.Type) != typeFilter {
			continue
		}
		if typesIn != nil {
			if _, ok := typesIn[string(rr.Type)]; !ok {
				continue
			}
		}

		obj, diags := rrsetToObjectValue(rr, zoneName)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		values = append(values, obj)
	}

	recordList, diags := types.ListValue(recordsListObjectType, values)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue(zoneName)
	data.Records = recordList

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// rrsetToObjectValue converts a single SDK RRSet into the framework object
// value used by the `records` list attribute.
// rrsetToObjectValue converts a single SDK RRSet into the framework object
// value used by the `records` list attribute. The RRSet's `name` is reported
// in its relative-to-zone form (e.g. `@` for apex, `www` for `www.<zone>.`)
// to match the user-facing convention used elsewhere in the provider.
func rrsetToObjectValue(rr models.RRSet, zoneName string) (attr.Value, diag.Diagnostics) {
	rdatas := make([]attr.Value, len(rr.Records))
	for i, r := range rr.Records {
		rdatas[i] = types.StringValue(normalizeRData(string(rr.Type), r.RData))
	}
	rdataList, diags := types.ListValue(types.StringType, rdatas)
	if diags.HasError() {
		return types.ObjectNull(recordsListObjectAttrTypes), diags
	}

	obj, oDiags := types.ObjectValue(recordsListObjectAttrTypes, map[string]attr.Value{
		"name":             types.StringValue(relativeRRSetName(rr.Name, zoneName)),
		"type":             types.StringValue(string(rr.Type)),
		"ttl":              types.Int64Value(int64(rr.TTL)),
		"records":          rdataList,
		"protected":        types.BoolValue(rr.Protected),
		"protected_reason": stringPtrToValue(rr.ProtectedReason),
	})
	diags.Append(oDiags...)
	return obj, diags
}
