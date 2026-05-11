package provider

import (
	"context"
	"fmt"

	opusdns "github.com/opusdns/opusdns-go-client/opusdns"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var _ resource.Resource = &DomainResource{}

type DomainResource struct {
	client *opusdns.Client
}

type DomainContactsModel struct {
	Registrant types.String `tfsdk:"registrant"`
	Admin      types.String `tfsdk:"admin"`
	Tech       types.String `tfsdk:"tech"`
	Billing    types.String `tfsdk:"billing"`
}

type DomainResourceModel struct {
	ID           types.String `tfsdk:"id"`
	DomainID     types.String `tfsdk:"domain_id"`
	Name         types.String `tfsdk:"name"`
	Period       types.Int64  `tfsdk:"period"`
	RenewalMode  types.String `tfsdk:"renewal_mode"`
	Contacts     types.Object `tfsdk:"contacts"`
	Nameservers  types.List   `tfsdk:"nameservers"`
	TransferLock types.Bool   `tfsdk:"transfer_lock"`
	CreateZone   types.Bool   `tfsdk:"create_zone"`
	ExpiresOn    types.String `tfsdk:"expires_on"`
	RegisteredOn types.String `tfsdk:"registered_on"`
	Status       types.String `tfsdk:"status"`
}

var contactsAttrTypes = map[string]attr.Type{
	"registrant": types.StringType,
	"admin":      types.StringType,
	"tech":       types.StringType,
	"billing":    types.StringType,
}

func NewDomainResource() resource.Resource {
	return &DomainResource{}
}

func (r *DomainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (r *DomainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a domain in OpusDNS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"domain_id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The domain name.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"period": schema.Int64Attribute{
				Required:    true,
				Description: "Registration period in years.",
			},
			"renewal_mode": schema.StringAttribute{
				Required:    true,
				Description: "Renewal mode: 'renew' or 'expire'.",
			},
			"contacts": schema.SingleNestedAttribute{
				Required:    true,
				Description: "Contact handles for the domain.",
				Attributes: map[string]schema.Attribute{
					"registrant": schema.StringAttribute{
						Required:    true,
						Description: "Registrant contact ID.",
					},
					"admin": schema.StringAttribute{
						Required:    true,
						Description: "Admin contact ID.",
					},
					"tech": schema.StringAttribute{
						Required:    true,
						Description: "Tech contact ID.",
					},
					"billing": schema.StringAttribute{
						Required:    true,
						Description: "Billing contact ID.",
					},
				},
			},
			"nameservers": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of nameserver hostnames for the domain.",
			},
			"transfer_lock": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether transfer lock is enabled.",
			},
			"create_zone": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether to create a DNS zone for the domain on registration.",
			},
			"expires_on": schema.StringAttribute{
				Computed:    true,
				Description: "The date the domain expires.",
			},
			"registered_on": schema.StringAttribute{
				Computed:    true,
				Description: "The date the domain was registered.",
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "The primary status of the domain.",
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
		resp.Diagnostics.AddError("Unexpected provider data type", fmt.Sprintf("Expected *opusdns.Client, got %T", req.ProviderData))
		return
	}
	r.client = client
}

