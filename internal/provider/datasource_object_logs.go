package provider

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure ObjectLogsDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &ObjectLogsDataSource{}

// ObjectLogsDataSource lists object change logs
// (`GET /v1/archive/object-logs`). When `object_id` is set without other
// filters, callers can use `opusdns_object_log` semantics by reading the
// dedicated `/object-logs/{object_id}` endpoint via the `object_id` filter
// here — the SDK exposes both as the same response shape so we expose a
// single data source.
type ObjectLogsDataSource struct {
	client *opusdns.Client
}

type ObjectLogsDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	Page          types.Int64  `tfsdk:"page"`
	PageSize      types.Int64  `tfsdk:"page_size"`
	SortBy        types.String `tfsdk:"sort_by"`
	SortOrder     types.String `tfsdk:"sort_order"`
	ObjectType    types.String `tfsdk:"object_type"`
	ObjectID      types.String `tfsdk:"object_id"`
	Action        types.String `tfsdk:"action"`
	UserID        types.String `tfsdk:"user_id"`
	CreatedAfter  types.String `tfsdk:"created_after"`
	CreatedBefore types.String `tfsdk:"created_before"`
	Logs          types.List   `tfsdk:"logs"`
	TotalReturned types.Int64  `tfsdk:"total_returned"`
}

var objectLogAttrTypes = map[string]attr.Type{
	"log_id":       types.StringType,
	"object_id":    types.StringType,
	"object_type":  types.StringType,
	"action":       types.StringType,
	"user_id":      types.StringType,
	"ip_address":   types.StringType,
	"created_on":   types.StringType,
	"changes_json": types.StringType,
}

func NewObjectLogsDataSource() datasource.DataSource {
	return &ObjectLogsDataSource{}
}

func (d *ObjectLogsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_object_logs"
}

func (d *ObjectLogsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists object change-log entries (`GET /v1/archive/object-logs`) " +
			"for the caller's organization. Returns a single page. Set `object_id` to scope " +
			"the read to a single object's history. The dynamic `changes` payload is exposed " +
			"as a JSON-encoded string in `changes_json` — decode with `jsondecode()` " +
			"to inspect `before`/`after`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Static identifier for this data source.",
			},
			"page":      schema.Int64Attribute{Optional: true, MarkdownDescription: "1-indexed page number."},
			"page_size": schema.Int64Attribute{Optional: true, MarkdownDescription: "Number of entries per page."},
			"sort_by":   schema.StringAttribute{Optional: true, MarkdownDescription: "Field to sort by (e.g. `created_on`)."},
			"sort_order": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Sort direction: `asc` or `desc`.",
			},
			"object_type":    schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by object type (e.g. `DOMAIN`, `ZONE`, `CONTACT`)."},
			"object_id":      schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by the related object's id."},
			"action":         schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by action name (`create`, `update`, `delete`, ...)."},
			"user_id":        schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by the user who performed the action."},
			"created_after":  schema.StringAttribute{Optional: true, MarkdownDescription: "RFC3339 timestamp; only entries created strictly after this are returned."},
			"created_before": schema.StringAttribute{Optional: true, MarkdownDescription: "RFC3339 timestamp; only entries created strictly before this are returned."},
			"total_returned": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Number of entries on this page (length of `logs`).",
			},
			"logs": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Object log entries matching the filters.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"log_id":       schema.StringAttribute{Computed: true, MarkdownDescription: "Unique log entry identifier."},
						"object_id":    schema.StringAttribute{Computed: true, MarkdownDescription: "ID of the object the log entry concerns."},
						"object_type":  schema.StringAttribute{Computed: true, MarkdownDescription: "Type of object (e.g. `DOMAIN`, `ZONE`)."},
						"action":       schema.StringAttribute{Computed: true, MarkdownDescription: "Action performed (`create`, `update`, `delete`, ...)."},
						"user_id":      schema.StringAttribute{Computed: true, MarkdownDescription: "User who performed the action, or empty if system-generated."},
						"ip_address":   schema.StringAttribute{Computed: true, MarkdownDescription: "IP address the action originated from, or empty if unset."},
						"created_on":   schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp of when the log entry was created."},
						"changes_json": schema.StringAttribute{Computed: true, MarkdownDescription: "JSON-encoded `{before, after}` change payload, or `null` if not an update."},
					},
				},
			},
		},
	}
}

func (d *ObjectLogsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c, ok := configureClientFromDataSourceProviderData(req.ProviderData, resp.Diagnostics.AddError); ok {
		d.client = c
	}
}

func (d *ObjectLogsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ObjectLogsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := &models.ListObjectLogsOptions{}
	if !data.Page.IsNull() && !data.Page.IsUnknown() {
		opts.Page = int(data.Page.ValueInt64())
	}
	if !data.PageSize.IsNull() && !data.PageSize.IsUnknown() {
		opts.PageSize = int(data.PageSize.ValueInt64())
	}
	if v := data.SortBy.ValueString(); v != "" {
		opts.SortBy = v
	}
	if v := data.SortOrder.ValueString(); v != "" {
		opts.SortOrder = models.SortOrder(v)
	}
	if v := data.ObjectType.ValueString(); v != "" {
		opts.ObjectType = models.EventObjectType(v)
	}
	if v := data.ObjectID.ValueString(); v != "" {
		opts.ObjectID = v
	}
	if v := data.Action.ValueString(); v != "" {
		opts.Action = v
	}
	if v := data.UserID.ValueString(); v != "" {
		opts.UserID = models.UserID(v)
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

	result, err := d.client.Events.ListObjectLogs(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error listing object logs", formatAPIError(err))
		return
	}

	objType := types.ObjectType{AttrTypes: objectLogAttrTypes}
	values := make([]attr.Value, len(result.Results))
	for i, e := range result.Results {
		obj, d := objectLogToObject(e)
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
		values[i] = obj
	}
	list, ld := types.ListValue(objType, values)
	resp.Diagnostics.Append(ld...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue("object_logs")
	data.Logs = list
	data.TotalReturned = types.Int64Value(int64(len(result.Results)))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func objectLogToObject(e models.ObjectLog) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	userID := ""
	if e.UserID != nil {
		userID = string(*e.UserID)
	}
	ip := ""
	if e.IPAddress != nil {
		ip = *e.IPAddress
	}
	createdOn := ""
	if e.CreatedOn != nil {
		createdOn = e.CreatedOn.Format("2006-01-02T15:04:05Z07:00")
	}
	changesJSON := "null"
	if e.Changes != nil {
		b, err := json.Marshal(e.Changes)
		if err != nil {
			diags.AddError("Error marshalling object log changes", err.Error())
			return types.ObjectNull(objectLogAttrTypes), diags
		}
		changesJSON = string(b)
	}
	obj, d := types.ObjectValue(objectLogAttrTypes, map[string]attr.Value{
		"log_id":       types.StringValue(string(e.LogID)),
		"object_id":    types.StringValue(e.ObjectID),
		"object_type":  types.StringValue(string(e.ObjectType)),
		"action":       types.StringValue(e.Action),
		"user_id":      types.StringValue(userID),
		"ip_address":   types.StringValue(ip),
		"created_on":   types.StringValue(createdOn),
		"changes_json": types.StringValue(changesJSON),
	})
	diags.Append(d...)
	return obj, diags
}
