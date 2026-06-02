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

type OpusDNSProviderModel struct {
	APIKey      types.String `tfsdk:"api_key"`
	APIEndpoint types.String `tfsdk:"api_endpoint"`
}

type providerConfig struct {
	APIKey      string
	APIEndpoint string
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
		MarkdownDescription: "The OpusDNS provider manages DNS zones, records, contacts, email forwards, and domain forwards via the OpusDNS API. Authentication uses a pre-minted OpusDNS API key sent via the `X-Api-Key` header on each request.",
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

	config := loadProviderConfig(data, os.Getenv)
	if config.APIKey == "" {
		resp.Diagnostics.AddError(
			"Missing OpusDNS Credentials",
			"Set `api_key` in the provider configuration or `OPUSDNS_API_KEY` in the environment.",
		)
		return
	}

	client, err := opusdns.NewClient(
		opusdns.WithAPIKey(config.APIKey),
		opusdns.WithAPIEndpoint(config.APIEndpoint),
		opusdns.WithUserAgent("terraform-provider-opusdns/"+p.version),
	)
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
		NewUserResource,
		NewUserRoleAssignmentResource,
		NewDomainResource,
		NewDomainDNSSECResource,
		NewTagResource,
		NewHostResource,
		NewParkingResource,
		NewRegistrarCredentialResource,
		NewContactAttributeSetResource,
		NewContactAttributeLinkResource,
		NewOrganizationIPRestrictionResource,
	}
}

func (p *OpusDNSProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewZoneDataSource,
		NewZonesDataSource,
		NewOrganizationDataSource,
		NewOrganizationsDataSource,
		NewUserDataSource,
		NewUsersDataSource,
		NewUserRoleAssignmentDataSource,
		NewUserPermissionsDataSource,
		NewContactDataSource,
		NewContactsDataSource,
		NewDomainDataSource,
		NewDomainsDataSource,
		NewDomainDNSSECDataSource,
		NewDNSRecordDataSource,
		NewDNSRecordsDataSource,
		NewDomainAvailabilityDataSource,
		NewDomainCheckDataSource,
		NewClaimsNoticeDataSource,
		NewTLDDataSource,
		NewTLDsDataSource,
		NewEmailForwardDataSource,
		NewEmailForwardsDataSource,
		NewDomainForwardDataSource,
		NewDomainForwardsDataSource,
		NewTagDataSource,
		NewTagsDataSource,
		NewHostDataSource,
		NewParkingDataSource,
		NewParkingsDataSource,
		NewRegistrarCredentialDataSource,
		NewRegistrarCredentialsDataSource,
		NewContactAttributeSetDataSource,
		NewContactAttributeSetsDataSource,
		NewRolesDataSource,
		NewOrganizationIPRestrictionDataSource,
		NewOrganizationIPRestrictionsDataSource,
		NewReportDataSource,
		NewReportsDataSource,
		NewTLDPortfolioDataSource,
		NewZonesSummaryDataSource,
		NewDomainsSummaryDataSource,
		NewDomainSuggestionsDataSource,
		NewEventDataSource,
		NewEventsDataSource,
	}
}

func (p *OpusDNSProvider) Functions(_ context.Context) []func() function.Function {
	return []func() function.Function{}
}

// stringValue returns the underlying string of a types.String, or "" if null/unknown.
func stringValue(s types.String) string {
	if s.IsNull() || s.IsUnknown() {
		return ""
	}
	return s.ValueString()
}

// firstNonEmpty returns the first non-empty string from the provided list.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func loadProviderConfig(data OpusDNSProviderModel, getenv func(string) string) providerConfig {
	return providerConfig{
		APIKey:      firstNonEmpty(stringValue(data.APIKey), getenv("OPUSDNS_API_KEY")),
		APIEndpoint: firstNonEmpty(stringValue(data.APIEndpoint), getenv("OPUSDNS_API_ENDPOINT"), opusdns.DefaultAPIEndpoint),
	}
}
