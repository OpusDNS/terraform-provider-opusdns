package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure OrganizationResource satisfies the resource.Resource interface.
var _ resource.Resource = &OrganizationResource{}
var _ resource.ResourceWithImportState = &OrganizationResource{}

// OrganizationResource manages /v1/organizations entries. Organizations are
// created as children of the authenticated organization (the API uses the
// caller's org as the parent automatically).
type OrganizationResource struct {
	client *opusdns.Client
}

// OrganizationResourceModel mirrors models.OrganizationUpdateRequest plus the
// computed fields returned by the API on read.
type OrganizationResourceModel struct {
	ID                   types.String `tfsdk:"id"`
	OrganizationID       types.String `tfsdk:"organization_id"`
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

// NewOrganizationResource returns a new OrganizationResource.
func NewOrganizationResource() resource.Resource {
	return &OrganizationResource{}
}

func (r *OrganizationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization"
}

func (r *OrganizationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	useStateForUnknown := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a (child) organization in OpusDNS via `/v1/organizations`. " +
			"On create, the new organization is provisioned as a child of the authenticated organization.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The organization id (mirrors `organization_id`).",
				PlanModifiers:       useStateForUnknown,
			},
			"organization_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier for the organization, e.g. `organization_...`.",
				PlanModifiers:       useStateForUnknown,
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Organization name.",
			},
			"address_1": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "First line of the organization address.",
				PlanModifiers:       useStateForUnknown,
			},
			"address_2": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Second line of the organization address.",
				PlanModifiers:       useStateForUnknown,
			},
			"city": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "City.",
				PlanModifiers:       useStateForUnknown,
			},
			"state": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "State or province.",
				PlanModifiers:       useStateForUnknown,
			},
			"postal_code": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Postal/ZIP code.",
				PlanModifiers:       useStateForUnknown,
			},
			"country_code": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "ISO 3166-1 alpha-2 country code (e.g. `US`).",
				PlanModifiers:       useStateForUnknown,
			},
			"business_number": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Government issued business identifier.",
				PlanModifiers:       useStateForUnknown,
			},
			"tax_id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Tax ID for the organization.",
				PlanModifiers:       useStateForUnknown,
			},
			"tax_id_type": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Type of tax ID.",
				PlanModifiers:       useStateForUnknown,
			},
			"currency": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Default currency for the organization (ISO 4217, e.g. `USD`).",
				PlanModifiers:       useStateForUnknown,
			},
			"default_locale": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Default locale for the organization (e.g. `en-US`).",
				PlanModifiers:       useStateForUnknown,
			},
			"parent_organization_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "ID of the parent organization (the authenticated caller's org).",
				PlanModifiers:       useStateForUnknown,
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Organization status (`active`, `suspended`, `deleted`).",
				PlanModifiers:       useStateForUnknown,
			},
			"tax_rate": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Tax rate applied to the organization (read-only).",
				PlanModifiers:       useStateForUnknown,
			},
			"created_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp the organization was created.",
				PlanModifiers:       useStateForUnknown,
			},
		},
	}
}

func (r *OrganizationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *OrganizationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OrganizationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The API derives parent_organization_id from the auth context, so we just
	// send the OrganizationCreate body (which inherits OrganizationBase).
	body := map[string]interface{}{
		"name": data.Name.ValueString(),
	}
	addOptionalString(body, "address_1", data.Address1)
	addOptionalString(body, "address_2", data.Address2)
	addOptionalString(body, "city", data.City)
	addOptionalString(body, "state", data.State)
	addOptionalString(body, "postal_code", data.PostalCode)
	addOptionalString(body, "country_code", data.CountryCode)
	addOptionalString(body, "business_number", data.BusinessNumber)
	addOptionalString(body, "tax_id", data.TaxID)
	addOptionalString(body, "tax_id_type", data.TaxIDType)
	addOptionalString(body, "currency", data.Currency)
	addOptionalString(body, "default_locale", data.DefaultLocale)

	org, err := rawCreateOrganization(ctx, r.client, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating organization", err.Error())
		return
	}

	populateOrganizationModel(&data, org)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OrganizationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	org, err := r.client.Organizations.GetOrganization(ctx, models.OrganizationID(data.OrganizationID.ValueString()))
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading organization", err.Error())
		return
	}

	populateOrganizationModel(&data, org)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan OrganizationResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := &models.OrganizationUpdateRequest{}
	updateReq.Name = optionalStringPtr(plan.Name)
	updateReq.Address1 = optionalStringPtr(plan.Address1)
	updateReq.Address2 = optionalStringPtr(plan.Address2)
	updateReq.City = optionalStringPtr(plan.City)
	updateReq.State = optionalStringPtr(plan.State)
	updateReq.PostalCode = optionalStringPtr(plan.PostalCode)
	updateReq.CountryCode = optionalStringPtr(plan.CountryCode)
	updateReq.BusinessNumber = optionalStringPtr(plan.BusinessNumber)
	updateReq.TaxID = optionalStringPtr(plan.TaxID)
	updateReq.TaxIDType = optionalStringPtr(plan.TaxIDType)
	updateReq.DefaultLocale = optionalStringPtr(plan.DefaultLocale)
	if !plan.Currency.IsNull() && !plan.Currency.IsUnknown() {
		c := models.Currency(plan.Currency.ValueString())
		updateReq.Currency = &c
	}

	org, err := r.client.Organizations.UpdateOrganization(ctx, models.OrganizationID(plan.OrganizationID.ValueString()), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating organization", err.Error())
		return
	}

	populateOrganizationModel(&plan, org)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *OrganizationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data OrganizationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := rawDeleteOrganization(ctx, r.client, models.OrganizationID(data.OrganizationID.ValueString())); err != nil {
		if !isNotFound(err) {
			resp.Diagnostics.AddError("Error deleting organization", err.Error())
		}
	}
}

func (r *OrganizationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("organization_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// populateOrganizationModel maps an API Organization onto the TF model.
func populateOrganizationModel(data *OrganizationResourceModel, org *models.Organization) {
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
