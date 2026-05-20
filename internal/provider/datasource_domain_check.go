package provider

import (
	"context"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

var _ datasource.DataSource = &DomainCheckDataSource{}

// DomainCheckDataSource performs a bulk-availability check
// (`GET /v1/domains/check`). Returns richer metadata than the basic
// `/v1/availability` endpoint (premium status, claims keys, pricing).
type DomainCheckDataSource struct {
	client *opusdns.Client
}

type DomainCheckDataSourceModel struct {
	ID      types.String `tfsdk:"id"`
	Domains types.List   `tfsdk:"domains"`
	Results types.List   `tfsdk:"results"`
}

// domainCheckAPIResponse mirrors DomainCheckResponse / DomainAvailabilityResponse.
type domainCheckAPIResponse struct {
	Results []domainAvailabilityEntry `json:"results"`
}

type domainAvailabilityEntry struct {
	Domain         string                  `json:"domain"`
	Available      bool                    `json:"available"`
	Reason         *string                 `json:"reason"`
	IsPremium      bool                    `json:"is_premium"`
	ClaimsKey      *string                 `json:"claims_key"`
	PremiumPricing *premiumPricingResponse `json:"premium_pricing"`
}

type premiumPricingResponse struct {
	Prices []premiumPricingAction `json:"prices"`
}

type premiumPricingAction struct {
	Action   string `json:"action"`
	Price    string `json:"price"`
	Currency string `json:"currency"`
}

var domainCheckPremiumPriceAttrTypes = map[string]attr.Type{
	"action":   types.StringType,
	"price":    types.StringType,
	"currency": types.StringType,
}

var domainCheckResultAttrTypes = map[string]attr.Type{
	"domain":          types.StringType,
	"available":       types.BoolType,
	"reason":          types.StringType,
	"is_premium":      types.BoolType,
	"claims_key":      types.StringType,
	"premium_pricing": types.ListType{ElemType: types.ObjectType{AttrTypes: domainCheckPremiumPriceAttrTypes}},
}

func NewDomainCheckDataSource() datasource.DataSource {
	return &DomainCheckDataSource{}
}

func (d *DomainCheckDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain_check"
}

func (d *DomainCheckDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Performs a bulk domain availability check (`GET /v1/domains/check`). " +
			"Unlike `data.opusdns_domain_availability`, this endpoint also returns premium-pricing " +
			"info and TMCH claims keys, making it the preferred precondition for `opusdns_domain` " +
			"registrations of premium or trademarked names.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true},
			"domains": schema.ListAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Domain names to check. May mix TLDs.",
				Validators: []validator.List{
					listvalidator.SizeAtLeast(1),
				},
			},
			"results": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Per-domain availability results.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"domain":     schema.StringAttribute{Computed: true},
						"available":  schema.BoolAttribute{Computed: true},
						"reason":     schema.StringAttribute{Computed: true, MarkdownDescription: "When `available` is false, the reason."},
						"is_premium": schema.BoolAttribute{Computed: true},
						"claims_key": schema.StringAttribute{Computed: true, MarkdownDescription: "TMCH claims key when the domain is in a trademark claims period."},
						"premium_pricing": schema.ListNestedAttribute{
							Computed:            true,
							MarkdownDescription: "Premium pricing entries (one per supported action, e.g. `create`, `renew`).",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"action":   schema.StringAttribute{Computed: true},
									"price":    schema.StringAttribute{Computed: true},
									"currency": schema.StringAttribute{Computed: true},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (d *DomainCheckDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DomainCheckDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DomainCheckDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var domains []string
	resp.Diagnostics.Append(data.Domains.ElementsAs(ctx, &domains, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	query := url.Values{}
	for _, domain := range domains {
		query.Add("domains", domain)
	}

	path := d.client.HTTPClient().BuildPath("domains", "check")
	httpResp, err := d.client.HTTPClient().Get(ctx, path, query)
	if err != nil {
		resp.Diagnostics.AddError("Error checking domains", formatAPIError(err))
		return
	}

	var out domainCheckAPIResponse
	if err := d.client.HTTPClient().DecodeResponse(httpResp, &out); err != nil {
		resp.Diagnostics.AddError("Error decoding domain check response", formatAPIError(err))
		return
	}

	resultObjType := types.ObjectType{AttrTypes: domainCheckResultAttrTypes}
	priceObjType := types.ObjectType{AttrTypes: domainCheckPremiumPriceAttrTypes}
	resultValues := make([]attr.Value, len(out.Results))
	for i := range out.Results {
		r := &out.Results[i]
		var priceValues []attr.Value
		if r.PremiumPricing != nil {
			priceValues = make([]attr.Value, len(r.PremiumPricing.Prices))
			for j, p := range r.PremiumPricing.Prices {
				obj, diags := types.ObjectValue(domainCheckPremiumPriceAttrTypes, map[string]attr.Value{
					"action":   types.StringValue(p.Action),
					"price":    types.StringValue(p.Price),
					"currency": types.StringValue(p.Currency),
				})
				resp.Diagnostics.Append(diags...)
				if resp.Diagnostics.HasError() {
					return
				}
				priceValues[j] = obj
			}
		}
		priceList, diags := types.ListValue(priceObjType, priceValues)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}

		obj, diags := types.ObjectValue(domainCheckResultAttrTypes, map[string]attr.Value{
			"domain":          types.StringValue(r.Domain),
			"available":       types.BoolValue(r.Available),
			"reason":          stringPtrToValue(r.Reason),
			"is_premium":      types.BoolValue(r.IsPremium),
			"claims_key":      stringPtrToValue(r.ClaimsKey),
			"premium_pricing": priceList,
		})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		resultValues[i] = obj
	}

	list, diags := types.ListValue(resultObjType, resultValues)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue("domain_check")
	data.Results = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
