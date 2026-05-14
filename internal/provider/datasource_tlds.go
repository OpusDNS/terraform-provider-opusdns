package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
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
// (pricing, restrictions, contact_config, phases) is available via the
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
		MarkdownDescription: "Lists all TLDs supported by the OpusDNS registry. Server-side filters " +
			"can narrow the list by name substring, TLD type, availability, " +
			"registration-enabled status, or DNSSEC support. Use the singular " +
			"`opusdns_tld` data source for rich per-TLD details (pricing, restrictions, phases).",
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
				MarkdownDescription: "Optional filter: only return TLDs whose `registration_enabled` flag matches.",
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
						"name":                    schema.StringAttribute{Computed: true, MarkdownDescription: "TLD name without leading dot (e.g. `com`)."},
						"type":                    schema.StringAttribute{Computed: true, MarkdownDescription: "TLD type (`gTLD`, `ccTLD`, `newGTLD`, `sponsoredGTLD`)."},
						"available":               schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the TLD is available for registration."},
						"registration_enabled":    schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether new registrations are accepted."},
						"transfer_enabled":        schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether transfers are accepted."},
						"idn_supported":           schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether internationalised domain names are supported."},
						"dnssec_supported":        schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether DNSSEC is supported."},
						"min_registration_period": schema.Int64Attribute{Computed: true, MarkdownDescription: "Minimum registration period in years."},
						"max_registration_period": schema.Int64Attribute{Computed: true, MarkdownDescription: "Maximum registration period in years."},
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

	opts := &models.ListTLDsOptions{}
	if v := data.Search.ValueString(); v != "" {
		opts.Search = v
	}
	if v := data.Type.ValueString(); v != "" {
		opts.Type = models.TLDType(v)
	}
	if !data.Available.IsNull() && !data.Available.IsUnknown() {
		b := data.Available.ValueBool()
		opts.Available = &b
	}
	if !data.RegistrationEnabled.IsNull() && !data.RegistrationEnabled.IsUnknown() {
		b := data.RegistrationEnabled.ValueBool()
		opts.RegistrationEnabled = &b
	}
	if !data.DNSSECSupported.IsNull() && !data.DNSSECSupported.IsUnknown() {
		b := data.DNSSECSupported.ValueBool()
		opts.DNSSECSupported = &b
	}

	tlds, err := d.client.TLDs.ListTLDs(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error listing TLDs", formatAPIError(err))
		return
	}

	// Apply client-side filters as a defence-in-depth: empirically the
	// upstream API ignores the `type`/`available`/`registration_enabled`/
	// `dnssec_supported` query parameters and returns the full list
	// regardless. Re-filter here so the data source's behaviour matches
	// its documented schema even if the server changes.
	filtered := tlds[:0]
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
		if !data.RegistrationEnabled.IsNull() && !data.RegistrationEnabled.IsUnknown() {
			if t.RegistrationEnabled != data.RegistrationEnabled.ValueBool() {
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
	tlds = filtered

	objType := types.ObjectType{AttrTypes: tldItemAttrTypes}
	values := make([]attr.Value, len(tlds))
	for i, t := range tlds {
		obj, diags := types.ObjectValue(tldItemAttrTypes, map[string]attr.Value{
			"name":                    types.StringValue(t.Name),
			"type":                    types.StringValue(string(t.Type)),
			"available":               types.BoolValue(t.Available),
			"registration_enabled":    types.BoolValue(t.RegistrationEnabled),
			"transfer_enabled":        types.BoolValue(t.TransferEnabled),
			"idn_supported":           types.BoolValue(t.IDNSupported),
			"dnssec_supported":        types.BoolValue(t.DNSSECSupported),
			"min_registration_period": types.Int64Value(int64(t.MinRegistrationPeriod)),
			"max_registration_period": types.Int64Value(int64(t.MaxRegistrationPeriod)),
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
