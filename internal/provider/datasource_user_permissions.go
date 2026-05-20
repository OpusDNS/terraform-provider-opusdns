package provider

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

var _ datasource.DataSource = &UserPermissionsDataSource{}

// UserPermissionsDataSource exposes a user's effective permission set
// (`GET /v1/users/{user_id}/permissions`).
//
// The OpenAPI spec leaves the response untyped, so this data source uses the
// same belt-and-braces approach as `data.opusdns_roles`: best-effort extract
// `permissions` (a list of strings) and always expose the raw `permissions_json`.
type UserPermissionsDataSource struct {
	client *opusdns.Client
}

type UserPermissionsDataSourceModel struct {
	UserID          types.String `tfsdk:"user_id"`
	Permissions     types.List   `tfsdk:"permissions"`
	PermissionsJSON types.String `tfsdk:"permissions_json"`
}

func NewUserPermissionsDataSource() datasource.DataSource {
	return &UserPermissionsDataSource{}
}

func (d *UserPermissionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_permissions"
}

func (d *UserPermissionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a user's effective permission set " +
			"(`GET /v1/users/{user_id}/permissions`). The OpenAPI spec leaves the response shape " +
			"untyped, so callers get both a best-effort `permissions` list and the raw " +
			"`permissions_json` body.",
		Attributes: map[string]schema.Attribute{
			"user_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "User id (e.g. `user_01j...`).",
			},
			"permissions": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Best-effort extraction of the user's effective permissions.",
			},
			"permissions_json": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Raw JSON response body. Use this if `permissions` comes back empty or you need fields beyond a flat name list.",
			},
		},
	}
}

func (d *UserPermissionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *UserPermissionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data UserPermissionsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	path := d.client.HTTPClient().BuildPath("users", data.UserID.ValueString(), "permissions")
	httpResp, err := d.client.HTTPClient().Get(ctx, path, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error reading user permissions", formatAPIError(err))
		return
	}

	var raw interface{}
	if err := d.client.HTTPClient().DecodeResponse(httpResp, &raw); err != nil {
		resp.Diagnostics.AddError("Error decoding permissions response", formatAPIError(err))
		return
	}
	jsonBytes, _ := json.Marshal(raw)

	names := extractPermissionNames(raw)
	list, diags := types.ListValueFrom(ctx, types.StringType, names)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Permissions = list
	data.PermissionsJSON = types.StringValue(string(jsonBytes))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// extractPermissionNames reuses the role-name extraction logic over the
// `permissions`/`relations`/`results` wrapper keys.
func extractPermissionNames(raw interface{}) []string {
	switch v := raw.(type) {
	case []interface{}:
		return extractFromSlice(v)
	case map[string]interface{}:
		for _, key := range []string{"permissions", "relations", "results", "data", "items"} {
			if inner, ok := v[key]; ok {
				if slice, ok := inner.([]interface{}); ok {
					return extractFromSlice(slice)
				}
			}
		}
	}
	return []string{}
}
