package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure VanityNameserverSetResource satisfies the resource.Resource interface.
var (
	_ resource.Resource                = &VanityNameserverSetResource{}
	_ resource.ResourceWithImportState = &VanityNameserverSetResource{}
)

// VanityNameserverSetResource implements `opusdns_vanity_nameserver_set` backed
// by `/v1/vanity-nameserver-sets`.
//
// The API exposes no update endpoint for a set's defining fields (name,
// parent_domain_name, soa_rname, hostnames), so those attributes force
// replacement. Only the organization-default flag (`is_default`) is updatable
// in place, via the SetDefault/ClearDefault endpoints.
type VanityNameserverSetResource struct {
	client *opusdns.Client
}

// VanityNameserverSetResourceModel is the state shape for the resource.
type VanityNameserverSetResourceModel struct {
	ID               types.String `tfsdk:"id"`
	SetID            types.String `tfsdk:"set_id"`
	OrganizationID   types.String `tfsdk:"organization_id"`
	Name             types.String `tfsdk:"name"`
	ParentDomainName types.String `tfsdk:"parent_domain_name"`
	SOARName         types.String `tfsdk:"soa_rname"`
	Hostnames        types.List   `tfsdk:"hostnames"`
	IsDefault        types.Bool   `tfsdk:"is_default"`
	Status           types.String `tfsdk:"status"`
	Nameservers      types.List   `tfsdk:"nameservers"`
}

// vanityNameserverAttrTypes is the object shape for each element of the
// computed `nameservers` list attribute.
var vanityNameserverAttrTypes = map[string]attr.Type{
	"hostname": types.StringType,
	"position": types.Int64Type,
}

// NewVanityNameserverSetResource returns a new VanityNameserverSetResource.
func NewVanityNameserverSetResource() resource.Resource {
	return &VanityNameserverSetResource{}
}

func (r *VanityNameserverSetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vanity_nameserver_set"
}

func (r *VanityNameserverSetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	requiresReplaceString := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a vanity nameserver set in OpusDNS (`/v1/vanity-nameserver-sets`). " +
			"A vanity nameserver set brands the apex `NS` and `SOA` records of DNS zones with your own " +
			"hostnames instead of the OpusDNS system defaults. Assign a set to a zone with the " +
			"`vanity_nameserver_set_id` attribute of `opusdns_zone`.\n\n" +
			"The API has no update endpoint for a set's defining fields, so `name`, `parent_domain_name`, " +
			"`soa_rname`, and `hostnames` force replacement when changed. Only `is_default` is updatable in place. " +
			"Provisioning is asynchronous: a newly created set starts in the `provisioning` status.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Mirror of `set_id`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"set_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier for the vanity nameserver set (e.g. `vns_...`).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"organization_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The organization that owns the set.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Human-readable name for the set.",
			},
			"parent_domain_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The parent domain used as the apex of the vanity NS zone. All `hostnames` must be subdomains of it. Forces replacement.",
				PlanModifiers:       requiresReplaceString,
			},
			"soa_rname": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The SOA RNAME (responsible-party email in DNS form, e.g. `hostmaster.example.com`) stamped verbatim into vanity-branded zones. Forces replacement.",
				PlanModifiers:       requiresReplaceString,
			},
			"hostnames": schema.ListAttribute{
				Required:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "The fully-qualified vanity nameserver hostnames, ordered by intended position (the lowest position becomes the SOA MNAME). Forces replacement, as the API has no update endpoint for a set's hostnames.",
				PlanModifiers: []planmodifier.List{
					// The set's hostnames cannot be changed after creation.
					listplanmodifier.RequiresReplace(),
				},
			},
			"is_default": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "Whether this is the organization's default vanity nameserver set. When `true`, zones created without an explicit set inherit this one. Updatable in place. Defaults to `false`.",
			},
			"status": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Lifecycle status of the set (`provisioning`, `active`, `suspended`, `failed`, or `deleting`). Provisioning is asynchronous.",
			},
			"nameservers": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The nameservers in the set, ordered by position.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"hostname": schema.StringAttribute{Computed: true, MarkdownDescription: "The vanity nameserver hostname."},
						"position": schema.Int64Attribute{Computed: true, MarkdownDescription: "Ordering within the set; the lowest position becomes the SOA MNAME."},
					},
				},
			},
		},
	}
}

