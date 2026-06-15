package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure RequestHistoryDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &RequestHistoryDataSource{}

// RequestHistoryDataSource lists API request history entries
// (`GET /v1/archive/request-history`). Returns a single page; callers can
// narrow with method/path/status/duration/actor/time filters.
type RequestHistoryDataSource struct {
	client *opusdns.Client
}

type RequestHistoryDataSourceModel struct {
	ID                   types.String  `tfsdk:"id"`
	Page                 types.Int64   `tfsdk:"page"`
	PageSize             types.Int64   `tfsdk:"page_size"`
	SortBy               types.String  `tfsdk:"sort_by"`
	SortOrder            types.String  `tfsdk:"sort_order"`
	Method               types.String  `tfsdk:"method"`
	Path                 types.String  `tfsdk:"path"`
	StatusCode           types.Int64   `tfsdk:"status_code"`
	MinStatusCode        types.Int64   `tfsdk:"min_status_code"`
	MaxStatusCode        types.Int64   `tfsdk:"max_status_code"`
	MinDuration          types.Float64 `tfsdk:"min_duration_ms"`
	MaxDuration          types.Float64 `tfsdk:"max_duration_ms"`
	ClientIP             types.String  `tfsdk:"client_ip"`
	ServerRequestID      types.String  `tfsdk:"server_request_id"`
	PerformedByType      types.String  `tfsdk:"performed_by_type"`
	PerformedByID        types.String  `tfsdk:"performed_by_id"`
	RequestStartedAfter  types.String  `tfsdk:"request_started_after"`
	RequestStartedBefore types.String  `tfsdk:"request_started_before"`
	Entries              types.List    `tfsdk:"entries"`
	TotalReturned        types.Int64   `tfsdk:"total_returned"`
}

var requestHistoryEntryAttrTypes = map[string]attr.Type{
	"request_id":  types.StringType,
	"method":      types.StringType,
	"path":        types.StringType,
	"status_code": types.Int64Type,
	"duration_ms": types.Int64Type,
	"user_id":     types.StringType,
	"api_key_id":  types.StringType,
	"ip_address":  types.StringType,
	"user_agent":  types.StringType,
	"created_on":  types.StringType,
}

func NewRequestHistoryDataSource() datasource.DataSource {
	return &RequestHistoryDataSource{}
}

func (d *RequestHistoryDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_request_history"
}

func (d *RequestHistoryDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists API request history entries (`GET /v1/archive/request-history`) " +
			"for the caller's organization. Returns a single page; use `page`/`page_size` to walk " +
			"results and the filter attributes to narrow them.",
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
			"method":                 schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by HTTP method (`GET`, `POST`, etc.)."},
			"path":                   schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by exact request path."},
			"status_code":            schema.Int64Attribute{Optional: true, MarkdownDescription: "Filter by exact HTTP status code."},
			"min_status_code":        schema.Int64Attribute{Optional: true, MarkdownDescription: "Filter to status codes ≥ this value."},
			"max_status_code":        schema.Int64Attribute{Optional: true, MarkdownDescription: "Filter to status codes ≤ this value."},
			"min_duration_ms":        schema.Float64Attribute{Optional: true, MarkdownDescription: "Filter to requests whose duration_ms ≥ this value."},
			"max_duration_ms":        schema.Float64Attribute{Optional: true, MarkdownDescription: "Filter to requests whose duration_ms ≤ this value."},
			"client_ip":              schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by client IP address."},
			"server_request_id":      schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by server-assigned request id."},
			"performed_by_type":      schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by actor type (`USER`, `API_KEY`, `SYSTEM`)."},
			"performed_by_id":        schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by actor id."},
			"request_started_after":  schema.StringAttribute{Optional: true, MarkdownDescription: "RFC3339 timestamp; only entries started strictly after this are returned."},
			"request_started_before": schema.StringAttribute{Optional: true, MarkdownDescription: "RFC3339 timestamp; only entries started strictly before this are returned."},
			"total_returned": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Number of entries on this page (length of `entries`).",
			},
			"entries": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Request history entries matching the filters.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"request_id":  schema.StringAttribute{Computed: true, MarkdownDescription: "Unique request identifier."},
						"method":      schema.StringAttribute{Computed: true, MarkdownDescription: "HTTP method."},
						"path":        schema.StringAttribute{Computed: true, MarkdownDescription: "Request path."},
						"status_code": schema.Int64Attribute{Computed: true, MarkdownDescription: "HTTP response status code."},
						"duration_ms": schema.Int64Attribute{Computed: true, MarkdownDescription: "Request duration in milliseconds."},
						"user_id":     schema.StringAttribute{Computed: true, MarkdownDescription: "User who made the request, or empty if not user-attributable."},
						"api_key_id":  schema.StringAttribute{Computed: true, MarkdownDescription: "API key used, or empty if a session token was used."},
						"ip_address":  schema.StringAttribute{Computed: true, MarkdownDescription: "Client IP, or empty if unset."},
						"user_agent":  schema.StringAttribute{Computed: true, MarkdownDescription: "Client user-agent, or empty if unset."},
						"created_on":  schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp of when the request was made."},
					},
				},
			},
		},
	}
}

