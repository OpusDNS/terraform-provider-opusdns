package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

var _ resource.Resource = &DomainForwardResource{}
var _ resource.ResourceWithImportState = &DomainForwardResource{}

type DomainForwardResource struct {
	client *opusdns.Client
}

type DomainForwardResourceModel struct {
	ID       types.String `tfsdk:"id"`
	Hostname types.String `tfsdk:"hostname"`
	Enabled  types.Bool   `tfsdk:"enabled"`
	HTTP     types.List   `tfsdk:"http"`
	HTTPS    types.List   `tfsdk:"https"`
}

type HttpRedirectModel struct {
	RequestPath    types.String `tfsdk:"request_path"`
	TargetProtocol types.String `tfsdk:"target_protocol"`
	TargetHostname types.String `tfsdk:"target_hostname"`
	TargetPath     types.String `tfsdk:"target_path"`
	RedirectCode   types.Int64  `tfsdk:"redirect_code"`
}

var httpRedirectAttrTypes = map[string]attr.Type{
	"request_path":    types.StringType,
	"target_protocol": types.StringType,
	"target_hostname": types.StringType,
	"target_path":     types.StringType,
	"redirect_code":   types.Int64Type,
}

func NewDomainForwardResource() resource.Resource {
	return &DomainForwardResource{}
}

func (r *DomainForwardResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain_forward"
}

func (r *DomainForwardResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	redirectSchema := schema.NestedAttributeObject{
		Attributes: map[string]schema.Attribute{
			"request_path": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The source path to match (e.g., `/` or `/old-path`).",
			},
			"target_protocol": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The destination protocol (`http` or `https`).",
			},
			"target_hostname": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The destination hostname.",
			},
			"target_path": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The destination path.",
			},
			"redirect_code": schema.Int64Attribute{
				Required:            true,
				MarkdownDescription: "The HTTP redirect status code (`301`, `302`, `307`, or `308`).",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
		},
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages URL/domain forwarding for a hostname in OpusDNS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The hostname (used as the unique identifier).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"hostname": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The source hostname to forward from (e.g., `www.example.com`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "Whether the domain forward is active. Defaults to `true`.",
			},
			"http": schema.ListNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "HTTP protocol redirect rules.",
				NestedObject:        redirectSchema,
			},
			"https": schema.ListNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "HTTPS protocol redirect rules.",
				NestedObject:        redirectSchema,
			},
		},
	}
}

