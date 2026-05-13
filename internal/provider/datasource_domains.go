package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure DomainsDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &DomainsDataSource{}

// DomainsDataSource lists domains in the authenticated caller's organization
// via `GET /v1/domains` (SDK: Domains.ListDomains, which auto-paginates).
// Optional filter inputs map onto models.ListDomainsOptions.
type DomainsDataSource struct {
	client *opusdns.Client
}

// DomainsDataSourceModel exposes the most useful server-side filters as
// inputs and a `domains` list of fully-populated objects as the result.
type DomainsDataSourceModel struct {
	ID           types.String `tfsdk:"id"`
	Search       types.String `tfsdk:"search"`
	Name         types.String `tfsdk:"name"`
	TLD          types.String `tfsdk:"tld"`
	SLD          types.String `tfsdk:"sld"`
	Status       types.String `tfsdk:"status"`
	RenewalMode  types.String `tfsdk:"renewal_mode"`
	TransferLock types.Bool   `tfsdk:"transfer_lock"`
	IsPremium    types.Bool   `tfsdk:"is_premium"`
	Domains      types.List   `tfsdk:"domains"`
}

// domainItemAttrTypes describes the per-item shape used in the `domains`
// list attribute.
var domainItemAttrTypes = map[string]attr.Type{
	"domain_id":            types.StringType,
	"name":                 types.StringType,
	"sld":                  types.StringType,
	"tld":                  types.StringType,
	"owner_id":             types.StringType,
	"registry_account_id":  types.StringType,
	"renewal_mode":         types.StringType,
	"transfer_lock":        types.BoolType,
	"is_premium":           types.BoolType,
	"nameservers":          types.ListType{ElemType: types.ObjectType{AttrTypes: nameserverAttrTypes}},
	"contacts":             types.MapType{ElemType: contactsMapElemType},
	"registry_statuses":    types.ListType{ElemType: types.StringType},
	"auth_code_expires_on": types.StringType,
	"registered_on":        types.StringType,
	"expires_on":           types.StringType,
	"created_on":           types.StringType,
	"updated_on":           types.StringType,
}

// NewDomainsDataSource returns a new DomainsDataSource.
func NewDomainsDataSource() datasource.DataSource {
	return &DomainsDataSource{}
}

func (d *DomainsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domains"
}

func (d *DomainsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists domains within the authenticated caller's organization (`GET /v1/domains`). Results are auto-paginated by the SDK; optional filters narrow the server-side query.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, MarkdownDescription: "Static identifier for this data source."},

			"search":        schema.StringAttribute{Optional: true, MarkdownDescription: "Free-text search over domain names."},
			"name":          schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by exact fully-qualified domain name."},
			"tld":           schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by top-level domain."},
			"sld":           schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by second-level domain."},
			"status":        schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by domain lifecycle status (e.g. `ok`, `pendingTransfer`)."},
			"renewal_mode":  schema.StringAttribute{Optional: true, MarkdownDescription: "Filter by renewal mode (`renew` or `expire`)."},
			"transfer_lock": schema.BoolAttribute{Optional: true, MarkdownDescription: "Filter by transfer-lock status."},
			"is_premium":    schema.BoolAttribute{Optional: true, MarkdownDescription: "Filter by premium classification."},

			"domains": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Matching domains.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"domain_id":           schema.StringAttribute{Computed: true},
						"name":                schema.StringAttribute{Computed: true},
						"sld":                 schema.StringAttribute{Computed: true},
						"tld":                 schema.StringAttribute{Computed: true},
						"owner_id":            schema.StringAttribute{Computed: true},
						"registry_account_id": schema.StringAttribute{Computed: true},
						"renewal_mode":        schema.StringAttribute{Computed: true},
						"transfer_lock":       schema.BoolAttribute{Computed: true},
						"is_premium":          schema.BoolAttribute{Computed: true},
						"nameservers": schema.ListNestedAttribute{
							Computed: true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"hostname":     schema.StringAttribute{Computed: true},
									"ip_addresses": schema.ListAttribute{Computed: true, ElementType: types.StringType},
								},
							},
						},
						"contacts":             schema.MapAttribute{Computed: true, ElementType: contactsMapElemType},
						"registry_statuses":    schema.ListAttribute{Computed: true, ElementType: types.StringType},
						"auth_code_expires_on": schema.StringAttribute{Computed: true},
						"registered_on":        schema.StringAttribute{Computed: true},
						"expires_on":           schema.StringAttribute{Computed: true},
						"created_on":           schema.StringAttribute{Computed: true},
						"updated_on":           schema.StringAttribute{Computed: true},
					},
				},
			},
		},
	}
}

