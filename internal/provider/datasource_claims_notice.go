package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

var _ datasource.DataSource = &ClaimsNoticeDataSource{}

// ClaimsNoticeDataSource retrieves a TMCH claims notice for a single claims
// key (`POST /v1/domains/claims-notices`). The API limits each call to one
// key; use multiple data sources if you need multiple notices.
//
// The acceptance hash returned here is required when registering the
// corresponding trademarked domain via `opusdns_domain` (passed as part of
// the registration body).
type ClaimsNoticeDataSource struct {
	client *opusdns.Client
}

type ClaimsNoticeDataSourceModel struct {
	ClaimsKey                  types.String `tfsdk:"claims_key"`
	Label                      types.String `tfsdk:"label"`
	ClaimsNoticeAcceptanceHash types.String `tfsdk:"claims_notice_acceptance_hash"`
	NoticeTitle                types.String `tfsdk:"notice_title"`
	NoticeIntro                types.String `tfsdk:"notice_intro"`
	NoticeNotExactMatchIntro   types.String `tfsdk:"notice_not_exact_match_intro"`
	NoticeFooter               types.String `tfsdk:"notice_footer"`
	NoticeFooterURL            types.String `tfsdk:"notice_footer_url"`
	RenderedHTML               types.String `tfsdk:"rendered_html"`
}

// claimsNoticesAPIResponse mirrors ClaimsNoticesResponse.
type claimsNoticesAPIResponse struct {
	ClaimsNotices []claimsNoticeEntry `json:"claims_notices"`
}

type claimsNoticeEntry struct {
	ClaimsKey                  string `json:"claims_key"`
	ClaimsNoticeAcceptanceHash string `json:"claims_notice_acceptance_hash"`
	Label                      string `json:"label"`
	NoticeTitle                string `json:"notice_title"`
	NoticeIntro                string `json:"notice_intro"`
	NoticeNotExactMatchIntro   string `json:"notice_not_exact_match_intro"`
	NoticeFooter               string `json:"notice_footer"`
	NoticeFooterURL            string `json:"notice_footer_url"`
	RenderedHTML               string `json:"rendered_html"`
}

func NewClaimsNoticeDataSource() datasource.DataSource {
	return &ClaimsNoticeDataSource{}
}

func (d *ClaimsNoticeDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_claims_notice"
}

func (d *ClaimsNoticeDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Retrieves a TMCH trademark claims notice for a single claims key " +
			"(`POST /v1/domains/claims-notices`). Use the value from " +
			"`data.opusdns_domain_check.results[*].claims_key`. The returned " +
			"`claims_notice_acceptance_hash` is required when registering the matching trademarked " +
			"domain via `opusdns_domain`.",
		Attributes: map[string]schema.Attribute{
			"claims_key": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Claims key (typically sourced from a `data.opusdns_domain_check` result).",
			},
			"label":                         schema.StringAttribute{Computed: true, MarkdownDescription: "Domain label covered by the notice."},
			"claims_notice_acceptance_hash": schema.StringAttribute{Computed: true, MarkdownDescription: "Hash to accept the notice during registration."},
			"notice_title":                  schema.StringAttribute{Computed: true},
			"notice_intro":                  schema.StringAttribute{Computed: true},
			"notice_not_exact_match_intro":  schema.StringAttribute{Computed: true},
			"notice_footer":                 schema.StringAttribute{Computed: true},
			"notice_footer_url":             schema.StringAttribute{Computed: true},
			"rendered_html":                 schema.StringAttribute{Computed: true, MarkdownDescription: "Pre-rendered HTML body of the notice."},
		},
	}
}

func (d *ClaimsNoticeDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ClaimsNoticeDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ClaimsNoticeDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]interface{}{
		"claims_keys": []string{data.ClaimsKey.ValueString()},
	}
	path := d.client.HTTPClient().BuildPath("domains", "claims-notices")
	httpResp, err := d.client.HTTPClient().Post(ctx, path, body)
	if err != nil {
		resp.Diagnostics.AddError("Error retrieving claims notice", formatAPIError(err))
		return
	}

	var out claimsNoticesAPIResponse
	if err := d.client.HTTPClient().DecodeResponse(httpResp, &out); err != nil {
		resp.Diagnostics.AddError("Error decoding claims notice response", formatAPIError(err))
		return
	}
	if len(out.ClaimsNotices) == 0 {
		resp.Diagnostics.AddError(
			"No claims notice returned",
			"The API returned an empty claims_notices list for the supplied key. The key may have expired or be invalid.",
		)
		return
	}

	n := out.ClaimsNotices[0]
	data.ClaimsKey = types.StringValue(n.ClaimsKey)
	data.Label = types.StringValue(n.Label)
	data.ClaimsNoticeAcceptanceHash = types.StringValue(n.ClaimsNoticeAcceptanceHash)
	data.NoticeTitle = types.StringValue(n.NoticeTitle)
	data.NoticeIntro = types.StringValue(n.NoticeIntro)
	data.NoticeNotExactMatchIntro = types.StringValue(n.NoticeNotExactMatchIntro)
	data.NoticeFooter = types.StringValue(n.NoticeFooter)
	data.NoticeFooterURL = types.StringValue(n.NoticeFooterURL)
	data.RenderedHTML = types.StringValue(n.RenderedHTML)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
