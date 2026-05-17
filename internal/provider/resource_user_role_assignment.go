package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure UserRoleAssignmentResource satisfies the resource.Resource interface.
var _ resource.Resource = &UserRoleAssignmentResource{}
var _ resource.ResourceWithImportState = &UserRoleAssignmentResource{}

// UserRoleAssignmentResource manages a user's role/relation set via
// PATCH /v1/users/{user_id}/roles. The endpoint is additive/subtractive
// (`{"add":[...], "remove":[...]}`), but this resource models the *desired
// total set* declaratively: on every Create/Update it diffs the current API
// state against the desired set and issues the appropriate add/remove batch.
type UserRoleAssignmentResource struct {
	client *opusdns.Client
}

// UserRoleAssignmentResourceModel is the TF schema-backed state shape.
type UserRoleAssignmentResourceModel struct {
	ID     types.String `tfsdk:"id"`
	UserID types.String `tfsdk:"user_id"`
	Roles  types.Set    `tfsdk:"roles"`
}

// validRoles is a curated subset of common/schemas/authorization/rbac.py:28-51
// (`class Relation(BaseRelation)`). Values are the raw `Relation` enum strings
// the API expects in the `add`/`remove` arrays. We intentionally omit roles
// that should not be user-assignable via Terraform (e.g. `accepted_tos`,
// `client_api_key`, `opusdns_internal_api_key`, `owner`, `parent`,
// `root_admin`, `self`) because they're managed implicitly by the API or
// granted out-of-band. The API may still return those roles from GET; we
// filter them out before writing to state so they don't trip the schema's
// `OneOf` validator on subsequent plans (see filterValidRoles).
var validRoles = []string{
	"admin",
	"api_admin",
	"billing_manager",
	"chat_manager",
	"cms_content_editor",
	"contact_manager",
	"domain_forward_manager",
	"domain_manager",
	"email_forward_manager",
	"events_manager",
	"host_manager",
	"member",
	"organization_manager",
	"product_manager",
	"registrar_credential_manager",
	"reseller_manager",
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
		MarkdownDescription: "Manages the set of roles (SpiceDB relations) assigned to a single user via " +
			"`PATCH /v1/users/{user_id}/roles`. The resource models the **desired total set** of roles " +
			"declaratively: each apply diffs the current API state against the configured set and issues the " +
			"minimum `add`/`remove` batch needed to converge. Destroying the resource removes the configured " +
			"roles from the user (but leaves the user itself intact). Valid role values come from the API's " +
			"`Relation` enum (see `common/schemas/authorization/rbac.py`).",
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
				MarkdownDescription: "ID of the user whose roles are being managed (e.g. `user_...`). Changing this forces replacement.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"roles": schema.SetAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Desired total set of roles the user should hold. Allowed values: " + formatRoleList() + ".",
				Validators: []validator.Set{
					setvalidator.ValueStringsAre(stringvalidator.OneOf(validRoles...)),
				},
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

	desired, diags := setToStringSlice(ctx, data.Roles)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	current, err := fetchUserRoles(ctx, r.client, userID)
	if err != nil {
		resp.Diagnostics.AddError("Error fetching user roles", formatAPIError(err))
		return
	}

	add, remove := diffRoleSets(current, desired)
	if len(add) > 0 || len(remove) > 0 {
		if _, err := patchUserRoles(ctx, r.client, userID, add, remove); err != nil {
			resp.Diagnostics.AddError("Error updating user roles", formatAPIError(err))
			return
		}
	}

	// Re-fetch to capture authoritative state (the API may add implicit
	// roles, e.g. `member`, that we should reflect rather than fight).
	final, err := fetchUserRoles(ctx, r.client, userID)
	if err != nil {
		resp.Diagnostics.AddError("Error fetching user roles after update", formatAPIError(err))
		return
	}

	data.ID = types.StringValue(userID)
	data.Roles = stringSliceToSet(final)
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
			"The resource has an empty `user_id` in state, which prevents reading roles from the API. "+
				"Remove the resource from state with `terraform state rm` and re-import or recreate it.",
		)
		return
	}

	current, err := fetchUserRoles(ctx, r.client, userID)
	if err != nil {
		if isNotFound(err) {
			// User itself is gone; drop the role assignment from state.
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading user roles", formatAPIError(err))
		return
	}

	data.ID = types.StringValue(userID)
	data.Roles = stringSliceToSet(current)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *UserRoleAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan UserRoleAssignmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userID := plan.UserID.ValueString()

	desired, diags := setToStringSlice(ctx, plan.Roles)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	current, err := fetchUserRoles(ctx, r.client, userID)
	if err != nil {
		resp.Diagnostics.AddError("Error fetching user roles", formatAPIError(err))
		return
	}

	add, remove := diffRoleSets(current, desired)
	if len(add) > 0 || len(remove) > 0 {
		if _, err := patchUserRoles(ctx, r.client, userID, add, remove); err != nil {
			resp.Diagnostics.AddError("Error updating user roles", formatAPIError(err))
			return
		}
	}

	final, err := fetchUserRoles(ctx, r.client, userID)
	if err != nil {
		resp.Diagnostics.AddError("Error fetching user roles after update", formatAPIError(err))
		return
	}

	plan.ID = types.StringValue(userID)
	plan.Roles = stringSliceToSet(final)
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

	configured, diags := setToStringSlice(ctx, data.Roles)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(configured) == 0 {
		return
	}

	// Remove exactly the roles this resource added (best-effort: a foreign
	// process may have already removed some, in which case the API will
	// likely succeed for a no-op or 404 — we ignore not-found errors).
	if _, err := patchUserRoles(ctx, r.client, userID, nil, configured); err != nil {
		if isNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error removing user roles", formatAPIError(err))
	}
}

