package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure TLDDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &TLDDataSource{}

// TLDDataSource fetches detailed information about a single TLD via
// `GET /v1/tlds/{tld}`. Compared to the `opusdns_tlds` list data source,
// this surfaces the rich nested data (pricing, restrictions, contact
// requirements, nameserver requirements, launch phases) returned by the
// detail endpoint.
type TLDDataSource struct {
	client *opusdns.Client
}

// TLDDataSourceModel is the data-source state shape.
type TLDDataSourceModel struct {
	ID                    types.String `tfsdk:"id"`
	Name                  types.String `tfsdk:"name"`
	Type                  types.String `tfsdk:"type"`
	Available             types.Bool   `tfsdk:"available"`
	RegistrationEnabled   types.Bool   `tfsdk:"registration_enabled"`
	TransferEnabled       types.Bool   `tfsdk:"transfer_enabled"`
	IDNSupported          types.Bool   `tfsdk:"idn_supported"`
	DNSSECSupported       types.Bool   `tfsdk:"dnssec_supported"`
	MinRegistrationPeriod types.Int64  `tfsdk:"min_registration_period"`
	MaxRegistrationPeriod types.Int64  `tfsdk:"max_registration_period"`
	GracePeriodDays       types.Int64  `tfsdk:"grace_period_days"`
	RedemptionPeriodDays  types.Int64  `tfsdk:"redemption_period_days"`
	Description           types.String `tfsdk:"description"`
	Registry              types.String `tfsdk:"registry"`
	WhoisServer           types.String `tfsdk:"whois_server"`
	RDAPServer            types.String `tfsdk:"rdap_server"`
	MinDomainLength       types.Int64  `tfsdk:"min_domain_length"`
	MaxDomainLength       types.Int64  `tfsdk:"max_domain_length"`
	Pricing               types.Object `tfsdk:"pricing"`
	Restrictions          types.Object `tfsdk:"restrictions"`
	NameserverConfig      types.Object `tfsdk:"nameserver_config"`
	ContactConfig         types.List   `tfsdk:"contact_config"`
	Phases                types.List   `tfsdk:"phases"`
}

// Attribute-type maps for the nested objects/lists. Centralised so the
// schema definitions and the value construction stay in lock-step.

var tldPricingAttrTypes = map[string]attr.Type{
	"register_price":  types.StringType,
	"renew_price":     types.StringType,
	"transfer_price":  types.StringType,
	"restore_price":   types.StringType,
	"currency":        types.StringType,
	"premium_pricing": types.BoolType,
}

var tldRestrictionsAttrTypes = map[string]attr.Type{
	"local_presence_required": types.BoolType,
	"requires_verification":   types.BoolType,
	"trademark_required":      types.BoolType,
	"restricted_countries":    types.ListType{ElemType: types.StringType},
	"allowed_countries":       types.ListType{ElemType: types.StringType},
	"registrant_types":        types.ListType{ElemType: types.StringType},
	"notes":                   types.StringType,
}

var tldNameserverConfigAttrTypes = map[string]attr.Type{
	"min":                   types.Int64Type,
	"max":                   types.Int64Type,
	"glue_records_required": types.BoolType,
}

var tldContactConfigAttrTypes = map[string]attr.Type{
	"type":     types.StringType,
	"min":      types.Int64Type,
	"max":      types.Int64Type,
	"required": types.BoolType,
}

var tldPhaseAttrTypes = map[string]attr.Type{
	"name":              types.StringType,
	"status":            types.StringType,
	"start_date":        types.StringType,
	"end_date":          types.StringType,
	"allocation_method": types.StringType,
	"requirements":      types.StringType,
}

// NewTLDDataSource returns a new TLDDataSource.
func NewTLDDataSource() datasource.DataSource {
	return &TLDDataSource{}
}

func (d *TLDDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tld"
}

