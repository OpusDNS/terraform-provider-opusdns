package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure HostDataSource satisfies the datasource.DataSource interface.
var _ datasource.DataSource = &HostDataSource{}
var _ datasource.DataSourceWithConfigValidators = &HostDataSource{}

// HostDataSource reads a single host object by `host_id` or `hostname`
// (`GET /v1/hosts/{host_reference}`).
//
// Exactly one of `host_id` or `hostname` must be set.
type HostDataSource struct {
	client *opusdns.Client
}

// HostDataSourceModel is the state shape for the singular host data source.
type HostDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	HostID      types.String `tfsdk:"host_id"`
	Hostname    types.String `tfsdk:"hostname"`
	IPAddresses types.Set    `tfsdk:"ip_addresses"`
	CreatedOn   types.String `tfsdk:"created_on"`
	UpdatedOn   types.String `tfsdk:"updated_on"`
}

// NewHostDataSource returns a new HostDataSource.
func NewHostDataSource() datasource.DataSource {
	return &HostDataSource{}
}

func (d *HostDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_host"
}

func (d *HostDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads a single OpusDNS host object (`GET /v1/hosts/{host_reference}`). " +
			"Exactly one of `host_id` or `hostname` must be supplied.",
		Attributes: map[string]schema.Attribute{
			"id":       schema.StringAttribute{Computed: true, MarkdownDescription: "Mirrors `host_id`."},
			"host_id":  schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Host id (e.g. `host_01j...`). Mutually exclusive with `hostname`."},
			"hostname": schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Fully-qualified hostname of the host object. Mutually exclusive with `host_id`."},
			"ip_addresses": schema.SetAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "IPv4/IPv6 addresses bound to the host. Modelled as a set; order is not significant.",
			},
			"created_on": schema.StringAttribute{Computed: true},
			"updated_on": schema.StringAttribute{Computed: true},
		},
	}
}

func (d *HostDataSource) ConfigValidators(_ context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(path.MatchRoot("host_id"), path.MatchRoot("hostname")),
	}
}

func (d *HostDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *HostDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data HostDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var ref string
	switch {
	case !data.HostID.IsNull() && !data.HostID.IsUnknown() && data.HostID.ValueString() != "":
		ref = data.HostID.ValueString()
	case !data.Hostname.IsNull() && !data.Hostname.IsUnknown() && data.Hostname.ValueString() != "":
		ref = data.Hostname.ValueString()
	default:
		resp.Diagnostics.AddError("Invalid host data source config", "Either `host_id` or `hostname` must be set.")
		return
	}

	host, err := rawGetHost(ctx, d.client, ref)
	if err != nil {
		resp.Diagnostics.AddError("Error reading host", formatAPIError(err))
		return
	}

	values := make([]attr.Value, len(host.IPAddresses))
	for i, ip := range host.IPAddresses {
		values[i] = types.StringValue(ip)
	}
	ipSet, diags := types.SetValue(types.StringType, values)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.ID = types.StringValue(host.HostID)
	data.HostID = types.StringValue(host.HostID)
	data.Hostname = types.StringValue(host.Hostname)
	data.IPAddresses = ipSet
	data.CreatedOn = types.StringValue(host.CreatedOn.Format(time.RFC3339))
	data.UpdatedOn = types.StringValue(host.UpdatedOn.Format(time.RFC3339))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
