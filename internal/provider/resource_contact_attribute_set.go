package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

var _ resource.Resource = &ContactAttributeSetResource{}
var _ resource.ResourceWithImportState = &ContactAttributeSetResource{}

// ContactAttributeSetResource implements `opusdns_contact_attribute_set`
// backed by `/v1/contacts/attribute-sets`. A contact attribute set holds the
// registry-specific extra attributes (legal form, ID numbers, etc.) required
// for some TLDs (.de, .eu, .at, .be, .uk, .fr, .nl, .it, .us, .ca, .ro...).
// `tld` and the `attributes` map are immutable at the API level — only
// `label` is accepted in PATCH bodies, so any change to `tld` or `attributes`
// forces replacement.
type ContactAttributeSetResource struct {
	client *opusdns.Client
}

type ContactAttributeSetResourceModel struct {
	ID                    types.String `tfsdk:"id"`
	ContactAttributeSetID types.String `tfsdk:"contact_attribute_set_id"`
	OrganizationID        types.String `tfsdk:"organization_id"`
	Label                 types.String `tfsdk:"label"`
	TLD                   types.String `tfsdk:"tld"`
	Attributes            types.Map    `tfsdk:"attributes"`
	LinkedContacts        types.Int64  `tfsdk:"linked_contacts"`
	CreatedOn             types.String `tfsdk:"created_on"`
	UpdatedOn             types.String `tfsdk:"updated_on"`
}

func NewContactAttributeSetResource() resource.Resource {
	return &ContactAttributeSetResource{}
}

func (r *ContactAttributeSetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_contact_attribute_set"
}

func (r *ContactAttributeSetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	useStateForUnknown := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a contact attribute set (`/v1/contacts/attribute-sets`). " +
			"Contact attribute sets hold registry-specific extra attributes (e.g. `DE_CONTACT_TYPE`, " +
			"`NOMINET_CO_NO`, `SIDN_LEGAL_FORM`) required by certain TLDs and are linked to one or more " +
			"contacts via `opusdns_contact_attribute_link`. `tld` and `attributes` are immutable at the " +
			"API level; changes force replacement. `label` is updatable in place.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Mirrors `contact_attribute_set_id`.",
				PlanModifiers:       useStateForUnknown,
			},
			"contact_attribute_set_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier (e.g. `contact_attribute_set_01j...`).",
				PlanModifiers:       useStateForUnknown,
			},
			"organization_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Owning organization id.",
				PlanModifiers:       useStateForUnknown,
			},
			"label": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Human-readable label (1\u2013255 chars).",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 255),
				},
			},
			"tld": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "TLD the attribute set applies to (e.g. `de`, `.de`, `DE`). " +
					"Immutable; changes force replacement.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"attributes": schema.MapAttribute{
				Required:    true,
				ElementType: types.StringType,
				MarkdownDescription: "Key/value map of registry-handle attributes. Keys must come from the " +
					"`RegistryHandleAttributeType` enum (e.g. `DE_CONTACT_TYPE`, `dnsbe:type`, " +
					"`NOMINET_CO_NO`, `SIDN_LEGAL_FORM`, `US_NEXUS_CATEGORY`). Immutable at the API level; " +
					"changes force replacement.",
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
			},
			"linked_contacts": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Number of contacts currently linked to this set.",
			},
			"created_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the set was created.",
				PlanModifiers:       useStateForUnknown,
			},
			"updated_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the set was last updated.",
			},
		},
	}
}

func (r *ContactAttributeSetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ContactAttributeSetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ContactAttributeSetResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	attrs, diags := mapToStringMap(ctx, data.Attributes)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	body := map[string]interface{}{
		"label":      data.Label.ValueString(),
		"tld":        data.TLD.ValueString(),
		"attributes": attrs,
	}

	set, err := rawCreateContactAttributeSet(ctx, r.client, body)
	if err != nil {
		resp.Diagnostics.AddError("Error creating contact attribute set", formatAPIError(err))
		return
	}

	populateContactAttributeSetModel(&data, set, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ContactAttributeSetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ContactAttributeSetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := data.ContactAttributeSetID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError(
			"Invalid contact attribute set state",
			"`contact_attribute_set_id` is empty; remove with `terraform state rm` and re-import or recreate.",
		)
		return
	}

	set, err := rawGetContactAttributeSet(ctx, r.client, id)
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading contact attribute set", formatAPIError(err))
		return
	}

	populateContactAttributeSetModel(&data, set, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ContactAttributeSetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state ContactAttributeSetResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id := state.ContactAttributeSetID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError("Invalid state", "contact_attribute_set_id is empty")
		return
	}

	var set *contactAttributeSetAPIResponse
	if !plan.Label.Equal(state.Label) {
		body := map[string]interface{}{"label": plan.Label.ValueString()}
		updated, err := rawUpdateContactAttributeSet(ctx, r.client, id, body)
		if err != nil {
			resp.Diagnostics.AddError("Error updating contact attribute set", formatAPIError(err))
			return
		}
		set = updated
	} else {
		current, err := rawGetContactAttributeSet(ctx, r.client, id)
		if err != nil {
			resp.Diagnostics.AddError("Error reading contact attribute set", formatAPIError(err))
			return
		}
		set = current
	}

	populateContactAttributeSetModel(&plan, set, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *ContactAttributeSetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ContactAttributeSetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := data.ContactAttributeSetID.ValueString()
	if id == "" {
		return
	}
	if err := rawDeleteContactAttributeSet(ctx, r.client, id); err != nil {
		if !isNotFound(err) {
			resp.Diagnostics.AddError("Error deleting contact attribute set", formatAPIError(err))
		}
	}
}

func (r *ContactAttributeSetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("contact_attribute_set_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// populateContactAttributeSetModel copies API response fields onto the model.
func populateContactAttributeSetModel(data *ContactAttributeSetResourceModel, s *contactAttributeSetAPIResponse, diags *diag.Diagnostics) {
	data.ID = types.StringValue(s.ContactAttributeSetID)
	data.ContactAttributeSetID = types.StringValue(s.ContactAttributeSetID)
	data.OrganizationID = types.StringValue(s.OrganizationID)
	data.Label = types.StringValue(s.Label)
	data.TLD = types.StringValue(s.TLD)
	data.LinkedContacts = types.Int64Value(s.LinkedContacts)
	data.CreatedOn = types.StringValue(s.CreatedOn.Format(time.RFC3339))
	data.UpdatedOn = types.StringValue(s.UpdatedOn.Format(time.RFC3339))

	elems := make(map[string]string, len(s.Attributes))
	for k, v := range s.Attributes {
		elems[k] = v
	}
	mv, d := types.MapValueFrom(context.Background(), types.StringType, elems)
	diags.Append(d...)
	data.Attributes = mv
}
