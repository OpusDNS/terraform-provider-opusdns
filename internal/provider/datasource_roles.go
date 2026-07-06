package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

var _ datasource.DataSource = &RolesDataSource{}

// RolesDataSource lists all roles available in the caller's organization
// (`GET /v1/organizations/roles`, SDK: Organizations.ListRoles). Both built-in
// and custom roles are returned, each with its label, display name, whether it
// is built-in, and the `resource:scope` permission strings it grants.
type RolesDataSource struct {
	client *opusdns.Client
}

type RolesDataSourceModel struct {
	ID    types.String `tfsdk:"id"`
	Roles types.List   `tfsdk:"roles"`
}

// roleAttrTypes is the object shape for each element of the `roles` list.
var roleAttrTypes = map[string]attr.Type{
	"label":       types.StringType,
	"name":        types.StringType,
	"description": types.StringType,
	"built_in":    types.BoolType,
	"permissions": types.ListType{ElemType: types.StringType},
}

func NewRolesDataSource() datasource.DataSource {
	return &RolesDataSource{}
}

func (d *RolesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_roles"
}

func (d *RolesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all roles available in the authenticated caller's organization " +
			"(`GET /v1/organizations/roles`). Includes both built-in and custom roles, each with its " +
			"URL-safe `label`, display `name`, `built_in` flag, and the list of `resource:scope` " +
			"`permissions` it grants. Use a role's `label` when assigning it via " +
			"`opusdns_user_role_assignment`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, MarkdownDescription: "Static identifier."},
			"roles": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "All roles available in the organization.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"label":       schema.StringAttribute{Computed: true, MarkdownDescription: "URL-safe, per-organization unique role identifier (e.g. `support_staff`). Use this when assigning the role."},
						"name":        schema.StringAttribute{Computed: true, MarkdownDescription: "Human-readable display name (e.g. `Support Staff`)."},
						"description": schema.StringAttribute{Computed: true, MarkdownDescription: "Optional description of the role, or empty if unset."},
						"built_in":    schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether this is an immutable built-in role (`true`) or an organization-owned custom role (`false`)."},
						"permissions": schema.ListAttribute{
							Computed:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "The `resource:scope` permission strings the role grants (e.g. `domains:read`, `dns:manage`).",
						},
					},
				},
			},
		},
	}
}

func (d *RolesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *RolesDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	roles, err := d.client.Organizations.ListRoles(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing roles", formatAPIError(err))
		return
	}

	objType := types.ObjectType{AttrTypes: roleAttrTypes}
	values := make([]attr.Value, len(roles))
	for i, role := range roles {
		obj, diags := roleDefinitionToObject(ctx, role)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		values[i] = obj
	}

	list, diags := types.ListValue(objType, values)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data := RolesDataSourceModel{
		ID:    types.StringValue("roles"),
		Roles: list,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// roleDefinitionToObject converts an SDK RoleDefinition into a Terraform Object
// value matching roleAttrTypes.
func roleDefinitionToObject(ctx context.Context, role models.RoleDefinition) (types.Object, diag.Diagnostics) {
	permissions, diags := types.ListValueFrom(ctx, types.StringType, role.Permissions)
	if diags.HasError() {
		return types.ObjectNull(roleAttrTypes), diags
	}
	description := ""
	if role.Description != nil {
		description = *role.Description
	}
	obj, oDiags := types.ObjectValue(roleAttrTypes, map[string]attr.Value{
		"label":       types.StringValue(role.Label),
		"name":        types.StringValue(role.Name),
		"description": types.StringValue(description),
		"built_in":    types.BoolValue(role.BuiltIn),
		"permissions": permissions,
	})
	diags.Append(oDiags...)
	return obj, diags
}
