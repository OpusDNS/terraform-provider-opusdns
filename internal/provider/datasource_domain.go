package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
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
	IncludeTags       types.Bool   `tfsdk:"include_tags"`
	Tags              types.List   `tfsdk:"tags"`
	StatusTags        types.List   `tfsdk:"status_tags"`
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
			"registry_statuses": schema.ListAttribute{Computed: true, ElementType: types.StringType},
			"include_tags":      schema.BoolAttribute{Optional: true, MarkdownDescription: "When true, request `include=tags` and populate `tags` and `status_tags` in state."},
			"tags": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Tags assigned to the domain when `include_tags` is true.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"tag_id": schema.StringAttribute{Computed: true},
					"label":  schema.StringAttribute{Computed: true},
					"color":  schema.StringAttribute{Computed: true},
				}},
			},
			"status_tags": schema.ListAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "System-managed status tag types on the domain (e.g. `VERIFICATION_REQUIRED`). Populated only when `include_tags` is true; otherwise empty.",
			},
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

	var domain *models.Domain
	var statusTags []string
	var err error
	includeTags := !data.IncludeTags.IsNull() && !data.IncludeTags.IsUnknown() && data.IncludeTags.ValueBool()
	if includeTags {
		// The SDK's models.Domain omits the status_tags array, so use the raw
		// include=tags wrapper to capture it alongside user tags.
		raw, rErr := rawGetDomainWithStatusTags(ctx, d.client, data.DomainRef.ValueString())
		if rErr != nil {
			resp.Diagnostics.AddError("Error reading domain", formatAPIError(rErr))
			return
		}
		domain = &raw.Domain
		statusTags = statusTagTypesToStrings(raw.StatusTags)
	} else {
		domain, err = d.client.Domains.GetDomain(ctx, data.DomainRef.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error reading domain", formatAPIError(err))
			return
		}
	}
	dn := domain

	data.ID = types.StringValue(string(dn.DomainID))
	data.DomainID = types.StringValue(string(dn.DomainID))
	data.Name = types.StringValue(dn.Name)
	data.SLD = types.StringValue(dn.SLD)
	data.TLD = types.StringValue(dn.TLD)
	data.OwnerID = types.StringValue(string(dn.OwnerID))
	data.RegistryAccountID = types.StringValue(string(dn.RegistryAccountID))
	data.IsPremium = types.BoolValue(dn.IsPremium)
	data.AuthCodeExpiresOn = timePtrToValue(dn.AuthCodeExpiresOn)
	data.RegisteredOn = timePtrToValue(dn.RegisteredOn)
	data.ExpiresOn = timePtrToValue(dn.ExpiresOn)
	data.CreatedOn = timePtrToValue(dn.CreatedOn)
	data.UpdatedOn = timePtrToValue(dn.UpdatedOn)
	if dn.RenewalMode != "" {
		data.RenewalMode = types.StringValue(string(dn.RenewalMode))
	} else {
		data.RenewalMode = types.StringNull()
	}
	data.TransferLock = types.BoolValue(domainHasClientTransferProhibited(dn))

	statusList, sd := stringSliceToList(dn.RegistryStatuses)
	resp.Diagnostics.Append(sd...)
	data.RegistryStatuses = statusList
	tagList, td := tagEnrichedListValue(dn.Tags)
	resp.Diagnostics.Append(td...)
	data.Tags = tagList
	statusTagList, std := stringSliceToList(statusTags)
	resp.Diagnostics.Append(std...)
	data.StatusTags = statusTagList

	nsList, nd := nameserversAPIToList(ctx, dn.Nameservers)
	resp.Diagnostics.Append(nd...)
	data.Nameservers = nsList

	cMap, cd := contactsAPIToMap(dn.Contacts, nil)
	resp.Diagnostics.Append(cd...)
	data.Contacts = cMap

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
