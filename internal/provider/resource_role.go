package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure RoleResource satisfies the resource.Resource interface.
var (
	_ resource.Resource                = &RoleResource{}
	_ resource.ResourceWithImportState = &RoleResource{}
)

// RoleResource implements `opusdns_role` for organization-owned custom roles,
// backed by `/v1/organizations/roles`. Built-in roles are immutable and are
// not managed by this resource; use the `opusdns_roles` data source to read
// them.
type RoleResource struct {
	client *opusdns.Client
}

// RoleResourceModel is the state shape for a custom role resource.
type RoleResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Label       types.String `tfsdk:"label"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Permissions types.Set    `tfsdk:"permissions"`
	BuiltIn     types.Bool   `tfsdk:"built_in"`
	CreatedOn   types.String `tfsdk:"created_on"`
	UpdatedOn   types.String `tfsdk:"updated_on"`
}

// NewRoleResource returns a new RoleResource.
func NewRoleResource() resource.Resource {
	return &RoleResource{}
}

func (r *RoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (r *RoleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a custom role in OpusDNS (`/v1/organizations/roles`). Custom roles bundle a set of " +
			"`resource:scope` permissions that can then be assigned to users via `opusdns_user_role_assignment`. " +
			"Built-in roles are immutable and cannot be managed with this resource; read them with the `opusdns_roles` data source. " +
			"The escalation-bearing admin/owner permissions cannot be granted to a custom role.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Mirror of `label` (the role's identifier).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"label": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The URL-safe, per-organization unique role identifier (snake_case, e.g. `support_staff`), derived by the server from `name`. Used when assigning the role.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Human-readable display name for the role (e.g. `Support Staff`).",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional free-text description of the role.",
			},
			"permissions": schema.SetAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "The set of `resource:scope` permission strings the role grants (e.g. `domains:read`, `dns:manage`). Order is not significant; updates replace the entire set.",
			},
			"built_in": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether this is a built-in role. Always `false` for roles managed by this resource.",
			},
			"created_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the role was created.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"updated_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the role was last updated.",
			},
		},
	}
}

func (r *RoleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *RoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	permissions, diags := setToStringSlice(ctx, data.Permissions)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &models.CustomRoleCreateRequest{
		Name:        data.Name.ValueString(),
		Description: optionalStringPtr(data.Description),
		Permissions: permissions,
	}

	role, err := r.client.Organizations.CreateRole(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating role", formatAPIError(err))
		return
	}

	resp.Diagnostics.Append(populateRoleModel(ctx, &data, role)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	label := data.Label.ValueString()
	if label == "" {
		resp.Diagnostics.AddError(
			"Invalid role state",
			"The opusdns_role resource has an empty `label` in state, which prevents reading it from the API. "+
				"Remove the resource from state with `terraform state rm` and re-import or recreate it.",
		)
		return
	}

	role, err := r.client.Organizations.GetRole(ctx, label)
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading role", formatAPIError(err))
		return
	}

	resp.Diagnostics.Append(populateRoleModel(ctx, &data, role)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state RoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	label := state.Label.ValueString()
	if label == "" {
		resp.Diagnostics.AddError(
			"Invalid role state",
			"The opusdns_role resource has an empty `label` in state, which prevents updating it. "+
				"Remove the resource from state with `terraform state rm` and re-import or recreate it.",
		)
		return
	}

	updateReq := &models.CustomRoleUpdateRequest{}
	hasChange := false

	if !plan.Name.Equal(state.Name) {
		name := plan.Name.ValueString()
		updateReq.Name = &name
		hasChange = true
	}
	if !plan.Description.Equal(state.Description) {
		updateReq.Description = optionalStringPtr(plan.Description)
		hasChange = true
	}
	if !plan.Permissions.Equal(state.Permissions) {
		permissions, diags := setToStringSlice(ctx, plan.Permissions)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		updateReq.Permissions = &permissions
		hasChange = true
	}

	var role *models.RoleDefinition
	if hasChange {
		updated, err := r.client.Organizations.UpdateRole(ctx, label, updateReq)
		if err != nil {
			resp.Diagnostics.AddError("Error updating role", formatAPIError(err))
			return
		}
		role = updated
	} else {
		current, err := r.client.Organizations.GetRole(ctx, label)
		if err != nil {
			resp.Diagnostics.AddError("Error reading role", formatAPIError(err))
			return
		}
		role = current
	}

	resp.Diagnostics.Append(populateRoleModel(ctx, &plan, role)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *RoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	label := data.Label.ValueString()
	if label == "" {
		resp.Diagnostics.AddError(
			"Invalid role state",
			"The opusdns_role resource has an empty `label` in state, which prevents deletion via the API. "+
				"Remove the resource from state with `terraform state rm` and, if the role still exists at OpusDNS, delete it manually or re-import then destroy.",
		)
		return
	}

	if err := r.client.Organizations.DeleteRole(ctx, label); err != nil {
		if !isNotFound(err) {
			resp.Diagnostics.AddError("Error deleting role", formatAPIError(err))
		}
	}
}

func (r *RoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by label; full state is hydrated by the subsequent Read.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("label"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// populateRoleModel copies API response fields onto a RoleResourceModel.
func populateRoleModel(ctx context.Context, data *RoleResourceModel, role *models.RoleDefinition) diag.Diagnostics {
	var diags diag.Diagnostics

	data.ID = types.StringValue(role.Label)
	data.Label = types.StringValue(role.Label)
	data.Name = types.StringValue(role.Name)
	data.Description = stringPtrToValue(role.Description)
	data.BuiltIn = types.BoolValue(role.BuiltIn)
	data.CreatedOn = timePtrToValue(role.CreatedOn)
	data.UpdatedOn = timePtrToValue(role.UpdatedOn)

	permissions, pd := types.SetValueFrom(ctx, types.StringType, role.Permissions)
	diags.Append(pd...)
	data.Permissions = permissions

	return diags
}
