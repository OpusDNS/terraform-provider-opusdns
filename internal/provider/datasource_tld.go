package provider

import (
	"context"
	"fmt"
	"strings"

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

// TLDDataSource fetches the specification of a single TLD via
// `GET /v1/tlds/{tld}`. Compared to the `opusdns_tlds` list data source, this
// surfaces the rich nested data (registration periods, nameserver and contact
// requirements, local presence, launch phases) returned by the detail
// endpoint.
type TLDDataSource struct {
	client *opusdns.Client
}

// TLDDataSourceModel is the data-source state shape.
type TLDDataSourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Type                types.String `tfsdk:"type"`
	IDNSupported        types.Bool   `tfsdk:"idn_supported"`
	DNSSECSupported     types.Bool   `tfsdk:"dnssec_supported"`
	RegistrationPeriods types.List   `tfsdk:"registration_periods"`
	RenewalPeriods      types.List   `tfsdk:"renewal_periods"`
	GracePeriod         types.String `tfsdk:"grace_period"`
	RedemptionPeriod    types.String `tfsdk:"redemption_period"`
	WhoisServer         types.String `tfsdk:"whois_server"`
	RDAPServer          types.String `tfsdk:"rdap_server"`
	MinDomainLength     types.Int64  `tfsdk:"min_domain_length"`
	MaxDomainLength     types.Int64  `tfsdk:"max_domain_length"`
	LocalPresence       types.Object `tfsdk:"local_presence"`
	NameserverConfig    types.Object `tfsdk:"nameserver_config"`
	ContactConfig       types.List   `tfsdk:"contact_config"`
	Phases              types.List   `tfsdk:"phases"`
}

// Attribute-type maps for the nested objects/lists. Centralised so the
// schema definitions and the value construction stay in lock-step.

var tldPeriodAttrTypes = map[string]attr.Type{
	"value": types.Int64Type,
	"unit":  types.StringType,
}

var tldLocalPresenceAttrTypes = map[string]attr.Type{
	"required":           types.BoolType,
	"eligible_countries": types.ListType{ElemType: types.StringType},
}

var tldNameserverConfigAttrTypes = map[string]attr.Type{
	"min": types.Int64Type,
	"max": types.Int64Type,
}

var tldContactConfigAttrTypes = map[string]attr.Type{
	"type":     types.StringType,
	"min":      types.Int64Type,
	"max":      types.Int64Type,
	"required": types.BoolType,
}

var tldPhaseAttrTypes = map[string]attr.Type{
	"name":              types.StringType,
	"supported":         types.BoolType,
	"start_date":        types.StringType,
	"end_date":          types.StringType,
	"allocation_method": types.StringType,
	"smd_required":      types.BoolType,
}

// NewTLDDataSource returns a new TLDDataSource.
func NewTLDDataSource() datasource.DataSource {
	return &TLDDataSource{}
}

func (d *TLDDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tld"
}

