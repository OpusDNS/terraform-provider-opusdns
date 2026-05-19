package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

var _ resource.Resource = &ContactAttributeLinkResource{}
var _ resource.ResourceWithImportState = &ContactAttributeLinkResource{}

// ContactAttributeLinkResource implements `opusdns_contact_attribute_link`
// backed by `PATCH /v1/contacts/{contact_id}/link/{contact_attribute_set_id}`.
//
// The OpusDNS API currently exposes only a link-create operation; there is no
// dedicated unlink endpoint. As a result `terraform destroy` cannot un-link a
// contact from an attribute set — the resource is removed from state with a
// warning. The underlying link is implicitly cleared when either the contact
// or the attribute set is deleted, or when the set is re-applied to a
// different contact for the same TLD.
type ContactAttributeLinkResource struct {
	client *opusdns.Client
}

type ContactAttributeLinkResourceModel struct {
	ID                     types.String `tfsdk:"id"`
	ContactAttributeLinkID types.String `tfsdk:"contact_attribute_link_id"`
	ContactID              types.String `tfsdk:"contact_id"`
	ContactAttributeSetID  types.String `tfsdk:"contact_attribute_set_id"`
	TLD                    types.String `tfsdk:"tld"`
	CreatedOn              types.String `tfsdk:"created_on"`
}

func NewContactAttributeLinkResource() resource.Resource {
	return &ContactAttributeLinkResource{}
}

func (r *ContactAttributeLinkResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_contact_attribute_link"
}

func (r *ContactAttributeLinkResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	requiresReplace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	useStateForUnknown := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Links a contact to a contact attribute set " +
			"(`PATCH /v1/contacts/{contact_id}/link/{contact_attribute_set_id}`). " +
			"Both `contact_id` and `contact_attribute_set_id` are immutable; any change forces replacement. " +
			"**Note:** the OpusDNS API has no unlink endpoint, so `terraform destroy` only removes the " +
			"resource from state and emits a warning. Re-linking happens automatically if you re-apply " +
			"the same configuration.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Mirrors `contact_attribute_link_id`.",
				PlanModifiers:       useStateForUnknown,
			},
			"contact_attribute_link_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Unique identifier for the link (e.g. `contact_attribute_link_01j...`).",
				PlanModifiers:       useStateForUnknown,
			},
			"contact_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Contact id to link.",
				PlanModifiers:       requiresReplace,
			},
			"contact_attribute_set_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Contact attribute set id to link to.",
				PlanModifiers:       requiresReplace,
			},
			"tld": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "TLD this link applies to (mirrors the attribute set's TLD).",
				PlanModifiers:       useStateForUnknown,
			},
			"created_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the link was created.",
				PlanModifiers:       useStateForUnknown,
			},
		},
	}
}

func (r *ContactAttributeLinkResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ContactAttributeLinkResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ContactAttributeLinkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	link, err := rawLinkContactToAttributeSet(ctx, r.client, data.ContactID.ValueString(), data.ContactAttributeSetID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error linking contact to attribute set", formatAPIError(err))
		return
	}

	data.ID = types.StringValue(link.ContactAttributeLinkID)
	data.ContactAttributeLinkID = types.StringValue(link.ContactAttributeLinkID)
	data.ContactID = types.StringValue(link.ContactID)
	data.ContactAttributeSetID = types.StringValue(link.ContactAttributeSetID)
	data.TLD = types.StringValue(link.TLD)
	data.CreatedOn = types.StringValue(link.CreatedOn.Format(time.RFC3339))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ContactAttributeLinkResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ContactAttributeLinkResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	contactID := data.ContactID.ValueString()
	setID := data.ContactAttributeSetID.ValueString()
	if contactID == "" || setID == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	// No dedicated GET-link endpoint exists. The link is observable via the
	// contact's `attribute_sets` array; find ours by attribute_set id.
	links, err := rawGetContactAttributeLinks(ctx, r.client, contactID)
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading contact attribute link", formatAPIError(err))
		return
	}

	for _, l := range links {
		if l.ContactAttributeSetID == setID {
			data.TLD = types.StringValue(l.TLD)
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}

	// Link not present on the contact anymore — drop from state.
	resp.State.RemoveResource(ctx)
}

func (r *ContactAttributeLinkResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// All inputs are RequiresReplace; nothing to do here. Refresh state.
	var data ContactAttributeLinkResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ContactAttributeLinkResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	// The OpusDNS API does not expose an unlink endpoint. We can only drop
	// the resource from Terraform state and warn the user. The actual link
	// will persist on the contact until the contact or the attribute set is
	// itself deleted.
	resp.Diagnostics.AddWarning(
		"opusdns_contact_attribute_link cannot be deleted via the API",
		"The OpusDNS API does not expose an unlink endpoint for contact attribute links. "+
			"Terraform has removed this resource from state, but the underlying link still exists on the "+
			"contact. To clear it, delete the contact or the attribute set.",
	)
}

func (r *ContactAttributeLinkResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import id is `<contact_id>:<contact_attribute_set_id>`.
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Invalid import id",
			"Expected `<contact_id>:<contact_attribute_set_id>`; got: "+req.ID,
		)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("contact_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("contact_attribute_set_id"), parts[1])...)
	// Other computed fields will be hydrated by the subsequent Read.
}
