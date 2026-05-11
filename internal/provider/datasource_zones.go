package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure ZonesDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &ZonesDataSource{}

// ZonesDataSource defines the data source implementation.
type ZonesDataSource struct {
	client *opusdns.Client
}

// ZonesDataSourceModel describes the data source data model.
type ZonesDataSourceModel struct {
	ID    types.String `tfsdk:"id"`
	Zones types.List   `tfsdk:"zones"`
}

// ZoneItemModel describes a single zone in the list.
type ZoneItemModel struct {
	Name         types.String `tfsdk:"name"`
	DNSSECStatus types.String `tfsdk:"dnssec_status"`
}

var zoneItemAttrTypes = map[string]attr.Type{
	"name":          types.StringType,
	"dnssec_status": types.StringType,
}

// NewZonesDataSource returns a new ZonesDataSource.
func NewZonesDataSource() datasource.DataSource {
	return &ZonesDataSource{}
}

func (d *ZonesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zones"
}

func (d *ZonesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches the list of all DNS zones in your OpusDNS account.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "A static identifier for this data source.",
			},
			"zones": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The list of DNS zones.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The domain name of the zone.",
						},
						"dnssec_status": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The DNSSEC status of the zone.",
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
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *opusdns.Client, got: %T.", req.ProviderData),
		)
		return
	}
	d.client = client
}

func (d *ZonesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ZonesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zones, err := d.client.DNS.ListZones(ctx, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error listing DNS zones", err.Error())
		return
	}

	zoneObjType := types.ObjectType{AttrTypes: zoneItemAttrTypes}
	zoneValues := make([]attr.Value, len(zones))
	for i, z := range zones {
		obj, diags := types.ObjectValue(zoneItemAttrTypes, map[string]attr.Value{
			"name":          types.StringValue(z.Name),
			"dnssec_status": types.StringValue(string(z.DNSSECStatus)),
		})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		zoneValues[i] = obj
	}

	zoneList, diags := types.ListValue(zoneObjType, zoneValues)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue("zones")
	data.Zones = zoneList

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
