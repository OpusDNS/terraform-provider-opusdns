package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure DomainsSummaryDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &DomainsSummaryDataSource{}

// DomainsSummaryDataSource exposes aggregate domain counts via
// `GET /v1/domains/summary`. Useful for renewal-window dashboards and
// budgeting alerts without paginating the full domain inventory.
type DomainsSummaryDataSource struct {
	client *opusdns.Client
}

type DomainsSummaryDataSourceModel struct {
	ID                   types.String `tfsdk:"id"`
	TotalDomains         types.Int64  `tfsdk:"total_domains"`
	DomainsByTLD         types.Map    `tfsdk:"domains_by_tld"`
	DomainsByStatus      types.Map    `tfsdk:"domains_by_status"`
	ExpiringWithin30Days types.Int64  `tfsdk:"expiring_within_30_days"`
	ExpiringWithin90Days types.Int64  `tfsdk:"expiring_within_90_days"`
}

func NewDomainsSummaryDataSource() datasource.DataSource {
	return &DomainsSummaryDataSource{}
}

func (d *DomainsSummaryDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domains_summary"
}

func (d *DomainsSummaryDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Returns aggregate counts for all domains owned by the caller's organization " +
			"(`GET /v1/domains/summary`), including counts grouped by TLD and status plus 30/90-day " +
			"expiration windows. Useful for renewal dashboards and budgeting alerts.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Static identifier for this data source.",
			},
			"total_domains": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Total number of domains.",
			},
			"domains_by_tld": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.Int64Type,
				MarkdownDescription: "Map of TLD name (without leading dot) → domain count.",
			},
			"domains_by_status": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.Int64Type,
				MarkdownDescription: "Map of domain status string → domain count.",
			},
			"expiring_within_30_days": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Count of domains expiring within the next 30 days.",
			},
			"expiring_within_90_days": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Count of domains expiring within the next 90 days.",
			},
		},
	}
}

func (d *DomainsSummaryDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DomainsSummaryDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	summary, err := d.client.Domains.GetSummary(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error fetching domains summary", formatAPIError(err))
		return
	}
	if summary == nil {
		resp.Diagnostics.AddError(
			"Empty domains summary response",
			"The /v1/domains/summary endpoint returned an empty result.",
		)
		return
	}

	byTLD := make(map[string]int64, len(summary.DomainsByTLD))
	for k, v := range summary.DomainsByTLD {
		byTLD[k] = int64(v)
	}
	byStatus := make(map[string]int64, len(summary.DomainsByStatus))
	for k, v := range summary.DomainsByStatus {
		byStatus[k] = int64(v)
	}

	byTLDValue, diags := types.MapValueFrom(ctx, types.Int64Type, byTLD)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	byStatusValue, diags := types.MapValueFrom(ctx, types.Int64Type, byStatus)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data := DomainsSummaryDataSourceModel{
		ID:                   types.StringValue("domains_summary"),
		TotalDomains:         types.Int64Value(int64(summary.TotalDomains)),
		DomainsByTLD:         byTLDValue,
		DomainsByStatus:      byStatusValue,
		ExpiringWithin30Days: types.Int64Value(int64(summary.ExpiringWithin30Days)),
		ExpiringWithin90Days: types.Int64Value(int64(summary.ExpiringWithin90Days)),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
