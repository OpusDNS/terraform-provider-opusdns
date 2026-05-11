package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
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

	zone, err := r.client.DNS.CreateZone(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating DNS zone", err.Error())
		return
	}

	data.ID = types.StringValue(zone.Name)
	data.Name = types.StringValue(zone.Name)
	data.DNSSECStatus = types.StringValue(string(zone.DNSSECStatus))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ZoneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ZoneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zone, err := r.client.DNS.GetZone(ctx, data.Name.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading DNS zone", err.Error())
		return
	}

	data.ID = types.StringValue(zone.Name)
	data.Name = types.StringValue(zone.Name)
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

	if !plan.DNSSECStatus.IsNull() && !plan.DNSSECStatus.IsUnknown() &&
		plan.DNSSECStatus.ValueString() != state.DNSSECStatus.ValueString() {
		switch plan.DNSSECStatus.ValueString() {
		case string(models.DNSSECStatusEnabled):
			if _, err := r.client.DNS.EnableDNSSEC(ctx, state.Name.ValueString()); err != nil {
				resp.Diagnostics.AddError("Error enabling DNSSEC", err.Error())
				return
			}
		case string(models.DNSSECStatusDisabled):
			if _, err := r.client.DNS.DisableDNSSEC(ctx, state.Name.ValueString()); err != nil {
				resp.Diagnostics.AddError("Error disabling DNSSEC", err.Error())
				return
			}
		}
	}

	// Re-read current state.
	zone, err := r.client.DNS.GetZone(ctx, state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading DNS zone after update", err.Error())
		return
	}

	plan.ID = types.StringValue(zone.Name)
	plan.Name = types.StringValue(zone.Name)
	plan.DNSSECStatus = types.StringValue(string(zone.DNSSECStatus))

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ZoneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ZoneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DNS.DeleteZone(ctx, data.Name.ValueString()); err != nil {
		if !isNotFound(err) {
			resp.Diagnostics.AddError("Error deleting DNS zone", err.Error())
		}
	}
}

func (r *ZoneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	zone, err := r.client.DNS.GetZone(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing DNS zone", err.Error())
		return
	}

	data := ZoneResourceModel{
		ID:           types.StringValue(zone.Name),
		Name:         types.StringValue(zone.Name),
		DNSSECStatus: types.StringValue(string(zone.DNSSECStatus)),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
