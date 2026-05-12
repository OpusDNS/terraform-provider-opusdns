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

// Ensure OrganizationDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &OrganizationDataSource{}

// OrganizationDataSource fetches a single organization by id, or — when
// `me = true` — the authenticated caller's organization (discovered via the
// current user record).
type OrganizationDataSource struct {
	client *opusdns.Client
}

// OrganizationDataSourceModel mirrors OrganizationResourceModel but adds a
// `me` toggle and makes `organization_id` optional.
type OrganizationDataSourceModel struct {
	ID                   types.String `tfsdk:"id"`
	OrganizationID       types.String `tfsdk:"organization_id"`
	Me                   types.Bool   `tfsdk:"me"`
	Name                 types.String `tfsdk:"name"`
	Address1             types.String `tfsdk:"address_1"`
	Address2             types.String `tfsdk:"address_2"`
	City                 types.String `tfsdk:"city"`
	State                types.String `tfsdk:"state"`
	PostalCode           types.String `tfsdk:"postal_code"`
	CountryCode          types.String `tfsdk:"country_code"`
	BusinessNumber       types.String `tfsdk:"business_number"`
	TaxID                types.String `tfsdk:"tax_id"`
	TaxIDType            types.String `tfsdk:"tax_id_type"`
	Currency             types.String `tfsdk:"currency"`
	DefaultLocale        types.String `tfsdk:"default_locale"`
	ParentOrganizationID types.String `tfsdk:"parent_organization_id"`
	Status               types.String `tfsdk:"status"`
	TaxRate              types.String `tfsdk:"tax_rate"`
	CreatedOn            types.String `tfsdk:"created_on"`
}

// NewOrganizationDataSource returns a new OrganizationDataSource.
func NewOrganizationDataSource() datasource.DataSource {
	return &OrganizationDataSource{}
}

func (d *OrganizationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization"
}

func (d *OrganizationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single organization from `/v1/organizations/{id}`. " +
			"Set `me = true` to look up the authenticated caller's own organization (resolved via `/v1/users/me`).",
		Attributes: map[string]schema.Attribute{
			"id":              schema.StringAttribute{Computed: true, MarkdownDescription: "Mirror of `organization_id`."},
			"organization_id": schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Organization id to look up. Required unless `me = true`."},
			"me":              schema.BoolAttribute{Optional: true, MarkdownDescription: "When true, resolve the authenticated user's organization id via `/v1/users/me` instead of using `organization_id`."},

			"name":                   schema.StringAttribute{Computed: true, MarkdownDescription: "Organization name."},
			"address_1":              schema.StringAttribute{Computed: true},
			"address_2":              schema.StringAttribute{Computed: true},
			"city":                   schema.StringAttribute{Computed: true},
			"state":                  schema.StringAttribute{Computed: true},
			"postal_code":            schema.StringAttribute{Computed: true},
			"country_code":           schema.StringAttribute{Computed: true},
			"business_number":        schema.StringAttribute{Computed: true},
			"tax_id":                 schema.StringAttribute{Computed: true},
			"tax_id_type":            schema.StringAttribute{Computed: true},
			"currency":               schema.StringAttribute{Computed: true},
			"default_locale":         schema.StringAttribute{Computed: true},
			"parent_organization_id": schema.StringAttribute{Computed: true},
			"status":                 schema.StringAttribute{Computed: true},
			"tax_rate":               schema.StringAttribute{Computed: true},
			"created_on":             schema.StringAttribute{Computed: true},
		},
	}
}

func (d *OrganizationDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *OrganizationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OrganizationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var orgID models.OrganizationID
	switch {
	case data.Me.ValueBool():
		user, err := d.client.Users.GetCurrentUser(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Error resolving current user", err.Error())
			return
		}
		orgID = user.OrganizationID
	case !data.OrganizationID.IsNull() && !data.OrganizationID.IsUnknown() && data.OrganizationID.ValueString() != "":
		orgID = models.OrganizationID(data.OrganizationID.ValueString())
	default:
		resp.Diagnostics.AddError(
			"Missing organization selector",
			"Either set `organization_id` or `me = true`.",
		)
		return
	}

	org, err := d.client.Organizations.GetOrganization(ctx, orgID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading organization", err.Error())
		return
	}

	populateOrganizationDataModel(&data, org)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// populateOrganizationDataModel mirrors populateOrganizationModel for the data
// source model (which carries the extra `me` field).
func populateOrganizationDataModel(data *OrganizationDataSourceModel, org *models.Organization) {
	data.ID = types.StringValue(string(org.OrganizationID))
	data.OrganizationID = types.StringValue(string(org.OrganizationID))
	data.Name = types.StringValue(org.Name)
	data.Status = types.StringValue(string(org.Status))

	data.Address1 = stringPtrToValue(org.Address1)
	data.Address2 = stringPtrToValue(org.Address2)
	data.City = stringPtrToValue(org.City)
	data.State = stringPtrToValue(org.State)
	data.PostalCode = stringPtrToValue(org.PostalCode)
	data.CountryCode = stringPtrToValue(org.CountryCode)
	data.BusinessNumber = stringPtrToValue(org.BusinessNumber)
	data.TaxID = stringPtrToValue(org.TaxID)
	data.TaxIDType = stringPtrToValue(org.TaxIDType)
	data.TaxRate = stringPtrToValue(org.TaxRate)
	data.DefaultLocale = stringPtrToValue(org.DefaultLocale)

	if org.Currency != nil {
		data.Currency = types.StringValue(string(*org.Currency))
	} else {
		data.Currency = types.StringNull()
	}
	if org.ParentOrganizationID != nil {
		data.ParentOrganizationID = types.StringValue(string(*org.ParentOrganizationID))
	} else {
		data.ParentOrganizationID = types.StringNull()
	}
	if org.CreatedOn != nil {
		data.CreatedOn = types.StringValue(org.CreatedOn.Format("2006-01-02T15:04:05Z07:00"))
	} else {
		data.CreatedOn = types.StringNull()
	}
}
