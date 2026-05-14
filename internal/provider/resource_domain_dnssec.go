package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure DomainDNSSECResource satisfies the resource interfaces.
var (
	_ resource.Resource                   = &DomainDNSSECResource{}
	_ resource.ResourceWithImportState    = &DomainDNSSECResource{}
	_ resource.ResourceWithValidateConfig = &DomainDNSSECResource{}
)

// DomainDNSSECResource manages the DNSSEC configuration for a single domain.
//
// The resource exposes two mutually exclusive workflows:
//
//  1. Registry-managed DNSSEC. Set `enabled = true` and leave `records` unset
//     or empty. The provider calls POST /v1/domains/{ref}/dnssec/enable, which
//     instructs OpusDNS to generate the keys for the matching zone (assumed to
//     be hosted at OpusDNS) and publish DS records to the registry. Destroy
//     calls POST /v1/domains/{ref}/dnssec/disable.
//
//  2. BYO records (manual DS or DNSKEY). Set `enabled = false` (the default)
//     and supply `records`. The provider calls PUT /v1/domains/{ref}/dnssec
//     with the supplied records, which replaces the registry-side DNSSEC data.
//     Destroy calls DELETE /v1/domains/{ref}/dnssec.
//
// The two modes cannot be combined in a single configuration; ValidateConfig
// rejects mixed input. Switching modes via Update is supported and translates
// into the appropriate sequence of API calls.
type DomainDNSSECResource struct {
	client *opusdns.Client
}

// DomainDNSSECResourceModel is the persisted state for the resource.
//
// `enabled` reflects the user's intent (registry-managed vs BYO) and is used
// by Delete to choose between the disable and delete endpoints. The API does
// not currently expose a direct way to introspect "this domain is in
// registry-managed mode", so this flag is config/state-driven.
type DomainDNSSECResourceModel struct {
	ID        types.String `tfsdk:"id"`
	DomainRef types.String `tfsdk:"domain_ref"`
	Enabled   types.Bool   `tfsdk:"enabled"`
	Records   types.List   `tfsdk:"records"`
}

// dnssecRecordObjectAttrTypes describes the schema of a single record entry as
// reflected through the framework's object type system. Shared by the resource
// and the data source so both produce identically typed values.
var dnssecRecordObjectAttrTypes = map[string]attr.Type{
	"id":          types.StringType,
	"record_type": types.StringType,
	"algorithm":   types.Int64Type,
	"digest":      types.StringType,
	"digest_type": types.Int64Type,
	"flags":       types.Int64Type,
	"key_tag":     types.Int64Type,
	"protocol":    types.Int64Type,
	"public_key":  types.StringType,
	"created_on":  types.StringType,
	"updated_on":  types.StringType,
}

// dnssecRecordObjectType is the framework object type for a single record.
var dnssecRecordObjectType = types.ObjectType{AttrTypes: dnssecRecordObjectAttrTypes}

// NewDomainDNSSECResource returns a new DomainDNSSECResource.
func NewDomainDNSSECResource() resource.Resource {
	return &DomainDNSSECResource{}
}

func (r *DomainDNSSECResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain_dnssec"
}

