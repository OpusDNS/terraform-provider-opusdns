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

// Ensure ContactDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &ContactDataSource{}

// ContactDataSource fetches a single contact via
// `GET /v1/contacts/{contact_id}` (SDK: Contacts.GetContact).
type ContactDataSource struct {
	client *opusdns.Client
}

// ContactDataSourceModel mirrors the resource model with all attributes
// computed and `contact_id` required as the lookup key. The phone/fax fields
// use plain types.String here (no semantic-equality games are needed for
// data sources, which never plan vs. state).
type ContactDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	ContactID      types.String `tfsdk:"contact_id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	FirstName      types.String `tfsdk:"first_name"`
	LastName       types.String `tfsdk:"last_name"`
	Org            types.String `tfsdk:"org"`
	Title          types.String `tfsdk:"title"`
	Email          types.String `tfsdk:"email"`
	Phone          types.String `tfsdk:"phone"`
	Fax            types.String `tfsdk:"fax"`
	Street         types.String `tfsdk:"street"`
	City           types.String `tfsdk:"city"`
	State          types.String `tfsdk:"state"`
	PostalCode     types.String `tfsdk:"postal_code"`
	Country        types.String `tfsdk:"country"`
	Disclose       types.Bool   `tfsdk:"disclose"`
	Verified       types.Bool   `tfsdk:"verified"`
	VerifiedOn     types.String `tfsdk:"verified_on"`
	CreatedOn      types.String `tfsdk:"created_on"`
	UpdatedOn      types.String `tfsdk:"updated_on"`
}

// NewContactDataSource returns a new ContactDataSource.
func NewContactDataSource() datasource.DataSource {
	return &ContactDataSource{}
}

func (d *ContactDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_contact"
}

func (d *ContactDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single contact from `GET /v1/contacts/{contact_id}`.",
		Attributes: map[string]schema.Attribute{
			"id":              schema.StringAttribute{Computed: true, MarkdownDescription: "Mirror of `contact_id`."},
			"contact_id":      schema.StringAttribute{Required: true, MarkdownDescription: "Contact id to look up (e.g. `contact_...`)."},
			"organization_id": schema.StringAttribute{Computed: true, MarkdownDescription: "Organization that owns this contact."},
			"first_name":      schema.StringAttribute{Computed: true},
			"last_name":       schema.StringAttribute{Computed: true},
			"org":             schema.StringAttribute{Computed: true, MarkdownDescription: "Contact's organization name (optional)."},
			"title":           schema.StringAttribute{Computed: true},
			"email":           schema.StringAttribute{Computed: true},
			"phone":           schema.StringAttribute{Computed: true},
			"fax":             schema.StringAttribute{Computed: true},
			"street":          schema.StringAttribute{Computed: true},
			"city":            schema.StringAttribute{Computed: true},
			"state":           schema.StringAttribute{Computed: true},
			"postal_code":     schema.StringAttribute{Computed: true},
			"country":         schema.StringAttribute{Computed: true, MarkdownDescription: "ISO 3166-1 alpha-2 country code."},
			"disclose":        schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the contact is publicly disclosed (WHOIS)."},
			"verified":        schema.BoolAttribute{Computed: true},
			"verified_on":     schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp when the contact was verified, if any."},
			"created_on":      schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp when the contact was created."},
			"updated_on":      schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp when the contact was last updated."},
		},
	}
}

func (d *ContactDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ContactDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ContactDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	contact, err := d.client.Contacts.GetContact(ctx, models.ContactID(data.ContactID.ValueString()))
	if err != nil {
		resp.Diagnostics.AddError("Error reading contact", formatAPIError(err))
		return
	}

	populateContactDataSourceModel(&data, contact)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// populateContactDataSourceModel fills a ContactDataSourceModel from an API
// contact response. Shared with the list data source's per-item population.
func populateContactDataSourceModel(data *ContactDataSourceModel, c *models.Contact) {
	data.ID = types.StringValue(string(c.ContactID))
	data.ContactID = types.StringValue(string(c.ContactID))
	data.OrganizationID = types.StringValue(string(c.OrganizationID))
	data.FirstName = types.StringValue(c.FirstName)
	data.LastName = types.StringValue(c.LastName)
	data.Org = stringPtrToValue(c.Org)
	data.Title = stringPtrToValue(c.Title)
	data.Email = types.StringValue(c.Email)
	data.Phone = types.StringValue(c.Phone)
	data.Fax = stringPtrToValue(c.Fax)
	data.Street = types.StringValue(c.Street)
	data.City = types.StringValue(c.City)
	data.State = stringPtrToValue(c.State)
	data.PostalCode = types.StringValue(c.PostalCode)
	data.Country = types.StringValue(c.Country)
	data.Disclose = types.BoolValue(c.Disclose)
	data.Verified = types.BoolValue(c.Verified)

	data.VerifiedOn = timePtrToValue(c.VerifiedOn)
	data.CreatedOn = timePtrToValue(c.CreatedOn)
	data.UpdatedOn = timePtrToValue(c.UpdatedOn)
}