func (d *TLDDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches detailed information for a single TLD via `GET /v1/tlds/{tld}`, " +
			"including pricing, registration restrictions, nameserver/contact requirements, and " +
			"launch phases.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The TLD name (used as the unique identifier).",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "TLD name without the leading dot (e.g. `com`, `de`, `app`).",
			},
			"type":                    schema.StringAttribute{Computed: true, MarkdownDescription: "TLD type (`gTLD`, `ccTLD`, `newGTLD`, `sponsoredGTLD`)."},
			"available":               schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the TLD is available for registration."},
			"registration_enabled":    schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether new registrations are accepted."},
			"transfer_enabled":        schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether transfers are accepted."},
			"idn_supported":           schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether internationalised domain names are supported."},
			"dnssec_supported":        schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether DNSSEC is supported."},
			"min_registration_period": schema.Int64Attribute{Computed: true, MarkdownDescription: "Minimum registration period in years."},
			"max_registration_period": schema.Int64Attribute{Computed: true, MarkdownDescription: "Maximum registration period in years."},
			"grace_period_days":       schema.Int64Attribute{Computed: true, MarkdownDescription: "Days in the grace period after expiration."},
			"redemption_period_days":  schema.Int64Attribute{Computed: true, MarkdownDescription: "Days in the redemption period."},
			"description":             schema.StringAttribute{Computed: true, MarkdownDescription: "Human-readable description of the TLD."},
			"registry":                schema.StringAttribute{Computed: true, MarkdownDescription: "Name of the registry operator."},
			"whois_server":            schema.StringAttribute{Computed: true, MarkdownDescription: "WHOIS server hostname for the TLD."},
			"rdap_server":             schema.StringAttribute{Computed: true, MarkdownDescription: "RDAP server URL for the TLD."},
			"min_domain_length":       schema.Int64Attribute{Computed: true, MarkdownDescription: "Minimum SLD length permitted."},
			"max_domain_length":       schema.Int64Attribute{Computed: true, MarkdownDescription: "Maximum SLD length permitted."},
			"pricing": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Pricing information for the TLD.",
				Attributes: map[string]schema.Attribute{
					"register_price":  schema.StringAttribute{Computed: true},
					"renew_price":     schema.StringAttribute{Computed: true},
					"transfer_price":  schema.StringAttribute{Computed: true},
					"restore_price":   schema.StringAttribute{Computed: true},
					"currency":        schema.StringAttribute{Computed: true},
					"premium_pricing": schema.BoolAttribute{Computed: true},
				},
			},
			"restrictions": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Registration restrictions for the TLD.",
				Attributes: map[string]schema.Attribute{
					"local_presence_required": schema.BoolAttribute{Computed: true},
					"requires_verification":   schema.BoolAttribute{Computed: true},
					"trademark_required":      schema.BoolAttribute{Computed: true},
					"restricted_countries":    schema.ListAttribute{Computed: true, ElementType: types.StringType},
					"allowed_countries":       schema.ListAttribute{Computed: true, ElementType: types.StringType},
					"registrant_types":        schema.ListAttribute{Computed: true, ElementType: types.StringType},
					"notes":                   schema.StringAttribute{Computed: true},
				},
			},
			"nameserver_config": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Nameserver requirements for the TLD.",
				Attributes: map[string]schema.Attribute{
					"min":                   schema.Int64Attribute{Computed: true, MarkdownDescription: "Minimum number of nameservers required."},
					"max":                   schema.Int64Attribute{Computed: true, MarkdownDescription: "Maximum number of nameservers allowed."},
					"glue_records_required": schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether glue records are required."},
				},
			},
			"contact_config": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Per-contact-role requirements for registration.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type":     schema.StringAttribute{Computed: true, MarkdownDescription: "Contact role (`registrant`, `admin`, `tech`, `billing`)."},
						"min":      schema.Int64Attribute{Computed: true, MarkdownDescription: "Minimum number of contacts of this type."},
						"max":      schema.Int64Attribute{Computed: true, MarkdownDescription: "Maximum number of contacts of this type."},
						"required": schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether this contact type is required."},
					},
				},
			},
			"phases": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Launch phases for the TLD (e.g. sunrise, landrush, general availability).",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name":              schema.StringAttribute{Computed: true},
						"status":            schema.StringAttribute{Computed: true},
						"start_date":        schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp when the phase starts (empty if unset)."},
						"end_date":          schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp when the phase ends (empty if unset)."},
						"allocation_method": schema.StringAttribute{Computed: true, MarkdownDescription: "How domains are allocated during this phase (`fcfs`, `auction`, `lottery`)."},
						"requirements":      schema.StringAttribute{Computed: true, MarkdownDescription: "Special requirements for this phase."},
					},
				},
			},
		},
	}
}

