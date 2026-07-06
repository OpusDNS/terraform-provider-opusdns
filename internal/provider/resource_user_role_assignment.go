package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure UserRoleAssignmentResource satisfies the resource interfaces.
var _ resource.Resource = &UserRoleAssignmentResource{}
var _ resource.ResourceWithImportState = &UserRoleAssignmentResource{}
var _ resource.ResourceWithUpgradeState = &UserRoleAssignmentResource{}

// UserRoleAssignmentResource manages a user's single role via
// GET/PUT /v1/users/{user_id}/role (SDK: Users.GetUserRole / Users.SetUserRole).
//
// The OpusDNS RBAC model assigns each user exactly one role: either a built-in
// role name or the label of a custom role owned by the user's organization.
// Setting the role replaces any existing role; clearing it (on destroy) sends a
// null role.
//
// NOTE: prior provider versions modelled this resource as a *set* of SpiceDB
// relations against the now-removed plural `/v1/users/{id}/roles` endpoint. The
// schema below is version 1; a state upgrader migrates version-0 state (which
// stored a `roles` set) to the new single `role` attribute by taking the first
// element of the old set, if any.
type UserRoleAssignmentResource struct {
	client *opusdns.Client
}

// UserRoleAssignmentResourceModel is the TF schema-backed state shape (v1).
type UserRoleAssignmentResourceModel struct {
	ID     types.String `tfsdk:"id"`
	UserID types.String `tfsdk:"user_id"`
	Role   types.String `tfsdk:"role"`
}

// NewUserRoleAssignmentResource returns a new UserRoleAssignmentResource.
func NewUserRoleAssignmentResource() resource.Resource {
	return &UserRoleAssignmentResource{}
}

func (r *UserRoleAssignmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_role_assignment"
}

func (r *UserRoleAssignmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 1,
		MarkdownDescription: "Manages the single role assigned to a user via `PUT /v1/users/{user_id}/role`. " +
			"A user has exactly one role: a built-in role name (e.g. `admin`, `member`) or the `label` of a " +
			"custom role owned by the user's organization (see the `opusdns_role` resource and `opusdns_roles` " +
			"data source). Setting `role` replaces any existing role; destroying the resource clears the user's " +
			"role. Changing `user_id` forces replacement.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Mirror of `user_id`. Useful as the resource address in state.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"user_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the user whose role is being managed (e.g. `user_...`). Changing this forces replacement.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The role to assign to the user: a built-in assignable role name or the " +
					"`label` of a custom role owned by the user's organization.",
			},
		},
	}
}

func (r *UserRoleAssignmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *UserRoleAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data UserRoleAssignmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userID := data.UserID.ValueString()
	role := data.Role.ValueString()

	assignment, err := r.client.Users.SetUserRole(ctx, models.UserID(userID), &role)
	if err != nil {
		resp.Diagnostics.AddError("Error setting user role", formatAPIError(err))
		return
	}

	data.ID = types.StringValue(userID)
	data.Role = roleAssignmentToValue(assignment)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserRoleAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data UserRoleAssignmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userID := data.UserID.ValueString()
	if userID == "" {
		resp.Diagnostics.AddError(
			"Invalid user_role_assignment state",
			"The resource has an empty `user_id` in state, which prevents reading the role from the API. "+
				"Remove the resource from state with `terraform state rm` and re-import or recreate it.",
		)
		return
	}

	assignment, err := r.client.Users.GetUserRole(ctx, models.UserID(userID))
	if err != nil {
		if isNotFound(err) {
			// User itself is gone; drop the role assignment from state.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading user role", formatAPIError(err))
		return
	}

	// If the API reports no role assigned, the resource no longer describes
	// anything meaningful; drop it from state so a subsequent apply recreates it.
	if assignment == nil || assignment.Role == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.ID = types.StringValue(userID)
	data.Role = roleAssignmentToValue(assignment)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserRoleAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan UserRoleAssignmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userID := plan.UserID.ValueString()
	role := plan.Role.ValueString()

	assignment, err := r.client.Users.SetUserRole(ctx, models.UserID(userID), &role)
	if err != nil {
		resp.Diagnostics.AddError("Error updating user role", formatAPIError(err))
		return
	}

	plan.ID = types.StringValue(userID)
	plan.Role = roleAssignmentToValue(assignment)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *UserRoleAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data UserRoleAssignmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userID := data.UserID.ValueString()
	if userID == "" {
		return
	}

	// Clear the user's role by sending a null role. Ignore not-found (the user
	// may already be gone).
	if _, err := r.client.Users.SetUserRole(ctx, models.UserID(userID), nil); err != nil {
		if isNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error clearing user role", formatAPIError(err))
	}
}

func (r *UserRoleAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by user_id: `terraform import opusdns_user_role_assignment.foo user_abc123`.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// UpgradeState migrates version-0 state (a `roles` set against the removed
// plural endpoint) to version-1 state (a single `role` string). The old set is
// collapsed to its first element, if any; an empty set becomes a null role,
// which the next Read/apply will reconcile.
func (r *UserRoleAssignmentResource) UpgradeState(ctx context.Context) map[int64]resource.StateUpgrader {
	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema: &schema.Schema{
				Attributes: map[string]schema.Attribute{
					"id":      schema.StringAttribute{Computed: true},
					"user_id": schema.StringAttribute{Required: true},
					"roles": schema.SetAttribute{
						Required:    true,
						ElementType: types.StringType,
					},
				},
			},
			StateUpgrader: func(ctx context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				type priorModel struct {
					ID     types.String `tfsdk:"id"`
					UserID types.String `tfsdk:"user_id"`
					Roles  types.Set    `tfsdk:"roles"`
				}
				var prior priorModel
				resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
				if resp.Diagnostics.HasError() {
					return
				}

				role := types.StringNull()
				if !prior.Roles.IsNull() && !prior.Roles.IsUnknown() {
					var roles []string
					resp.Diagnostics.Append(prior.Roles.ElementsAs(ctx, &roles, false)...)
					if resp.Diagnostics.HasError() {
						return
					}
					if len(roles) > 0 {
						role = types.StringValue(roles[0])
					}
				}

				upgraded := UserRoleAssignmentResourceModel{
					ID:     prior.ID,
					UserID: prior.UserID,
					Role:   role,
				}
				resp.Diagnostics.Append(resp.State.Set(ctx, &upgraded)...)
			},
		},
	}
}

// roleAssignmentToValue converts a *models.RoleAssignment into a types.String,
// mapping a nil assignment or nil role to types.StringNull().
func roleAssignmentToValue(a *models.RoleAssignment) types.String {
	if a == nil || a.Role == nil {
		return types.StringNull()
	}
	return types.StringValue(*a.Role)
}
