package provider

import (
	"context"
	"fmt"

	opusdns "github.com/opusdns/opusdns-go-client/opusdns"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &ContactResource{}

type ContactResource struct {
	client *opusdns.Client
}

type ContactResourceModel struct {
	ID         types.String `tfsdk:"id"`
	ContactID  types.String `tfsdk:"contact_id"`
	FirstName  types.String `tfsdk:"first_name"`
	LastName   types.String `tfsdk:"last_name"`
	Email      types.String `tfsdk:"email"`
	Phone      types.String `tfsdk:"phone"`
	Street     types.String `tfsdk:"street"`
	City       types.String `tfsdk:"city"`
	State      types.String `tfsdk:"state"`
	PostalCode types.String `tfsdk:"postal_code"`
	Country    types.String `tfsdk:"country"`
	Org        types.String `tfsdk:"org"`
	Disclose   types.Bool   `tfsdk:"disclose"`
	Verified   types.Bool   `tfsdk:"verified"`
}

func NewContactResource() resource.Resource {
	return &ContactResource{}
}

func (r *ContactResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_contact"
}

func (r *ContactResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a contact in OpusDNS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"contact_id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"first_name": schema.StringAttribute{
				Required:    true,
				Description: "Contact first name.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"last_name": schema.StringAttribute{
				Required:    true,
				Description: "Contact last name.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"email": schema.StringAttribute{
				Required:    true,
				Description: "Contact email address.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"phone": schema.StringAttribute{
				Required:    true,
				Description: "Contact phone number.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"street": schema.StringAttribute{
				Required:    true,
				Description: "Contact street address.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"city": schema.StringAttribute{
				Required:    true,
				Description: "Contact city.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"state": schema.StringAttribute{
				Optional:    true,
				Description: "Contact state or province.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"postal_code": schema.StringAttribute{
				Required:    true,
				Description: "Contact postal code.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"country": schema.StringAttribute{
				Required:    true,
				Description: "Contact country code (ISO 3166-1 alpha-2).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"org": schema.StringAttribute{
				Optional:    true,
				Description: "Contact organization name.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"disclose": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether to disclose contact information in WHOIS.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"verified": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the contact has been verified.",
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
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *opusdns.Client, got %T", req.ProviderData))
		return
	}
	r.client = client
}

func (r *ContactResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ContactResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &models.ContactCreateRequest{
		FirstName:  plan.FirstName.ValueString(),
		LastName:   plan.LastName.ValueString(),
		Email:      plan.Email.ValueString(),
		Phone:      plan.Phone.ValueString(),
		Street:     plan.Street.ValueString(),
		City:       plan.City.ValueString(),
		PostalCode: plan.PostalCode.ValueString(),
		Country:    plan.Country.ValueString(),
		Disclose:   plan.Disclose.ValueBool(),
	}

	if !plan.State.IsNull() && !plan.State.IsUnknown() {
		s := plan.State.ValueString()
		createReq.State = &s
	}

	if !plan.Org.IsNull() && !plan.Org.IsUnknown() {
		o := plan.Org.ValueString()
		createReq.Org = &o
	}

	contact, err := r.client.Contacts.CreateContact(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create contact", err.Error())
		return
	}

	flattenContact(contact, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ContactResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ContactResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	contact, err := r.client.Contacts.GetContact(ctx, models.ContactID(state.ContactID.ValueString()))
	if err != nil {
		if opusdns.IsNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read contact", err.Error())
		return
	}

	flattenContact(contact, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is not supported by the API; all mutable fields use RequiresReplace.
func (r *ContactResource) Update(_ context.Context, _ resource.UpdateRequest, _ *resource.UpdateResponse) {
}

func (r *ContactResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ContactResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Contacts.DeleteContact(ctx, models.ContactID(state.ContactID.ValueString())); err != nil {
		if !opusdns.IsNotFoundError(err) {
			resp.Diagnostics.AddError("Failed to delete contact", err.Error())
		}
	}
}

func flattenContact(c *models.Contact, model *ContactResourceModel) {
	model.ID = types.StringValue(string(c.ContactID))
	model.ContactID = types.StringValue(string(c.ContactID))
	model.FirstName = types.StringValue(c.FirstName)
	model.LastName = types.StringValue(c.LastName)
	model.Email = types.StringValue(c.Email)
	model.Phone = types.StringValue(c.Phone)
	model.Street = types.StringValue(c.Street)
	model.City = types.StringValue(c.City)
	model.PostalCode = types.StringValue(c.PostalCode)
	model.Country = types.StringValue(c.Country)
	model.Disclose = types.BoolValue(c.Disclose)
	model.Verified = types.BoolValue(c.Verified)

	if c.State != nil {
		model.State = types.StringValue(*c.State)
	} else {
		model.State = types.StringNull()
	}

	if c.Org != nil {
		model.Org = types.StringValue(*c.Org)
	} else {
		model.Org = types.StringNull()
	}
}
