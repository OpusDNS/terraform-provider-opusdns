package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure OrganizationPricingDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &OrganizationPricingDataSource{}

// OrganizationPricingDataSource reads product prices for an organization
// (`GET /v1/organizations/{organization_id}/pricing/product-type/{product_type}`).
//
// The SDK's `Organizations.GetPricing` returns a `ProductPricing` struct
// whose shape is completely different from the live API response
// (`GetPricesResponse{prices: [PriceInfo{...}]}` in
// common/lib/utils/billing_gateway_client.py:313), and it also omits the
// `product_action`/`product_class` query parameters the endpoint accepts.
// This data source bypasses the SDK helper and hits the endpoint via the
// raw HTTP client.
type OrganizationPricingDataSource struct {
	client *opusdns.Client
}

type OrganizationPricingDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Me             types.Bool   `tfsdk:"me"`
	ProductType    types.String `tfsdk:"product_type"`
	ProductAction  types.String `tfsdk:"product_action"`
	ProductClass   types.String `tfsdk:"product_class"`
	Prices         types.List   `tfsdk:"prices"`
	TotalReturned  types.Int64  `tfsdk:"total_returned"`
}

var pricingPeriodAttrTypes = map[string]attr.Type{
	"value": types.Int64Type,
	"unit":  types.StringType,
}

var pricingPriceAttrTypes = map[string]attr.Type{
	"product_type":   types.StringType,
	"product_action": types.StringType,
	"product_class":  types.StringType,
	"price":          types.StringType,
	"currency":       types.StringType,
	"period":         types.ObjectType{AttrTypes: pricingPeriodAttrTypes},
}

// pricingWire mirrors the live API's PriceInfo / GetPricesResponse
// (common/lib/utils/billing_gateway_client.py:286,313).
type pricingPeriodWire struct {
	Value int    `json:"value"`
	Unit  string `json:"unit"`
}

type pricingPriceWire struct {
	ProductType   string             `json:"product_type"`
	ProductAction *string            `json:"product_action,omitempty"`
	ProductClass  *string            `json:"product_class,omitempty"`
	Price         json.Number        `json:"price"`
	Currency      string             `json:"currency"`
	Period        *pricingPeriodWire `json:"period,omitempty"`
}

type pricingResponseWire struct {
	Prices []pricingPriceWire `json:"prices"`
}

func NewOrganizationPricingDataSource() datasource.DataSource {
	return &OrganizationPricingDataSource{}
}

func (d *OrganizationPricingDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_pricing"
}

func (d *OrganizationPricingDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads product prices configured for an organization " +
			"(`GET /v1/organizations/{organization_id}/pricing/product-type/{product_type}`). " +
			"Either set `organization_id` or `me = true`. Narrow the result with " +
			"`product_action` (e.g. `create`, `renew`, `transfer`) and `product_class` " +
			"(e.g. a specific TLD when `product_type = domain`). Money values are returned " +
			"as decimal strings to preserve precision.",
		Attributes: map[string]schema.Attribute{
			"id":              schema.StringAttribute{Computed: true, MarkdownDescription: "Static identifier for this data source."},
			"organization_id": schema.StringAttribute{Optional: true, MarkdownDescription: "Organization id. Mutually exclusive with `me`."},
			"me":              schema.BoolAttribute{Optional: true, MarkdownDescription: "When true, resolve the caller's organization id via `/v1/users/me`."},
			"product_type":    schema.StringAttribute{Required: true, MarkdownDescription: "Product type to fetch prices for (e.g. `domain`, `email_forward`, `domain_forward`, `zones`, `account_wallet`)."},
			"product_action":  schema.StringAttribute{Optional: true, MarkdownDescription: "Optional action filter (e.g. `create`, `renew`, `transfer`, `restore`)."},
			"product_class":   schema.StringAttribute{Optional: true, MarkdownDescription: "Optional product class filter (e.g. a specific TLD like `com` when `product_type = domain`)."},
			"total_returned": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Number of price entries returned (length of `prices`).",
			},
			"prices": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Price entries matching the filters.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"product_type":   schema.StringAttribute{Computed: true, MarkdownDescription: "Product type."},
						"product_action": schema.StringAttribute{Computed: true, MarkdownDescription: "Product action, or empty if unset."},
						"product_class":  schema.StringAttribute{Computed: true, MarkdownDescription: "Product class, or empty if unset."},
						"price":          schema.StringAttribute{Computed: true, MarkdownDescription: "Price as a decimal string."},
						"currency":       schema.StringAttribute{Computed: true, MarkdownDescription: "Currency code."},
						"period": schema.SingleNestedAttribute{
							Computed:            true,
							MarkdownDescription: "Pricing period (e.g. `{value:1, unit:\"year\"}`), or null if not period-based.",
							Attributes: map[string]schema.Attribute{
								"value": schema.Int64Attribute{Computed: true, MarkdownDescription: "Period value (the `1` in `1 year`)."},
								"unit":  schema.StringAttribute{Computed: true, MarkdownDescription: "Period unit (e.g. `year`, `month`)."},
							},
						},
					},
				},
			},
		},
	}
}

