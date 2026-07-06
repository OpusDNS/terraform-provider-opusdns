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

// Ensure DomainSuggestionsDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &DomainSuggestionsDataSource{}

// DomainSuggestionsDataSource exposes the registry's name-suggestion engine
// via `GET /v1/domain-search/suggest`. Given a seed query (often the SLD a
// customer originally wanted) the API returns ranked alternatives across
// the caller's TLD portfolio, complete with availability status and pricing.
type DomainSuggestionsDataSource struct {
	client *opusdns.Client
}

type DomainSuggestionsDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	Query            types.String `tfsdk:"query"`
	TLDs             types.List   `tfsdk:"tlds"`
	Limit            types.Int64  `tfsdk:"limit"`
	Premium          types.Bool   `tfsdk:"premium"`
	Suggestions      types.List   `tfsdk:"suggestions"`
	Total            types.Int64  `tfsdk:"total"`
	ProcessingTimeMs types.Int64  `tfsdk:"processing_time_ms"`
}

// domainSuggestionPriceAttrTypes is the nested object shape for the
// `price` / `renewal_price` fields on each suggestion. The SDK's
// DomainSearchSuggestionPriceData carries a nullable amount, a currency,
// and a period; we surface amount as an empty string when the API omits it.
var domainSuggestionPriceAttrTypes = map[string]attr.Type{
	"amount":   types.StringType,
	"currency": types.StringType,
	"period":   types.StringType,
}

var domainSuggestionAttrTypes = map[string]attr.Type{
	"domain":        types.StringType,
	"available":     types.BoolType,
	"premium":       types.BoolType,
	"price":         types.ObjectType{AttrTypes: domainSuggestionPriceAttrTypes},
	"renewal_price": types.ObjectType{AttrTypes: domainSuggestionPriceAttrTypes},
}

func NewDomainSuggestionsDataSource() datasource.DataSource {
	return &DomainSuggestionsDataSource{}
}

func (d *DomainSuggestionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain_suggestions"
}

func (d *DomainSuggestionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Returns ranked domain-name suggestions for a query string " +
			"(`GET /v1/domain-search/suggest`). Each suggestion includes the candidate domain, " +
			"whether it is available, whether it is premium, and optional registration/renewal pricing.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Static identifier derived from the query.",
			},
			"query": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Seed query or SLD to generate suggestions for (e.g. `acme`).",
			},
			"tlds": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Optional list of TLDs (without leading dot) to bias suggestions to.",
			},
			"limit": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Maximum number of suggestions to return. Server default applies when omitted.",
			},
			"premium": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Whether to include premium domains in the suggestions. Server default applies when omitted.",
			},
			"total": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Total number of domains the API considered (from response metadata).",
			},
			"processing_time_ms": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Server-side processing time for this suggestion request, in milliseconds.",
			},
			"suggestions": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Ranked list of suggested domains.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"domain":    schema.StringAttribute{Computed: true, MarkdownDescription: "Suggested domain name."},
						"available": schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the suggested domain is available for registration."},
						"premium":   schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the suggested domain is a premium domain."},
						"price": schema.SingleNestedAttribute{
							Computed:            true,
							MarkdownDescription: "Registration pricing for the suggested domain. `amount` defaults to empty when the API omits it.",
							Attributes: map[string]schema.Attribute{
								"amount":   schema.StringAttribute{Computed: true},
								"currency": schema.StringAttribute{Computed: true},
								"period":   schema.StringAttribute{Computed: true},
							},
						},
						"renewal_price": schema.SingleNestedAttribute{
							Computed:            true,
							MarkdownDescription: "Renewal pricing for the suggested domain, if provided. `amount` defaults to empty when the API omits it.",
							Attributes: map[string]schema.Attribute{
								"amount":   schema.StringAttribute{Computed: true},
								"currency": schema.StringAttribute{Computed: true},
								"period":   schema.StringAttribute{Computed: true},
							},
						},
					},
				},
			},
		},
	}
}

func (d *DomainSuggestionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DomainSuggestionsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DomainSuggestionsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	query := data.Query.ValueString()
	if query == "" {
		resp.Diagnostics.AddError(
			"Invalid query",
			"The `query` attribute must be a non-empty seed string.",
		)
		return
	}

	opts := &models.DomainSuggestRequest{Query: query}
	if !data.TLDs.IsNull() && !data.TLDs.IsUnknown() {
		var tlds []string
		diags := data.TLDs.ElementsAs(ctx, &tlds, false)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		opts.TLDs = tlds
	}
	if !data.Limit.IsNull() && !data.Limit.IsUnknown() {
		opts.Limit = int(data.Limit.ValueInt64())
	}
	if !data.Premium.IsNull() && !data.Premium.IsUnknown() {
		premium := data.Premium.ValueBool()
		opts.Premium = &premium
	}

	suggestResp, err := d.client.Availability.GetSuggestions(ctx, query, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error fetching domain suggestions", formatAPIError(err))
		return
	}
	if suggestResp == nil {
		resp.Diagnostics.AddError(
			"Empty suggestions response",
			"The /v1/domain-search/suggest endpoint returned an empty result.",
		)
		return
	}

	objType := types.ObjectType{AttrTypes: domainSuggestionAttrTypes}
	values := make([]attr.Value, len(suggestResp.Suggestions))
	for i, s := range suggestResp.Suggestions {
		obj, diags := types.ObjectValue(domainSuggestionAttrTypes, map[string]attr.Value{
			"domain":        types.StringValue(s.Domain),
			"available":     types.BoolValue(s.Available),
			"premium":       types.BoolValue(s.Premium),
			"price":         buildSuggestionPriceObject(&s.Price),
			"renewal_price": buildSuggestionPriceObject(s.RenewalPrice),
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

	data.ID = types.StringValue("suggestions:" + query)
	data.Suggestions = list
	data.Total = types.Int64Value(int64(suggestResp.Meta.Total))
	data.ProcessingTimeMs = types.Int64Value(int64(suggestResp.Meta.ProcessingTimeMs))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// buildSuggestionPriceObject returns a fully-populated object for the
// nested `price` / `renewal_price` attributes. A nil pointer yields an
// object with all fields set to zero values (rather than a null object) so
// the schema's static type is satisfied unconditionally.
func buildSuggestionPriceObject(p *models.DomainSearchSuggestionPriceData) types.Object {
	amount, currency, period := "", "", ""
	if p != nil {
		if p.Amount != nil {
			amount = *p.Amount
		}
		currency = p.Currency
		period = fmt.Sprintf("%d%s", p.Period.Value, string(p.Period.Unit))
	}
	obj, _ := types.ObjectValue(domainSuggestionPriceAttrTypes, map[string]attr.Value{
		"amount":   types.StringValue(amount),
		"currency": types.StringValue(currency),
		"period":   types.StringValue(period),
	})
	return obj
}
