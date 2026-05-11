package provider

import (
	"context"
	"fmt"

	opusdns "github.com/opusdns/opusdns-go-client/opusdns"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &ZoneResource{}

type ZoneResource struct {
	client *opusdns.Client
}

type ZoneResourceModel struct {
	ID            types.String `tfsdk:"id"`
	ZoneID        types.String `tfsdk:"zone_id"`
	Name          types.String `tfsdk:"name"`
	DNSSECEnabled types.Bool   `tfsdk:"dnssec_enabled"`
	DNSSECStatus  types.String `tfsdk:"dnssec_status"`
	CreatedOn     types.String `tfsdk:"created_on"`
	UpdatedOn     types.String `tfsdk:"updated_on"`
}

func NewZoneResource() resource.Resource {
	return &ZoneResource{}
}

func (r *ZoneResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zone"
}

func (r *ZoneResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a DNS zone in OpusDNS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"zone_id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The zone name (e.g., 'example.com').",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"dnssec_enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether DNSSEC is enabled for the zone.",
			},
			"dnssec_status": schema.StringAttribute{
				Computed:    true,
				Description: "The DNSSEC status of the zone.",
			},
			"created_on": schema.StringAttribute{
				Computed: true,
			},
			"updated_on": schema.StringAttribute{
				Computed: true,
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
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *opusdns.Client, got %T", req.ProviderData))
		return
	}
	r.client = client
}

func (r *ZoneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ZoneResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &models.ZoneCreateRequest{
		Name: plan.Name.ValueString(),
	}

	zone, err := r.client.DNS.CreateZone(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create zone", err.Error())
		return
	}

	if plan.DNSSECEnabled.ValueBool() {
		if _, err := r.client.DNS.EnableDNSSEC(ctx, zone.Name); err != nil {
			resp.Diagnostics.AddError("Failed to enable DNSSEC", err.Error())
			return
		}
		zone, err = r.client.DNS.GetZone(ctx, zone.Name)
		if err != nil {
			resp.Diagnostics.AddError("Failed to read zone after enabling DNSSEC", err.Error())
			return
		}
	}

	flattenZone(zone, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ZoneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ZoneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zone, err := r.client.DNS.GetZone(ctx, state.Name.ValueString())
	if err != nil {
		if opusdns.IsNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read zone", err.Error())
		return
	}

	flattenZone(zone, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *ZoneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state ZoneResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneName := state.Name.ValueString()

	if !plan.DNSSECEnabled.Equal(state.DNSSECEnabled) {
		if plan.DNSSECEnabled.ValueBool() {
			if _, err := r.client.DNS.EnableDNSSEC(ctx, zoneName); err != nil {
				resp.Diagnostics.AddError("Failed to enable DNSSEC", err.Error())
				return
			}
		} else {
			if _, err := r.client.DNS.DisableDNSSEC(ctx, zoneName); err != nil {
				resp.Diagnostics.AddError("Failed to disable DNSSEC", err.Error())
				return
			}
		}
	}

	zone, err := r.client.DNS.GetZone(ctx, zoneName)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read zone after update", err.Error())
		return
	}

	flattenZone(zone, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ZoneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ZoneResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.DNS.DeleteZone(ctx, state.Name.ValueString()); err != nil {
		if !opusdns.IsNotFoundError(err) {
			resp.Diagnostics.AddError("Failed to delete zone", err.Error())
		}
	}
}

func flattenZone(zone *models.Zone, model *ZoneResourceModel) {
	model.ID = types.StringValue(zone.Name)
	model.ZoneID = types.StringValue(string(zone.ZoneID))
	model.Name = types.StringValue(zone.Name)
	model.DNSSECStatus = types.StringValue(string(zone.DNSSECStatus))
	model.DNSSECEnabled = types.BoolValue(zone.DNSSECStatus == models.DNSSECStatusEnabled)

	if zone.CreatedOn != nil {
		model.CreatedOn = types.StringValue(zone.CreatedOn.String())
	} else {
		model.CreatedOn = types.StringValue("")
	}

	if zone.UpdatedOn != nil {
		model.UpdatedOn = types.StringValue(zone.UpdatedOn.String())
	} else {
		model.UpdatedOn = types.StringValue("")
	}
}
