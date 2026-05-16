package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure ParkingDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &ParkingDataSource{}
var _ datasource.DataSourceWithConfigValidators = &ParkingDataSource{}

// ParkingDataSource reads a single parking entry by `parking_id` or `domain`
// (`GET /v1/parking/{parking_reference}`).
//
// Exactly one of `parking_id` or `domain` must be set.
type ParkingDataSource struct {
	client *opusdns.Client
}

// ParkingDataSourceModel is the state shape for the singular parking data source.
type ParkingDataSourceModel struct {
	ID               types.String `tfsdk:"id"`
	ParkingID        types.String `tfsdk:"parking_id"`
	Domain           types.String `tfsdk:"domain"`
	Enabled          types.Bool   `tfsdk:"enabled"`
	ComplianceStatus types.String `tfsdk:"compliance_status"`
	ContentLanguage  types.String `tfsdk:"content_language"`
	Note             types.String `tfsdk:"note"`
	ContentURL       types.String `tfsdk:"content_url"`
	CreatedOn        types.String `tfsdk:"created_on"`
	UpdatedOn        types.String `tfsdk:"updated_on"`
}

// NewParkingDataSource returns a new ParkingDataSource.
func NewParkingDataSource() datasource.DataSource {
	return &ParkingDataSource{}
}

func (d *ParkingDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_parking"
}

func (d *ParkingDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a single OpusDNS parking entry (`GET /v1/parking/{parking_reference}`). " +
			"Exactly one of `parking_id` or `domain` must be supplied.",
		Attributes: map[string]schema.Attribute{
			"id":                schema.StringAttribute{Computed: true, MarkdownDescription: "Mirrors `parking_id`."},
			"parking_id":        schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Parking entry id (e.g. `parking_01j...`). Mutually exclusive with `domain`."},
			"domain":            schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Domain name of the parking entry. Mutually exclusive with `parking_id`."},
			"enabled":           schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether parking is enabled for this domain."},
			"compliance_status": schema.StringAttribute{Computed: true, MarkdownDescription: "Compliance status (`preparing`/`pending`/`approved`/`disapproved`/`expired` or null)."},
			"content_language":  schema.StringAttribute{Computed: true},
			"note":              schema.StringAttribute{Computed: true},
			"content_url":       schema.StringAttribute{Computed: true},
			"created_on":        schema.StringAttribute{Computed: true},
			"updated_on":        schema.StringAttribute{Computed: true},
		},
	}
}

func (d *ParkingDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(path.MatchRoot("parking_id"), path.MatchRoot("domain")),
	}
}

func (d *ParkingDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ParkingDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ParkingDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var ref string
	switch {
	case !data.ParkingID.IsNull() && !data.ParkingID.IsUnknown() && data.ParkingID.ValueString() != "":
		ref = data.ParkingID.ValueString()
	case !data.Domain.IsNull() && !data.Domain.IsUnknown() && data.Domain.ValueString() != "":
		ref = data.Domain.ValueString()
	default:
		resp.Diagnostics.AddError("Invalid parking data source config", "Either `parking_id` or `domain` must be set.")
		return
	}

	parking, err := rawGetParking(ctx, d.client, ref)
	if err != nil {
		resp.Diagnostics.AddError("Error reading parking entry", formatAPIError(err))
		return
	}

	// Reuse the resource model populator via a thin adapter: copy fields directly.
	var rm ParkingResourceModel
	applyParkingToModel(&rm, parking, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	data.ID = rm.ID
	data.ParkingID = rm.ParkingID
	data.Domain = rm.Domain
	data.Enabled = rm.Enabled
	data.ComplianceStatus = rm.ComplianceStatus
	data.ContentLanguage = rm.ContentLanguage
	data.Note = rm.Note
	data.ContentURL = rm.ContentURL
	data.CreatedOn = rm.CreatedOn
	data.UpdatedOn = rm.UpdatedOn

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
