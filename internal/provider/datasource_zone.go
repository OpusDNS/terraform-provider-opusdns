package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure ZoneDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &ZoneDataSource{}

// ZoneDataSource defines the data source implementation.
type ZoneDataSource struct {
	client *opusdns.Client
}

// ZoneDataSourceModel describes the data source data model.
type ZoneDataSourceModel struct {
	ID           fqdnValue    `tfsdk:"id"`
	Name         fqdnValue    `tfsdk:"name"`
	DNSSECStatus types.String `tfsdk:"dnssec_status"`
}

// NewZoneDataSource returns a new ZoneDataSource.
func NewZoneDataSource() datasource.DataSource {
	return &ZoneDataSource{}
}

func (d *ZoneDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zone"
}

func (d *ZoneDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches information about an existing DNS zone in OpusDNS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				CustomType:          fqdnType{},
				Computed:            true,
				MarkdownDescription: "The zone name (used as the unique identifier).",
			},
			"name": schema.StringAttribute{
				CustomType:          fqdnType{},
				Required:            true,
				MarkdownDescription: "The domain name of the DNS zone (e.g., `example.com`).",
			},
			"dnssec_status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The DNSSEC status of the zone (`enabled` or `disabled`).",
			},
		},
	}
}

func (d *ZoneDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ZoneDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ZoneDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zone, err := d.client.DNS.GetZone(ctx, data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading DNS zone", formatAPIError(err))
		return
	}

	// fqdnType semantic equality keeps the user-supplied form in state even
	// when the API returns the FQDN with a trailing dot.
	data.ID = fqdnValue{StringValue: types.StringValue(zone.Name)}
	data.Name = fqdnValue{StringValue: types.StringValue(zone.Name)}
	data.DNSSECStatus = normalizedDNSSECStatus(string(zone.DNSSECStatus))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