func (d *OrganizationPricingDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c, ok := configureClientFromDataSourceProviderData(req.ProviderData, resp.Diagnostics.AddError); ok {
		d.client = c
	}
}

func (d *OrganizationPricingDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OrganizationPricingDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	orgID, idLabel, diags := resolveOrgSelector(ctx, d.client, data.OrganizationID, data.Me)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if orgID == "" {
		resp.Diagnostics.AddError(
			"Missing organization selector",
			"`organization_id` or `me = true` is required for the pricing endpoint.",
		)
		return
	}

	productType := data.ProductType.ValueString()
	if productType == "" {
		resp.Diagnostics.AddError(
			"Missing product_type",
			"`product_type` is required (e.g. `domain`, `email_forward`).",
		)
		return
	}

	path := d.client.HTTPClient().BuildPath("organizations", string(orgID), "pricing", "product-type", productType)
	query := url.Values{}
	if v := data.ProductAction.ValueString(); v != "" {
		query.Set("product_action", v)
	}
	if v := data.ProductClass.ValueString(); v != "" {
		query.Set("product_class", v)
	}

	httpResp, err := d.client.HTTPClient().Get(ctx, path, query)
	if err != nil {
		resp.Diagnostics.AddError("Error fetching organization pricing", formatAPIError(err))
		return
	}
	var pr pricingResponseWire
	if err := d.client.HTTPClient().DecodeResponse(httpResp, &pr); err != nil {
		var raw json.RawMessage
		_ = d.client.HTTPClient().DecodeResponse(httpResp, &raw)
		resp.Diagnostics.AddError(
			"Error decoding organization pricing response",
			fmt.Sprintf("expected a GetPricesResponse envelope: %s\nraw body: %s",
				err.Error(), string(raw)),
		)
		return
	}

	objType := types.ObjectType{AttrTypes: pricingPriceAttrTypes}
	values := make([]attr.Value, len(pr.Prices))
	for i, p := range pr.Prices {
		obj, ed := pricingPriceWireToObject(p)
		resp.Diagnostics.Append(ed...)
		if resp.Diagnostics.HasError() {
			return
		}
		values[i] = obj
	}
	list, ld := types.ListValue(objType, values)
	resp.Diagnostics.Append(ld...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue(fmt.Sprintf("pricing:%s:%s", idLabel, productType))
	data.Prices = list
	data.TotalReturned = types.Int64Value(int64(len(pr.Prices)))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func pricingPriceWireToObject(p pricingPriceWire) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics

	action := ""
	if p.ProductAction != nil {
		action = *p.ProductAction
	}
	class := ""
	if p.ProductClass != nil {
		class = *p.ProductClass
	}

	var period attr.Value
	if p.Period != nil {
		pObj, pd := types.ObjectValue(pricingPeriodAttrTypes, map[string]attr.Value{
			"value": types.Int64Value(int64(p.Period.Value)),
			"unit":  types.StringValue(p.Period.Unit),
		})
		diags.Append(pd...)
		if diags.HasError() {
			return types.ObjectNull(pricingPriceAttrTypes), diags
		}
		period = pObj
	} else {
		period = types.ObjectNull(pricingPeriodAttrTypes)
	}

	obj, od := types.ObjectValue(pricingPriceAttrTypes, map[string]attr.Value{
		"product_type":   types.StringValue(p.ProductType),
		"product_action": types.StringValue(action),
		"product_class":  types.StringValue(class),
		"price":          types.StringValue(string(p.Price)),
		"currency":       types.StringValue(p.Currency),
		"period":         period,
	})
	diags.Append(od...)
	return obj, diags
}