func (d *TLDDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	periodList := func(description string) schema.ListNestedAttribute {
		return schema.ListNestedAttribute{
			Computed:            true,
			MarkdownDescription: description,
			NestedObject: schema.NestedAttributeObject{
				Attributes: map[string]schema.Attribute{
					"value": schema.Int64Attribute{Computed: true, MarkdownDescription: "Length of the period in `unit`."},
					"unit":  schema.StringAttribute{Computed: true, MarkdownDescription: "Period unit: `y` (years), `m` (months) or `d` (days)."},
				},
			},
		}
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches the specification of a single TLD via `GET /v1/tlds/{tld}`, " +
			"including registration and renewal periods, grace periods, nameserver and contact " +
			"requirements, local presence rules and launch phases. Pricing is not part of the TLD " +
			"specification and is not exposed by this data source.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The TLD name (used as the unique identifier).",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "TLD name without the leading dot (e.g. `com`, `de`, `app`).",
			},
			"type":                 schema.StringAttribute{Computed: true, MarkdownDescription: "TLD type (`gTLD` or `ccTLD`). Null when the specification does not state it."},
			"idn_supported":        schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether internationalised domain names are supported."},
			"dnssec_supported":     schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether DNSSEC can be enabled."},
			"registration_periods": periodList("Periods a domain can be registered for."),
			"renewal_periods":      periodList("Periods a domain can be renewed for."),
			"grace_period":         schema.StringAttribute{Computed: true, MarkdownDescription: "Grace period after expiration as an ISO 8601 duration (e.g. `P30D`). Null when the TLD has none."},
			"redemption_period":    schema.StringAttribute{Computed: true, MarkdownDescription: "Redemption period as an ISO 8601 duration (e.g. `P30D`). Null when the TLD has none."},
			"whois_server":         schema.StringAttribute{Computed: true, MarkdownDescription: "WHOIS server hostname for the TLD. Null when the registry has none."},
			"rdap_server":          schema.StringAttribute{Computed: true, MarkdownDescription: "RDAP server URL for the TLD. Null when the registry has none."},
			"min_domain_length":    schema.Int64Attribute{Computed: true, MarkdownDescription: "Minimum SLD length permitted."},
			"max_domain_length":    schema.Int64Attribute{Computed: true, MarkdownDescription: "Maximum SLD length permitted."},
			"local_presence": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Local presence requirements of the TLD. Null when the specification defines none.",
				Attributes: map[string]schema.Attribute{
					"required":           schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether a local presence is required."},
					"eligible_countries": schema.ListAttribute{Computed: true, ElementType: types.StringType, MarkdownDescription: "Countries that satisfy the requirement."},
				},
			},
			"nameserver_config": schema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Nameserver requirements for the TLD.",
				Attributes: map[string]schema.Attribute{
					"min": schema.Int64Attribute{Computed: true, MarkdownDescription: "Minimum number of nameservers required."},
					"max": schema.Int64Attribute{Computed: true, MarkdownDescription: "Maximum number of nameservers allowed."},
				},
			},
			"contact_config": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Per-contact-role requirements for registration. Empty when the specification defines none.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"type":     schema.StringAttribute{Computed: true, MarkdownDescription: "Contact role (`registrant`, `admin`, `tech`, `billing`)."},
						"min":      schema.Int64Attribute{Computed: true, MarkdownDescription: "Minimum number of contacts of this type."},
						"max":      schema.Int64Attribute{Computed: true, MarkdownDescription: "Maximum number of contacts of this type."},
						"required": schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether this contact type is required (`min` is greater than zero)."},
					},
				},
			},
			"phases": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Launch phases before general availability (e.g. sunrise, landrush). Empty when the specification defines none.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name":              schema.StringAttribute{Computed: true, MarkdownDescription: "Phase type (`sunrise`, `landrush`, `eap`)."},
						"supported":         schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether OpusDNS supports the phase."},
						"start_date":        schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp when the phase starts (null if unset)."},
						"end_date":          schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp when the phase ends (null if unset)."},
						"allocation_method": schema.StringAttribute{Computed: true, MarkdownDescription: "How domains are allocated during this phase (`fcfs`, `auction`, `lottery`). Null if unset."},
						"smd_required":      schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether a signed mark data (SMD) file is required. Null if unset."},
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

	spec, err := d.client.TLDs.GetTLD(ctx, tld)
	if err != nil {
		resp.Diagnostics.AddError("Error fetching TLD details", formatAPIError(err))
		return
	}
	if spec == nil {
		resp.Diagnostics.AddError(
			"Empty TLD response",
			fmt.Sprintf("The /v1/tlds/%s endpoint returned an empty result.", tld),
		)
		return
	}

	// One specification can cover several TLDs (e.g. a ccTLD and its
	// second-level zones); pick the entry for the requested TLD to read the
	// type. `name` is left as configured.
	data.ID = types.StringValue(tld)
	data.Type = types.StringNull()
	if info := findTLDInfo(spec.TLDs, tld); info != nil && info.Type != "" {
		data.Type = types.StringValue(string(info.Type))
	}

	// Flat scalars.
	data.IDNSupported = types.BoolValue(spec.IDN.IDNCapable)
	data.DNSSECSupported = types.BoolValue(spec.DNSConfiguration.DNSSECAllowed)
	data.GracePeriod = stringPtrToValue(spec.DomainLifecycle.GracePeriod)
	data.RedemptionPeriod = stringPtrToValue(spec.DomainLifecycle.RedemptionPeriod)
	data.WhoisServer = types.StringNull()
	if spec.Whois != nil {
		data.WhoisServer = stringPtrToValue(spec.Whois.WhoisServer)
	}
	data.RDAPServer = types.StringNull()
	if spec.RDAP != nil {
		data.RDAPServer = stringPtrToValue(spec.RDAP.RDAPServer)
	}
	data.MinDomainLength = types.Int64Value(int64(spec.Characters.Min))
	data.MaxDomainLength = types.Int64Value(int64(spec.Characters.Max))

	// Nested objects and lists.
	registrationPeriods, diags := buildTLDPeriodList(spec.DomainLifecycle.RegistrationPeriods)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.RegistrationPeriods = registrationPeriods

	renewalPeriods, diags := buildTLDPeriodList(spec.DomainLifecycle.RenewalPeriods)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.RenewalPeriods = renewalPeriods

	localPresence, diags := buildTLDLocalPresenceObject(spec.LocalPresence)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.LocalPresence = localPresence

	nsConfig, diags := buildTLDNameserverConfigObject(spec.DNSConfiguration.AllowedNumberOfNameservers)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.NameserverConfig = nsConfig

	var roles []models.TLDContactRole
	if spec.Contacts != nil {
		roles = spec.Contacts.SupportedRoles
	}
	contactConfig, diags := buildTLDContactConfigList(roles)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.ContactConfig = contactConfig

	var launchPhases []models.TLDLaunchPhase
	if spec.LaunchPhases != nil {
		launchPhases = spec.LaunchPhases.Phases
	}
	phases, diags := buildTLDPhasesList(launchPhases)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data.Phases = phases

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// findTLDInfo returns the entry of infos named tld (ignoring case and a
// leading dot), falling back to the only entry when the specification covers
// a single TLD under a different spelling. It returns nil when there is no
// unambiguous match.
func findTLDInfo(infos []models.TLDInfo, tld string) *models.TLDInfo {
	want := strings.ToLower(strings.TrimPrefix(tld, "."))
	for i := range infos {
		if strings.ToLower(strings.TrimPrefix(infos[i].Name, ".")) == want {
			return &infos[i]
		}
	}
	if len(infos) == 1 {
		return &infos[0]
	}
	return nil
}