func (d *DomainsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DomainsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DomainsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts := &models.ListDomainsOptions{
		Search: stringValue(data.Search),
		Name:   stringValue(data.Name),
		TLD:    stringValue(data.TLD),
		SLD:    stringValue(data.SLD),
	}
	if !data.Status.IsNull() && !data.Status.IsUnknown() {
		opts.Status = models.DomainStatus(data.Status.ValueString())
	}
	if !data.RenewalMode.IsNull() && !data.RenewalMode.IsUnknown() {
		mode := models.RenewalMode(data.RenewalMode.ValueString())
		opts.RenewalMode = &mode
	}
	if !data.TransferLock.IsNull() && !data.TransferLock.IsUnknown() {
		v := data.TransferLock.ValueBool()
		opts.TransferLock = &v
	}
	if !data.IsPremium.IsNull() && !data.IsPremium.IsUnknown() {
		v := data.IsPremium.ValueBool()
		opts.IsPremium = &v
	}

	domains, err := d.client.Domains.ListDomains(ctx, opts)
	if err != nil {
		resp.Diagnostics.AddError("Error listing domains", formatAPIError(err))
		return
	}

	objType := types.ObjectType{AttrTypes: domainItemAttrTypes}
	values := make([]attr.Value, len(domains))
	for i := range domains {
		dn := &domains[i]

		nsList, nd := nameserversAPIToList(ctx, dn.Nameservers)
		resp.Diagnostics.Append(nd...)
		if resp.Diagnostics.HasError() {
			return
		}
		cMap, cd := contactsAPIToMap(dn.Contacts, nil)
		resp.Diagnostics.Append(cd...)
		if resp.Diagnostics.HasError() {
			return
		}
		statusList, sd := stringSliceToList(dn.RegistryStatuses)
		resp.Diagnostics.Append(sd...)
		if resp.Diagnostics.HasError() {
			return
		}

		renewalMode := types.StringNull()
		if dn.RenewalMode != "" {
			renewalMode = types.StringValue(string(dn.RenewalMode))
		}

		obj, diags := types.ObjectValue(domainItemAttrTypes, map[string]attr.Value{
			"domain_id":            types.StringValue(string(dn.DomainID)),
			"name":                 types.StringValue(dn.Name),
			"sld":                  types.StringValue(dn.SLD),
			"tld":                  types.StringValue(dn.TLD),
			"owner_id":             types.StringValue(string(dn.OwnerID)),
			"registry_account_id":  types.StringValue(string(dn.RegistryAccountID)),
			"renewal_mode":         renewalMode,
			"transfer_lock":        types.BoolValue(domainHasClientTransferProhibited(dn)),
			"is_premium":           types.BoolValue(dn.IsPremium),
			"nameservers":          nsList,
			"contacts":             cMap,
			"registry_statuses":    statusList,
			"auth_code_expires_on": timePtrToValue(dn.AuthCodeExpiresOn),
			"registered_on":        timePtrToValue(dn.RegisteredOn),
			"expires_on":           timePtrToValue(dn.ExpiresOn),
			"created_on":           timePtrToValue(dn.CreatedOn),
			"updated_on":           timePtrToValue(dn.UpdatedOn),
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

	data.ID = types.StringValue("domains")
	data.Domains = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
