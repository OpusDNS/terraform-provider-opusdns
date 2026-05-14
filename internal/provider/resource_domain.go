package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure DomainResource satisfies the resource.Resource interface.
var (
	_ resource.Resource                = &DomainResource{}
	_ resource.ResourceWithImportState = &DomainResource{}
)

// DomainResource manages a registered domain.
type DomainResource struct {
	client *opusdns.Client
}

// DomainResourceModel describes the resource data model.
type DomainResourceModel struct {
	ID                types.String `tfsdk:"id"`
	DomainID          types.String `tfsdk:"domain_id"`
	Name              types.String `tfsdk:"name"`
	SLD               types.String `tfsdk:"sld"`
	TLD               types.String `tfsdk:"tld"`
	OwnerID           types.String `tfsdk:"owner_id"`
	RegistryAccountID types.String `tfsdk:"registry_account_id"`
	PeriodValue       types.Int64  `tfsdk:"period_value"`
	PeriodUnit        types.String `tfsdk:"period_unit"`
	CreateZone        types.Bool   `tfsdk:"create_zone"`
	AuthCode          types.String `tfsdk:"auth_code"`
	RenewalMode       types.String `tfsdk:"renewal_mode"`
	TransferLock      types.Bool   `tfsdk:"transfer_lock"`
	Nameservers       types.List   `tfsdk:"nameservers"`
	Contacts          types.Map    `tfsdk:"contacts"`
	RegistryStatuses  types.List   `tfsdk:"registry_statuses"`
	IsPremium         types.Bool   `tfsdk:"is_premium"`
	AuthCodeExpiresOn types.String `tfsdk:"auth_code_expires_on"`
	RegisteredOn      types.String `tfsdk:"registered_on"`
	ExpiresOn         types.String `tfsdk:"expires_on"`
	CreatedOn         types.String `tfsdk:"created_on"`
	UpdatedOn         types.String `tfsdk:"updated_on"`
}

// nameserverAttrTypes describes the object type used for items in the
// `nameservers` list attribute.
var nameserverAttrTypes = map[string]attr.Type{
	"hostname":     fqdnType{},
	"ip_addresses": types.ListType{ElemType: types.StringType},
}

// contactsMapElemType is the element type of the `contacts` map attribute:
// each value is a list of contact ids for the contact type (the API permits
// multiple contacts per type).
var contactsMapElemType = types.ListType{ElemType: types.StringType}

// clientTransferProhibitedStatus is the EPP status string toggled by the
// `transfer_lock` attribute via the StatusChanges add/remove API.
const clientTransferProhibitedStatus = "clientTransferProhibited"

// NewDomainResource returns a new DomainResource.
func NewDomainResource() resource.Resource {
	return &DomainResource{}
}

func (r *DomainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (r *DomainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	requiresReplaceString := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		MarkdownDescription: "Registers and manages a domain in OpusDNS via `POST /v1/domains` and `PATCH /v1/domains/{ref}`. " +
			"Updatable in place: `contacts`, `nameservers`, `renewal_mode`, `transfer_lock`. " +
			"Other inputs (`name`, `period_*`, `create_zone`, `auth_code`) require replacement. " +
			"Premium pricing confirmation, TMCH claims acceptance, transfer-in, restore, and DNSSEC are not modeled here; use the dedicated APIs / a future `opusdns_domain_dnssec` resource for those.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Mirror of `domain_id`.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"domain_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Server-assigned domain id (e.g. `domain_...`).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Fully-qualified domain name to register (e.g. `example.com`).",
				PlanModifiers:       requiresReplaceString,
			},
			"sld": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Second-level label of `name` (e.g. `example` in `example.com`).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"tld": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Top-level label of `name` (e.g. `com` in `example.com`).",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"owner_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Organization id that owns the domain.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"registry_account_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Registry account id used for this domain.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"period_value": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(1),
				MarkdownDescription: "Initial registration period length. Defaults to `1`. Forces replacement.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"period_unit": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(string(models.PeriodUnitYear)),
				MarkdownDescription: "Initial registration period unit. One of `y`, `m`, `d`. Defaults to `y`. Forces replacement.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"create_zone": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "When `true`, also creates a DNS zone for the domain on OpusDNS nameserver infrastructure. Defaults to `false`. Forces replacement.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"auth_code": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Optional auth code (EPP authInfo) to set on the domain at registration. Forces replacement.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"renewal_mode": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(string(models.RenewalModeRenew)),
				MarkdownDescription: "Renewal mode: `renew` (auto-renew) or `expire`. Defaults to `renew`. Updatable in place.",
			},
			"transfer_lock": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
				MarkdownDescription: "When `true`, sets the `clientTransferProhibited` EPP status on the domain. Defaults to `true`. Updatable in place via the StatusChanges add/remove API.",
			},
			"nameservers": schema.ListNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Authoritative nameservers for the domain. Order is significant. Updatable in place.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"hostname": schema.StringAttribute{
							CustomType:          fqdnType{},
							Required:            true,
							MarkdownDescription: "Nameserver hostname (e.g. `ns1.example.com`). Semantic equality is used so the trailing dot the API serialises is treated as equivalent to the user-supplied form.",
						},
						"ip_addresses": schema.ListAttribute{
							Optional:            true,
							Computed:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "Glue IP addresses (IPv4 and/or IPv6). Required only when the nameserver is in-bailiwick.",
						},
					},
				},
			},
			"contacts": schema.MapAttribute{
				Required:            true,
				ElementType:         contactsMapElemType,
				MarkdownDescription: "Contacts assigned to the domain, keyed by contact type (`registrant`, `admin`, `tech`, `billing`). Each value is a list of contact ids. Updatable in place.",
			},
			"registry_statuses": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "EPP status strings reported by the registry.",
			},
			"is_premium": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the registry classifies this domain as premium-priced.",
			},
			"auth_code_expires_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the auth code expires, if any.",
			},
			"registered_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the domain was registered.",
			},
			"expires_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the domain expires.",
			},
			"created_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the domain record was created.",
			},
			"updated_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the domain record was last updated.",
			},
		},
	}
}