func (r *DomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DomainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	contacts, diags := expandContacts(ctx, plan.Contacts)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq := &models.DomainCreateRequest{
		Name:        plan.Name.ValueString(),
		Contacts:    contacts,
		RenewalMode: models.RenewalMode(plan.RenewalMode.ValueString()),
		Period: models.DomainPeriod{
			Value: int(plan.Period.ValueInt64()),
			Unit:  models.PeriodUnitYear,
		},
		CreateZone: plan.CreateZone.ValueBool(),
	}

	if !plan.Nameservers.IsNull() && !plan.Nameservers.IsUnknown() {
		var nsList []string
		resp.Diagnostics.Append(plan.Nameservers.ElementsAs(ctx, &nsList, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		createReq.Nameservers = stringsToNameservers(nsList)
	}

	domain, err := r.client.Domains.CreateDomain(ctx, createReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create domain", err.Error())
		return
	}

	resp.Diagnostics.Append(flattenDomain(ctx, domain, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DomainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domain, err := r.client.Domains.GetDomain(ctx, state.Name.ValueString())
	if err != nil {
		if opusdns.IsNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read domain", err.Error())
		return
	}

	resp.Diagnostics.Append(flattenDomain(ctx, domain, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state DomainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	contacts, diags := expandContacts(ctx, plan.Contacts)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	renewalMode := models.RenewalMode(plan.RenewalMode.ValueString())
	updateReq := &models.DomainUpdateRequest{
		Contacts:    contacts,
		RenewalMode: &renewalMode,
	}

	if !plan.Nameservers.IsNull() && !plan.Nameservers.IsUnknown() {
		var nsList []string
		resp.Diagnostics.Append(plan.Nameservers.ElementsAs(ctx, &nsList, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		updateReq.Nameservers = stringsToNameservers(nsList)
	}

	domain, err := r.client.Domains.UpdateDomain(ctx, state.Name.ValueString(), updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update domain", err.Error())
		return
	}

	resp.Diagnostics.Append(flattenDomain(ctx, domain, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete removes the domain resource from state only; domains cannot be deleted via API.
func (r *DomainResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func expandContacts(ctx context.Context, obj types.Object) (map[models.DomainContactType][]models.ContactHandle, diag.Diagnostics) {
	var c DomainContactsModel
	diags := obj.As(ctx, &c, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}

	return map[models.DomainContactType][]models.ContactHandle{
		models.DomainContactTypeRegistrant: {{ContactID: models.ContactID(c.Registrant.ValueString())}},
		models.DomainContactTypeAdmin:      {{ContactID: models.ContactID(c.Admin.ValueString())}},
		models.DomainContactTypeTech:       {{ContactID: models.ContactID(c.Tech.ValueString())}},
		models.DomainContactTypeBilling:    {{ContactID: models.ContactID(c.Billing.ValueString())}},
	}, nil
}

func stringsToNameservers(hostnames []string) []models.Nameserver {
	ns := make([]models.Nameserver, len(hostnames))
	for i, h := range hostnames {
		ns[i] = models.Nameserver{Hostname: h}
	}
	return ns
}

func flattenDomain(ctx context.Context, domain *models.Domain, model *DomainResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	model.ID = types.StringValue(domain.Name)
	model.DomainID = types.StringValue(string(domain.DomainID))
	model.Name = types.StringValue(domain.Name)
	model.RenewalMode = types.StringValue(string(domain.RenewalMode))
	model.TransferLock = types.BoolValue(domain.TransferLock)

	if domain.ExpiresOn != nil {
		model.ExpiresOn = types.StringValue(domain.ExpiresOn.String())
	} else {
		model.ExpiresOn = types.StringValue("")
	}

	if domain.RegisteredOn != nil {
		model.RegisteredOn = types.StringValue(domain.RegisteredOn.String())
	} else {
		model.RegisteredOn = types.StringValue("")
	}

	if len(domain.RegistryStatuses) > 0 {
		model.Status = types.StringValue(domain.RegistryStatuses[0])
	} else {
		model.Status = types.StringValue("")
	}

	// Flatten nameservers as a list of hostnames
	nsHostnames := make([]attr.Value, len(domain.Nameservers))
	for i, ns := range domain.Nameservers {
		nsHostnames[i] = types.StringValue(ns.Hostname)
	}
	nsList, d := types.ListValue(types.StringType, nsHostnames)
	diags.Append(d...)
	model.Nameservers = nsList

	// Flatten contacts from domain.Contacts slice
	registrant, admin, tech, billing := "", "", "", ""
	for _, dc := range domain.Contacts {
		switch dc.ContactType {
		case models.DomainContactTypeRegistrant:
			registrant = string(dc.ContactID)
		case models.DomainContactTypeAdmin:
			admin = string(dc.ContactID)
		case models.DomainContactTypeTech:
			tech = string(dc.ContactID)
		case models.DomainContactTypeBilling:
			billing = string(dc.ContactID)
		}
	}

	contactsObj, d := types.ObjectValue(contactsAttrTypes, map[string]attr.Value{
		"registrant": types.StringValue(registrant),
		"admin":      types.StringValue(admin),
		"tech":       types.StringValue(tech),
		"billing":    types.StringValue(billing),
	})
	diags.Append(d...)
	model.Contacts = contactsObj

	return diags
}
