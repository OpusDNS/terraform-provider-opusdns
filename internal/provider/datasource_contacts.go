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

// Ensure ContactsDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &ContactsDataSource{}

// ContactsDataSource lists all contacts in the authenticated caller's
// organization via `GET /v1/contacts` (SDK: Contacts.ListContacts, which
// auto-paginates). Optional filter inputs map onto models.ListContactsOptions.
type ContactsDataSource struct {
	client *opusdns.Client
}

// ContactsDataSourceModel is the top-level data-source state shape, with a
// few simple filters surfaced as inputs.
type ContactsDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	Search        types.String `tfsdk:"search"`
	FirstName     types.String `tfsdk:"first_name"`
	LastName      types.String `tfsdk:"last_name"`
	Email         types.String `tfsdk:"email"`
	Country       types.String `tfsdk:"country"`
	Verified      types.Bool   `tfsdk:"verified"`
	TagIDs        types.List   `tfsdk:"tag_ids"`
	TagMode       types.String `tfsdk:"tag_mode"`
	CreatedAfter  types.String `tfsdk:"created_after"`
	CreatedBefore types.String `tfsdk:"created_before"`
	Contacts      types.List   `tfsdk:"contacts"`
}

var contactItemAttrTypes = map[string]attr.Type{
	"contact_id":      types.StringType,
	"organization_id": types.StringType,
	"first_name":      types.StringType,
	"last_name":       types.StringType,
	"org":             types.StringType,
	"title":           types.StringType,
	"email":           types.StringType,
	"phone":           types.StringType,
	"fax":             types.StringType,
	"street":          types.StringType,
	"city":            types.StringType,
	"state":           types.StringType,
	"postal_code":     types.StringType,
	"country":         types.StringType,
	"disclose":        types.BoolType,
	"verified":        types.BoolType,
	"verified_on":     types.StringType,
	"created_on":      types.StringType,
	"updated_on":      types.StringType,
}

// NewContactsDataSource returns a new ContactsDataSource.
func NewContactsDataSource() datasource.DataSource {
	return &ContactsDataSource{}
}

func (d *ContactsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_contacts"
}

func (d *ContactsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists contacts within the authenticated caller's organization (`GET /v1/contacts`). Results are auto-paginated by the SDK; optional filters narrow the server-side query.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, MarkdownDescription: "Static identifier for this data source."},

			"search":     schema.StringAttribute{Optional: true, MarkdownDescription: "Free-text search over contacts."},
			"first_name": schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by exact first name."},
			"last_name":  schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by exact last name."},
			"email":      schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by exact email address."},
			"country":    schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by ISO 3166-1 alpha-2 country code."},
			"verified":   schema.BoolAttribute{Optional: true, MarkdownDescription: "Filter by verification status."},
			"tag_ids": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Filter by tag IDs. Multiple values are sent as repeated `tag_ids` query parameters.",
			},
			"tag_mode":       schema.StringAttribute{Optional: true, MarkdownDescription: "Tag filter mode. Use `match_any` or `match_all` according to the API."},
			"created_after":  schema.StringAttribute{Optional: true, MarkdownDescription: "Filter contacts created after this RFC3339 timestamp."},
			"created_before": schema.StringAttribute{Optional: true, MarkdownDescription: "Filter contacts created before this RFC3339 timestamp."},

			"contacts": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Matching contacts.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"contact_id":      schema.StringAttribute{Computed: true},
						"organization_id": schema.StringAttribute{Computed: true},
						"first_name":      schema.StringAttribute{Computed: true},
						"last_name":       schema.StringAttribute{Computed: true},
						"org":             schema.StringAttribute{Computed: true},
						"title":           schema.StringAttribute{Computed: true},
						"email":           schema.StringAttribute{Computed: true},
						"phone":           schema.StringAttribute{Computed: true},
						"fax":             schema.StringAttribute{Computed: true},
						"street":          schema.StringAttribute{Computed: true},
						"city":            schema.StringAttribute{Computed: true},
						"state":           schema.StringAttribute{Computed: true},
						"postal_code":     schema.StringAttribute{Computed: true},
						"country":         schema.StringAttribute{Computed: true},
						"disclose":        schema.BoolAttribute{Computed: true},
						"verified":        schema.BoolAttribute{Computed: true},
						"verified_on":     schema.StringAttribute{Computed: true},
						"created_on":      schema.StringAttribute{Computed: true},
						"updated_on":      schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *ContactsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *ContactsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ContactsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := &models.ListContactsOptions{
		Search:    stringValue(data.Search),
		FirstName: stringValue(data.FirstName),
		LastName:  stringValue(data.LastName),
		Email:     stringValue(data.Email),
		Country:   stringValue(data.Country),
	}
	if !data.Verified.IsNull() && !data.Verified.IsUnknown() {
		v := data.Verified.ValueBool()
		opts.Verified = &v
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
	var diags diag.Diagnostics
	if opts.CreatedAfter, diags = parseOptionalRFC3339(data.CreatedAfter, "created_after"); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	if opts.CreatedBefore, diags = parseOptionalRFC3339(data.CreatedBefore, "created_before"); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	contacts, err := d.client.Contacts.ListContacts(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error listing contacts", formatAPIError(err))
		return
	}

	objType := types.ObjectType{AttrTypes: contactItemAttrTypes}
	values := make([]attr.Value, len(contacts))
	for i := range contacts {
		c := &contacts[i]
		obj, diags := types.ObjectValue(contactItemAttrTypes, map[string]attr.Value{
			"contact_id":      types.StringValue(string(c.ContactID)),
			"organization_id": types.StringValue(string(c.OrganizationID)),
			"first_name":      types.StringValue(c.FirstName),
			"last_name":       types.StringValue(c.LastName),
			"org":             stringPtrToValue(c.Org),
			"title":           stringPtrToValue(c.Title),
			"email":           types.StringValue(c.Email),
			"phone":           types.StringValue(c.Phone),
			"fax":             stringPtrToValue(c.Fax),
			"street":          types.StringValue(c.Street),
			"city":            types.StringValue(c.City),
			"state":           stringPtrToValue(c.State),
			"postal_code":     types.StringValue(c.PostalCode),
			"country":         types.StringValue(c.Country),
			"disclose":        types.BoolValue(c.Disclose),
			"verified":        types.BoolValue(c.Verified),
			"verified_on":     timePtrToValue(c.VerifiedOn),
			"created_on":      timePtrToValue(c.CreatedOn),
			"updated_on":      timePtrToValue(c.UpdatedOn),
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

	data.ID = types.StringValue("contacts")
	data.Contacts = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