func (r *VanityNameserverSetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *VanityNameserverSetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data VanityNameserverSetResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hostnames, diags := stringListValueToStrings(ctx, data.Hostnames)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &models.VanityNameserverSetCreateRequest{
		Name:             data.Name.ValueString(),
		ParentDomainName: data.ParentDomainName.ValueString(),
		SOARName:         data.SOARName.ValueString(),
		Hostnames:        hostnames,
	}

	set, err := r.client.VanityNameservers.CreateSet(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating vanity nameserver set", formatAPIError(err))
		return
	}

	// is_default is not part of the create request; reconcile it afterwards.
	if data.IsDefault.ValueBool() && !set.IsDefault {
		if _, err := r.client.VanityNameservers.SetDefault(ctx, set.SetID); err != nil {
			resp.Diagnostics.AddError("Error setting vanity nameserver set as default", formatAPIError(err))
			return
		}
		set.IsDefault = true
	}

	resp.Diagnostics.Append(populateVanityNameserverSetModel(&data, set)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VanityNameserverSetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data VanityNameserverSetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	setID := data.SetID.ValueString()
	if setID == "" {
		resp.Diagnostics.AddError(
			"Invalid vanity nameserver set state",
			"The opusdns_vanity_nameserver_set resource has an empty `set_id` in state, which prevents reading it from the API. "+
				"Remove the resource from state with `terraform state rm` and re-import or recreate it.",
		)
		return
	}

	set, err := r.client.VanityNameservers.GetSet(ctx, models.VanityNameserverSetID(setID))
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading vanity nameserver set", formatAPIError(err))
		return
	}

	resp.Diagnostics.Append(populateVanityNameserverSetModel(&data, set)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *VanityNameserverSetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state VanityNameserverSetResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	setID := state.SetID.ValueString()
	if setID == "" {
		resp.Diagnostics.AddError(
			"Invalid vanity nameserver set state",
			"The opusdns_vanity_nameserver_set resource has an empty `set_id` in state, which prevents updating it. "+
				"Remove the resource from state with `terraform state rm` and re-import or recreate it.",
		)
		return
	}

	// Only is_default is updatable in place; all other changed fields force
	// replacement via RequiresReplace plan modifiers.
	if !plan.IsDefault.Equal(state.IsDefault) {
		if plan.IsDefault.ValueBool() {
			if _, err := r.client.VanityNameservers.SetDefault(ctx, models.VanityNameserverSetID(setID)); err != nil {
				resp.Diagnostics.AddError("Error setting vanity nameserver set as default", formatAPIError(err))
				return
			}
		} else {
			if _, err := r.client.VanityNameservers.ClearDefault(ctx); err != nil {
				resp.Diagnostics.AddError("Error clearing default vanity nameserver set", formatAPIError(err))
				return
			}
		}
	}

	set, err := r.client.VanityNameservers.GetSet(ctx, models.VanityNameserverSetID(setID))
	if err != nil {
		resp.Diagnostics.AddError("Error reading vanity nameserver set after update", formatAPIError(err))
		return
	}

	resp.Diagnostics.Append(populateVanityNameserverSetModel(&plan, set)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *VanityNameserverSetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data VanityNameserverSetResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	setID := data.SetID.ValueString()
	if setID == "" {
		resp.Diagnostics.AddError(
			"Invalid vanity nameserver set state",
			"The opusdns_vanity_nameserver_set resource has an empty `set_id` in state, which prevents deletion via the API. "+
				"Remove the resource from state with `terraform state rm` and, if the set still exists at OpusDNS, delete it manually or re-import then destroy.",
		)
		return
	}

	if err := r.client.VanityNameservers.DeleteSet(ctx, models.VanityNameserverSetID(setID)); err != nil {
		if !isNotFound(err) {
			resp.Diagnostics.AddError("Error deleting vanity nameserver set", formatAPIError(err))
		}
	}
}

func (r *VanityNameserverSetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by set_id; full state is hydrated by the subsequent Read.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("set_id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}

// populateVanityNameserverSetModel copies API response fields onto the model.
func populateVanityNameserverSetModel(data *VanityNameserverSetResourceModel, set *models.VanityNameserverSet) diag.Diagnostics {
	var diags diag.Diagnostics

	data.ID = types.StringValue(string(set.SetID))
	data.SetID = types.StringValue(string(set.SetID))
	data.OrganizationID = types.StringValue(string(set.OrganizationID))
	data.Name = types.StringValue(set.Name)
	data.ParentDomainName = types.StringValue(set.ParentDomainName)
	data.SOARName = types.StringValue(set.SOARName)
	data.IsDefault = types.BoolValue(set.IsDefault)
	data.Status = types.StringValue(string(set.Status))

	hostnames := make([]string, len(set.Nameservers))
	for i, ns := range set.Nameservers {
		hostnames[i] = ns.Hostname
	}
	hostList, hd := stringSliceToList(hostnames)
	diags.Append(hd...)
	data.Hostnames = hostList

	nsList, nd := vanityNameserversToList(set.Nameservers)
	diags.Append(nd...)
	data.Nameservers = nsList

	return diags
}

// vanityNameserversToList converts SDK VanityNameservers into a Terraform list
// value matching vanityNameserverAttrTypes.
func vanityNameserversToList(ns []models.VanityNameserver) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	objType := types.ObjectType{AttrTypes: vanityNameserverAttrTypes}
	values := make([]attr.Value, len(ns))
	for i, n := range ns {
		obj, d := types.ObjectValue(vanityNameserverAttrTypes, map[string]attr.Value{
			"hostname": types.StringValue(n.Hostname),
			"position": types.Int64Value(int64(n.Position)),
		})
		diags.Append(d...)
		if diags.HasError() {
			return types.ListNull(objType), diags
		}
		values[i] = obj
	}
	list, d := types.ListValue(objType, values)
	diags.Append(d...)
	return list, diags
}
