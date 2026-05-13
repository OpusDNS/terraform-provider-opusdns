package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure DomainDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &DomainDataSource{}

// DomainDataSource fetches a single domain via `GET /v1/domains/{ref}`
// (SDK: Domains.GetDomain). The lookup key may be a domain id or name.
type DomainDataSource struct {
	client *opusdns.Client
}

// DomainDataSourceModel mirrors the resource model with all attributes
// computed and `domain_ref` required as the lookup key. `domain_ref` accepts
// either a `domain_...` typeid or a domain name (e.g. `example.com`).
type DomainDataSourceModel struct {
	ID                types.String `tfsdk:"id"`
	DomainRef         types.String `tfsdk:"domain_ref"`
	DomainID          types.String `tfsdk:"domain_id"`
	Name              types.String `tfsdk:"name"`
	SLD               types.String `tfsdk:"sld"`
	TLD               types.String `tfsdk:"tld"`
	OwnerID           types.String `tfsdk:"owner_id"`
	RegistryAccountID types.String `tfsdk:"registry_account_id"`
	RenewalMode       types.String `tfsdk:"renewal_mode"`
	TransferLock      types.Bool   `tfsdk:"transfer_lock"`
	IsPremium         types.Bool   `tfsdk:"is_premium"`
	Nameservers       types.List   `tfsdk:"nameservers"`
	Contacts          types.Map    `tfsdk:"contacts"`
	RegistryStatuses  types.List   `tfsdk:"registry_statuses"`
	AuthCodeExpiresOn types.String `tfsdk:"auth_code_expires_on"`
	RegisteredOn      types.String `tfsdk:"registered_on"`
	ExpiresOn         types.String `tfsdk:"expires_on"`
	CreatedOn         types.String `tfsdk:"created_on"`
	UpdatedOn         types.String `tfsdk:"updated_on"`
}

// NewDomainDataSource returns a new DomainDataSource.
func NewDomainDataSource() datasource.DataSource {
	return &DomainDataSource{}
}

func (d *DomainDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain"
}

func (d *DomainDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single domain from `GET /v1/domains/{ref}`. `domain_ref` may be either a `domain_...` typeid or a domain name (e.g. `example.com`).",
		Attributes: map[string]schema.Attribute{
			"id":                  schema.StringAttribute{Computed: true, MarkdownDescription: "Mirror of `domain_id`."},
			"domain_ref":          schema.StringAttribute{Required: true, MarkdownDescription: "Domain id (`domain_...`) or fully-qualified name to look up."},
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
			"contacts": schema.MapAttribute{
				Computed:            true,
				ElementType:         contactsMapElemType,
				MarkdownDescription: "Contacts grouped by type (`registrant`, `admin`, `tech`, `billing`).",
			},
			"registry_statuses":    schema.ListAttribute{Computed: true, ElementType: types.StringType},
			"auth_code_expires_on": schema.StringAttribute{Computed: true},
			"registered_on":        schema.StringAttribute{Computed: true},
			"expires_on":           schema.StringAttribute{Computed: true},
			"created_on":           schema.StringAttribute{Computed: true},
			"updated_on":           schema.StringAttribute{Computed: true},
		},
	}
}

func (d *DomainDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DomainDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DomainDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	domain, err := d.client.Domains.GetDomain(ctx, data.DomainRef.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading domain", formatAPIError(err))
		return
	}

	data.ID = types.StringValue(string(domain.DomainID))
	data.DomainID = types.StringValue(string(domain.DomainID))
	data.Name = types.StringValue(domain.Name)
	data.SLD = types.StringValue(domain.SLD)
	data.TLD = types.StringValue(domain.TLD)
	data.OwnerID = types.StringValue(string(domain.OwnerID))
	data.RegistryAccountID = types.StringValue(string(domain.RegistryAccountID))
	data.IsPremium = types.BoolValue(domain.IsPremium)
	data.AuthCodeExpiresOn = timePtrToValue(domain.AuthCodeExpiresOn)
	data.RegisteredOn = timePtrToValue(domain.RegisteredOn)
	data.ExpiresOn = timePtrToValue(domain.ExpiresOn)
	data.CreatedOn = timePtrToValue(domain.CreatedOn)
	data.UpdatedOn = timePtrToValue(domain.UpdatedOn)
	if domain.RenewalMode != "" {
		data.RenewalMode = types.StringValue(string(domain.RenewalMode))
	} else {
		data.RenewalMode = types.StringNull()
	}
	data.TransferLock = types.BoolValue(domainHasClientTransferProhibited(domain))

	statusList, sd := stringSliceToList(domain.RegistryStatuses)
	resp.Diagnostics.Append(sd...)
	data.RegistryStatuses = statusList

	nsList, nd := nameserversAPIToList(ctx, domain.Nameservers)
	resp.Diagnostics.Append(nd...)
	data.Nameservers = nsList

	cMap, cd := contactsAPIToMap(domain.Contacts, nil)
	resp.Diagnostics.Append(cd...)
	data.Contacts = cMap

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
