package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure UserRoleAssignmentDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &UserRoleAssignmentDataSource{}

// UserRoleAssignmentDataSource reads the set of roles assigned to a single
// user via `GET /v1/users/{user_id}/roles`. Mirrors the read side of
// UserRoleAssignmentResource, including the validRoles filter so that the
// `roles` attribute here is directly comparable to
// `opusdns_user_role_assignment.<name>.roles`.
type UserRoleAssignmentDataSource struct {
	client *opusdns.Client
}

// UserRoleAssignmentDataSourceModel is the TF schema-backed config/state shape.
type UserRoleAssignmentDataSourceModel struct {
	ID     types.String `tfsdk:"id"`
	UserID types.String `tfsdk:"user_id"`
	Roles  types.Set    `tfsdk:"roles"`
}

// NewUserRoleAssignmentDataSource returns a new UserRoleAssignmentDataSource.
func NewUserRoleAssignmentDataSource() datasource.DataSource {
	return &UserRoleAssignmentDataSource{}
}

func (d *UserRoleAssignmentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_role_assignment"
}

func (d *UserRoleAssignmentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches the set of roles (SpiceDB relations) currently assigned to a user via " +
			"`GET /v1/users/{user_id}/roles`. The returned `roles` set is filtered to the provider-managed " +
			"subset (see the `opusdns_user_role_assignment` resource for the allow-list); roles managed " +
			"implicitly by the API (e.g. `accepted_tos`, `owner`, `self`) are omitted so the value lines up " +
			"with what the matching resource would store in state.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Mirror of `user_id`.",
			},
			"user_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the user whose roles to fetch (e.g. `user_...`).",
			},
			"roles": schema.SetAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Roles assigned to the user, filtered to the provider-managed subset.",
			},
		},
	}
}

func (d *UserRoleAssignmentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*opusdns.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *opusdns.Client, got: %T.", req.ProviderData),
		)
		return
	}
	d.client = client
}

func (d *UserRoleAssignmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data UserRoleAssignmentDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	userID := data.UserID.ValueString()
	if userID == "" {
		resp.Diagnostics.AddError(
			"Missing user_id",
			"`user_id` must be a non-empty string.",
		)
		return
	}

	// fetchUserRoles already filters through validRoles; see
	// resource_user_role_assignment.go.
	roles, err := fetchUserRoles(ctx, d.client, userID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading user roles", formatAPIError(err))
		return
	}

	data.ID = types.StringValue(userID)
	data.Roles = stringSliceToSet(roles)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
