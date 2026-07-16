package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure RolePermissionsDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &RolePermissionsDataSource{}

// RolePermissionsDataSource exposes the catalog of `resource:scope` permission
// strings a custom role may grant, via `GET /v1/organizations/role-permissions`
// (SDK: Organizations.ListRolePermissions). Use it to discover valid values for
// the `permissions` attribute of `opusdns_role`.
type RolePermissionsDataSource struct {
	client *opusdns.Client
}

// RolePermissionsDataSourceModel is the state shape for the data source.
type RolePermissionsDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Permissions types.List   `tfsdk:"permissions"`
}

// NewRolePermissionsDataSource returns a new RolePermissionsDataSource.
func NewRolePermissionsDataSource() datasource.DataSource {
	return &RolePermissionsDataSource{}
}

func (d *RolePermissionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role_permissions"
}

func (d *RolePermissionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists the catalog of `resource:scope` permission strings a custom role may grant " +
			"(`GET /v1/organizations/role-permissions`). Use these values for the `permissions` attribute of " +
			"the `opusdns_role` resource. Note that the escalation-bearing admin/owner permissions are not grantable " +
			"to custom roles and are therefore excluded from this catalog.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, MarkdownDescription: "Static identifier for this data source."},
			"permissions": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "The grantable `resource:scope` permission strings (e.g. `domains:read`, `dns:manage`).",
			},
		},
	}
}

func (d *RolePermissionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *RolePermissionsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	catalog, err := d.client.Organizations.ListRolePermissions(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing role permissions", formatAPIError(err))
		return
	}

	permissions, diags := types.ListValueFrom(ctx, types.StringType, catalog.Permissions)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data := RolePermissionsDataSourceModel{
		ID:          types.StringValue("role_permissions"),
		Permissions: permissions,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
