package provider

import (
	"context"
	"fmt"

	opusdns "github.com/opusdns/opusdns-go-client/opusdns"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &ZoneDataSource{}

type ZoneDataSource struct {
	client *opusdns.Client
}

type ZoneDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	ZoneID       types.String `tfsdk:"zone_id"`
	Name         types.String `tfsdk:"name"`
	DNSSECStatus types.String `tfsdk:"dnssec_status"`
	CreatedOn    types.String `tfsdk:"created_on"`
	UpdatedOn    types.String `tfsdk:"updated_on"`
}

func NewZoneDataSource() datasource.DataSource {
	return &ZoneDataSource{}
}

func (d *ZoneDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zone"
}

func (d *ZoneDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Fetches information about a DNS zone in OpusDNS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"zone_id": schema.StringAttribute{
				Computed:    true,
				Description: "The unique identifier for the zone.",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The zone name.",
			},
			"dnssec_status": schema.StringAttribute{
				Computed:    true,
				Description: "The DNSSEC status of the zone.",
			},
			"created_on": schema.StringAttribute{
				Computed:    true,
				Description: "When the zone was created.",
			},
			"updated_on": schema.StringAttribute{
				Computed:    true,
				Description: "When the zone was last updated.",
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
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *opusdns.Client, got %T", req.ProviderData))
		return
	}
	d.client = client
}

func (d *ZoneDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config ZoneDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zone, err := d.client.DNS.GetZone(ctx, config.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read zone", err.Error())
		return
	}

	config.ID = types.StringValue(zone.Name)
	config.ZoneID = types.StringValue(string(zone.ZoneID))
	config.Name = types.StringValue(zone.Name)
	config.DNSSECStatus = types.StringValue(string(zone.DNSSECStatus))

	if zone.CreatedOn != nil {
		config.CreatedOn = types.StringValue(zone.CreatedOn.String())
	} else {
		config.CreatedOn = types.StringValue("")
	}

	if zone.UpdatedOn != nil {
		config.UpdatedOn = types.StringValue(zone.UpdatedOn.String())
	} else {
		config.UpdatedOn = types.StringValue("")
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
