package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/models"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure DomainForwardDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &DomainForwardDataSource{}

// DomainForwardDataSource fetches a single domain forward by its hostname
// via `GET /v1/domain-forwards/{hostname}`.
type DomainForwardDataSource struct {
	client *opusdns.Client
}

// DomainForwardDataSourceModel is the data-source state shape. Mirrors the
// resource model but adds timestamps the resource does not surface.
type DomainForwardDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	Hostname  types.String `tfsdk:"hostname"`
	Enabled   types.Bool   `tfsdk:"enabled"`
	HTTP      types.List   `tfsdk:"http"`
	HTTPS     types.List   `tfsdk:"https"`
	CreatedOn types.String `tfsdk:"created_on"`
	UpdatedOn types.String `tfsdk:"updated_on"`
}

// NewDomainForwardDataSource returns a new DomainForwardDataSource.
func NewDomainForwardDataSource() datasource.DataSource {
	return &DomainForwardDataSource{}
}

func (d *DomainForwardDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_domain_forward"
}

func (d *DomainForwardDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	redirectAttrs := map[string]schema.Attribute{
		"request_path":    schema.StringAttribute{Computed: true, MarkdownDescription: "The source path to match."},
		"target_protocol": schema.StringAttribute{Computed: true, MarkdownDescription: "The destination protocol (`http` or `https`)."},
		"target_hostname": schema.StringAttribute{Computed: true, MarkdownDescription: "The destination hostname."},
		"target_path":     schema.StringAttribute{Computed: true, MarkdownDescription: "The destination path."},
		"redirect_code":   schema.Int64Attribute{Computed: true, MarkdownDescription: "The HTTP redirect status code."},
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Fetches a single domain forward by its `hostname`. " +
			"Use `opusdns_domain_forwards` to list all domain forwards in a zone.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Same value as `hostname`.",
			},
			"hostname": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The source hostname to fetch the forward for (e.g. `www.example.com`).",
			},
			"enabled": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the domain forward is currently active.",
			},
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
			"created_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp of when the domain forward was created.",
			},
			"updated_on": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp of when the domain forward was last updated.",
			},
		},
	}
}

func (d *DomainForwardDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DomainForwardDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DomainForwardDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hostname := data.Hostname.ValueString()
	if hostname == "" {
		resp.Diagnostics.AddError(
			"Invalid hostname",
			"The `hostname` attribute must be a non-empty hostname.",
		)
		return
	}

	df, err := d.client.DomainForwards.GetDomainForward(ctx, hostname)
	if err != nil {
		resp.Diagnostics.AddError("Error reading domain forward", formatAPIError(err))
		return
	}

	resp.Diagnostics.Append(setDomainForwardDataSourceState(ctx, &data, df)...)
	if !resp.Diagnostics.HasError() {
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	}
}

// setDomainForwardDataSourceState populates the data-source model from an
// API response, including timestamps the resource state builder omits.
// Reuses `httpRedirectAttrTypes` so nested values are interchangeable with
// the resource.
func setDomainForwardDataSourceState(ctx context.Context, data *DomainForwardDataSourceModel, df *models.DomainForward) diag.Diagnostics {
	var diags diag.Diagnostics

	data.ID = types.StringValue(trimTrailingDot(df.Hostname))
	data.Hostname = types.StringValue(trimTrailingDot(df.Hostname))
	data.Enabled = types.BoolValue(df.Enabled)
	data.CreatedOn = types.StringValue(df.CreatedOn.Format(rfc3339))
	data.UpdatedOn = types.StringValue(df.UpdatedOn.Format(rfc3339))

	redirectObjType := types.ObjectType{AttrTypes: httpRedirectAttrTypes}

	httpList, d := protocolSetToList(ctx, df.HTTP, redirectObjType)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	data.HTTP = httpList

	httpsList, d := protocolSetToList(ctx, df.HTTPS, redirectObjType)
	diags.Append(d...)
	if diags.HasError() {
		return diags
	}
	data.HTTPS = httpsList

	return diags
}