func buildTLDPeriodList(periods []models.DomainPeriod) (types.List, diag.Diagnostics) {
	objType := types.ObjectType{AttrTypes: tldPeriodAttrTypes}
	values := make([]attr.Value, len(periods))
	for i, p := range periods {
		obj, diags := types.ObjectValue(tldPeriodAttrTypes, map[string]attr.Value{
			"value": types.Int64Value(int64(p.Value)),
			"unit":  types.StringValue(string(p.Unit)),
		})
		if diags.HasError() {
			return types.ListNull(objType), diags
		}
		values[i] = obj
	}
	return types.ListValue(objType, values)
}

// buildTLDLocalPresenceObject converts the optional local presence block into
// the nested object representation. A nil block yields a null object.
func buildTLDLocalPresenceObject(lp *models.TLDLocalPresence) (types.Object, diag.Diagnostics) {
	if lp == nil {
		return types.ObjectNull(tldLocalPresenceAttrTypes), nil
	}
	countries, diags := stringSliceToList(lp.EligibleCountries)
	if diags.HasError() {
		return types.ObjectNull(tldLocalPresenceAttrTypes), diags
	}
	return types.ObjectValue(tldLocalPresenceAttrTypes, map[string]attr.Value{
		"required":           types.BoolValue(lp.Required),
		"eligible_countries": countries,
	})
}

func buildTLDNameserverConfigObject(n models.TLDRange) (types.Object, diag.Diagnostics) {
	return types.ObjectValue(tldNameserverConfigAttrTypes, map[string]attr.Value{
		"min": types.Int64Value(int64(n.Min)),
		"max": types.Int64Value(int64(n.Max)),
	})
}

// buildTLDContactConfigList converts the contact roles a registration takes.
// A role is required when at least one contact of that role is needed.
func buildTLDContactConfigList(roles []models.TLDContactRole) (types.List, diag.Diagnostics) {
	objType := types.ObjectType{AttrTypes: tldContactConfigAttrTypes}
	values := make([]attr.Value, len(roles))
	for i, r := range roles {
		obj, diags := types.ObjectValue(tldContactConfigAttrTypes, map[string]attr.Value{
			"type":     types.StringValue(string(r.Type)),
			"min":      types.Int64Value(int64(r.Min)),
			"max":      types.Int64Value(int64(r.Max)),
			"required": types.BoolValue(r.Min > 0),
		})
		if diags.HasError() {
			return types.ListNull(objType), diags
		}
		values[i] = obj
	}
	return types.ListValue(objType, values)
}

func buildTLDPhasesList(phases []models.TLDLaunchPhase) (types.List, diag.Diagnostics) {
	objType := types.ObjectType{AttrTypes: tldPhaseAttrTypes}
	values := make([]attr.Value, len(phases))
	for i, p := range phases {
		allocation := types.StringNull()
		if p.Allocation != nil {
			allocation = types.StringValue(string(*p.Allocation))
		}
		smdRequired := types.BoolNull()
		if p.SMDRequired != nil {
			smdRequired = types.BoolValue(*p.SMDRequired)
		}
		obj, diags := types.ObjectValue(tldPhaseAttrTypes, map[string]attr.Value{
			"name":              types.StringValue(string(p.Type)),
			"supported":         types.BoolValue(p.Supported),
			"start_date":        timePtrToValue(p.StartDate),
			"end_date":          timePtrToValue(p.EndDate),
			"allocation_method": allocation,
			"smd_required":      smdRequired,
		})
		if diags.HasError() {
			return types.ListNull(objType), diags
		}
		values[i] = obj
	}
	return types.ListValue(objType, values)
}
