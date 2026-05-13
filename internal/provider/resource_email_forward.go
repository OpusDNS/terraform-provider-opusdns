package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

var _ resource.Resource = &EmailForwardResource{}
var _ resource.ResourceWithImportState = &EmailForwardResource{}

type EmailForwardResource struct {
	client *opusdns.Client
}

type EmailForwardResourceModel struct {
	ID             types.String `tfsdk:"id"`
	EmailForwardID types.String `tfsdk:"email_forward_id"`
	Hostname       types.String `tfsdk:"hostname"`
	Enabled        types.Bool   `tfsdk:"enabled"`
	Aliases        types.List   `tfsdk:"aliases"`
}

type EmailForwardAliasModel struct {
	AliasID   types.String `tfsdk:"alias_id"`
	Alias     types.String `tfsdk:"alias"`
	ForwardTo types.List   `tfsdk:"forward_to"`
}

var emailForwardAliasAttrTypes = map[string]attr.Type{
	"alias_id":   types.StringType,
	"alias":      types.StringType,
	"forward_to": types.ListType{ElemType: types.StringType},
}

func NewEmailForwardResource() resource.Resource {
	return &EmailForwardResource{}
}

func (r *EmailForwardResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_email_forward"
}

func (r *EmailForwardResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages email forwarding for a hostname in OpusDNS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The email forward ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"email_forward_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier for the email forward.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"hostname": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The hostname to enable email forwarding for (e.g., `example.com`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Whether email forwarding is active. Defaults to `true`.",
			},
			"aliases": schema.ListNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "The list of email aliases for this hostname.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"alias_id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "The unique identifier for this alias.",
							PlanModifiers: []planmodifier.String{
								stringplanmodifier.UseStateForUnknown(),
							},
						},
						"alias": schema.StringAttribute{
							Required:            true,
							MarkdownDescription: "The alias part (e.g., `info` for `info@example.com`). Use `*` for a catch-all.",
						},
						"forward_to": schema.ListAttribute{
							Required:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "The list of destination email addresses.",
						},
					},
				},
			},
		},
	}
}

func (r *EmailForwardResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*opusdns.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *opusdns.Client, got: %T.", req.ProviderData))
		return
	}
	r.client = client
}

