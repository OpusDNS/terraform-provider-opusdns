package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
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
	ID            types.String `tfsdk:"id"`
	Search        types.String `tfsdk:"search"`
	Name          types.String `tfsdk:"name"`
	Suffix        types.String `tfsdk:"suffix"`
	DNSSECStatus  types.String `tfsdk:"dnssec_status"`
	TagIDs        types.List   `tfsdk:"tag_ids"`
	TagMode       types.String `tfsdk:"tag_mode"`
	IncludeTags   types.Bool   `tfsdk:"include_tags"`
	CreatedAfter  types.String `tfsdk:"created_after"`
	CreatedBefore types.String `tfsdk:"created_before"`
	UpdatedAfter  types.String `tfsdk:"updated_after"`
	UpdatedBefore types.String `tfsdk:"updated_before"`
	Zones         types.List   `tfsdk:"zones"`
}

// ZoneItemModel describes a single zone in the list.
type ZoneItemModel struct {
	ZoneID                types.String `tfsdk:"zone_id"`
	Name                  types.String `tfsdk:"name"`
	DNSSECStatus          types.String `tfsdk:"dnssec_status"`
	VanityNameserverSetID types.String `tfsdk:"vanity_nameserver_set_id"`
	Tags                  types.List   `tfsdk:"tags"`
	CreatedOn             types.String `tfsdk:"created_on"`
	UpdatedOn             types.String `tfsdk:"updated_on"`
}

var zoneItemAttrTypes = map[string]attr.Type{
	"zone_id":                  types.StringType,
	"name":                     types.StringType,
	"dnssec_status":            types.StringType,
	"vanity_nameserver_set_id": types.StringType,
	"tags":                     types.ListType{ElemType: types.ObjectType{AttrTypes: tagEnrichedAttrTypes}},
	"created_on":               types.StringType,
	"updated_on":               types.StringType,
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
			"search":        schema.StringAttribute{Optional: true, MarkdownDescription: "Free-text search over zone names."},
			"name":          schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by exact zone name."},
			"suffix":        schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by domain suffix, such as `.com`."},
			"dnssec_status": schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by DNSSEC status (`enabled` or `disabled`)."},
			"tag_ids": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Filter by tag IDs. Multiple values are sent as repeated `tag_ids` query parameters.",
			},
			"tag_mode":       schema.StringAttribute{Optional: true, MarkdownDescription: "Tag filter mode. Use `match_any` or `match_all` according to the API."},
			"include_tags":   schema.BoolAttribute{Optional: true, MarkdownDescription: "When true, request `include=tags` and populate the computed `tags` field for each zone."},
			"created_after":  schema.StringAttribute{Optional: true, MarkdownDescription: "Filter zones created after this RFC3339 timestamp."},
			"created_before": schema.StringAttribute{Optional: true, MarkdownDescription: "Filter zones created before this RFC3339 timestamp."},
			"updated_after":  schema.StringAttribute{Optional: true, MarkdownDescription: "Filter zones updated after this RFC3339 timestamp."},
			"updated_before": schema.StringAttribute{Optional: true, MarkdownDescription: "Filter zones updated before this RFC3339 timestamp."},
			"zones": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The list of DNS zones.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"zone_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Server-assigned DNS zone id (`dns_zone_id`).",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The domain name of the zone.",
						},
						"dnssec_status": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The DNSSEC status of the zone.",
						},
						"vanity_nameserver_set_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The id of the vanity nameserver set (`vns_...`) branding this zone, or null when the zone uses OpusDNS system defaults.",
						},
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

	opts := &models.ListZonesOptions{
		Search: stringValue(data.Search),
		Name:   stringValue(data.Name),
		Suffix: stringValue(data.Suffix),
	}
	if !data.DNSSECStatus.IsNull() && !data.DNSSECStatus.IsUnknown() {
		opts.DNSSECStatus = models.DNSSECStatus(data.DNSSECStatus.ValueString())
	}
	if !data.TagIDs.IsNull() && !data.TagIDs.IsUnknown() {
		raw, diags := stringListValueToStrings(ctx, data.TagIDs)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		opts.TagIDs = make([]models.TagID, 0, len(raw))
		for _, id := range raw {
			opts.TagIDs = append(opts.TagIDs, models.TagID(id))
		}
	}
	if !data.TagMode.IsNull() && !data.TagMode.IsUnknown() {
		opts.TagMode = models.TagFilterMode(data.TagMode.ValueString())
	}
	if !data.IncludeTags.IsNull() && !data.IncludeTags.IsUnknown() && data.IncludeTags.ValueBool() {
		opts.Include = append(opts.Include, models.ZoneIncludeTags)
	}
	var diags diag.Diagnostics
	if opts.CreatedAfter, diags = parseOptionalRFC3339(data.CreatedAfter, "created_after"); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	if opts.CreatedBefore, diags = parseOptionalRFC3339(data.CreatedBefore, "created_before"); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	if opts.UpdatedAfter, diags = parseOptionalRFC3339(data.UpdatedAfter, "updated_after"); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	if opts.UpdatedBefore, diags = parseOptionalRFC3339(data.UpdatedBefore, "updated_before"); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	zones, err := d.client.DNS.ListZones(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error listing DNS zones", formatAPIError(err))
		return
	}

	zoneObjType := types.ObjectType{AttrTypes: zoneItemAttrTypes}
	zoneValues := make([]attr.Value, len(zones))
	for i, z := range zones {
		tagList, td := tagEnrichedListValue(z.Tags)
		resp.Diagnostics.Append(td...)
		if resp.Diagnostics.HasError() {
			return
		}
		obj, diags := types.ObjectValue(zoneItemAttrTypes, map[string]attr.Value{
			"zone_id":                  types.StringValue(string(z.ZoneID)),
			"name":                     types.StringValue(canonicalZoneName(z.Name)),
			"dnssec_status":            normalizedDNSSECStatus(string(z.DNSSECStatus)),
			"vanity_nameserver_set_id": vanityNameserverSetIDToValue(z.VanityNameserverSetID),
			"tags":                     tagList,
			"created_on":               timePtrToValue(z.CreatedOn),
			"updated_on":               timePtrToValue(z.UpdatedOn),
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
