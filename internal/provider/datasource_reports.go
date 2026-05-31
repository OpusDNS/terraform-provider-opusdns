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

var _ datasource.DataSource = &ReportsDataSource{}

type ReportsDataSource struct {
	client *opusdns.Client
}

type ReportsDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	ReportTypes   types.List   `tfsdk:"report_types"`
	Statuses      types.List   `tfsdk:"statuses"`
	TriggerType   types.String `tfsdk:"trigger_type"`
	CreatedAfter  types.String `tfsdk:"created_after"`
	CreatedBefore types.String `tfsdk:"created_before"`
	Reports       types.List   `tfsdk:"reports"`
}

var reportItemAttrTypes = map[string]attr.Type{
	"report_id":       types.StringType,
	"organization_id": types.StringType,
	"report_type":     types.StringType,
	"status":          types.StringType,
	"trigger_type":    types.StringType,
	"file_size_bytes": types.Int64Type,
	"record_count":    types.Int64Type,
	"generated_on":    types.StringType,
	"created_on":      types.StringType,
	"updated_on":      types.StringType,
}

func NewReportsDataSource() datasource.DataSource {
	return &ReportsDataSource{}
}

func (d *ReportsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_reports"
}

func (d *ReportsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists generated OpusDNS reports with optional server-side filters.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, MarkdownDescription: "Static identifier for this data source."},
			"report_types": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Filter by report type. Multiple values match any.",
			},
			"statuses": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Filter by report status. Multiple values match any.",
			},
			"trigger_type":   schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by trigger type, such as `on_demand` or `scheduled`."},
			"created_after":  schema.StringAttribute{Optional: true, MarkdownDescription: "Filter reports created after this RFC3339 timestamp."},
			"created_before": schema.StringAttribute{Optional: true, MarkdownDescription: "Filter reports created before this RFC3339 timestamp."},
			"reports": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Matching reports.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"report_id":       schema.StringAttribute{Computed: true},
					"organization_id": schema.StringAttribute{Computed: true},
					"report_type":     schema.StringAttribute{Computed: true},
					"status":          schema.StringAttribute{Computed: true},
					"trigger_type":    schema.StringAttribute{Computed: true},
					"file_size_bytes": schema.Int64Attribute{Computed: true},
					"record_count":    schema.Int64Attribute{Computed: true},
					"generated_on":    schema.StringAttribute{Computed: true},
					"created_on":      schema.StringAttribute{Computed: true},
					"updated_on":      schema.StringAttribute{Computed: true},
				}},
			},
		},
	}
}

func (d *ReportsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*opusdns.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Data Source Configure Type", fmt.Sprintf("Expected *opusdns.Client, got: %T.", req.ProviderData))
		return
	}
	d.client = client
}

func (d *ReportsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ReportsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := &models.ListReportsOptions{}
	if !data.ReportTypes.IsNull() && !data.ReportTypes.IsUnknown() {
		raw, diags := stringListValueToStrings(ctx, data.ReportTypes)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		opts.ReportType = make([]models.ReportType, 0, len(raw))
		for _, reportType := range raw {
			opts.ReportType = append(opts.ReportType, models.ReportType(reportType))
		}
	}
	if !data.Statuses.IsNull() && !data.Statuses.IsUnknown() {
		raw, diags := stringListValueToStrings(ctx, data.Statuses)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		opts.Status = make([]models.ReportStatus, 0, len(raw))
		for _, status := range raw {
			opts.Status = append(opts.Status, models.ReportStatus(status))
		}
	}
	if !data.TriggerType.IsNull() && !data.TriggerType.IsUnknown() {
		opts.TriggerType = models.ReportTriggerType(data.TriggerType.ValueString())
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

	reports, err := d.client.Reports.ListReports(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error listing reports", formatAPIError(err))
		return
	}

	objType := types.ObjectType{AttrTypes: reportItemAttrTypes}
	values := make([]attr.Value, len(reports))
	for i := range reports {
		r := &reports[i]
		obj, diags := types.ObjectValue(reportItemAttrTypes, map[string]attr.Value{
			"report_id":       types.StringValue(string(r.ReportID)),
			"organization_id": types.StringValue(string(r.OrganizationID)),
			"report_type":     types.StringValue(string(r.ReportType)),
			"status":          types.StringValue(string(r.Status)),
			"trigger_type":    types.StringValue(string(r.TriggerType)),
			"file_size_bytes": intPtrToValue(r.FileSizeBytes),
			"record_count":    intPtrToValue(r.RecordCount),
			"generated_on":    timePtrToValue(r.GeneratedOn),
			"created_on":      timePtrToValue(r.CreatedOn),
			"updated_on":      timePtrToValue(r.UpdatedOn),
		})
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

	data.ID = types.StringValue("reports")
	data.Reports = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
