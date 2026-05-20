package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

var _ datasource.DataSource = &ContactAttributeSetDataSource{}

// ContactAttributeSetDataSource reads a single contact attribute set
// (`GET /v1/contacts/attribute-sets/{id}`).
type ContactAttributeSetDataSource struct {
	client *opusdns.Client
}

type ContactAttributeSetDataSourceModel struct {
	ContactAttributeSetID types.String `tfsdk:"contact_attribute_set_id"`
	OrganizationID        types.String `tfsdk:"organization_id"`
	Label                 types.String `tfsdk:"label"`
	TLD                   types.String `tfsdk:"tld"`
	Attributes            types.Map    `tfsdk:"attributes"`
	LinkedContacts        types.Int64  `tfsdk:"linked_contacts"`
	CreatedOn             types.String `tfsdk:"created_on"`
	UpdatedOn             types.String `tfsdk:"updated_on"`
}

func NewContactAttributeSetDataSource() datasource.DataSource {
	return &ContactAttributeSetDataSource{}
}

func (d *ContactAttributeSetDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_contact_attribute_set"
}

func (d *ContactAttributeSetDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a single contact attribute set " +
			"(`GET /v1/contacts/attribute-sets/{contact_attribute_set_id}`).",
		Attributes: map[string]schema.Attribute{
			"contact_attribute_set_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Attribute set id (e.g. `contact_attribute_set_01j...`).",
			},
			"organization_id": schema.StringAttribute{Computed: true},
			"label":           schema.StringAttribute{Computed: true},
			"tld":             schema.StringAttribute{Computed: true},
			"attributes":      schema.MapAttribute{Computed: true, ElementType: types.StringType},
			"linked_contacts": schema.Int64Attribute{Computed: true},
			"created_on":      schema.StringAttribute{Computed: true},
			"updated_on":      schema.StringAttribute{Computed: true},
		},
	}
}

func (d *ContactAttributeSetDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ContactAttributeSetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ContactAttributeSetDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	set, err := rawGetContactAttributeSet(ctx, d.client, data.ContactAttributeSetID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading contact attribute set", formatAPIError(err))
		return
	}

	data.OrganizationID = types.StringValue(set.OrganizationID)
	data.Label = types.StringValue(set.Label)
	data.TLD = types.StringValue(set.TLD)
	data.LinkedContacts = types.Int64Value(set.LinkedContacts)
	data.CreatedOn = types.StringValue(set.CreatedOn.Format(time.RFC3339))
	data.UpdatedOn = types.StringValue(set.UpdatedOn.Format(time.RFC3339))

	mv, diags := types.MapValueFrom(ctx, types.StringType, set.Attributes)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Attributes = mv

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
