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

// Ensure DomainForwardsDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &DomainForwardsDataSource{}

// DomainForwardsDataSource lists all domain forwards for a given zone via
// `ListDomainForwardsByZone`. Required `zone_name`; matches the convention
// used by `opusdns_records` and `opusdns_email_forwards`.
type DomainForwardsDataSource struct {
	client *opusdns.Client
}

// DomainForwardsDataSourceModel describes the data-source state.
type DomainForwardsDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	ZoneName       types.String `tfsdk:"zone_name"`
	DomainForwards types.List   `tfsdk:"domain_forwards"`
}

// domainForwardListObjectAttrTypes is the attribute schema of each entry
// in the `domain_forwards` list. Reuses `httpRedirectAttrTypes` so values
// are interchangeable with the resource and singular data source.
var domainForwardListObjectAttrTypes = map[string]attr.Type{
	"id":         types.StringType,
	"hostname":   types.StringType,
	"enabled":    types.BoolType,
	"http":       types.ListType{ElemType: types.ObjectType{AttrTypes: httpRedirectAttrTypes}},
	"https":      types.ListType{ElemType: types.ObjectType{AttrTypes: httpRedirectAttrTypes}},
	"created_on": types.StringType,
	"updated_on": types.StringType,
}

var domainForwardListObjectType = types.ObjectType{AttrTypes: domainForwardListObjectAttrTypes}

// NewDomainForwardsDataSource returns a new DomainForwardsDataSource.
func NewDomainForwardsDataSource() datasource.DataSource {
	return &DomainForwardsDataSource{}
}

func (d *DomainForwardsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain_forwards"
}

func (d *DomainForwardsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	redirectAttrs := map[string]schema.Attribute{
		"request_path":    schema.StringAttribute{Computed: true, MarkdownDescription: "The source path to match."},
		"target_protocol": schema.StringAttribute{Computed: true, MarkdownDescription: "The destination protocol (`http` or `https`)."},
		"target_hostname": schema.StringAttribute{Computed: true, MarkdownDescription: "The destination hostname."},
		"target_path":     schema.StringAttribute{Computed: true, MarkdownDescription: "The destination path."},
		"redirect_code":   schema.Int64Attribute{Computed: true, MarkdownDescription: "The HTTP redirect status code."},
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all domain forwards configured for a zone via `GET /v1/zones/{zone}/domain-forwards`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Synthetic identifier (the zone name).",
			},
			"zone_name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the zone whose domain forwards to list (e.g. `example.com`).",
			},
			"domain_forwards": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "All domain forwards configured for the zone.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":       schema.StringAttribute{Computed: true, MarkdownDescription: "Same value as `hostname`."},
						"hostname": schema.StringAttribute{Computed: true, MarkdownDescription: "The source hostname."},
						"enabled":  schema.BoolAttribute{Computed: true, MarkdownDescription: "Whether the domain forward is active."},
						"http": schema.ListNestedAttribute{
							Computed:            true,
							MarkdownDescription: "HTTP protocol redirect rules.",
							NestedObject:        schema.NestedAttributeObject{Attributes: redirectAttrs},
						},
						"https": schema.ListNestedAttribute{
							Computed:            true,
							MarkdownDescription: "HTTPS protocol redirect rules.",
							NestedObject:        schema.NestedAttributeObject{Attributes: redirectAttrs},
						},
						"created_on": schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp of when the domain forward was created."},
						"updated_on": schema.StringAttribute{Computed: true, MarkdownDescription: "RFC3339 timestamp of when the domain forward was last updated."},
					},
				},
			},
		},
	}
}

func (d *DomainForwardsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DomainForwardsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DomainForwardsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	zoneName := data.ZoneName.ValueString()
	if zoneName == "" {
		resp.Diagnostics.AddError(
			"Invalid zone_name",
			"The `zone_name` attribute must be a non-empty zone name.",
		)
		return
	}

	dfs, err := d.client.DomainForwards.ListDomainForwardsByZone(ctx, zoneName)
	if err != nil {
		resp.Diagnostics.AddError("Error listing domain forwards", formatAPIError(err))
		return
	}

	values := make([]attr.Value, 0, len(dfs))
	for i := range dfs {
		obj, diags := domainForwardToObjectValue(ctx, &dfs[i])
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		values = append(values, obj)
	}

	list, diags := types.ListValue(domainForwardListObjectType, values)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue(zoneName)
	data.DomainForwards = list
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// domainForwardToObjectValue converts a single SDK DomainForward into the
// framework object value used by the `domain_forwards` list attribute.
func domainForwardToObjectValue(ctx context.Context, df *models.DomainForward) (attr.Value, diag.Diagnostics) {
	var diags diag.Diagnostics

	redirectObjType := types.ObjectType{AttrTypes: httpRedirectAttrTypes}

	httpList, d := protocolSetToList(ctx, df.HTTP, redirectObjType)
	diags.Append(d...)
	if diags.HasError() {
		return types.ObjectNull(domainForwardListObjectAttrTypes), diags
	}
	httpsList, d := protocolSetToList(ctx, df.HTTPS, redirectObjType)
	diags.Append(d...)
	if diags.HasError() {
		return types.ObjectNull(domainForwardListObjectAttrTypes), diags
	}

	obj, d := types.ObjectValue(domainForwardListObjectAttrTypes, map[string]attr.Value{
		"id":         types.StringValue(trimTrailingDot(df.Hostname)),
		"hostname":   types.StringValue(trimTrailingDot(df.Hostname)),
		"enabled":    types.BoolValue(df.Enabled),
		"http":       httpList,
		"https":      httpsList,
		"created_on": types.StringValue(df.CreatedOn.Format(rfc3339)),
		"updated_on": types.StringValue(df.UpdatedOn.Format(rfc3339)),
	})
	diags.Append(d...)
	return obj, diags
}
