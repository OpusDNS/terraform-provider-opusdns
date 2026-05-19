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

var _ datasource.DataSource = &RolesDataSource{}

// RolesDataSource lists all roles available in the organization
// (`GET /v1/organizations/roles`).
//
// The OpenAPI spec declares no typed response schema for this endpoint, so
// the data source exposes both:
//   - `role_names`: best-effort list of string role names extracted from the
//     response (works whether the API returns `[{"name": "..."}]`,
//     `[{"role": "..."}]`, or bare `["..."]`)
//   - `roles_json`: the raw JSON body for callers that need richer metadata
type RolesDataSource struct {
	client *opusdns.Client
}

type RolesDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	RoleNames types.List   `tfsdk:"role_names"`
	RolesJSON types.String `tfsdk:"roles_json"`
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
			"(`GET /v1/organizations/roles`). The API's response is untyped in the OpenAPI spec, so " +
			"this data source surfaces both a best-effort `role_names` list and the raw `roles_json` body.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, MarkdownDescription: "Static identifier."},
			"role_names": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				MarkdownDescription: "Role names extracted from the response. Each entry is the `name`, " +
					"`role`, `id`, or `value` field of an object element, or the element itself if the " +
					"response is a bare list of strings.",
			},
			"roles_json": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Raw JSON response body. Useful when callers need fields beyond `role_names`.",
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
	path := d.client.HTTPClient().BuildPath("organizations", "roles")
	httpResp, err := d.client.HTTPClient().Get(ctx, path, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error listing roles", formatAPIError(err))
		return
	}

	// Decode into a generic interface so we can both stash the raw bytes and
	// best-effort extract role names.
	var raw interface{}
	if err := d.client.HTTPClient().DecodeResponse(httpResp, &raw); err != nil {
		resp.Diagnostics.AddError("Error decoding roles response", formatAPIError(err))
		return
	}

	jsonBytes, _ := json.Marshal(raw)

	names := extractRoleNames(raw)
	listValue, diags := types.ListValueFrom(ctx, types.StringType, names)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data := RolesDataSourceModel{
		ID:        types.StringValue("roles"),
		RoleNames: listValue,
		RolesJSON: types.StringValue(string(jsonBytes)),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// extractRoleNames pulls role names out of an arbitrarily-shaped JSON
// response. It handles the three shapes most likely to be returned:
//
//   - ["admin", "member", ...]
//   - [{"name": "admin"}, {"name": "member"}, ...]
//   - {"relations": [...]}, {"results": [...]}, {"roles": [...]}, etc.
func extractRoleNames(raw interface{}) []string {
	switch v := raw.(type) {
	case []interface{}:
		return extractFromSlice(v)
	case map[string]interface{}:
		for _, key := range []string{"relations", "results", "roles", "role_names", "data", "items"} {
			if inner, ok := v[key]; ok {
				if slice, ok := inner.([]interface{}); ok {
					return extractFromSlice(slice)
				}
			}
		}
	}
	return []string{}
}

func extractFromSlice(items []interface{}) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		switch v := it.(type) {
		case string:
			out = append(out, v)
		case map[string]interface{}:
			for _, key := range []string{"name", "role", "id", "value", "role_name"} {
				if s, ok := v[key].(string); ok {
					out = append(out, s)
					break
				}
			}
		}
	}
	return out
}
