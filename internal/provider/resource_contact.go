package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure ContactResource satisfies the resource.Resource interface.
var _ resource.Resource = &ContactResource{}
var _ resource.ResourceWithImportState = &ContactResource{}

// ContactResource defines the resource implementation.
type ContactResource struct {
	client *opusdns.Client
}

// ContactResourceModel describes the resource data model.
type ContactResourceModel struct {
	ID         types.String `tfsdk:"id"`
	ContactID  types.String `tfsdk:"contact_id"`
	FirstName  types.String `tfsdk:"first_name"`
	LastName   types.String `tfsdk:"last_name"`
	Org        types.String `tfsdk:"org"`
	Title      types.String `tfsdk:"title"`
	Email      types.String `tfsdk:"email"`
	Phone      phoneValue   `tfsdk:"phone"`
	Fax        phoneValue   `tfsdk:"fax"`
	Street     types.String `tfsdk:"street"`
	City       types.String `tfsdk:"city"`
	State      types.String `tfsdk:"state"`
	PostalCode types.String `tfsdk:"postal_code"`
	Country    types.String `tfsdk:"country"`
	Disclose   types.Bool   `tfsdk:"disclose"`
}

// NewContactResource returns a new ContactResource.
func NewContactResource() resource.Resource {
	return &ContactResource{}
}

func (r *ContactResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_contact"
}

func (r *ContactResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	requiresReplace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a contact in OpusDNS. Contacts are used for domain registration.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The contact ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"contact_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier for the contact.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"first_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The contact's first name.",
				PlanModifiers:       requiresReplace,
			},
			"last_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The contact's last name.",
				PlanModifiers:       requiresReplace,
			},
			"org": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The contact's organization name.",
				PlanModifiers:       requiresReplace,
			},
			"title": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The contact's title (e.g., `Mr.`, `Dr.`).",
				PlanModifiers:       requiresReplace,
			},
			"email": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The contact's email address.",
				PlanModifiers:       requiresReplace,
			},
			"phone": schema.StringAttribute{
				CustomType:          phoneType{},
				Required:            true,
				MarkdownDescription: "The contact's phone number in E.164 format (e.g., `+1.2125551234`). The server may normalize the value (e.g., reformat punctuation); semantic equality is used so reformatted values that canonicalise to the same digits do not trigger drift.",
				PlanModifiers:       requiresReplace,
			},
			"fax": schema.StringAttribute{
				CustomType:          phoneType{},
				Optional:            true,
				MarkdownDescription: "The contact's fax number in E.164 format. The server may normalize the value; semantic equality is used so reformatted values do not trigger drift.",
				PlanModifiers:       requiresReplace,
			},
			"street": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The street address.",
				PlanModifiers:       requiresReplace,
			},
			"city": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The city.",
				PlanModifiers:       requiresReplace,
			},
			"state": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The state or province.",
				PlanModifiers:       requiresReplace,
			},
			"postal_code": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The postal or ZIP code.",
				PlanModifiers:       requiresReplace,
			},
			"country": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The two-letter country code (ISO 3166-1 alpha-2, e.g., `US`).",
				PlanModifiers:       requiresReplace,
			},
			"disclose": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Whether to publicly disclose contact information (WHOIS). Defaults to `false`.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *ContactResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*opusdns.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *opusdns.Client, got: %T.", req.ProviderData),
		)
		return
	}
	r.client = client
}

func (r *ContactResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ContactResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &models.ContactCreateRequest{
		FirstName:  data.FirstName.ValueString(),
		LastName:   data.LastName.ValueString(),
		Email:      data.Email.ValueString(),
		Phone:      data.Phone.ValueString(),
		Street:     data.Street.ValueString(),
		City:       data.City.ValueString(),
		PostalCode: data.PostalCode.ValueString(),
		Country:    data.Country.ValueString(),
		Disclose:   data.Disclose.ValueBool(),
	}
	if !data.Org.IsNull() {
		s := data.Org.ValueString()
		createReq.Org = &s
	}
	if !data.Title.IsNull() {
		s := data.Title.ValueString()
		createReq.Title = &s
	}
	if !data.Fax.IsNull() {
		s := data.Fax.ValueString()
		createReq.Fax = &s
	}
	if !data.State.IsNull() {
		s := data.State.ValueString()
		createReq.State = &s
	}

	contact, err := r.client.Contacts.CreateContact(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating contact", formatAPIError(err))
		return
	}

	populateContactModel(&data, contact)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ContactResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ContactResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	contactIDStr := data.ContactID.ValueString()
	if contactIDStr == "" {
		resp.Diagnostics.AddError(
			"Invalid contact state",
			"The opusdns_contact resource has an empty `contact_id` in state, which prevents reading it from the API. "+
				"Remove the resource from state with `terraform state rm` and re-import or recreate it.",
		)
		return
	}

	contact, err := r.client.Contacts.GetContact(ctx, models.ContactID(contactIDStr))
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading contact", formatAPIError(err))
		return
	}

	populateContactModel(&data, contact)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ContactResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
	// All fields have RequiresReplace, so Update is never called.
}

func (r *ContactResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ContactResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	contactIDStr := data.ContactID.ValueString()
	if contactIDStr == "" {
		resp.Diagnostics.AddError(
			"Invalid contact state",
			"The opusdns_contact resource has an empty `contact_id` in state, which prevents deletion via the API. "+
				"Remove the resource from state with `terraform state rm` and, if the contact still exists at OpusDNS, delete it manually or re-import then destroy.",
		)
		return
	}

	if err := r.client.Contacts.DeleteContact(ctx, models.ContactID(contactIDStr)); err != nil {
		if !isNotFound(err) {
			resp.Diagnostics.AddError("Error deleting contact", formatAPIError(err))
		}
	}
}

func (r *ContactResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	contact, err := r.client.Contacts.GetContact(ctx, models.ContactID(req.ID))
	if err != nil {
		resp.Diagnostics.AddError("Error importing contact", formatAPIError(err))
		return
	}

	var data ContactResourceModel
	populateContactModel(&data, contact)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// populateContactModel sets the model fields from an API contact response.
func populateContactModel(data *ContactResourceModel, c *models.Contact) {
	data.ID = types.StringValue(string(c.ContactID))
	data.ContactID = types.StringValue(string(c.ContactID))
	data.FirstName = types.StringValue(c.FirstName)
	data.LastName = types.StringValue(c.LastName)
	data.Email = types.StringValue(c.Email)
	data.Phone = normalizedPhoneValue(&c.Phone)
	data.Street = types.StringValue(c.Street)
	data.City = types.StringValue(c.City)
	data.PostalCode = types.StringValue(c.PostalCode)
	data.Country = types.StringValue(c.Country)
	data.Disclose = types.BoolValue(c.Disclose)

	if c.Org != nil {
		data.Org = types.StringValue(*c.Org)
	} else {
		data.Org = types.StringNull()
	}
	if c.Title != nil {
		data.Title = types.StringValue(*c.Title)
	} else {
		data.Title = types.StringNull()
	}
	if c.Fax != nil {
		data.Fax = normalizedPhoneValue(c.Fax)
	} else {
		data.Fax = phoneValue{StringValue: types.StringNull()}
	}
	if c.State != nil {
		data.State = types.StringValue(*c.State)
	} else {
		data.State = types.StringNull()
	}
}
