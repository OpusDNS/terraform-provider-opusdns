package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

var _ datasource.DataSource = &OrganizationIPRestrictionsDataSource{}

type OrganizationIPRestrictionsDataSource struct {
	client *opusdns.Client
}

type OrganizationIPRestrictionsDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	IPRestrictions types.List   `tfsdk:"ip_restrictions"`
}

var organizationIPRestrictionItemAttrTypes = map[string]attr.Type{
	"ip_restriction_id": types.StringType,
	"organization_id":   types.StringType,
	"ip_network":        types.StringType,
	"last_used_on":      types.StringType,
	"created_on":        types.StringType,
}

func NewOrganizationIPRestrictionsDataSource() datasource.DataSource {
	return &OrganizationIPRestrictionsDataSource{}
}

func (d *OrganizationIPRestrictionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_ip_restrictions"
}

func (d *OrganizationIPRestrictionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists organization IP restrictions for OpusDNS API access.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, MarkdownDescription: "Static identifier for this data source."},
			"ip_restrictions": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Organization IP restrictions.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"ip_restriction_id": schema.StringAttribute{Computed: true},
					"organization_id":   schema.StringAttribute{Computed: true},
					"ip_network":        schema.StringAttribute{Computed: true},
					"last_used_on":      schema.StringAttribute{Computed: true},
					"created_on":        schema.StringAttribute{Computed: true},
				}},
			},
		},
	}
}

func (d *OrganizationIPRestrictionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*opusdns.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *opusdns.Client, got: %T.", req.ProviderData))
		return
	}
	d.client = client
}

func (d *OrganizationIPRestrictionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OrganizationIPRestrictionsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	listResp, err := d.client.Organizations.ListIPRestrictions(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error listing organization IP restrictions", formatAPIError(err))
		return
	}

	objType := types.ObjectType{AttrTypes: organizationIPRestrictionItemAttrTypes}
	values := make([]attr.Value, len(listResp.Results))
	for i := range listResp.Results {
		r := &listResp.Results[i]
		obj, diags := types.ObjectValue(organizationIPRestrictionItemAttrTypes, map[string]attr.Value{
			"ip_restriction_id": intToStringValue(r.IPRestrictionID),
			"organization_id":   types.StringValue(string(r.OrganizationID)),
			"ip_network":        types.StringValue(r.IPNetwork),
			"last_used_on":      timePtrToValue(r.LastUsedOn),
			"created_on":        types.StringValue(r.CreatedOn.Format(time.RFC3339)),
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

	data.ID = types.StringValue("organization_ip_restrictions")
	data.IPRestrictions = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