func (r *EmailForwardResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data EmailForwardResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &models.EmailForwardCreateRequest{Hostname: data.Hostname.ValueString()}

	if !data.Aliases.IsNull() && !data.Aliases.IsUnknown() {
		var aliasModels []EmailForwardAliasModel
		resp.Diagnostics.Append(data.Aliases.ElementsAs(ctx, &aliasModels, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		for _, am := range aliasModels {
			var forwardTo []string
			resp.Diagnostics.Append(am.ForwardTo.ElementsAs(ctx, &forwardTo, false)...)
			if resp.Diagnostics.HasError() {
				return
			}
			createReq.Aliases = append(createReq.Aliases, models.EmailForwardAliasCreate{
				Alias:     am.Alias.ValueString(),
				ForwardTo: forwardTo,
			})
		}
	}

	ef, err := r.client.EmailForwards.CreateEmailForward(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating email forward", formatAPIError(err))
		return
	}

	if !data.Enabled.ValueBool() {
		if err := r.client.EmailForwards.DisableEmailForward(ctx, ef.EmailForwardID); err != nil {
			resp.Diagnostics.AddError("Error disabling email forward", formatAPIError(err))
			return
		}
	}

	ef, err = r.client.EmailForwards.GetEmailForward(ctx, ef.EmailForwardID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading email forward after create", formatAPIError(err))
		return
	}

	resp.Diagnostics.Append(setEmailForwardState(ctx, &data, ef)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}

func (r *EmailForwardResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data EmailForwardResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ef, err := r.client.EmailForwards.GetEmailForward(ctx, models.EmailForwardID(data.EmailForwardID.ValueString()))
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading email forward", formatAPIError(err))
		return
	}

	resp.Diagnostics.Append(setEmailForwardState(ctx, &data, ef)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}

func (r *EmailForwardResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state, plan EmailForwardResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	efID := models.EmailForwardID(state.EmailForwardID.ValueString())

	if plan.Enabled.ValueBool() != state.Enabled.ValueBool() {
		if plan.Enabled.ValueBool() {
			if err := r.client.EmailForwards.EnableEmailForward(ctx, efID); err != nil {
				resp.Diagnostics.AddError("Error enabling email forward", formatAPIError(err))
				return
			}
		} else {
			if err := r.client.EmailForwards.DisableEmailForward(ctx, efID); err != nil {
				resp.Diagnostics.AddError("Error disabling email forward", formatAPIError(err))
				return
			}
		}
	}

	var stateAliases, planAliases []EmailForwardAliasModel
	if !state.Aliases.IsNull() && !state.Aliases.IsUnknown() {
		resp.Diagnostics.Append(state.Aliases.ElementsAs(ctx, &stateAliases, false)...)
	}
	if !plan.Aliases.IsNull() && !plan.Aliases.IsUnknown() {
		resp.Diagnostics.Append(plan.Aliases.ElementsAs(ctx, &planAliases, false)...)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	stateAliasMap := make(map[string]EmailForwardAliasModel)
	for _, a := range stateAliases {
		stateAliasMap[a.Alias.ValueString()] = a
	}
	planAliasMap := make(map[string]EmailForwardAliasModel)
	for _, a := range planAliases {
		planAliasMap[a.Alias.ValueString()] = a
	}

	for aliasName, stateAlias := range stateAliasMap {
		if _, exists := planAliasMap[aliasName]; !exists {
			aliasID := models.EmailForwardAliasID(stateAlias.AliasID.ValueString())
			if err := r.client.EmailForwards.DeleteAlias(ctx, efID, aliasID); err != nil && !isNotFound(err) {
				resp.Diagnostics.AddError("Error deleting email alias", formatAPIError(err))
				return
			}
		}
	}

	for aliasName, planAlias := range planAliasMap {
		var forwardTo []string
		resp.Diagnostics.Append(planAlias.ForwardTo.ElementsAs(ctx, &forwardTo, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		if existing, exists := stateAliasMap[aliasName]; exists {
			aliasID := models.EmailForwardAliasID(existing.AliasID.ValueString())
			if _, err := r.client.EmailForwards.UpdateAlias(ctx, efID, aliasID,
				&models.EmailForwardAliasUpdate{ForwardTo: forwardTo}); err != nil {
				resp.Diagnostics.AddError("Error updating email alias", formatAPIError(err))
				return
			}
		} else {
			if _, err := r.client.EmailForwards.CreateAlias(ctx, efID, &models.EmailForwardAliasCreate{
				Alias:     aliasName,
				ForwardTo: forwardTo,
			}); err != nil {
				resp.Diagnostics.AddError("Error creating email alias", formatAPIError(err))
				return
			}
		}
	}

	ef, err := r.client.EmailForwards.GetEmailForward(ctx, efID)
	if err != nil {
		resp.Diagnostics.AddError("Error reading email forward after update", formatAPIError(err))
		return
	}

	resp.Diagnostics.Append(setEmailForwardState(ctx, &plan, ef)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}

func (r *EmailForwardResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data EmailForwardResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.EmailForwards.DeleteEmailForward(ctx, models.EmailForwardID(data.EmailForwardID.ValueString())); err != nil {
		if !isNotFound(err) {
			resp.Diagnostics.AddError("Error deleting email forward", formatAPIError(err))
		}
	}
}

func (r *EmailForwardResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	ef, err := r.client.EmailForwards.GetEmailForward(ctx, models.EmailForwardID(req.ID))
	if err != nil {
		resp.Diagnostics.AddError("Error importing email forward", formatAPIError(err))
		return
	}
	var data EmailForwardResourceModel
	resp.Diagnostics.Append(setEmailForwardState(ctx, &data, ef)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}

func setEmailForwardState(ctx context.Context, data *EmailForwardResourceModel, ef *models.EmailForward) diag.Diagnostics {
	var diagnostics diag.Diagnostics

	data.ID = types.StringValue(string(ef.EmailForwardID))
	data.EmailForwardID = types.StringValue(string(ef.EmailForwardID))
	data.Hostname = types.StringValue(ef.Hostname)
	data.Enabled = types.BoolValue(ef.Enabled)

	aliasObjType := types.ObjectType{AttrTypes: emailForwardAliasAttrTypes}
	aliasValues := make([]attr.Value, len(ef.Aliases))
	for i, a := range ef.Aliases {
		forwardToList, d := types.ListValueFrom(ctx, types.StringType, a.ForwardTo)
		diagnostics.Append(d...)
		if diagnostics.HasError() {
			return diagnostics
		}
		aliasObj, d := types.ObjectValue(emailForwardAliasAttrTypes, map[string]attr.Value{
			"alias_id":   types.StringValue(string(a.EmailForwardAliasID)),
			"alias":      types.StringValue(a.Alias),
			"forward_to": forwardToList,
		})
		diagnostics.Append(d...)
		if diagnostics.HasError() {
			return diagnostics
		}
		aliasValues[i] = aliasObj
	}

	aliasList, d := types.ListValue(aliasObjType, aliasValues)
	diagnostics.Append(d...)
	data.Aliases = aliasList
	return diagnostics
}
