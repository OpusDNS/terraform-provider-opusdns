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
	ID                 types.String `tfsdk:"id"`
	Query              types.String `tfsdk:"query"`
	TLDs               types.List   `tfsdk:"tlds"`
	Limit              types.Int64  `tfsdk:"limit"`
	IncludeUnavailable types.Bool   `tfsdk:"include_unavailable"`
	Suggestions        types.List   `tfsdk:"suggestions"`
	Total              types.Int64  `tfsdk:"total"`
	ProcessingTimeMs   types.Int64  `tfsdk:"processing_time_ms"`
}

// domainSuggestionPriceAttrTypes is the nested object shape for the
// optional `price` field on each suggestion. Pricing pointers in the SDK
// are nullable; we surface them as empty strings here for simplicity.
var domainSuggestionPriceAttrTypes = map[string]attr.Type{
	"register_price": types.StringType,
	"renew_price":    types.StringType,
	"transfer_price": types.StringType,
	"currency":       types.StringType,
	"period":         types.Int64Type,
}

var domainSuggestionAttrTypes = map[string]attr.Type{
	"domain": types.StringType,
	"status": types.StringType,
	"score":  types.Float64Type,
	"price":  types.ObjectType{AttrTypes: domainSuggestionPriceAttrTypes},
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
			"its current availability status, a relevance score, and optional pricing.",
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
			"include_unavailable": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Whether to include unavailable suggestions in the response.",
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
						"domain": schema.StringAttribute{Computed: true, MarkdownDescription: "Suggested domain name."},
						"status": schema.StringAttribute{Computed: true, MarkdownDescription: "Availability status (e.g. `available`, `unavailable`, `premium`)."},
						"score":  schema.Float64Attribute{Computed: true, MarkdownDescription: "Relevance score (higher is more relevant)."},
						"price": schema.SingleNestedAttribute{
							Computed:            true,
							MarkdownDescription: "Optional pricing for the suggested domain. All string fields default to empty when the API omits them.",
							Attributes: map[string]schema.Attribute{
								"register_price": schema.StringAttribute{Computed: true},
								"renew_price":    schema.StringAttribute{Computed: true},
								"transfer_price": schema.StringAttribute{Computed: true},
								"currency":       schema.StringAttribute{Computed: true},
								"period":         schema.Int64Attribute{Computed: true},
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
	if !data.IncludeUnavailable.IsNull() && !data.IncludeUnavailable.IsUnknown() {
		opts.IncludeUnavailable = data.IncludeUnavailable.ValueBool()
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
		priceObj := buildSuggestionPriceObject(s.Price)
		obj, diags := types.ObjectValue(domainSuggestionAttrTypes, map[string]attr.Value{
			"domain": types.StringValue(s.Domain),
			"status": types.StringValue(string(s.Status)),
			"score":  types.Float64Value(s.Score),
			"price":  priceObj,
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
// nested `price` attribute. A nil pointer yields an object with all
// fields set to zero values (rather than a null object) so the schema's
// static type is satisfied unconditionally.
func buildSuggestionPriceObject(p *models.DomainPrice) types.Object {
	if p == nil {
		obj, _ := types.ObjectValue(domainSuggestionPriceAttrTypes, map[string]attr.Value{
			"register_price": types.StringValue(""),
			"renew_price":    types.StringValue(""),
			"transfer_price": types.StringValue(""),
			"currency":       types.StringValue(""),
			"period":         types.Int64Value(0),
		})
		return obj
	}
	reg, ren, tx := "", "", ""
	if p.RegisterPrice != nil {
		reg = *p.RegisterPrice
	}
	if p.RenewPrice != nil {
		ren = *p.RenewPrice
	}
	if p.TransferPrice != nil {
		tx = *p.TransferPrice
	}
	obj, _ := types.ObjectValue(domainSuggestionPriceAttrTypes, map[string]attr.Value{
		"register_price": types.StringValue(reg),
		"renew_price":    types.StringValue(ren),
		"transfer_price": types.StringValue(tx),
		"currency":       types.StringValue(string(p.Currency)),
		"period":         types.Int64Value(int64(p.Period)),
	})
	return obj
}
