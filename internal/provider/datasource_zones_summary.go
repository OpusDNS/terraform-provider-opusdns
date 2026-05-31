package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure ZonesSummaryDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &ZonesSummaryDataSource{}

// ZonesSummaryDataSource exposes aggregate zone counts via
// `GET /v1/dns/summary`. Use this to drive dashboards, capacity alerts, or
// conditional logic without paginating the full zone list.
type ZonesSummaryDataSource struct {
	client *opusdns.Client
}

type ZonesSummaryDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	TotalZones    types.Int64  `tfsdk:"total_zones"`
	ZonesByDNSSEC types.Map    `tfsdk:"zones_by_dnssec"`
}

func NewZonesSummaryDataSource() datasource.DataSource {
	return &ZonesSummaryDataSource{}
}

func (d *ZonesSummaryDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zones_summary"
}

func (d *ZonesSummaryDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Returns aggregate counts for all DNS zones owned by the caller's organization " +
			"(`GET /v1/dns/summary`). Useful for dashboards or conditional plan logic without paginating " +
			"the full `opusdns_zones` list.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Static identifier for this data source.",
			},
			"total_zones": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Total number of DNS zones.",
			},
			"zones_by_dnssec": schema.MapAttribute{
				Computed:            true,
				ElementType:         types.Int64Type,
				MarkdownDescription: "Map of DNSSEC status (e.g. `enabled`, `disabled`) → zone count.",
			},
		},
	}
}

func (d *ZonesSummaryDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ZonesSummaryDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	summary, err := d.client.DNS.GetSummary(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Error fetching zones summary", formatAPIError(err))
		return
	}
	if summary == nil {
		resp.Diagnostics.AddError(
			"Empty zones summary response",
			"The /v1/dns/summary endpoint returned an empty result.",
		)
		return
	}

	// Convert map[DNSSECStatus]int → map[string]int with stable iteration for
	// deterministic state output. Terraform's Map value is itself unordered,
	// but sorting keeps the source-of-truth slice deterministic if a future
	// refactor switches representations.
	byDNSSEC := make(map[string]int64, len(summary.ZonesByDNSSEC))
	keys := make([]string, 0, len(summary.ZonesByDNSSEC))
	for k := range summary.ZonesByDNSSEC {
		keys = append(keys, string(k))
	}
	sort.Strings(keys)
	for _, k := range keys {
		// Re-look up via the original (typed) key to read the value.
		for kk, v := range summary.ZonesByDNSSEC {
			if string(kk) == k {
				byDNSSEC[k] = int64(v)
				break
			}
		}
	}
	mapValue, diags := types.MapValueFrom(ctx, types.Int64Type, byDNSSEC)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data := ZonesSummaryDataSourceModel{
		ID:            types.StringValue("zones_summary"),
		TotalZones:    types.Int64Value(int64(summary.TotalZones)),
		ZonesByDNSSEC: mapValue,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