func (r *UserRoleAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by user_id: `terraform import opusdns_user_role_assignment.foo user_abc123`.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// roleListResponse mirrors common/schemas/authorization/spicedb.py:22 (`RelationSet`).
type roleListResponse struct {
	Relations []string `json:"relations"`
}

// roleUpdateRequest mirrors common/schemas/authorization/spicedb.py:26
// (`SpiceDbRelationshipUpdate`). Both fields are nullable on the API side; we
// only marshal a non-nil pointer when there's at least one entry, so empty
// batches don't waste a round trip (callers should guard against that).
type roleUpdateRequest struct {
	Add    []string `json:"add,omitempty"`
	Remove []string `json:"remove,omitempty"`
}

// fetchUserRoles wraps `GET /v1/users/{user_id}/roles`. The SDK as of v1.0.9
// has no helper for this endpoint.
func fetchUserRoles(ctx context.Context, c *opusdns.Client, userID string) ([]string, error) {
	p := c.HTTPClient().BuildPath("users", userID, "roles")
	resp, err := c.HTTPClient().Get(ctx, p, nil)
	if err != nil {
		return nil, err
	}
	var out roleListResponse
	if err := c.HTTPClient().DecodeResponse(resp, &out); err != nil {
		return nil, err
	}
	// Scrub roles the provider doesn't manage so they don't leak into diff
	// computations or state. See the validRoles / filterValidRoles comments.
	return filterValidRoles(out.Relations), nil
}

// patchUserRoles wraps `PATCH /v1/users/{user_id}/roles`. Returns the updated
// RelationSet for callers that want it; we currently re-fetch via GET instead
// because the API may include implicit relations the PATCH response omits.
func patchUserRoles(ctx context.Context, c *opusdns.Client, userID string, add, remove []string) ([]string, error) {
	p := c.HTTPClient().BuildPath("users", userID, "roles")
	body := roleUpdateRequest{Add: add, Remove: remove}
	resp, err := c.HTTPClient().Patch(ctx, p, body)
	if err != nil {
		return nil, err
	}
	var out roleListResponse
	if err := c.HTTPClient().DecodeResponse(resp, &out); err != nil {
		return nil, err
	}
	return filterValidRoles(out.Relations), nil
}

// diffRoleSets returns the slices of roles to add and to remove so that the
// current set converges on the desired set. Both inputs are treated as sets;
// duplicates are de-duped. Output slices are sorted for stable PATCH bodies.
func diffRoleSets(current, desired []string) (add, remove []string) {
	cur := toStringSet(current)
	des := toStringSet(desired)

	for v := range des {
		if _, ok := cur[v]; !ok {
			add = append(add, v)
		}
	}
	for v := range cur {
		if _, ok := des[v]; !ok {
			remove = append(remove, v)
		}
	}
	sort.Strings(add)
	sort.Strings(remove)
	return add, remove
}

func toStringSet(in []string) map[string]struct{} {
	out := make(map[string]struct{}, len(in))
	for _, v := range in {
		out[v] = struct{}{}
	}
	return out
}

// filterValidRoles returns the subset of `in` whose values appear in
// validRoles, preserving order. Used to scrub API responses of roles the
// provider intentionally does not manage (see the validRoles comment) so we
// don't (a) try to PATCH `remove` for them on diff or delete, or (b) write
// them into state where they'd fail the schema's `OneOf` validator.
func filterValidRoles(in []string) []string {
	allowed := toStringSet(validRoles)
	out := make([]string, 0, len(in))
	for _, v := range in {
		if _, ok := allowed[v]; ok {
			out = append(out, v)
		}
	}
	return out
}

// setToStringSlice extracts the element strings from a Terraform Set value.
func setToStringSlice(ctx context.Context, s types.Set) ([]string, diag.Diagnostics) {
	if s.IsNull() || s.IsUnknown() {
		return nil, nil
	}
	var out []string
	diags := s.ElementsAs(ctx, &out, false)
	return out, diags
}

// stringSliceToSet builds a Terraform Set[string] value from a Go slice.
// Falls back to a null set on diag failure rather than poisoning state.
func stringSliceToSet(in []string) types.Set {
	vals := make([]string, len(in))
	copy(vals, in)
	sort.Strings(vals)
	elems := make([]attr.Value, len(vals))
	for i, v := range vals {
		elems[i] = types.StringValue(v)
	}
	set, diags := types.SetValue(types.StringType, elems)
	if diags.HasError() {
		return types.SetNull(types.StringType)
	}
	return set
}

// formatRoleList returns the validRoles slice as a backticked, comma-separated
// markdown fragment for use in MarkdownDescription.
func formatRoleList() string {
	var b []byte
	for i, r := range validRoles {
		if i > 0 {
			b = append(b, ", "...)
		}
		b = append(b, '`')
		b = append(b, r...)
		b = append(b, '`')
	}
	return string(b)
}
