package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure TLDsDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &TLDsDataSource{}

// TLDsDataSource lists all TLDs from the registry, optionally filtered.
type TLDsDataSource struct {
	client *opusdns.Client
}

// TLDsDataSourceModel is the top-level data-source state shape.
type TLDsDataSourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Search              types.String `tfsdk:"search"`
	Type                types.String `tfsdk:"type"`
	Available           types.Bool   `tfsdk:"available"`
	RegistrationEnabled types.Bool   `tfsdk:"registration_enabled"`
	DNSSECSupported     types.Bool   `tfsdk:"dnssec_supported"`
	TLDs                types.List   `tfsdk:"tlds"`
}

// tldItemAttrTypes is the shape of each entry in the `tlds` list.
//
// We surface only the flat scalar fields here. Rich nested data
// (contact_config, local_presence, phases) is available via the
// single `opusdns_tld` data source.
var tldItemAttrTypes = map[string]attr.Type{
	"name":                    types.StringType,
	"type":                    types.StringType,
	"available":               types.BoolType,
	"registration_enabled":    types.BoolType,
	"transfer_enabled":        types.BoolType,
	"idn_supported":           types.BoolType,
	"dnssec_supported":        types.BoolType,
	"min_registration_period": types.Int64Type,
	"max_registration_period": types.Int64Type,
}

// NewTLDsDataSource returns a new TLDsDataSource.
func NewTLDsDataSource() datasource.DataSource {
	return &TLDsDataSource{}
}

func (d *TLDsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tlds"
}

func (d *TLDsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all TLDs supported by the OpusDNS registry. Optional filters " +
			"narrow the list by name substring, TLD type, availability, or DNSSEC " +
			"support. Use the singular `opusdns_tld` data source for rich per-TLD " +
			"details (contacts, local presence, phases).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Static identifier for this data source.",
			},
			"search": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional case-insensitive substring filter on TLD name.",
			},
			"type": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional filter on TLD type. One of `gTLD`, `ccTLD`, `newGTLD`, or `sponsoredGTLD`.",
			},
			"available": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Optional filter: only return TLDs whose `available` flag matches.",
			},
			"registration_enabled": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Deprecated: the OpusDNS API has no registration-enabled flag, so this filter has no effect. Use `available`.",
				DeprecationMessage:  tldRegistrationEnabledDeprecation,
			},
			"dnssec_supported": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Optional filter: only return TLDs whose `dnssec_supported` flag matches.",
			},
			"tlds": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "The list of TLDs matching the supplied filters.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name":      schema.StringAttribute{Computed: true, MarkdownDescription: "TLD name without leading dot (e.g. `com`)."},
						"type":      schema.StringAttribute{Computed: true, MarkdownDescription: "TLD type (`gTLD`, `ccTLD`, `newGTLD`, `sponsoredGTLD`)."},
						"available": schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the TLD is available for registration."},
						"registration_enabled": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Deprecated: always null. The OpusDNS API has no registration-enabled flag; use `available`.",
							DeprecationMessage:  tldRegistrationEnabledDeprecation,
						},
						"transfer_enabled": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Deprecated: always null. The OpusDNS API has no transfer-enabled flag.",
							DeprecationMessage:  "The OpusDNS API has no transfer-enabled flag; this attribute is always null and will be removed in a future release.",
						},
						"idn_supported":           schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether internationalised domain names are supported."},
						"dnssec_supported":        schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether DNSSEC is supported."},
						"min_registration_period": schema.Int64Attribute{Computed: true, MarkdownDescription: "Shortest registration period in whole years. Null when the TLD offers no whole-year period."},
						"max_registration_period": schema.Int64Attribute{Computed: true, MarkdownDescription: "Longest registration period in whole years. Null when the TLD offers no whole-year period."},
					},
				},
			},
		},
	}
}

func (d *TLDsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *TLDsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data TLDsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tlds, err := d.client.TLDs.ListTLDs(ctx, nil)
	if err != nil {
		resp.Diagnostics.AddError("Error listing TLDs", formatAPIError(err))
		return
	}

	if !data.RegistrationEnabled.IsNull() && !data.RegistrationEnabled.IsUnknown() {
		resp.Diagnostics.AddAttributeWarning(path.Root("registration_enabled"),
			"Filter has no effect", tldRegistrationEnabledDeprecation)
	}
	tlds = filterTLDs(tlds, data)

	objType := types.ObjectType{AttrTypes: tldItemAttrTypes}
	values := make([]attr.Value, len(tlds))
	for i, t := range tlds {
		obj, diags := types.ObjectValue(tldItemAttrTypes, map[string]attr.Value{
			"name":                    types.StringValue(t.Name),
			"type":                    types.StringValue(string(t.Type)),
			"available":               types.BoolValue(t.Available),
			"registration_enabled":    types.BoolNull(),
			"transfer_enabled":        types.BoolNull(),
			"idn_supported":           types.BoolValue(t.IDNSupported),
			"dnssec_supported":        types.BoolValue(t.DNSSECSupported),
			"min_registration_period": registrationYearsValue(t.MinRegistrationPeriod),
			"max_registration_period": registrationYearsValue(t.MaxRegistrationPeriod),
		})
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		values[i] = obj
	}

	list, diags := types.ListValue(objType, values)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue("tlds")
	data.TLDs = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

const tldRegistrationEnabledDeprecation = "The OpusDNS API has no registration-enabled flag, so this has no effect and will be removed in a future release. Use `available`."

// filterTLDs applies the data source's filters client-side: the list endpoint
// only filters by TLD name. The deprecated `registration_enabled` filter is
// ignored, because the API has no such flag.
func filterTLDs(tlds []models.TLD, data TLDsDataSourceModel) []models.TLD {
	filtered := make([]models.TLD, 0, len(tlds))
	for _, t := range tlds {
		if search := strings.ToLower(data.Search.ValueString()); search != "" {
			if !strings.Contains(strings.ToLower(t.Name), search) {
				continue
			}
		}
		if v := data.Type.ValueString(); v != "" {
			if string(t.Type) != v {
				continue
			}
		}
		if !data.Available.IsNull() && !data.Available.IsUnknown() {
			if t.Available != data.Available.ValueBool() {
				continue
			}
		}
		if !data.DNSSECSupported.IsNull() && !data.DNSSECSupported.IsUnknown() {
			if t.DNSSECSupported != data.DNSSECSupported.ValueBool() {
				continue
			}
		}
		filtered = append(filtered, t)
	}
	return filtered
}

// registrationYearsValue maps the SDK's period in whole years to state; 0
// means the TLD offers no whole-year period.
func registrationYearsValue(years int) types.Int64 {
	if years == 0 {
		return types.Int64Null()
	}
	return types.Int64Value(int64(years))
}
