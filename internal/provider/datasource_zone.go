package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
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
	ZoneID       types.String `tfsdk:"zone_id"`
	Name         fqdnValue    `tfsdk:"name"`
	DNSSECStatus types.String `tfsdk:"dnssec_status"`
	IncludeTags  types.Bool   `tfsdk:"include_tags"`
	Tags         types.List   `tfsdk:"tags"`
	CreatedOn    types.String `tfsdk:"created_on"`
	UpdatedOn    types.String `tfsdk:"updated_on"`
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
			"zone_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Server-assigned DNS zone id (`dns_zone_id`).",
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
			"include_tags": schema.BoolAttribute{Optional: true, MarkdownDescription: "When true, request `include=tags` and populate `tags` in state."},
			"tags": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Tags assigned to the zone when `include_tags` is true.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"tag_id": schema.StringAttribute{Computed: true},
					"label":  schema.StringAttribute{Computed: true},
					"color":  schema.StringAttribute{Computed: true},
				}},
			},
			"created_on": schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp when the zone was created."},
			"updated_on": schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp when the zone was last updated."},
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

	var zone *models.Zone
	var err error
	if !data.IncludeTags.IsNull() && !data.IncludeTags.IsUnknown() && data.IncludeTags.ValueBool() {
		zone, err = d.client.DNS.GetZoneWithOptions(ctx, data.Name.ValueString(), &models.GetZoneOptions{Include: []models.ZoneIncludeField{models.ZoneIncludeTags}})
	} else {
		zone, err = d.client.DNS.GetZone(ctx, data.Name.ValueString())
	}
	if err != nil {
		resp.Diagnostics.AddError("Error reading DNS zone", formatAPIError(err))
		return
	}

	resolvedName := canonicalZoneName(zone.Name)
	data.ID = fqdnValue{StringValue: types.StringValue(resolvedName)}
	data.ZoneID = types.StringValue(string(zone.ZoneID))
	data.Name = fqdnValue{StringValue: types.StringValue(resolvedName)}
	data.DNSSECStatus = normalizedDNSSECStatus(string(zone.DNSSECStatus))
	tagList, diags := tagEnrichedListValue(zone.Tags)
	resp.Diagnostics.Append(diags...)
	data.Tags = tagList
	data.CreatedOn = timePtrToValue(zone.CreatedOn)
	data.UpdatedOn = timePtrToValue(zone.UpdatedOn)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
