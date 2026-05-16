package provider

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure ParkingsDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &ParkingsDataSource{}

// ParkingsDataSource lists parking entries via `GET /v1/parking`,
// auto-paginated by the raw helper.
type ParkingsDataSource struct {
	client *opusdns.Client
}

// ParkingsDataSourceModel is the state shape for the parking list data source.
type ParkingsDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	Search           types.String `tfsdk:"search"`
	Enabled          types.Bool   `tfsdk:"enabled"`
	ComplianceStatus types.String `tfsdk:"compliance_status"`
	SortBy           types.String `tfsdk:"sort_by"`
	SortOrder        types.String `tfsdk:"sort_order"`
	Parkings         types.List   `tfsdk:"parkings"`
}

var parkingItemAttrTypes = map[string]attr.Type{
	"parking_id":        types.StringType,
	"domain":            types.StringType,
	"enabled":           types.BoolType,
	"compliance_status": types.StringType,
	"content_language":  types.StringType,
	"note":              types.StringType,
	"content_url":       types.StringType,
	"created_on":        types.StringType,
	"updated_on":        types.StringType,
}

// NewParkingsDataSource returns a new ParkingsDataSource.
func NewParkingsDataSource() datasource.DataSource {
	return &ParkingsDataSource{}
}

func (d *ParkingsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_parkings"
}

func (d *ParkingsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists parking entries within the authenticated caller's organization (`GET /v1/parking`). " +
			"Results are auto-paginated; optional filters narrow the server-side query.",
		Attributes: map[string]schema.Attribute{
			"id":     schema.StringAttribute{Computed: true, MarkdownDescription: "Static identifier for this data source."},
			"search": schema.StringAttribute{Optional: true, MarkdownDescription: "Search parking entries by domain name (substring)."},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Restrict results to entries with the given enabled status.",
			},
			"compliance_status": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Filter by compliance status. One of `preparing`, `pending`, `approved`, `disapproved`, `expired`.",
				Validators: []validator.String{
					stringvalidator.OneOf("preparing", "pending", "approved", "disapproved", "expired"),
				},
			},
			"sort_by": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Field to sort by. One of `domain`, `created_on`, `updated_on`. Defaults to `created_on` server-side.",
				Validators: []validator.String{
					stringvalidator.OneOf("domain", "created_on", "updated_on"),
				},
			},
			"sort_order": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Sort direction. One of `asc`, `desc`. Defaults to `desc` server-side.",
				Validators: []validator.String{
					stringvalidator.OneOf("asc", "desc"),
				},
			},
			"parkings": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Matching parking entries.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"parking_id":        schema.StringAttribute{Computed: true},
						"domain":            schema.StringAttribute{Computed: true},
						"enabled":           schema.BoolAttribute{Computed: true},
						"compliance_status": schema.StringAttribute{Computed: true},
						"content_language":  schema.StringAttribute{Computed: true},
						"note":              schema.StringAttribute{Computed: true},
						"content_url":       schema.StringAttribute{Computed: true},
						"created_on":        schema.StringAttribute{Computed: true},
						"updated_on":        schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *ParkingsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ParkingsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ParkingsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	filters := map[string]string{}
	if !data.Search.IsNull() && !data.Search.IsUnknown() {
		filters["search"] = data.Search.ValueString()
	}
	if !data.Enabled.IsNull() && !data.Enabled.IsUnknown() {
		filters["enabled"] = strconv.FormatBool(data.Enabled.ValueBool())
	}
	if !data.ComplianceStatus.IsNull() && !data.ComplianceStatus.IsUnknown() {
		filters["compliance_status"] = data.ComplianceStatus.ValueString()
	}
	if !data.SortBy.IsNull() && !data.SortBy.IsUnknown() {
		filters["sort_by"] = data.SortBy.ValueString()
	}
	if !data.SortOrder.IsNull() && !data.SortOrder.IsUnknown() {
		filters["sort_order"] = data.SortOrder.ValueString()
	}

	parkings, err := rawListParking(ctx, d.client, filters)
	if err != nil {
		resp.Diagnostics.AddError("Error listing parking entries", formatAPIError(err))
		return
	}

	objType := types.ObjectType{AttrTypes: parkingItemAttrTypes}
	values := make([]attr.Value, len(parkings))
	for i := range parkings {
		p := &parkings[i]
		obj, diags := types.ObjectValue(parkingItemAttrTypes, map[string]attr.Value{
			"parking_id":        types.StringValue(p.ParkingID),
			"domain":            types.StringValue(p.Domain),
			"enabled":           types.BoolValue(p.Enabled),
			"compliance_status": stringPtrToValue(p.ComplianceStatus),
			"content_language":  stringPtrToValue(p.ContentLanguage),
			"note":              stringPtrToValue(p.Note),
			"content_url":       stringPtrToValue(p.ContentURL),
			"created_on":        types.StringValue(p.CreatedOn.Format(time.RFC3339)),
			"updated_on":        types.StringValue(p.UpdatedOn.Format(time.RFC3339)),
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

	data.ID = types.StringValue("parkings")
	data.Parkings = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
