package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

var _ resource.Resource = &OrganizationIPRestrictionResource{}
var _ resource.ResourceWithImportState = &OrganizationIPRestrictionResource{}

type OrganizationIPRestrictionResource struct {
	client *opusdns.Client
}

type OrganizationIPRestrictionResourceModel struct {
	ID              types.String `tfsdk:"id"`
	IPRestrictionID types.String `tfsdk:"ip_restriction_id"`
	OrganizationID  types.String `tfsdk:"organization_id"`
	IPNetwork       types.String `tfsdk:"ip_network"`
	LastUsedOn      types.String `tfsdk:"last_used_on"`
	CreatedOn       types.String `tfsdk:"created_on"`
}

func NewOrganizationIPRestrictionResource() resource.Resource {
	return &OrganizationIPRestrictionResource{}
}

func (r *OrganizationIPRestrictionResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_ip_restriction"
}

func (r *OrganizationIPRestrictionResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an organization IP restriction for OpusDNS API access (`/v1/organizations/ip-restrictions`).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Mirror of `ip_restriction_id`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"ip_restriction_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Server-assigned IP restriction identifier.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"organization_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Organization that owns the IP restriction.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"ip_network": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "IP address or CIDR network range allowed by this restriction.",
			},
			"last_used_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the restriction was last used, if reported by the API.",
			},
			"created_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the restriction was created.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *OrganizationIPRestrictionResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*opusdns.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *opusdns.Client, got: %T.", req.ProviderData))
		return
	}
	r.client = client
}

func (r *OrganizationIPRestrictionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OrganizationIPRestrictionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	restriction, err := r.client.Organizations.CreateIPRestriction(ctx, &models.IPRestrictionCreateRequest{IPNetwork: data.IPNetwork.ValueString()})
	if err != nil {
		resp.Diagnostics.AddError("Error creating organization IP restriction", formatAPIError(err))
		return
	}
	populateOrganizationIPRestrictionModel(&data, restriction)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationIPRestrictionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OrganizationIPRestrictionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	restriction, err := r.client.Organizations.GetIPRestriction(ctx, models.TypeID(data.IPRestrictionID.ValueString()))
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading organization IP restriction", formatAPIError(err))
		return
	}
	populateOrganizationIPRestrictionModel(&data, restriction)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationIPRestrictionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan OrganizationIPRestrictionResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ipNetwork := plan.IPNetwork.ValueString()
	restriction, err := r.client.Organizations.UpdateIPRestriction(ctx, models.TypeID(plan.IPRestrictionID.ValueString()), &models.IPRestrictionUpdateRequest{IPNetwork: &ipNetwork})
	if err != nil {
		resp.Diagnostics.AddError("Error updating organization IP restriction", formatAPIError(err))
		return
	}
	populateOrganizationIPRestrictionModel(&plan, restriction)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *OrganizationIPRestrictionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data OrganizationIPRestrictionResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Organizations.DeleteIPRestriction(ctx, models.TypeID(data.IPRestrictionID.ValueString())); err != nil && !isNotFound(err) {
		resp.Diagnostics.AddError("Error deleting organization IP restriction", formatAPIError(err))
	}
}

func (r *OrganizationIPRestrictionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("ip_restriction_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

func populateOrganizationIPRestrictionModel(data *OrganizationIPRestrictionResourceModel, restriction *models.IPRestriction) {
	id := fmt.Sprintf("%d", restriction.IPRestrictionID)
	data.ID = types.StringValue(id)
	data.IPRestrictionID = types.StringValue(id)
	data.OrganizationID = types.StringValue(string(restriction.OrganizationID))
	data.IPNetwork = types.StringValue(restriction.IPNetwork)
	data.LastUsedOn = timePtrToValue(restriction.LastUsedOn)
	data.CreatedOn = types.StringValue(restriction.CreatedOn.Format(time.RFC3339))
}
