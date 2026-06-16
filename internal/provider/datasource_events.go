package provider

import (
	"context"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure EventsDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &EventsDataSource{}

// EventsDataSource lists audit events with optional server-side filtering
// (`GET /v1/events`). It returns a single page only — callers that need
// more pages should narrow the filters or use the API directly. Page size
// defaults to whatever the server uses (currently ~50 at time of writing).
type EventsDataSource struct {
	client *opusdns.Client
}

type EventsDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	Page          types.Int64  `tfsdk:"page"`
	PageSize      types.Int64  `tfsdk:"page_size"`
	Type          types.String `tfsdk:"type"`
	Subtype       types.String `tfsdk:"subtype"`
	ObjectType    types.String `tfsdk:"object_type"`
	ObjectID      types.String `tfsdk:"object_id"`
	Acknowledged  types.Bool   `tfsdk:"acknowledged"`
	CreatedAfter  types.String `tfsdk:"created_after"`
	CreatedBefore types.String `tfsdk:"created_before"`
	Events        types.List   `tfsdk:"events"`
	TotalReturned types.Int64  `tfsdk:"total_returned"`
}

func NewEventsDataSource() datasource.DataSource {
	return &EventsDataSource{}
}

func (d *EventsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_events"
}

func (d *EventsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists audit events for the caller's organization with optional filters " +
			"(`GET /v1/events`). Returns a single page only; use the `page`/`page_size`/filter " +
			"attributes to narrow results. Per-event `event_data` payloads are surfaced as " +
			"JSON-encoded strings — decode with `jsondecode()`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Static identifier for this data source.",
			},
			"page": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "1-indexed page number to fetch.",
			},
			"page_size": schema.Int64Attribute{
				Optional:            true,
				MarkdownDescription: "Number of events per page.",
			},
			"type": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Filter by event type (e.g. `DOMAIN_CREATE`).",
			},
			"subtype": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Filter by event subtype.",
			},
			"object_type": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Filter by related object type (e.g. `DOMAIN`, `ZONE`).",
			},
			"object_id": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Filter by the related object's ID.",
			},
			"acknowledged": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Filter by acknowledgement status.",
			},
			"created_after": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "RFC3339 timestamp; only events created strictly after this are returned.",
			},
			"created_before": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "RFC3339 timestamp; only events created strictly before this are returned.",
			},
			"total_returned": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Number of events returned on this page (length of `events`).",
			},
			"events": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Audit events matching the filters.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: eventAttributeSchema(true),
				},
			},
		},
	}
}

func (d *EventsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c, ok := configureClientFromDataSourceProviderData(req.ProviderData, resp.Diagnostics.AddError); ok {
		d.client = c
	}
}

func (d *EventsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data EventsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := &models.ListEventsOptions{}
	if !data.Page.IsNull() && !data.Page.IsUnknown() {
		opts.Page = int(data.Page.ValueInt64())
	}
	if !data.PageSize.IsNull() && !data.PageSize.IsUnknown() {
		opts.PageSize = int(data.PageSize.ValueInt64())
	}
	if v := data.Type.ValueString(); v != "" {
		opts.Type = models.EventType(v)
	}
	if v := data.Subtype.ValueString(); v != "" {
		opts.Subtype = models.EventSubtype(v)
	}
	if v := data.ObjectType.ValueString(); v != "" {
		opts.ObjectType = models.EventObjectType(v)
	}
	if v := data.ObjectID.ValueString(); v != "" {
		opts.ObjectID = v
	}
	if !data.Acknowledged.IsNull() && !data.Acknowledged.IsUnknown() {
		b := data.Acknowledged.ValueBool()
		opts.Acknowledged = &b
	}
	if v := data.CreatedAfter.ValueString(); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			resp.Diagnostics.AddError("Invalid created_after timestamp", err.Error())
			return
		}
		opts.CreatedAfter = &t
	}
	if v := data.CreatedBefore.ValueString(); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			resp.Diagnostics.AddError("Invalid created_before timestamp", err.Error())
			return
		}
		opts.CreatedBefore = &t
	}

	events, err := d.client.Events.ListEvents(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error listing events", formatAPIError(err))
		return
	}

	objType := types.ObjectType{AttrTypes: eventAttrTypes}
	values := make([]attr.Value, len(events))
	for i, e := range events {
		obj, diags := eventToObject(e)
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

	data.ID = types.StringValue("events")
	data.Events = list
	data.TotalReturned = types.Int64Value(int64(len(events)))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