func (d *TLDDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*opusdns.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *opusdns.Client, got: %T.", req.ProviderData),
		)
		return
	}
	d.client = client
}

func (d *TLDDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data TLDDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tld := data.Name.ValueString()
	if tld == "" {
		resp.Diagnostics.AddError(
			"Invalid TLD",
			"The `name` attribute must be a non-empty TLD (e.g. `com`).",
		)
		return
	}

	details, err := d.client.TLDs.GetTLD(ctx, tld)
	if err != nil {
		resp.Diagnostics.AddError("Error fetching TLD details", formatAPIError(err))
		return
	}
	if details == nil {
		resp.Diagnostics.AddError(
			"Empty TLD response",
			fmt.Sprintf("The /v1/tlds/%s endpoint returned an empty result.", tld),
		)
		return
	}

	// Flat scalars.
	data.ID = types.StringValue(details.Name)
	data.Name = types.StringValue(details.Name)
	data.Type = types.StringValue(string(details.Type))
	data.Available = types.BoolValue(details.Available)
	data.RegistrationEnabled = types.BoolValue(details.RegistrationEnabled)
	data.TransferEnabled = types.BoolValue(details.TransferEnabled)
	data.IDNSupported = types.BoolValue(details.IDNSupported)
	data.DNSSECSupported = types.BoolValue(details.DNSSECSupported)
	data.MinRegistrationPeriod = types.Int64Value(int64(details.MinRegistrationPeriod))
	data.MaxRegistrationPeriod = types.Int64Value(int64(details.MaxRegistrationPeriod))
	data.GracePeriodDays = types.Int64Value(int64(details.GracePeriodDays))
	data.RedemptionPeriodDays = types.Int64Value(int64(details.RedemptionPeriodDays))
	data.Description = stringPtrToValue(details.Description)
	data.Registry = stringPtrToValue(details.Registry)
	data.WhoisServer = stringPtrToValue(details.WhoisServer)
	data.RDAPServer = stringPtrToValue(details.RDAPServer)
	data.MinDomainLength = types.Int64Value(int64(details.MinDomainLength))
	data.MaxDomainLength = types.Int64Value(int64(details.MaxDomainLength))

	// Nested objects.
	pricing, diags := buildTLDPricingObject(details.Pricing)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Pricing = pricing

	restrictions, diags := buildTLDRestrictionsObject(details.Restrictions)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Restrictions = restrictions

	nsConfig, diags := buildTLDNameserverConfigObject(details.NameserverConfig)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.NameserverConfig = nsConfig

	contactConfig, diags := buildTLDContactConfigList(details.ContactConfig)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.ContactConfig = contactConfig

	phases, diags := buildTLDPhasesList(details.Phases)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Phases = phases

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// buildTLDPricingObject converts an optional TLDPricing pointer into the
// nested object representation. A nil pointer yields an object with all
// fields set to their zero/null equivalents (rather than a null object)
// so that the schema's static type is satisfied unconditionally.
func buildTLDPricingObject(p *models.TLDPricing) (types.Object, diag.Diagnostics) {
	if p == nil {
		return types.ObjectNull(tldPricingAttrTypes), nil
	}
	obj, diags := types.ObjectValue(tldPricingAttrTypes, map[string]attr.Value{
		"register_price":  types.StringValue(p.RegisterPrice),
		"renew_price":     types.StringValue(p.RenewPrice),
		"transfer_price":  types.StringValue(p.TransferPrice),
		"restore_price":   types.StringValue(p.RestorePrice),
		"currency":        types.StringValue(string(p.Currency)),
		"premium_pricing": types.BoolValue(p.PremiumPricing),
	})
	return obj, diags
}

func buildTLDRestrictionsObject(r *models.TLDRestrictions) (types.Object, diag.Diagnostics) {
	if r == nil {
		return types.ObjectNull(tldRestrictionsAttrTypes), nil
	}
	restricted, diags := stringSliceToList(r.RestrictedCountries)
	if diags.HasError() {
		return types.ObjectNull(tldRestrictionsAttrTypes), diags
	}
	allowed, diags2 := stringSliceToList(r.AllowedCountries)
	if diags2.HasError() {
		return types.ObjectNull(tldRestrictionsAttrTypes), diags2
	}
	regTypes, diags3 := stringSliceToList(r.RegistrantTypes)
	if diags3.HasError() {
		return types.ObjectNull(tldRestrictionsAttrTypes), diags3
	}
	obj, diags4 := types.ObjectValue(tldRestrictionsAttrTypes, map[string]attr.Value{
		"local_presence_required": types.BoolValue(r.LocalPresenceRequired),
		"requires_verification":   types.BoolValue(r.RequiresVerification),
		"trademark_required":      types.BoolValue(r.TrademarkRequired),
		"restricted_countries":    restricted,
		"allowed_countries":       allowed,
		"registrant_types":        regTypes,
		"notes":                   stringPtrToValue(r.Notes),
	})
	return obj, diags4
}

func buildTLDNameserverConfigObject(n *models.NameserverConfig) (types.Object, diag.Diagnostics) {
	if n == nil {
		return types.ObjectNull(tldNameserverConfigAttrTypes), nil
	}
	obj, diags := types.ObjectValue(tldNameserverConfigAttrTypes, map[string]attr.Value{
		"min":                   types.Int64Value(int64(n.Min)),
		"max":                   types.Int64Value(int64(n.Max)),
		"glue_records_required": types.BoolValue(n.GlueRecordsRequired),
	})
	return obj, diags
}

func buildTLDContactConfigList(cc []models.ContactConfig) (types.List, diag.Diagnostics) {
	objType := types.ObjectType{AttrTypes: tldContactConfigAttrTypes}
	values := make([]attr.Value, len(cc))
	for i, c := range cc {
		obj, diags := types.ObjectValue(tldContactConfigAttrTypes, map[string]attr.Value{
			"type":     types.StringValue(string(c.Type)),
			"min":      types.Int64Value(int64(c.Min)),
			"max":      types.Int64Value(int64(c.Max)),
			"required": types.BoolValue(c.Required),
		})
		if diags.HasError() {
			return types.ListNull(objType), diags
		}
		values[i] = obj
	}
	list, diags := types.ListValue(objType, values)
	return list, diags
}

func buildTLDPhasesList(phases []models.TLDPhase) (types.List, diag.Diagnostics) {
	objType := types.ObjectType{AttrTypes: tldPhaseAttrTypes}
	values := make([]attr.Value, len(phases))
	for i, p := range phases {
		startDate := ""
		if p.StartDate != nil {
			startDate = p.StartDate.Format("2006-01-02T15:04:05Z07:00")
		}
		endDate := ""
		if p.EndDate != nil {
			endDate = p.EndDate.Format("2006-01-02T15:04:05Z07:00")
		}
		requirements := ""
		if p.Requirements != nil {
			requirements = *p.Requirements
		}
		obj, diags := types.ObjectValue(tldPhaseAttrTypes, map[string]attr.Value{
			"name":              types.StringValue(string(p.Name)),
			"status":            types.StringValue(p.Status),
			"start_date":        types.StringValue(startDate),
			"end_date":          types.StringValue(endDate),
			"allocation_method": types.StringValue(string(p.AllocationMethod)),
			"requirements":      types.StringValue(requirements),
		})
		if diags.HasError() {
			return types.ListNull(objType), diags
		}
		values[i] = obj
	}
	list, diags := types.ListValue(objType, values)
	return list, diags
}
