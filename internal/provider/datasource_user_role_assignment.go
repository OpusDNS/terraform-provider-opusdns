package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure UserRoleAssignmentDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &UserRoleAssignmentDataSource{}

// UserRoleAssignmentDataSource reads the single role assigned to a user via
// `GET /v1/users/{user_id}/role` (SDK: Users.GetUserRole). Mirrors the read
// side of UserRoleAssignmentResource so the `role` attribute here is directly
// comparable to `opusdns_user_role_assignment.<name>.role`.
type UserRoleAssignmentDataSource struct {
	client *opusdns.Client
}

// UserRoleAssignmentDataSourceModel is the TF schema-backed config/state shape.
type UserRoleAssignmentDataSourceModel struct {
	ID     types.String `tfsdk:"id"`
	UserID types.String `tfsdk:"user_id"`
	Role   types.String `tfsdk:"role"`
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
		MarkdownDescription: "Fetches the single role currently assigned to a user via " +
			"`GET /v1/users/{user_id}/role`. The `role` is a built-in role name or the `label` of a custom " +
			"role, or empty when the user has no role assigned.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Mirror of `user_id`.",
			},
			"user_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "ID of the user whose role to fetch (e.g. `user_...`).",
			},
			"role": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The role assigned to the user, or empty when no role is assigned.",
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

	assignment, err := d.client.Users.GetUserRole(ctx, models.UserID(userID))
	if err != nil {
		resp.Diagnostics.AddError("Error reading user role", formatAPIError(err))
		return
	}

	data.ID = types.StringValue(userID)
	data.Role = roleAssignmentToValue(assignment)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
