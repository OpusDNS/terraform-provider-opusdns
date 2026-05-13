package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure ZoneResource satisfies the resource.Resource interface.
var _ resource.Resource = &ZoneResource{}
var _ resource.ResourceWithImportState = &ZoneResource{}

// ZoneResource defines the resource implementation.
type ZoneResource struct {
	client *opusdns.Client
}

// ZoneResourceModel describes the resource data model.
type ZoneResourceModel struct {
	ID           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	DNSSECStatus types.String `tfsdk:"dnssec_status"`
}

// NewZoneResource returns a new ZoneResource.
func NewZoneResource() resource.Resource {
	return &ZoneResource{}
}

func (r *ZoneResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zone"
}

func (r *ZoneResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a DNS zone in OpusDNS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The zone name (used as the unique identifier).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The domain name for the DNS zone (e.g., `example.com`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"dnssec_status": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The DNSSEC status of the zone. Valid values: `enabled`, `disabled`.",
			},
		},
	}
}

func (r *ZoneResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ZoneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ZoneResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &models.ZoneCreateRequest{
		Name: data.Name.ValueString(),
	}
	if !data.DNSSECStatus.IsNull() && !data.DNSSECStatus.IsUnknown() {
		createReq.DNSSECStatus = models.DNSSECStatus(data.DNSSECStatus.ValueString())
	}

	requestedName := data.Name.ValueString()
	zone, err := r.client.DNS.CreateZone(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating DNS zone", formatAPIError(err))
		return
	}

	// Defensive: if the API response omits the zone name, fall back to the
	// name we requested. Without this guard, an empty name would be persisted
	// to state and subsequent Read/Delete calls would issue malformed requests
	// (e.g. DELETE /v1/dns/ -> 405 Method Not Allowed).
	resolvedName := zone.Name
	if resolvedName == "" {
		resolvedName = requestedName
		tflog.Warn(ctx, "CreateZone response did not include a zone name; falling back to requested name", map[string]interface{}{
			"requested_name": requestedName,
		})
	}

	data.ID = types.StringValue(resolvedName)
	data.Name = types.StringValue(resolvedName)
	data.DNSSECStatus = types.StringValue(string(zone.DNSSECStatus))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ZoneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ZoneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	stateName := data.Name.ValueString()
	if stateName == "" {
		resp.Diagnostics.AddError(
			"Invalid DNS zone state",
			"The zone resource has an empty name in state, which prevents reading it from the API. "+
				"This typically indicates a prior create did not persist the zone name. "+
				"Remove the resource from state with `terraform state rm` and re-import or recreate it.",
		)
		return
	}

	zone, err := r.client.DNS.GetZone(ctx, stateName)
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading DNS zone", formatAPIError(err))
		return
	}

	resolvedName := zone.Name
	if resolvedName == "" {
		resolvedName = stateName
	}

	data.ID = types.StringValue(resolvedName)
	data.Name = types.StringValue(resolvedName)
	data.DNSSECStatus = types.StringValue(string(zone.DNSSECStatus))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ZoneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Zones have no updatable fields (name requires replace, dnssec_status is computed from the API).
	// If DNSSEC status is changed, we call the appropriate enable/disable endpoint.
	var state, plan ZoneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	stateName := state.Name.ValueString()
	if stateName == "" {
		resp.Diagnostics.AddError(
			"Invalid DNS zone state",
			"The zone resource has an empty name in state, which prevents updating it. "+
				"Remove the resource from state with `terraform state rm` and re-import or recreate it.",
		)
		return
	}

	if !plan.DNSSECStatus.IsNull() && !plan.DNSSECStatus.IsUnknown() &&
		plan.DNSSECStatus.ValueString() != state.DNSSECStatus.ValueString() {
		switch plan.DNSSECStatus.ValueString() {
		case string(models.DNSSECStatusEnabled):
			if _, err := r.client.DNS.EnableDNSSEC(ctx, stateName); err != nil {
				resp.Diagnostics.AddError("Error enabling DNSSEC", formatAPIError(err))
				return
			}
		case string(models.DNSSECStatusDisabled):
			if _, err := r.client.DNS.DisableDNSSEC(ctx, stateName); err != nil {
				resp.Diagnostics.AddError("Error disabling DNSSEC", formatAPIError(err))
				return
			}
		}
	}

	// Re-read current state.
	zone, err := r.client.DNS.GetZone(ctx, stateName)
	if err != nil {
		resp.Diagnostics.AddError("Error reading DNS zone after update", formatAPIError(err))
		return
	}

	resolvedName := zone.Name
	if resolvedName == "" {
		resolvedName = stateName
	}

	plan.ID = types.StringValue(resolvedName)
	plan.Name = types.StringValue(resolvedName)
	plan.DNSSECStatus = types.StringValue(string(zone.DNSSECStatus))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ZoneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ZoneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneName := data.Name.ValueString()
	if zoneName == "" {
		// Empty name in state means we cannot construct a valid DELETE URL
		// (e.g. DELETE /v1/dns/ would 405). Surface a clear, actionable error
		// instead of issuing a malformed request.
		resp.Diagnostics.AddError(
			"Invalid DNS zone state",
			"The zone resource has an empty name in state, which prevents deletion via the API. "+
				"This typically indicates a prior create did not persist the zone name. "+
				"Remove the resource from state with `terraform state rm <address>` and, if the "+
				"zone still exists at OpusDNS, delete it manually or re-import then destroy.",
		)
		return
	}

	tflog.Debug(ctx, "Deleting DNS zone", map[string]interface{}{
		"zone_name": zoneName,
	})

	if err := r.client.DNS.DeleteZone(ctx, zoneName); err != nil {
		if !isNotFound(err) {
			resp.Diagnostics.AddError(
				"Error deleting DNS zone",
				fmt.Sprintf("zone_name=%q: %s", zoneName, formatAPIError(err)),
			)
		}
	}
}

func (r *ZoneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID == "" {
		resp.Diagnostics.AddError(
			"Invalid import ID",
			"The import ID for an opusdns_zone resource must be the zone name (e.g., `example.com`).",
		)
		return
	}

	zone, err := r.client.DNS.GetZone(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing DNS zone", formatAPIError(err))
		return
	}

	resolvedName := zone.Name
	if resolvedName == "" {
		resolvedName = req.ID
	}

	data := ZoneResourceModel{
		ID:           types.StringValue(resolvedName),
		Name:         types.StringValue(resolvedName),
		DNSSECStatus: types.StringValue(string(zone.DNSSECStatus)),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
