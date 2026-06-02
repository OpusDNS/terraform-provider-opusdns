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

var _ datasource.DataSource = &ReportDataSource{}

type ReportDataSource struct {
	client *opusdns.Client
}

type ReportDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	ReportID       types.String `tfsdk:"report_id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	ReportType     types.String `tfsdk:"report_type"`
	Status         types.String `tfsdk:"status"`
	TriggerType    types.String `tfsdk:"trigger_type"`
	FileSizeBytes  types.Int64  `tfsdk:"file_size_bytes"`
	RecordCount    types.Int64  `tfsdk:"record_count"`
	GeneratedOn    types.String `tfsdk:"generated_on"`
	CreatedOn      types.String `tfsdk:"created_on"`
	UpdatedOn      types.String `tfsdk:"updated_on"`
}

func NewReportDataSource() datasource.DataSource {
	return &ReportDataSource{}
}

func (d *ReportDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_report"
}

func (d *ReportDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a generated OpusDNS report by id.",
		Attributes: map[string]schema.Attribute{
			"id":              schema.StringAttribute{Computed: true, MarkdownDescription: "Mirror of `report_id`."},
			"report_id":       schema.StringAttribute{Required: true, MarkdownDescription: "Report id (`report_...`)."},
			"organization_id": schema.StringAttribute{Computed: true},
			"report_type":     schema.StringAttribute{Computed: true},
			"status":          schema.StringAttribute{Computed: true},
			"trigger_type":    schema.StringAttribute{Computed: true},
			"file_size_bytes": schema.Int64Attribute{Computed: true},
			"record_count":    schema.Int64Attribute{Computed: true},
			"generated_on":    schema.StringAttribute{Computed: true},
			"created_on":      schema.StringAttribute{Computed: true},
			"updated_on":      schema.StringAttribute{Computed: true},
		},
	}
}

func (d *ReportDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ReportDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ReportDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	report, err := d.client.Reports.GetReport(ctx, models.ReportID(data.ReportID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Error reading report", formatAPIError(err))
		return
	}
	populateReportDataSourceModel(&data, report)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func populateReportDataSourceModel(data *ReportDataSourceModel, r *models.Report) {
	data.ID = types.StringValue(string(r.ReportID))
	data.ReportID = types.StringValue(string(r.ReportID))
	data.OrganizationID = types.StringValue(string(r.OrganizationID))
	data.ReportType = types.StringValue(string(r.ReportType))
	data.Status = types.StringValue(string(r.Status))
	data.TriggerType = types.StringValue(string(r.TriggerType))
	data.FileSizeBytes = intPtrToValue(r.FileSizeBytes)
	data.RecordCount = intPtrToValue(r.RecordCount)
	data.GeneratedOn = timePtrToValue(r.GeneratedOn)
	data.CreatedOn = timePtrToValue(r.CreatedOn)
	data.UpdatedOn = timePtrToValue(r.UpdatedOn)
}
