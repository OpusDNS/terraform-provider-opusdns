package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure VanityNameserverSetDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &VanityNameserverSetDataSource{}

// VanityNameserverSetDataSource fetches a single vanity nameserver set by id
// via `GET /v1/vanity-nameserver-sets/{set_id}`.
type VanityNameserverSetDataSource struct {
	client *opusdns.Client
}

// VanityNameserverSetDataSourceModel is the state shape for the data source.
type VanityNameserverSetDataSourceModel struct {
	SetID            types.String `tfsdk:"set_id"`
	OrganizationID   types.String `tfsdk:"organization_id"`
	Name             types.String `tfsdk:"name"`
	ParentDomainName types.String `tfsdk:"parent_domain_name"`
	SOARName         types.String `tfsdk:"soa_rname"`
	IsDefault        types.Bool   `tfsdk:"is_default"`
	Status           types.String `tfsdk:"status"`
	Nameservers      types.List   `tfsdk:"nameservers"`
}

// NewVanityNameserverSetDataSource returns a new VanityNameserverSetDataSource.
func NewVanityNameserverSetDataSource() datasource.DataSource {
	return &VanityNameserverSetDataSource{}
}

func (d *VanityNameserverSetDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vanity_nameserver_set"
}

func (d *VanityNameserverSetDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single vanity nameserver set by id (`GET /v1/vanity-nameserver-sets/{set_id}`).",
		Attributes: map[string]schema.Attribute{
			"set_id":             schema.StringAttribute{Required: true, MarkdownDescription: "The unique identifier of the vanity nameserver set (e.g. `vns_...`)."},
			"organization_id":    schema.StringAttribute{Computed: true, MarkdownDescription: "The organization that owns the set."},
			"name":               schema.StringAttribute{Computed: true, MarkdownDescription: "Human-readable name for the set."},
			"parent_domain_name": schema.StringAttribute{Computed: true, MarkdownDescription: "The parent domain used as the apex of the vanity NS zone."},
			"soa_rname":          schema.StringAttribute{Computed: true, MarkdownDescription: "The SOA RNAME stamped into vanity-branded zones."},
			"is_default":         schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether this is the organization's default vanity nameserver set."},
			"status":             schema.StringAttribute{Computed: true, MarkdownDescription: "Lifecycle status of the set."},
			"nameservers": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The nameservers in the set, ordered by position.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"hostname": schema.StringAttribute{Computed: true, MarkdownDescription: "The vanity nameserver hostname."},
						"position": schema.Int64Attribute{Computed: true, MarkdownDescription: "Ordering within the set; the lowest position becomes the SOA MNAME."},
					},
				},
			},
		},
	}
}

func (d *VanityNameserverSetDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *VanityNameserverSetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data VanityNameserverSetDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	set, err := d.client.VanityNameservers.GetSet(ctx, models.VanityNameserverSetID(data.SetID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Error reading vanity nameserver set", formatAPIError(err))
		return
	}

	data.SetID = types.StringValue(string(set.SetID))
	data.OrganizationID = types.StringValue(string(set.OrganizationID))
	data.Name = types.StringValue(set.Name)
	data.ParentDomainName = types.StringValue(set.ParentDomainName)
	data.SOARName = types.StringValue(set.SOARName)
	data.IsDefault = types.BoolValue(set.IsDefault)
	data.Status = types.StringValue(string(set.Status))

	nsList, nd := vanityNameserversToList(set.Nameservers)
	resp.Diagnostics.Append(nd...)
	data.Nameservers = nsList

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// vanityNameserverSetItemAttrTypes is the object shape for each element of the
// `sets` list attribute of the plural data source.
var vanityNameserverSetItemAttrTypes = map[string]attr.Type{
	"set_id":             types.StringType,
	"organization_id":    types.StringType,
	"name":               types.StringType,
	"parent_domain_name": types.StringType,
	"soa_rname":          types.StringType,
	"is_default":         types.BoolType,
	"status":             types.StringType,
	"nameservers":        types.ListType{ElemType: types.ObjectType{AttrTypes: vanityNameserverAttrTypes}},
}
