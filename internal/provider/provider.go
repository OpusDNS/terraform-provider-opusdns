package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// Ensure OpusDNSProvider satisfies various provider interfaces.
var _ provider.Provider = &OpusDNSProvider{}
var _ provider.ProviderWithFunctions = &OpusDNSProvider{}

// OpusDNSProvider defines the provider implementation.
type OpusDNSProvider struct {
	// version is set to the provider version on release, "dev" when the provider is built and run locally.
	version string
}

// OpusDNSProviderModel describes the provider data model.
type OpusDNSProviderModel struct {
	APIKey      types.String `tfsdk:"api_key"`
	APIEndpoint types.String `tfsdk:"api_endpoint"`
}

// New returns a provider.Provider.
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
		MarkdownDescription: "The OpusDNS provider manages DNS zones, records, contacts, email forwards, and domain forwards via the OpusDNS API.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				MarkdownDescription: "OpusDNS API key. Can also be set via the `OPUSDNS_API_KEY` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"api_endpoint": schema.StringAttribute{
				MarkdownDescription: "OpusDNS API endpoint URL. Defaults to `https://api.opusdns.com`. Can also be set via the `OPUSDNS_API_ENDPOINT` environment variable.",
				Optional:            true,
			},
		},
	}
}

func (p *OpusDNSProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data OpusDNSProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey := os.Getenv("OPUSDNS_API_KEY")
	if !data.APIKey.IsNull() {
		apiKey = data.APIKey.ValueString()
	}

	if apiKey == "" {
		resp.Diagnostics.AddError(
			"Missing OpusDNS API Key",
			"The provider cannot create an OpusDNS API client because an API key was not supplied. "+
				"Set the api_key provider attribute or the OPUSDNS_API_KEY environment variable.",
		)
		return
	}

	opts := []opusdns.Option{
		opusdns.WithAPIKey(apiKey),
		opusdns.WithUserAgent("terraform-provider-opusdns/" + p.version),
	}

	if !data.APIEndpoint.IsNull() && data.APIEndpoint.ValueString() != "" {
		opts = append(opts, opusdns.WithAPIEndpoint(data.APIEndpoint.ValueString()))
	} else if endpoint := os.Getenv("OPUSDNS_API_ENDPOINT"); endpoint != "" {
		opts = append(opts, opusdns.WithAPIEndpoint(endpoint))
	}

	client, err := opusdns.NewClient(opts...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create OpusDNS API Client",
			"An unexpected error occurred when creating the OpusDNS API client: "+err.Error(),
		)
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
		NewEmailForwardResource,
		NewDomainForwardResource,
	}
}

func (p *OpusDNSProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewZoneDataSource,
		NewZonesDataSource,
	}
}

func (p *OpusDNSProvider) Functions(_ context.Context) []func() function.Function {
	return []func() function.Function{}
}
