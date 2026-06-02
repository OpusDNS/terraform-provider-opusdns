package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

var _ datasource.DataSource = &OrganizationIPRestrictionDataSource{}

type OrganizationIPRestrictionDataSource struct {
	client *opusdns.Client
}

type OrganizationIPRestrictionDataSourceModel struct {
	ID              types.String `tfsdk:"id"`
	IPRestrictionID types.String `tfsdk:"ip_restriction_id"`
	OrganizationID  types.String `tfsdk:"organization_id"`
	IPNetwork       types.String `tfsdk:"ip_network"`
	LastUsedOn      types.String `tfsdk:"last_used_on"`
	CreatedOn       types.String `tfsdk:"created_on"`
}

func NewOrganizationIPRestrictionDataSource() datasource.DataSource {
	return &OrganizationIPRestrictionDataSource{}
}

func (d *OrganizationIPRestrictionDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_ip_restriction"
}

func (d *OrganizationIPRestrictionDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a single organization IP restriction by id.",
		Attributes: map[string]schema.Attribute{
			"id":                schema.StringAttribute{Computed: true, MarkdownDescription: "Mirror of `ip_restriction_id`."},
			"ip_restriction_id": schema.StringAttribute{Required: true, MarkdownDescription: "IP restriction identifier."},
			"organization_id":   schema.StringAttribute{Computed: true},
			"ip_network":        schema.StringAttribute{Computed: true},
			"last_used_on":      schema.StringAttribute{Computed: true},
			"created_on":        schema.StringAttribute{Computed: true},
		},
	}
}

func (d *OrganizationIPRestrictionDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *OrganizationIPRestrictionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OrganizationIPRestrictionDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	restriction, err := d.client.Organizations.GetIPRestriction(ctx, models.TypeID(data.IPRestrictionID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Error reading organization IP restriction", formatAPIError(err))
		return
	}
	populateOrganizationIPRestrictionDataSourceModel(&data, restriction)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func populateOrganizationIPRestrictionDataSourceModel(data *OrganizationIPRestrictionDataSourceModel, restriction *models.IPRestriction) {
	id := fmt.Sprintf("%d", restriction.IPRestrictionID)
	data.ID = types.StringValue(id)
	data.IPRestrictionID = types.StringValue(id)
	data.OrganizationID = types.StringValue(string(restriction.OrganizationID))
	data.IPNetwork = types.StringValue(restriction.IPNetwork)
	data.LastUsedOn = timePtrToValue(restriction.LastUsedOn)
	data.CreatedOn = types.StringValue(restriction.CreatedOn.Format(time.RFC3339))
}
