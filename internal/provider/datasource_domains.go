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
	ID                 types.String `tfsdk:"id"`
	Search             types.String `tfsdk:"search"`
	Name               types.String `tfsdk:"name"`
	TLD                types.String `tfsdk:"tld"`
	SLD                types.String `tfsdk:"sld"`
	Status             types.String `tfsdk:"status"`
	RenewalMode        types.String `tfsdk:"renewal_mode"`
	TransferLock       types.Bool   `tfsdk:"transfer_lock"`
	IsPremium          types.Bool   `tfsdk:"is_premium"`
	TagIDs             types.List   `tfsdk:"tag_ids"`
	TagMode            types.String `tfsdk:"tag_mode"`
	IncludeTags        types.Bool   `tfsdk:"include_tags"`
	CreatedAfter       types.String `tfsdk:"created_after"`
	CreatedBefore      types.String `tfsdk:"created_before"`
	UpdatedAfter       types.String `tfsdk:"updated_after"`
	UpdatedBefore      types.String `tfsdk:"updated_before"`
	ExpiresAfter       types.String `tfsdk:"expires_after"`
	ExpiresBefore      types.String `tfsdk:"expires_before"`
	ExpiresIn30Days    types.Bool   `tfsdk:"expires_in_30_days"`
	ExpiresIn60Days    types.Bool   `tfsdk:"expires_in_60_days"`
	ExpiresIn90Days    types.Bool   `tfsdk:"expires_in_90_days"`
	RegisteredAfter    types.String `tfsdk:"registered_after"`
	RegisteredBefore   types.String `tfsdk:"registered_before"`
	RegistryStatusesIn types.List   `tfsdk:"registry_statuses_in"`
	StatusTags         types.List   `tfsdk:"status_tags"`
	StatusTagMode      types.String `tfsdk:"status_tag_mode"`
	Domains            types.List   `tfsdk:"domains"`
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
	"status_tags":          types.ListType{ElemType: types.StringType},
	"tags":                 types.ListType{ElemType: types.ObjectType{AttrTypes: tagEnrichedAttrTypes}},
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
			"tag_ids": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Filter by tag IDs. Multiple values are sent as repeated `tag_ids` query parameters.",
			},
			"tag_mode":       schema.StringAttribute{Optional: true, MarkdownDescription: "Tag filter mode. Use `match_any` or `match_all` according to the API."},
			"include_tags":   schema.BoolAttribute{Optional: true, MarkdownDescription: "When true, request `include=tags` and populate the computed `tags` field for each domain."},
			"created_after":  schema.StringAttribute{Optional: true, MarkdownDescription: "Filter domains created after this RFC3339 timestamp."},
			"created_before": schema.StringAttribute{Optional: true, MarkdownDescription: "Filter domains created before this RFC3339 timestamp."},
			"updated_after":  schema.StringAttribute{Optional: true, MarkdownDescription: "Filter domains updated after this RFC3339 timestamp."},
			"updated_before": schema.StringAttribute{Optional: true, MarkdownDescription: "Filter domains updated before this RFC3339 timestamp."},
			"expires_after":  schema.StringAttribute{Optional: true, MarkdownDescription: "Filter domains expiring after this RFC3339 timestamp."},
			"expires_before": schema.StringAttribute{Optional: true, MarkdownDescription: "Filter domains expiring before this RFC3339 timestamp."},
			"expires_in_30_days": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Filter domains expiring within 30 days.",
			},
			"expires_in_60_days": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Filter domains expiring within 60 days.",
			},
			"expires_in_90_days": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Filter domains expiring within 90 days.",
			},
			"registered_after":  schema.StringAttribute{Optional: true, MarkdownDescription: "Filter domains registered after this RFC3339 timestamp."},
			"registered_before": schema.StringAttribute{Optional: true, MarkdownDescription: "Filter domains registered before this RFC3339 timestamp."},
			"registry_statuses_in": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Filter by registry statuses. Multiple values are sent as repeated API parameters.",
			},
			"status_tags": schema.ListAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Filter by system-managed status tag types (e.g. `VERIFICATION_REQUIRED`). Multiple values are sent as repeated `status_tags` query parameters. When set, each domain's computed `status_tags` is also populated.",
			},
			"status_tag_mode": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "How to combine `status_tags`: `match_any` (default) matches domains with at least one of the tags; `match_all` matches domains with every tag.",
			},

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
						"contacts":          schema.MapAttribute{Computed: true, ElementType: contactsMapElemType},
						"registry_statuses": schema.ListAttribute{Computed: true, ElementType: types.StringType},
						"status_tags": schema.ListAttribute{
							Computed:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "System-managed status tag types on the domain (e.g. `VERIFICATION_REQUIRED`). Populated only when `status_tags` filters are set or `include_tags` is true; otherwise empty.",
						},
						"tags": schema.ListNestedAttribute{
							Computed:            true,
							MarkdownDescription: "Tags assigned to the domain when `include_tags` is true.",
							NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
								"tag_id": schema.StringAttribute{Computed: true},
								"label":  schema.StringAttribute{Computed: true},
								"color":  schema.StringAttribute{Computed: true},
							}},
						},
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
	if !data.TagIDs.IsNull() && !data.TagIDs.IsUnknown() {
		raw, diags := stringListValueToStrings(ctx, data.TagIDs)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		opts.TagIDs = make([]models.TagID, 0, len(raw))
		for _, id := range raw {
			opts.TagIDs = append(opts.TagIDs, models.TagID(id))
		}
	}
	if !data.TagMode.IsNull() && !data.TagMode.IsUnknown() {
		opts.TagMode = models.TagFilterMode(data.TagMode.ValueString())
	}
	if !data.IncludeTags.IsNull() && !data.IncludeTags.IsUnknown() && data.IncludeTags.ValueBool() {
		opts.Include = append(opts.Include, models.DomainIncludeTags)
	}
	var diags diag.Diagnostics
	if opts.CreatedAfter, diags = parseOptionalRFC3339(data.CreatedAfter, "created_after"); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	if opts.CreatedBefore, diags = parseOptionalRFC3339(data.CreatedBefore, "created_before"); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	if opts.UpdatedAfter, diags = parseOptionalRFC3339(data.UpdatedAfter, "updated_after"); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	if opts.UpdatedBefore, diags = parseOptionalRFC3339(data.UpdatedBefore, "updated_before"); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	if opts.ExpiresAfter, diags = parseOptionalRFC3339(data.ExpiresAfter, "expires_after"); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	if opts.ExpiresBefore, diags = parseOptionalRFC3339(data.ExpiresBefore, "expires_before"); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	if opts.RegisteredAfter, diags = parseOptionalRFC3339(data.RegisteredAfter, "registered_after"); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	if opts.RegisteredBefore, diags = parseOptionalRFC3339(data.RegisteredBefore, "registered_before"); diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}
	if !data.ExpiresIn30Days.IsNull() && !data.ExpiresIn30Days.IsUnknown() {
		v := data.ExpiresIn30Days.ValueBool()
		opts.ExpiresIn30Days = &v
	}
	if !data.ExpiresIn60Days.IsNull() && !data.ExpiresIn60Days.IsUnknown() {
		v := data.ExpiresIn60Days.ValueBool()
		opts.ExpiresIn60Days = &v
	}
	if !data.ExpiresIn90Days.IsNull() && !data.ExpiresIn90Days.IsUnknown() {
		v := data.ExpiresIn90Days.ValueBool()
		opts.ExpiresIn90Days = &v
	}
	if !data.RegistryStatusesIn.IsNull() && !data.RegistryStatusesIn.IsUnknown() {
		raw, diags := stringListValueToStrings(ctx, data.RegistryStatusesIn)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		opts.RegistryStatuses = raw
	}

	statusTagFilters, diags := stringListValueToStrings(ctx, data.StatusTags)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	statusTagMode := stringValue(data.StatusTagMode)
	includeTags := !data.IncludeTags.IsNull() && !data.IncludeTags.IsUnknown() && data.IncludeTags.ValueBool()

	// The SDK's ListDomainsOptions cannot express the status-tag filters, and
	// its models.Domain omits the status_tags array. Fall back to the raw HTTP
	// wrapper only when the caller filters by status tags or requests tags via
	// include_tags (which also carries status_tags). Otherwise keep the SDK
	// path so no existing filter behaviour regresses.
	useRawStatusTags := len(statusTagFilters) > 0 || includeTags

	objType := types.ObjectType{AttrTypes: domainItemAttrTypes}
	var values []attr.Value

	if useRawStatusTags {
		rawDomains, err := rawListDomainsWithStatusTags(ctx, d.client, opts, statusTagFilters, statusTagMode)
		if err != nil {
			resp.Diagnostics.AddError("Error listing domains", formatAPIError(err))
			return
		}
		values = make([]attr.Value, len(rawDomains))
		for i := range rawDomains {
			obj, diags := domainListItemObject(ctx, &rawDomains[i].Domain, statusTagTypesToStrings(rawDomains[i].StatusTags))
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			values[i] = obj
		}
	} else {
		domains, err := d.client.Domains.ListDomains(ctx, opts)
		if err != nil {
			resp.Diagnostics.AddError("Error listing domains", formatAPIError(err))
			return
		}
		values = make([]attr.Value, len(domains))
		for i := range domains {
			obj, diags := domainListItemObject(ctx, &domains[i], nil)
			resp.Diagnostics.Append(diags...)
			if resp.Diagnostics.HasError() {
				return
			}
			values[i] = obj
		}
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

// domainListItemObject builds the per-domain object value for the `domains`
// list attribute. statusTags, when non-nil, populates the computed
// `status_tags` field; pass nil to leave it as an empty list.
func domainListItemObject(ctx context.Context, dn *models.Domain, statusTags []string) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics

	nsList, nd := nameserversAPIToList(ctx, dn.Nameservers)
	diags.Append(nd...)
	if diags.HasError() {
		return types.ObjectNull(domainItemAttrTypes), diags
	}
	cMap, cd := contactsAPIToMap(dn.Contacts, nil)
	diags.Append(cd...)
	if diags.HasError() {
		return types.ObjectNull(domainItemAttrTypes), diags
	}
	statusList, sd := stringSliceToList(dn.RegistryStatuses)
	diags.Append(sd...)
	if diags.HasError() {
		return types.ObjectNull(domainItemAttrTypes), diags
	}
	statusTagList, std := stringSliceToList(statusTags)
	diags.Append(std...)
	if diags.HasError() {
		return types.ObjectNull(domainItemAttrTypes), diags
	}
	tagsList, td := tagEnrichedListValue(dn.Tags)
	diags.Append(td...)
	if diags.HasError() {
		return types.ObjectNull(domainItemAttrTypes), diags
	}

	renewalMode := types.StringNull()
	if dn.RenewalMode != "" {
		renewalMode = types.StringValue(string(dn.RenewalMode))
	}

	obj, oDiags := types.ObjectValue(domainItemAttrTypes, map[string]attr.Value{
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
		"status_tags":          statusTagList,
		"tags":                 tagsList,
		"auth_code_expires_on": timePtrToValue(dn.AuthCodeExpiresOn),
		"registered_on":        timePtrToValue(dn.RegisteredOn),
		"expires_on":           timePtrToValue(dn.ExpiresOn),
		"created_on":           timePtrToValue(dn.CreatedOn),
		"updated_on":           timePtrToValue(dn.UpdatedOn),
	})
	diags.Append(oDiags...)
	return obj, diags
}

// statusTagTypesToStrings maps StatusTagResponse rows to their tag_type
// string values for exposure in state.
func statusTagTypesToStrings(tags []models.StatusTagResponse) []string {
	out := make([]string, len(tags))
	for i, t := range tags {
		out[i] = string(t.TagType)
	}
	return out
}
