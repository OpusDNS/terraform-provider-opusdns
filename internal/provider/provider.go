package provider

import (
	"context"
	"os"

	opusdns "github.com/opusdns/opusdns-go-client/opusdns"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &OpusDNSProvider{}

type OpusDNSProvider struct {
	version string
}

type OpusDNSProviderModel struct {
	APIKey      types.String `tfsdk:"api_key"`
	APIEndpoint types.String `tfsdk:"api_endpoint"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &OpusDNSProvider{
			version: version,
		}
	}
}

func (p *OpusDNSProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "opusdns"
	resp.Version = p.version
}

func (p *OpusDNSProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "OpusDNS API key. Can also be set via OPUSDNS_API_KEY environment variable.",
			},
			"api_endpoint": schema.StringAttribute{
				Optional:    true,
				Description: "OpusDNS API endpoint. Can also be set via OPUSDNS_API_ENDPOINT environment variable.",
			},
		},
	}
}

func (p *OpusDNSProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config OpusDNSProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := os.Getenv("OPUSDNS_API_KEY")
	if !config.APIKey.IsNull() && !config.APIKey.IsUnknown() {
		apiKey = config.APIKey.ValueString()
	}

	apiEndpoint := os.Getenv("OPUSDNS_API_ENDPOINT")
	if !config.APIEndpoint.IsNull() && !config.APIEndpoint.IsUnknown() {
		apiEndpoint = config.APIEndpoint.ValueString()
	}

	opts := []opusdns.Option{opusdns.WithAPIKey(apiKey)}
	if apiEndpoint != "" {
		opts = append(opts, opusdns.WithAPIEndpoint(apiEndpoint))
	}

	client, err := opusdns.NewClient(opts...)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create OpusDNS client", err.Error())
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *OpusDNSProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewZoneResource,
		NewRecordResource,
		NewContactResource,
		NewDomainResource,
	}
}

func (p *OpusDNSProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewZoneDataSource,
		NewZonesDataSource,
	}
}
