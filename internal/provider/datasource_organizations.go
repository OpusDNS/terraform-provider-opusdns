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

// Ensure OrganizationsDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &OrganizationsDataSource{}

// OrganizationsDataSource lists the child organizations of the authenticated
// organization via `/v1/organizations`.
type OrganizationsDataSource struct {
	client *opusdns.Client
}

// OrganizationsDataSourceModel is the top-level data-source state shape.
type OrganizationsDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	Organizations types.List   `tfsdk:"organizations"`
}

var organizationItemAttrTypes = map[string]attr.Type{
	"organization_id":        types.StringType,
	"name":                   types.StringType,
	"status":                 types.StringType,
	"country_code":           types.StringType,
	"currency":               types.StringType,
	"parent_organization_id": types.StringType,
}

// NewOrganizationsDataSource returns a new OrganizationsDataSource.
func NewOrganizationsDataSource() datasource.DataSource {
	return &OrganizationsDataSource{}
}

func (d *OrganizationsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organizations"
}

func (d *OrganizationsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all child organizations of the authenticated organization (`GET /v1/organizations`).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, MarkdownDescription: "Static identifier for this data source."},
			"organizations": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of organizations.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"organization_id":        schema.StringAttribute{Computed: true},
						"name":                   schema.StringAttribute{Computed: true},
						"status":                 schema.StringAttribute{Computed: true},
						"country_code":           schema.StringAttribute{Computed: true},
						"currency":               schema.StringAttribute{Computed: true},
						"parent_organization_id": schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *OrganizationsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *OrganizationsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OrganizationsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	orgs, err := d.client.Organizations.ListOrganizations(ctx, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error listing organizations", err.Error())
		return
	}

	objType := types.ObjectType{AttrTypes: organizationItemAttrTypes}
	values := make([]attr.Value, len(orgs))
	for i, org := range orgs {
		currency := types.StringNull()
		if org.Currency != nil {
			currency = types.StringValue(string(*org.Currency))
		}
		parent := types.StringNull()
		if org.ParentOrganizationID != nil {
			parent = types.StringValue(string(*org.ParentOrganizationID))
		}
		obj, diags := types.ObjectValue(organizationItemAttrTypes, map[string]attr.Value{
			"organization_id":        types.StringValue(string(org.OrganizationID)),
			"name":                   types.StringValue(org.Name),
			"status":                 types.StringValue(string(org.Status)),
			"country_code":           stringPtrToValue(org.CountryCode),
			"currency":               currency,
			"parent_organization_id": parent,
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

	data.ID = types.StringValue("organizations")
	data.Organizations = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
