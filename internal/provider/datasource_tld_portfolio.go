package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure TLDPortfolioDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &TLDPortfolioDataSource{}

// TLDPortfolioDataSource exposes the TLD portfolio enabled for the
// caller's organization (`GET /v1/tlds/portfolio`).
//
// The endpoint returns a bare JSON array of `{"tld": "...", "type": "..."}`
// objects, which `TLDs.GetPortfolio` decodes. Only the two fields the
// upstream returns are surfaced; richer per-TLD details are still available
// via the `opusdns_tld` data source.
type TLDPortfolioDataSource struct {
	client *opusdns.Client
}

type TLDPortfolioDataSourceModel struct {
	ID    types.String `tfsdk:"id"`
	Total types.Int64  `tfsdk:"total"`
	TLDs  types.List   `tfsdk:"tlds"`
}

// portfolioTLDAttrTypes only contains the fields the API actually
// returns. Anything richer should be looked up per-TLD via `opusdns_tld`.
var portfolioTLDAttrTypes = map[string]attr.Type{
	"name": types.StringType,
	"type": types.StringType,
}

func NewTLDPortfolioDataSource() datasource.DataSource {
	return &TLDPortfolioDataSource{}
}

func (d *TLDPortfolioDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tld_portfolio"
}

func (d *TLDPortfolioDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Returns the TLD portfolio enabled for the calling organization " +
			"(`GET /v1/tlds/portfolio`). Use this to discover which TLDs are usable for " +
			"`opusdns_domain` registrations under the current account, as opposed to the " +
			"full registry-wide listing exposed by `opusdns_tlds`. " +
			"Per-TLD details (pricing, restrictions, phases) are available via the " +
			"`opusdns_tld` data source.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Static identifier for this data source.",
			},
			"total": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Number of TLDs returned (length of `tlds`).",
			},
			"tlds": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The list of TLDs in the portfolio.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{Computed: true, MarkdownDescription: "TLD name without leading dot (e.g. `com`)."},
						"type": schema.StringAttribute{Computed: true, MarkdownDescription: "TLD type (`gTLD`, `ccTLD`, `newGTLD`, `sponsoredGTLD`)."},
					},
				},
			},
		},
	}
}

func (d *TLDPortfolioDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c, ok := configureClientFromDataSourceProviderData(req.ProviderData, resp.Diagnostics.AddError); ok {
		d.client = c
	}
}

func (d *TLDPortfolioDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	entries, err := d.client.TLDs.GetPortfolio(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error fetching TLD portfolio", formatAPIError(err))
		return
	}

	objType := types.ObjectType{AttrTypes: portfolioTLDAttrTypes}
	values := make([]attr.Value, len(entries))
	for i, e := range entries {
		obj, diags := types.ObjectValue(portfolioTLDAttrTypes, map[string]attr.Value{
			"name": types.StringValue(e.TLD),
			"type": types.StringValue(string(e.Type)),
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

	data := TLDPortfolioDataSourceModel{
		ID:    types.StringValue("tld_portfolio"),
		Total: types.Int64Value(int64(len(entries))),
		TLDs:  list,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