func (r *DomainDNSSECResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages DNSSEC configuration for an OpusDNS domain. " +
			"Supports two mutually exclusive modes: registry-managed (`enabled = true`, " +
			"OpusDNS generates and publishes the DS records for the zone hosted at OpusDNS) " +
			"and BYO records (`enabled = false`, supply `records` containing externally " +
			"managed DS or DNSKEY data).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Resource identifier (the domain reference used to manage DNSSEC).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"domain_ref": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Reference to the domain whose DNSSEC configuration is being managed. " +
					"Accepts either the domain id (e.g. `domain_...`) or the domain name (e.g. `example.com`). " +
					"Changing this value forces a new resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"enabled": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				MarkdownDescription: "When `true`, DNSSEC is enabled in registry-managed mode: OpusDNS generates the " +
					"keys for the matching OpusDNS-hosted zone and publishes the DS records to the registry. " +
					"`records` must be unset or empty in this mode. When `false` (the default), the resource " +
					"manages DNSSEC by replacing the registry-side records with the contents of `records`; if " +
					"`records` is also empty/unset the resource clears any existing DNSSEC data.",
			},
			"records": schema.ListNestedAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "Externally managed DNSSEC records (DS or DNSKEY data). When set with " +
					"`enabled = false`, these records replace the registry-side DNSSEC data on every apply. " +
					"When `enabled = true` this list is populated from the API after enabling.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Server-assigned identifier for the DNSSEC record.",
						},
						"record_type": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Record type: `ds_data` (DS record) or `key_data` (DNSKEY).",
						},
						"algorithm": schema.Int64Attribute{
							Required:            true,
							MarkdownDescription: "DNSSEC algorithm number (e.g. 13 for ECDSAP256SHA256).",
						},
						"digest": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "DS record digest (hex-encoded). Required for `ds_data`.",
						},
						"digest_type": schema.Int64Attribute{
							Optional:            true,
							MarkdownDescription: "DS digest type (e.g. 2 for SHA-256). Required for `ds_data`.",
						},
						"flags": schema.Int64Attribute{
							Optional:            true,
							MarkdownDescription: "DNSKEY flags (e.g. 257 for KSK). Required for `key_data`.",
						},
						"key_tag": schema.Int64Attribute{
							Optional:            true,
							MarkdownDescription: "Key tag identifying the corresponding DNSKEY. Required for `ds_data`.",
						},
						"protocol": schema.Int64Attribute{
							Optional:            true,
							MarkdownDescription: "DNSKEY protocol field (always 3 per RFC 4034). Required for `key_data`.",
						},
						"public_key": schema.StringAttribute{
							Optional:            true,
							MarkdownDescription: "Base64-encoded DNSKEY public key. Required for `key_data`.",
						},
						"created_on": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "RFC3339 timestamp when the record was created.",
						},
						"updated_on": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "RFC3339 timestamp when the record was last updated.",
						},
					},
				},
			},
		},
	}
}

func (r *DomainDNSSECResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*opusdns.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *opusdns.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = client
}

// ValidateConfig enforces the mutual exclusivity of `enabled = true` and
// non-empty `records`. Without this guard a user could set both, leading to a
// race between the enable and put endpoints during Update.
func (r *DomainDNSSECResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data DomainDNSSECResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	enabled := !data.Enabled.IsNull() && !data.Enabled.IsUnknown() && data.Enabled.ValueBool()
	hasRecords := !data.Records.IsNull() && !data.Records.IsUnknown() && len(data.Records.Elements()) > 0

	if enabled && hasRecords {
		resp.Diagnostics.AddAttributeError(
			path.Root("records"),
			"Conflicting DNSSEC configuration",
			"`records` must be empty or unset when `enabled = true`. Registry-managed DNSSEC and BYO records cannot be combined; pick one.",
		)
	}
}

func (r *DomainDNSSECResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DomainDNSSECResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainRef := plan.DomainRef.ValueString()
	enabled := !plan.Enabled.IsNull() && !plan.Enabled.IsUnknown() && plan.Enabled.ValueBool()

	apiRecords, diags := r.applyDesiredState(ctx, domainRef, enabled, plan.Records)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.ID = types.StringValue(domainRef)
	plan.Enabled = types.BoolValue(enabled)
	plan.Records, diags = dnssecRecordsToList(ctx, apiRecords)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DomainDNSSECResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DomainDNSSECResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainRef := state.DomainRef.ValueString()
	if domainRef == "" {
		resp.Diagnostics.AddError(
			"Invalid DNSSEC state",
			"The opusdns_domain_dnssec resource has an empty `domain_ref` in state. Remove it with `terraform state rm` and re-import.",
		)
		return
	}

	apiRecords, err := r.client.Domains.GetDNSSEC(ctx, domainRef)
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading DNSSEC data", formatAPIError(err))
		return
	}

	state.ID = types.StringValue(domainRef)
	// Preserve `enabled` from prior state. The API does not return a flag
	// distinguishing registry-managed vs BYO mode; intent is config-driven.
	if state.Enabled.IsNull() || state.Enabled.IsUnknown() {
		state.Enabled = types.BoolValue(false)
	}

	records, diags := dnssecRecordsToList(ctx, apiRecords)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state.Records = records

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DomainDNSSECResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state DomainDNSSECResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainRef := plan.DomainRef.ValueString()
	if domainRef == "" {
		resp.Diagnostics.AddError(
			"Invalid DNSSEC state",
			"The opusdns_domain_dnssec resource has an empty `domain_ref` in plan, which prevents updating it. "+
				"Remove the resource from state with `terraform state rm` and re-import.",
		)
		return
	}
	wasEnabled := !state.Enabled.IsNull() && !state.Enabled.IsUnknown() && state.Enabled.ValueBool()
	enabled := !plan.Enabled.IsNull() && !plan.Enabled.IsUnknown() && plan.Enabled.ValueBool()

	// If transitioning out of registry-managed mode, disable first so the
	// subsequent PUT/DELETE doesn't conflict with auto-published records.
	if wasEnabled && !enabled {
		if err := r.client.Domains.DisableDNSSEC(ctx, domainRef); err != nil && !isNotFound(err) {
			resp.Diagnostics.AddError("Error disabling registry-managed DNSSEC", formatAPIError(err))
			return
		}
	}

	apiRecords, diags := r.applyDesiredState(ctx, domainRef, enabled, plan.Records)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.ID = types.StringValue(domainRef)
	plan.Enabled = types.BoolValue(enabled)
	plan.Records, diags = dnssecRecordsToList(ctx, apiRecords)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DomainDNSSECResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DomainDNSSECResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domainRef := state.DomainRef.ValueString()
	if domainRef == "" {
		// Nothing actionable; let the framework drop the resource.
		return
	}

	enabled := !state.Enabled.IsNull() && !state.Enabled.IsUnknown() && state.Enabled.ValueBool()

	if enabled {
		if err := r.client.Domains.DisableDNSSEC(ctx, domainRef); err != nil && !isNotFound(err) {
			resp.Diagnostics.AddError("Error disabling registry-managed DNSSEC", formatAPIError(err))
			return
		}
		return
	}

	if err := r.client.Domains.DeleteDNSSEC(ctx, domainRef); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("Error deleting DNSSEC data", formatAPIError(err))
	}
}