func (r *DomainForwardResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DomainForwardResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DomainForwardResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &models.DomainForwardCreateRequest{
		Hostname: data.Hostname.ValueString(),
		Enabled:  data.Enabled.ValueBool(),
	}

	httpSet, d := buildProtocolSetRequest(ctx, data.HTTP)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	createReq.HTTP = httpSet

	httpsSet, d := buildProtocolSetRequest(ctx, data.HTTPS)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	createReq.HTTPS = httpsSet

	df, err := r.client.DomainForwards.CreateDomainForward(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating domain forward", err.Error())
		return
	}

	resp.Diagnostics.Append(setDomainForwardState(ctx, &data, df)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}

func (r *DomainForwardResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DomainForwardResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	df, err := r.client.DomainForwards.GetDomainForward(ctx, data.Hostname.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading domain forward", err.Error())
		return
	}

	resp.Diagnostics.Append(setDomainForwardState(ctx, &data, df)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}

func (r *DomainForwardResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var state, plan DomainForwardResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hostname := state.Hostname.ValueString()

	if plan.Enabled.ValueBool() != state.Enabled.ValueBool() {
		if plan.Enabled.ValueBool() {
			if err := r.client.DomainForwards.EnableDomainForward(ctx, hostname); err != nil {
				resp.Diagnostics.AddError("Error enabling domain forward", err.Error())
				return
			}
		} else {
			if err := r.client.DomainForwards.DisableDomainForward(ctx, hostname); err != nil {
				resp.Diagnostics.AddError("Error disabling domain forward", err.Error())
				return
			}
		}
	}

	// Update HTTP config if changed.
	if !plan.HTTP.Equal(state.HTTP) {
		httpSet, d := buildProtocolSetRequest(ctx, plan.HTTP)
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
		if httpSet != nil {
			if _, err := r.client.DomainForwards.UpdateDomainForwardConfig(ctx, hostname, models.HttpProtocolHTTP, httpSet); err != nil {
				resp.Diagnostics.AddError("Error updating HTTP domain forward config", err.Error())
				return
			}
		} else {
			// Delete HTTP config if removed.
			if err := r.client.DomainForwards.DeleteDomainForwardConfig(ctx, hostname, models.HttpProtocolHTTP); err != nil && !isNotFound(err) {
				resp.Diagnostics.AddError("Error deleting HTTP domain forward config", err.Error())
				return
			}
		}
	}

	// Update HTTPS config if changed.
	if !plan.HTTPS.Equal(state.HTTPS) {
		httpsSet, d := buildProtocolSetRequest(ctx, plan.HTTPS)
		resp.Diagnostics.Append(d...)
		if resp.Diagnostics.HasError() {
			return
		}
		if httpsSet != nil {
			if _, err := r.client.DomainForwards.UpdateDomainForwardConfig(ctx, hostname, models.HttpProtocolHTTPS, httpsSet); err != nil {
				resp.Diagnostics.AddError("Error updating HTTPS domain forward config", err.Error())
				return
			}
		} else {
			if err := r.client.DomainForwards.DeleteDomainForwardConfig(ctx, hostname, models.HttpProtocolHTTPS); err != nil && !isNotFound(err) {
				resp.Diagnostics.AddError("Error deleting HTTPS domain forward config", err.Error())
				return
			}
		}
	}

	df, err := r.client.DomainForwards.GetDomainForward(ctx, hostname)
	if err != nil {
		resp.Diagnostics.AddError("Error reading domain forward after update", err.Error())
		return
	}

	resp.Diagnostics.Append(setDomainForwardState(ctx, &plan, df)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	}
}

func (r *DomainForwardResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DomainForwardResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.DomainForwards.DeleteDomainForward(ctx, data.Hostname.ValueString()); err != nil {
		if !isNotFound(err) {
			resp.Diagnostics.AddError("Error deleting domain forward", err.Error())
		}
	}
}

func (r *DomainForwardResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	df, err := r.client.DomainForwards.GetDomainForward(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing domain forward", err.Error())
		return
	}
	var data DomainForwardResourceModel
	resp.Diagnostics.Append(setDomainForwardState(ctx, &data, df)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}

// buildProtocolSetRequest converts a list of redirect models into a DomainForwardProtocolSetRequest.
// Returns nil if the list is null/unknown/empty.
func buildProtocolSetRequest(ctx context.Context, redirectList types.List) (*models.DomainForwardProtocolSetRequest, diag.Diagnostics) {
	var diagnostics diag.Diagnostics

	if redirectList.IsNull() || redirectList.IsUnknown() || len(redirectList.Elements()) == 0 {
		return nil, diagnostics
	}

	var redirectModels []HttpRedirectModel
	diagnostics.Append(redirectList.ElementsAs(ctx, &redirectModels, false)...)
	if diagnostics.HasError() {
		return nil, diagnostics
	}

	redirects := make([]models.HttpRedirectRequest, len(redirectModels))
	for i, rm := range redirectModels {
		redirects[i] = models.HttpRedirectRequest{
			RequestPath:    rm.RequestPath.ValueString(),
			TargetProtocol: models.HttpProtocol(rm.TargetProtocol.ValueString()),
			TargetHostname: rm.TargetHostname.ValueString(),
			TargetPath:     rm.TargetPath.ValueString(),
			RedirectCode:   models.RedirectCode(rm.RedirectCode.ValueInt64()),
		}
	}

	return &models.DomainForwardProtocolSetRequest{Redirects: redirects}, diagnostics
}

// setDomainForwardState populates the model from an API response.
func setDomainForwardState(ctx context.Context, data *DomainForwardResourceModel, df *models.DomainForward) diag.Diagnostics {
	var diagnostics diag.Diagnostics

	data.ID = types.StringValue(df.Hostname)
	data.Hostname = types.StringValue(df.Hostname)
	data.Enabled = types.BoolValue(df.Enabled)

	redirectObjType := types.ObjectType{AttrTypes: httpRedirectAttrTypes}

	httpList, d := protocolSetToList(ctx, df.HTTP, redirectObjType)
	diagnostics.Append(d...)
	if diagnostics.HasError() {
		return diagnostics
	}
	data.HTTP = httpList

	httpsList, d := protocolSetToList(ctx, df.HTTPS, redirectObjType)
	diagnostics.Append(d...)
	if diagnostics.HasError() {
		return diagnostics
	}
	data.HTTPS = httpsList

	return diagnostics
}

// protocolSetToList converts a DomainForwardProtocolSet into a Terraform list value.
func protocolSetToList(ctx context.Context, ps *models.DomainForwardProtocolSet, objType types.ObjectType) (types.List, diag.Diagnostics) {
	var diagnostics diag.Diagnostics

	if ps == nil || len(ps.Redirects) == 0 {
		emptyList, d := types.ListValue(objType, []attr.Value{})
		diagnostics.Append(d...)
		return emptyList, diagnostics
	}

	values := make([]attr.Value, len(ps.Redirects))
	for i, r := range ps.Redirects {
		obj, d := types.ObjectValue(httpRedirectAttrTypes, map[string]attr.Value{
			"request_path":    types.StringValue(r.RequestPath),
			"target_protocol": types.StringValue(string(r.TargetProtocol)),
			"target_hostname": types.StringValue(r.TargetHostname),
			"target_path":     types.StringValue(r.TargetPath),
			"redirect_code":   types.Int64Value(int64(r.RedirectCode)),
		})
		diagnostics.Append(d...)
		if diagnostics.HasError() {
			return types.ListNull(objType), diagnostics
		}
		values[i] = obj
	}

	list, d := types.ListValue(objType, values)
	diagnostics.Append(d...)
	return list, diagnostics
}