func (r *DomainResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data DomainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	contacts, diags := contactsMapToAPI(ctx, data.Contacts)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	nameservers, diags := nameserversListToAPI(ctx, data.Nameservers)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &models.DomainCreateRequest{
		Name:        data.Name.ValueString(),
		Contacts:    contacts,
		RenewalMode: models.RenewalMode(data.RenewalMode.ValueString()),
		Period: models.DomainPeriod{
			Value: int(data.PeriodValue.ValueInt64()),
			Unit:  models.PeriodUnit(data.PeriodUnit.ValueString()),
		},
		Nameservers: nameservers,
		AuthCode:    optionalStringPtr(data.AuthCode),
		CreateZone:  data.CreateZone.ValueBool(),
	}

	domain, err := r.client.Domains.CreateDomain(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating domain", formatAPIError(err))
		return
	}

	// If the user wants the transfer lock applied (default true) and the
	// freshly-registered domain doesn't already have it, follow up with a
	// PATCH to add the clientTransferProhibited status. The API doesn't
	// accept transfer_lock at create time.
	if data.TransferLock.ValueBool() && !domainHasClientTransferProhibited(domain) {
		updated, uErr := r.client.Domains.UpdateDomain(ctx, string(domain.DomainID), &models.DomainUpdateRequest{
			StatusChanges: &models.StatusChanges{Add: []string{clientTransferProhibitedStatus}},
		})
		if uErr != nil {
			resp.Diagnostics.AddError("Error setting transfer_lock on new domain", formatAPIError(uErr))
			return
		}
		domain = updated
	}

	resp.Diagnostics.Append(populateDomainResourceModel(ctx, &data, domain)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data DomainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domain, err := r.client.Domains.GetDomain(ctx, data.DomainID.ValueString())
	if err != nil {
		if isNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading domain", formatAPIError(err))
		return
	}

	resp.Diagnostics.Append(populateDomainResourceModel(ctx, &data, domain)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *DomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state DomainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	updateReq := &models.DomainUpdateRequest{}
	hasChange := false

	if !plan.Contacts.Equal(state.Contacts) {
		contacts, diags := contactsMapToAPI(ctx, plan.Contacts)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		updateReq.Contacts = contacts
		hasChange = true
	}

	if !plan.Nameservers.Equal(state.Nameservers) {
		nameservers, diags := nameserversListToAPI(ctx, plan.Nameservers)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		updateReq.Nameservers = nameservers
		hasChange = true
	}

	if !plan.RenewalMode.Equal(state.RenewalMode) {
		mode := models.RenewalMode(plan.RenewalMode.ValueString())
		updateReq.RenewalMode = &mode
		hasChange = true
	}

	if !plan.TransferLock.Equal(state.TransferLock) {
		changes := &models.StatusChanges{}
		if plan.TransferLock.ValueBool() {
			changes.Add = []string{clientTransferProhibitedStatus}
		} else {
			changes.Remove = []string{clientTransferProhibitedStatus}
		}
		updateReq.StatusChanges = changes
		hasChange = true
	}

	var domain *models.Domain
	if hasChange {
		updated, err := r.client.Domains.UpdateDomain(ctx, state.DomainID.ValueString(), updateReq)
		if err != nil {
			resp.Diagnostics.AddError("Error updating domain", formatAPIError(err))
			return
		}
		domain = updated
	} else {
		current, err := r.client.Domains.GetDomain(ctx, state.DomainID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error reading domain", formatAPIError(err))
			return
		}
		domain = current
	}

	resp.Diagnostics.Append(populateDomainResourceModel(ctx, &plan, domain)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data DomainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Domains.DeleteDomain(ctx, data.DomainID.ValueString()); err != nil {
		if !isNotFound(err) {
			resp.Diagnostics.AddError("Error deleting domain", formatAPIError(err))
		}
	}
}

func (r *DomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	domain, err := r.client.Domains.GetDomain(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing domain", formatAPIError(err))
		return
	}

	var data DomainResourceModel
	// Inputs not returned by GET fall back to defaults; users can correct via config.
	data.PeriodValue = types.Int64Value(1)
	data.PeriodUnit = types.StringValue(string(models.PeriodUnitYear))
	data.CreateZone = types.BoolValue(false)
	data.AuthCode = types.StringNull()

	resp.Diagnostics.Append(populateDomainResourceModel(ctx, &data, domain)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// populateDomainResourceModel sets the model fields from an API domain response.
// `data.Contacts` is read before being overwritten so any contact-type keys
// the user requested but the API didn't echo back are preserved as empty
// lists (see contactsAPIToMap).
func populateDomainResourceModel(ctx context.Context, data *DomainResourceModel, d *models.Domain) diag.Diagnostics {
	var diags diag.Diagnostics

	expectedContactKeys := contactKeysFromMap(data.Contacts)

	data.ID = types.StringValue(string(d.DomainID))
	data.DomainID = types.StringValue(string(d.DomainID))
	data.Name = types.StringValue(d.Name)
	data.SLD = types.StringValue(d.SLD)
	data.TLD = types.StringValue(d.TLD)
	data.OwnerID = types.StringValue(string(d.OwnerID))
	data.RegistryAccountID = types.StringValue(string(d.RegistryAccountID))
	data.IsPremium = types.BoolValue(d.IsPremium)
	data.AuthCode = stringPtrToValue(d.AuthCode)
	data.AuthCodeExpiresOn = timePtrToValue(d.AuthCodeExpiresOn)
	data.RegisteredOn = timePtrToValue(d.RegisteredOn)
	data.ExpiresOn = timePtrToValue(d.ExpiresOn)
	data.CreatedOn = timePtrToValue(d.CreatedOn)
	data.UpdatedOn = timePtrToValue(d.UpdatedOn)

	if d.RenewalMode != "" {
		data.RenewalMode = types.StringValue(string(d.RenewalMode))
	} else {
		data.RenewalMode = types.StringValue(string(models.RenewalModeRenew))
	}
	data.TransferLock = types.BoolValue(domainHasClientTransferProhibited(d))

	statusList, sd := stringSliceToList(d.RegistryStatuses)
	diags.Append(sd...)
	data.RegistryStatuses = statusList

	nsList, nd := nameserversAPIToList(ctx, d.Nameservers)
	diags.Append(nd...)
	data.Nameservers = nsList

	cMap, cd := contactsAPIToMap(d.Contacts, expectedContactKeys)
	diags.Append(cd...)
	data.Contacts = cMap

	return diags
}

// --- conversion helpers ---

func nameserversListToAPI(ctx context.Context, list types.List) ([]models.Nameserver, diag.Diagnostics) {
	var diags diag.Diagnostics
	if list.IsNull() || list.IsUnknown() {
		return nil, diags
	}
	type nsObj struct {
		Hostname    fqdnValue  `tfsdk:"hostname"`
		IPAddresses types.List `tfsdk:"ip_addresses"`
	}
	var items []nsObj
	diags.Append(list.ElementsAs(ctx, &items, false)...)
	if diags.HasError() {
		return nil, diags
	}
	out := make([]models.Nameserver, 0, len(items))
	for _, it := range items {
		ns := models.Nameserver{Hostname: it.Hostname.ValueString()}
		if !it.IPAddresses.IsNull() && !it.IPAddresses.IsUnknown() {
			var ips []string
			diags.Append(it.IPAddresses.ElementsAs(ctx, &ips, false)...)
			if diags.HasError() {
				return nil, diags
			}
			ns.IPAddresses = ips
		}
		out = append(out, ns)
	}
	return out, diags
}

func nameserversAPIToList(ctx context.Context, ns []models.Nameserver) (types.List, diag.Diagnostics) {
	_ = ctx
	objType := types.ObjectType{AttrTypes: nameserverAttrTypes}
	if len(ns) == 0 {
		return types.ListValueMust(objType, []attr.Value{}), nil
	}
	var diags diag.Diagnostics
	values := make([]attr.Value, len(ns))
	for i, n := range ns {
		ipVals := make([]attr.Value, len(n.IPAddresses))
		for j, ip := range n.IPAddresses {
			ipVals[j] = types.StringValue(ip)
		}
		ipList, d := types.ListValue(types.StringType, ipVals)
		diags.Append(d...)
		if diags.HasError() {
			return types.ListNull(objType), diags
		}
		obj, d := types.ObjectValue(nameserverAttrTypes, map[string]attr.Value{
			"hostname":     fqdnValue{StringValue: types.StringValue(n.Hostname)},
			"ip_addresses": ipList,
		})
		diags.Append(d...)
		if diags.HasError() {
			return types.ListNull(objType), diags
		}
		values[i] = obj
	}
	l, d := types.ListValue(objType, values)
	diags.Append(d...)
	return l, diags
}

func contactsMapToAPI(ctx context.Context, m types.Map) (map[models.DomainContactType][]models.ContactHandle, diag.Diagnostics) {
	var diags diag.Diagnostics
	if m.IsNull() || m.IsUnknown() {
		return nil, diags
	}
	raw := map[string][]string{}
	diags.Append(m.ElementsAs(ctx, &raw, false)...)
	if diags.HasError() {
		return nil, diags
	}
	out := make(map[models.DomainContactType][]models.ContactHandle, len(raw))
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		ids := raw[k]
		handles := make([]models.ContactHandle, len(ids))
		for i, id := range ids {
			handles[i] = models.ContactHandle{ContactID: models.ContactID(id)}
		}
		out[models.DomainContactType(k)] = handles
	}
	return out, diags
}

// contactsAPIToMap converts the API's flat list of (type,id) contacts into
// the resource's map[type] -> list(id) shape.
//
// expectedKeys, when non-nil, lists contact-type keys that must appear in the
// returned map even if the API didn't echo them back. Missing keys are filled
// with empty lists, so subsequent plans surface the discrepancy as drift
// (rather than the framework rejecting the apply for a "vanished" key).
// Pass nil from data sources where there's no plan to reconcile against.
func contactsAPIToMap(contacts []models.DomainContact, expectedKeys []string) (types.Map, diag.Diagnostics) {
	var diags diag.Diagnostics
	grouped := map[string][]string{}
	for _, c := range contacts {
		key := string(c.ContactType)
		grouped[key] = append(grouped[key], string(c.ContactID))
	}
	for _, k := range expectedKeys {
		if _, ok := grouped[k]; !ok {
			grouped[k] = []string{}
		}
	}
	values := make(map[string]attr.Value, len(grouped))
	for k, ids := range grouped {
		strVals := make([]attr.Value, len(ids))
		for i, id := range ids {
			strVals[i] = types.StringValue(id)
		}
		l, d := types.ListValue(types.StringType, strVals)
		diags.Append(d...)
		if diags.HasError() {
			return types.MapNull(contactsMapElemType), diags
		}
		values[k] = l
	}
	m, d := types.MapValue(contactsMapElemType, values)
	diags.Append(d...)
	return m, diags
}

// contactKeysFromMap returns the keys of `m`, or nil if m is null/unknown.
// Used to remember which contact-type keys the user/state requested so they
// can be preserved through populateDomainResourceModel even when the API
// response omits them.
func contactKeysFromMap(m types.Map) []string {
	if m.IsNull() || m.IsUnknown() {
		return nil
	}
	elems := m.Elements()
	if len(elems) == 0 {
		return nil
	}
	keys := make([]string, 0, len(elems))
	for k := range elems {
		keys = append(keys, k)
	}
	return keys
}

func stringSliceToList(in []string) (types.List, diag.Diagnostics) {
	var diags diag.Diagnostics
	values := make([]attr.Value, len(in))
	for i, s := range in {
		values[i] = types.StringValue(s)
	}
	l, d := types.ListValue(types.StringType, values)
	diags.Append(d...)
	return l, diags
}

func domainHasClientTransferProhibited(d *models.Domain) bool {
	for _, s := range d.RegistryStatuses {
		if s == clientTransferProhibitedStatus {
			return true
		}
	}
	return d.TransferLock
}