// ImportState imports an existing DNSSEC configuration by domain reference.
//
// Because the API does not surface registry-managed vs BYO mode, the imported
// resource defaults to `enabled = false` (BYO). Operators using the
// registry-managed workflow should set `enabled = true` explicitly after
// import; the next plan will be a no-op against the API but will reconcile
// state with intent so destroy uses the correct disable endpoint.
func (r *DomainDNSSECResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			"The import ID must be a domain reference (id or name).",
		)
		return
	}

	apiRecords, err := r.client.Domains.GetDNSSEC(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing DNSSEC data", formatAPIError(err))
		return
	}

	records, diags := dnssecRecordsToList(ctx, apiRecords)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state := DomainDNSSECResourceModel{
		ID:        types.StringValue(req.ID),
		DomainRef: types.StringValue(req.ID),
		Enabled:   types.BoolValue(false),
		Records:   records,
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// applyDesiredState reconciles the desired (enabled, records) tuple against
// the API and returns the records the API reports afterwards.
//
// Cases:
//   - enabled=true             -> POST /dnssec/enable   (records arg ignored)
//   - enabled=false, records>0 -> PUT  /dnssec
//   - enabled=false, records=0 -> DELETE /dnssec        (returns empty list)
func (r *DomainDNSSECResource) applyDesiredState(
	ctx context.Context,
	domainRef string,
	enabled bool,
	records types.List,
) ([]models.DomainDNSSECDataResponse, diag.Diagnostics) {
	var diags diag.Diagnostics

	if enabled {
		tflog.Debug(ctx, "Enabling registry-managed DNSSEC", map[string]interface{}{"domain_ref": domainRef})
		out, err := r.client.Domains.EnableDNSSEC(ctx, domainRef)
		if err != nil {
			diags.AddError("Error enabling registry-managed DNSSEC", formatAPIError(err))
			return nil, diags
		}
		return out, diags
	}

	apiRecords, convertDiags := dnssecRecordsFromList(ctx, records)
	diags.Append(convertDiags...)
	if diags.HasError() {
		return nil, diags
	}

	if len(apiRecords) == 0 {
		tflog.Debug(ctx, "Clearing DNSSEC records", map[string]interface{}{"domain_ref": domainRef})
		if err := r.client.Domains.DeleteDNSSEC(ctx, domainRef); err != nil && !isNotFound(err) {
			diags.AddError("Error clearing DNSSEC data", formatAPIError(err))
			return nil, diags
		}
		return []models.DomainDNSSECDataResponse{}, diags
	}

	tflog.Debug(ctx, "Replacing DNSSEC records", map[string]interface{}{
		"domain_ref": domainRef,
		"count":      len(apiRecords),
	})
	out, err := r.client.Domains.PutDNSSEC(ctx, domainRef, apiRecords)
	if err != nil {
		diags.AddError("Error updating DNSSEC data", formatAPIError(err))
		return nil, diags
	}
	return out, diags
}
