package provider

import (
	"context"
	"fmt"

	opusdns "github.com/opusdns/opusdns-go-client/opusdns"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &ZonesDataSource{}

type ZonesDataSource struct {
	client *opusdns.Client
}

type ZonesDataSourceModel struct {
	ID    types.String `tfsdk:"id"`
	Zones types.List   `tfsdk:"zones"`
}

func NewZonesDataSource() datasource.DataSource {
	return &ZonesDataSource{}
}

func (d *ZonesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zones"
}

func (d *ZonesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Lists all DNS zones in OpusDNS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"zones": schema.ListNestedAttribute{
				Computed:    true,
				Description: "List of zones.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Computed:    true,
							Description: "Zone name.",
						},
						"zone_id": schema.StringAttribute{
							Computed:    true,
							Description: "Zone ID.",
						},
					},
				},
			},
		},
	}
}

func (d *ZonesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ZonesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config ZonesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zones, err := d.client.DNS.ListZones(ctx, nil)
	if err != nil {
		resp.Diagnostics.AddError("Failed to list zones", err.Error())
		return
	}

	zoneAttrTypes := map[string]attr.Type{
		"name":    types.StringType,
		"zone_id": types.StringType,
	}

	zoneObjects := make([]attr.Value, 0, len(zones))
	for _, z := range zones {
		obj, diags := types.ObjectValue(zoneAttrTypes, map[string]attr.Value{
			"name":    types.StringValue(z.Name),
			"zone_id": types.StringValue(string(z.ZoneID)),
		})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		zoneObjects = append(zoneObjects, obj)
	}

	zonesList, diags := types.ListValue(types.ObjectType{AttrTypes: zoneAttrTypes}, zoneObjects)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	config.ID = types.StringValue("zones")
	config.Zones = zonesList

	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}
