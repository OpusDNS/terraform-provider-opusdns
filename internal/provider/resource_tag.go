package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure TagResource satisfies the resource.Resource interface.
var _ resource.Resource = &TagResource{}
var _ resource.ResourceWithImportState = &TagResource{}

// TagResource implements `opusdns_tag` backed by `/v1/tags`.
//
// Tag `type` is immutable at the API level (only label, color, and
// description are accepted in the PATCH body — see
// api/api/tag/v1_routes.py:88), so changes to `type` force replacement.
type TagResource struct {
	client *opusdns.Client
}

// TagResourceModel is the state shape for a tag resource.
type TagResourceModel struct {
	ID          types.String `tfsdk:"id"`
	TagID       types.String `tfsdk:"tag_id"`
	Label       types.String `tfsdk:"label"`
	Type        types.String `tfsdk:"type"`
	Color       types.String `tfsdk:"color"`
	Description types.String `tfsdk:"description"`
	ObjectCount types.Int64  `tfsdk:"object_count"`
	CreatedOn   types.String `tfsdk:"created_on"`
	UpdatedOn   types.String `tfsdk:"updated_on"`
}

// NewTagResource returns a new TagResource.
func NewTagResource() resource.Resource {
	return &TagResource{}
}

func (r *TagResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tag"
}

func (r *TagResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a tag in OpusDNS. Tags categorize and group resources (domains, contacts, zones) " +
			"and are surfaced across the API and console for filtering. Backed by `/v1/tags`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The tag ID (mirrors `tag_id`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"tag_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier for the tag (e.g. `tag_01j...`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"label": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Human-readable label for the tag.",
			},
			"type": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "The resource category this tag applies to. One of `DOMAIN`, `CONTACT`, `ZONE`. " +
					"This is immutable at the API level; changes force replacement.",
				Validators: []validator.String{
					stringvalidator.OneOf(
						string(models.TagTypeDomain),
						string(models.TagTypeContact),
						string(models.TagTypeZone),
					),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"color": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "The tag color palette slot. One of `color-1` through `color-10`. " +
					"Computed by the server when omitted.",
				Validators: []validator.String{
					stringvalidator.OneOf(
						string(models.TagColor1), string(models.TagColor2), string(models.TagColor3),
						string(models.TagColor4), string(models.TagColor5), string(models.TagColor6),
						string(models.TagColor7), string(models.TagColor8), string(models.TagColor9),
						string(models.TagColor10),
					),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional free-text description of the tag.",
			},
			"object_count": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Number of objects currently associated with this tag.",
			},
			"created_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the tag was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the tag was last updated.",
			},
		},
	}
}

func (r *TagResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *TagResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &models.TagCreateRequest{
		Label: data.Label.ValueString(),
		Type:  models.TagType(data.Type.ValueString()),
	}
	if !data.Color.IsNull() && !data.Color.IsUnknown() {
		c := models.TagColor(data.Color.ValueString())
		createReq.Color = &c
	}
	createReq.Description = optionalStringPtr(data.Description)

	tag, err := r.client.Tags.CreateTag(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating tag", formatAPIError(err))
		return
	}

	populateTagModel(&data, tag)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TagResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tagIDStr := data.TagID.ValueString()
	if tagIDStr == "" {
		resp.Diagnostics.AddError(
			"Invalid tag state",
			"The opusdns_tag resource has an empty `tag_id` in state, which prevents reading it from the API. "+
				"Remove the resource from state with `terraform state rm` and re-import or recreate it.",
		)
		return
	}

	tag, err := r.client.Tags.GetTag(ctx, models.TagID(tagIDStr))
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading tag", formatAPIError(err))
		return
	}

	populateTagModel(&data, tag)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TagResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state TagResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tagIDStr := state.TagID.ValueString()
	if tagIDStr == "" {
		resp.Diagnostics.AddError(
			"Invalid tag state",
			"The opusdns_tag resource has an empty `tag_id` in state, which prevents updating it. "+
				"Remove the resource from state with `terraform state rm` and re-import or recreate it.",
		)
		return
	}

	updateReq := &models.TagUpdateRequest{}
	hasChange := false

	if !plan.Label.Equal(state.Label) {
		s := plan.Label.ValueString()
		updateReq.Label = &s
		hasChange = true
	}
	if !plan.Color.Equal(state.Color) {
		// Color is Optional+Computed: skip update when the plan value is
		// unknown (i.e. it will be filled in from state by the server).
		if !plan.Color.IsUnknown() && !plan.Color.IsNull() {
			c := models.TagColor(plan.Color.ValueString())
			updateReq.Color = &c
			hasChange = true
		}
	}
	if !plan.Description.Equal(state.Description) {
		updateReq.Description = optionalStringPtr(plan.Description)
		hasChange = true
	}

	var tag *models.Tag
	if hasChange {
		updated, err := r.client.Tags.UpdateTag(ctx, models.TagID(tagIDStr), updateReq)
		if err != nil {
			resp.Diagnostics.AddError("Error updating tag", formatAPIError(err))
			return
		}
		tag = updated
	} else {
		current, err := r.client.Tags.GetTag(ctx, models.TagID(tagIDStr))
		if err != nil {
			resp.Diagnostics.AddError("Error reading tag", formatAPIError(err))
			return
		}
		tag = current
	}

	populateTagModel(&plan, tag)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *TagResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TagResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tagIDStr := data.TagID.ValueString()
	if tagIDStr == "" {
		resp.Diagnostics.AddError(
			"Invalid tag state",
			"The opusdns_tag resource has an empty `tag_id` in state, which prevents deletion via the API. "+
				"Remove the resource from state with `terraform state rm` and, if the tag still exists at OpusDNS, delete it manually or re-import then destroy.",
		)
		return
	}

	if err := r.client.Tags.DeleteTag(ctx, models.TagID(tagIDStr)); err != nil {
		if !isNotFound(err) {
			resp.Diagnostics.AddError("Error deleting tag", formatAPIError(err))
		}
	}
}

func (r *TagResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by tag_id; full state is hydrated by the subsequent Read.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("tag_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// populateTagModel copies API response fields onto a TagResourceModel.
func populateTagModel(data *TagResourceModel, t *models.Tag) {
	data.ID = types.StringValue(string(t.TagID))
	data.TagID = types.StringValue(string(t.TagID))
	data.Label = types.StringValue(t.Label)
	data.Type = types.StringValue(string(t.Type))
	data.Color = types.StringValue(string(t.Color))
	if t.Description != nil {
		data.Description = types.StringValue(*t.Description)
	} else {
		data.Description = types.StringNull()
	}
	data.ObjectCount = types.Int64Value(int64(t.ObjectCount))
	data.CreatedOn = types.StringValue(t.CreatedOn.Format(time.RFC3339))
	data.UpdatedOn = types.StringValue(t.UpdatedOn.Format(time.RFC3339))
}
