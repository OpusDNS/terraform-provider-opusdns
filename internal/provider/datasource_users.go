package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure UsersDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &UsersDataSource{}

// UsersDataSource lists users for the authenticated caller's organization via
// `/v1/organizations/users` (the SDK exposes this through Users.ListUsers).
type UsersDataSource struct {
	client *opusdns.Client
}

// UsersDataSourceModel is the top-level data-source state shape.
type UsersDataSourceModel struct {
	ID    types.String `tfsdk:"id"`
	Users types.List   `tfsdk:"users"`
}

var userItemAttrTypes = map[string]attr.Type{
	"user_id":         types.StringType,
	"username":        types.StringType,
	"first_name":      types.StringType,
	"last_name":       types.StringType,
	"email":           types.StringType,
	"status":          types.StringType,
	"organization_id": types.StringType,
}

// NewUsersDataSource returns a new UsersDataSource.
func NewUsersDataSource() datasource.DataSource {
	return &UsersDataSource{}
}

func (d *UsersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_users"
}

func (d *UsersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists users within the authenticated caller's organization (`GET /v1/organizations/users`).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, MarkdownDescription: "Static identifier for this data source."},
			"users": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of users.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"user_id":         schema.StringAttribute{Computed: true},
						"username":        schema.StringAttribute{Computed: true},
						"first_name":      schema.StringAttribute{Computed: true},
						"last_name":       schema.StringAttribute{Computed: true},
						"email":           schema.StringAttribute{Computed: true},
						"status":          schema.StringAttribute{Computed: true},
						"organization_id": schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *UsersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *UsersDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data UsersDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	users, err := d.client.Users.ListUsers(ctx, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error listing users", formatAPIError(err))
		return
	}

	objType := types.ObjectType{AttrTypes: userItemAttrTypes}
	values := make([]attr.Value, len(users))
	for i, u := range users {
		obj, diags := types.ObjectValue(userItemAttrTypes, map[string]attr.Value{
			"user_id":         types.StringValue(string(u.UserID)),
			"username":        types.StringValue(u.Username),
			"first_name":      types.StringValue(u.FirstName),
			"last_name":       types.StringValue(u.LastName),
			"email":           types.StringValue(u.Email),
			"status":          types.StringValue(string(u.Status)),
			"organization_id": types.StringValue(string(u.OrganizationID)),
		})
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

	data.ID = types.StringValue("users")
	data.Users = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