func (d *RequestHistoryDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if c, ok := configureClientFromDataSourceProviderData(req.ProviderData, resp.Diagnostics.AddError); ok {
		d.client = c
	}
}

func (d *RequestHistoryDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data RequestHistoryDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := &models.ListOptions{}
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
	if v := data.Method.ValueString(); v != "" {
		opts.Method = models.HTTPMethod(v)
	}
	if v := data.Path.ValueString(); v != "" {
		opts.Path = v
	}
	if !data.StatusCode.IsNull() && !data.StatusCode.IsUnknown() {
		sc := int(data.StatusCode.ValueInt64())
		opts.StatusCode = &sc
	}
	if !data.MinStatusCode.IsNull() && !data.MinStatusCode.IsUnknown() {
		sc := int(data.MinStatusCode.ValueInt64())
		opts.MinStatusCode = &sc
	}
	if !data.MaxStatusCode.IsNull() && !data.MaxStatusCode.IsUnknown() {
		sc := int(data.MaxStatusCode.ValueInt64())
		opts.MaxStatusCode = &sc
	}
	if !data.MinDuration.IsNull() && !data.MinDuration.IsUnknown() {
		v := data.MinDuration.ValueFloat64()
		opts.MinDuration = &v
	}
	if !data.MaxDuration.IsNull() && !data.MaxDuration.IsUnknown() {
		v := data.MaxDuration.ValueFloat64()
		opts.MaxDuration = &v
	}
	if v := data.ClientIP.ValueString(); v != "" {
		opts.ClientIP = v
	}
	if v := data.ServerRequestID.ValueString(); v != "" {
		opts.ServerRequestID = v
	}
	if v := data.PerformedByType.ValueString(); v != "" {
		opts.PerformedByType = models.ExecutingEntity(v)
	}
	if v := data.PerformedByID.ValueString(); v != "" {
		opts.PerformedByID = v
	}

	var diags diag.Diagnostics
	if opts.RequestStartedAfter, diags = parseOptionalRFC3339(data.RequestStartedAfter, "request_started_after"); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	if opts.RequestStartedBefore, diags = parseOptionalRFC3339(data.RequestStartedBefore, "request_started_before"); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	result, err := d.client.Events.ListRequestHistory(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error listing request history", formatAPIError(err))
		return
	}

	objType := types.ObjectType{AttrTypes: requestHistoryEntryAttrTypes}
	values := make([]attr.Value, len(result.Results))
	for i, e := range result.Results {
		obj, d := requestHistoryEntryToObject(e)
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

	data.ID = types.StringValue("request_history")
	data.Entries = list
	data.TotalReturned = types.Int64Value(int64(len(result.Results)))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func requestHistoryEntryToObject(e models.RequestHistoryEntry) (types.Object, diag.Diagnostics) {
	userID := ""
	if e.UserID != nil {
		userID = string(*e.UserID)
	}
	apiKeyID := ""
	if e.APIKeyID != nil {
		apiKeyID = string(*e.APIKeyID)
	}
	ip := ""
	if e.IPAddress != nil {
		ip = *e.IPAddress
	}
	ua := ""
	if e.UserAgent != nil {
		ua = *e.UserAgent
	}
	createdOn := ""
	if e.CreatedOn != nil {
		createdOn = e.CreatedOn.Format("2006-01-02T15:04:05Z07:00")
	}
	return types.ObjectValue(requestHistoryEntryAttrTypes, map[string]attr.Value{
		"request_id":  types.StringValue(string(e.RequestID)),
		"method":      types.StringValue(e.Method),
		"path":        types.StringValue(e.Path),
		"status_code": types.Int64Value(int64(e.StatusCode)),
		"duration_ms": types.Int64Value(int64(e.Duration)),
		"user_id":     types.StringValue(userID),
		"api_key_id":  types.StringValue(apiKeyID),
		"ip_address":  types.StringValue(ip),
		"user_agent":  types.StringValue(ua),
		"created_on":  types.StringValue(createdOn),
	})
}
