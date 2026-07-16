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

// Ensure VanityNameserverSetsDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &VanityNameserverSetsDataSource{}

// VanityNameserverSetsDataSource lists all vanity nameserver sets in the
// caller's organization via `GET /v1/vanity-nameserver-sets` (SDK auto-paginates).
type VanityNameserverSetsDataSource struct {
	client *opusdns.Client
}

// VanityNameserverSetsDataSourceModel is the state shape for the data source.
type VanityNameserverSetsDataSourceModel struct {
	ID   types.String `tfsdk:"id"`
	Sets types.List   `tfsdk:"sets"`
}

// NewVanityNameserverSetsDataSource returns a new VanityNameserverSetsDataSource.
func NewVanityNameserverSetsDataSource() datasource.DataSource {
	return &VanityNameserverSetsDataSource{}
}

func (d *VanityNameserverSetsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vanity_nameserver_sets"
}

func (d *VanityNameserverSetsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all vanity nameserver sets in the authenticated caller's organization (`GET /v1/vanity-nameserver-sets`). Results are auto-paginated by the SDK.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, MarkdownDescription: "Static identifier for this data source."},
			"sets": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "All vanity nameserver sets in the organization.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"set_id":             schema.StringAttribute{Computed: true, MarkdownDescription: "The unique identifier of the set (e.g. `vns_...`)."},
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
									"hostname": schema.StringAttribute{Computed: true},
									"position": schema.Int64Attribute{Computed: true},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (d *VanityNameserverSetsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *VanityNameserverSetsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	sets, err := d.client.VanityNameservers.ListSets(ctx, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error listing vanity nameserver sets", formatAPIError(err))
		return
	}

	objType := types.ObjectType{AttrTypes: vanityNameserverSetItemAttrTypes}
	values := make([]attr.Value, len(sets))
	for i := range sets {
		obj, diags := vanityNameserverSetToObject(&sets[i])
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

	data := VanityNameserverSetsDataSourceModel{
		ID:   types.StringValue("vanity_nameserver_sets"),
		Sets: list,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// vanityNameserverSetToObject converts an SDK VanityNameserverSet into a
// Terraform Object value matching vanityNameserverSetItemAttrTypes.
func vanityNameserverSetToObject(set *models.VanityNameserverSet) (types.Object, diag.Diagnostics) {
	nsList, diags := vanityNameserversToList(set.Nameservers)
	if diags.HasError() {
		return types.ObjectNull(vanityNameserverSetItemAttrTypes), diags
	}
	obj, oDiags := types.ObjectValue(vanityNameserverSetItemAttrTypes, map[string]attr.Value{
		"set_id":             types.StringValue(string(set.SetID)),
		"organization_id":    types.StringValue(string(set.OrganizationID)),
		"name":               types.StringValue(set.Name),
		"parent_domain_name": types.StringValue(set.ParentDomainName),
		"soa_rname":          types.StringValue(set.SOARName),
		"is_default":         types.BoolValue(set.IsDefault),
		"status":             types.StringValue(string(set.Status)),
		"nameservers":        nsList,
	})
	diags.Append(oDiags...)
	return obj, diags
}
