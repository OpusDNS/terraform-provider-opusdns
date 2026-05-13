package provider

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

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
//
// Authentication follows the OAuth2 flows described in
// api/dev-resources/neovim-api-requests/{auth-login,api-key-connect-test}.http.
//
// Three credential modes are supported, selected in this priority order:
//
//  1. Pre-minted client credentials: supply `org_id` + `client_secret` (and
//     optionally `api_key` for logging). The provider performs only the final
//     /v1/auth/token client_credentials exchange to obtain a bearer access
//     token. Best for automation.
//  2. Full 3-step bootstrap: supply `username` + `password` + `org_id`. The
//     provider runs password grant -> mint api_key/client_secret ->
//     client_credentials grant. A new API key is minted on every Configure
//     call.
//  3. User-token: supply `username` + `password` only (no `org_id`,
//     no `client_secret`). The provider performs the single-step password
//     grant and uses the resulting user access_token directly as the bearer
//     token. The token is already scoped to the user's organization via the
//     JWT `oid` claim. Suitable for endpoints that accept either a user token
//     or client_id+client_secret.
type OpusDNSProviderModel struct {
	Username     types.String `tfsdk:"username"`
	Password     types.String `tfsdk:"password"`
	OrgID        types.String `tfsdk:"org_id"`
	APIKey       types.String `tfsdk:"api_key"`
	ClientSecret types.String `tfsdk:"client_secret"`
	APIEndpoint  types.String `tfsdk:"api_endpoint"`
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
		MarkdownDescription: "The OpusDNS provider manages DNS zones, records, contacts, email forwards, and domain forwards via the OpusDNS API. " +
			"Authentication is performed via the `/v1/auth` OAuth2 endpoints. Three modes are supported (in priority order): " +
			"(1) supply `client_secret` (with `org_id`) to skip directly to the client_credentials grant; " +
			"(2) supply `username` + `password` + `org_id` to run the full password \u2192 mint \u2192 client_credentials flow; " +
			"(3) supply `username` + `password` (without `org_id`) to use the user access_token from the password grant directly as the bearer token.",
		Attributes: map[string]schema.Attribute{
			"username": schema.StringAttribute{
				MarkdownDescription: "OpusDNS user username for the password grant. Ignored when `client_secret` is set. Can also be set via the `OPUSDNS_USERNAME` environment variable.",
				Optional:            true,
			},
			"password": schema.StringAttribute{
				MarkdownDescription: "OpusDNS user password for the password grant. Ignored when `client_secret` is set. Can also be set via the `OPUSDNS_PASSWORD` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"org_id": schema.StringAttribute{
				MarkdownDescription: "OpusDNS organization id (used as the `client_id` in the client_credentials grant, e.g. `organization_...`). Required when authenticating via `client_secret` or the full 3-step password flow. Omit to use the single-step user-token flow (the user's org is taken from the JWT `oid` claim). Can also be set via the `OPUSDNS_ORG_ID` environment variable.",
				Optional:            true,
			},
			"api_key": schema.StringAttribute{
				MarkdownDescription: "Pre-minted OpusDNS API key (the `api_key` returned by `/v1/auth/client_credentials`). Optional companion to `client_secret`; not required for the token exchange but accepted for completeness/logging. Can also be set via the `OPUSDNS_API_KEY` environment variable.",
				Optional:            true,
				Sensitive:           true,
			},
			"client_secret": schema.StringAttribute{
				MarkdownDescription: "Pre-minted OpusDNS client secret (the `client_secret` returned by `/v1/auth/client_credentials`). When set (together with `org_id`), the provider skips the user password grant and the API-key minting step, and exchanges the secret directly for a bearer access token. Can also be set via the `OPUSDNS_CLIENT_SECRET` environment variable.",
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

	username := firstNonEmpty(stringValue(data.Username), os.Getenv("OPUSDNS_USERNAME"))
	password := firstNonEmpty(stringValue(data.Password), os.Getenv("OPUSDNS_PASSWORD"))
	orgID := firstNonEmpty(stringValue(data.OrgID), os.Getenv("OPUSDNS_ORG_ID"))
	clientSecret := firstNonEmpty(stringValue(data.ClientSecret), os.Getenv("OPUSDNS_CLIENT_SECRET"))
	// api_key is accepted but not strictly required for the token exchange.
	_ = firstNonEmpty(stringValue(data.APIKey), os.Getenv("OPUSDNS_API_KEY"))
	endpoint := firstNonEmpty(stringValue(data.APIEndpoint), os.Getenv("OPUSDNS_API_ENDPOINT"), opusdns.DefaultAPIEndpoint)

	// org_id is required only for client_credentials-based modes; the
	// single-step user-token flow derives the org from the JWT.
	auth := &authClient{
		endpoint: endpoint,
		http:     &http.Client{Timeout: 30 * time.Second},
	}

	var (
		accessToken string
		err         error
	)

	switch {
	case clientSecret != "":
		// Preferred path: skip user login and api_key minting; go straight to
		// the client_credentials grant with the pre-minted secret. Requires
		// org_id as the client_id.
		if orgID == "" {
			resp.Diagnostics.AddError(
				"Missing OpusDNS Credentials",
				"`org_id` (or OPUSDNS_ORG_ID) is required when authenticating with `client_secret`.",
			)
			return
		}
		accessToken, err = auth.clientCredentialsGrant(ctx, orgID, clientSecret)
		if err != nil {
			resp.Diagnostics.AddError(
				"OpusDNS Authentication Failed",
				fmt.Sprintf("client_credentials grant against %s failed: %s", endpoint, err.Error()),
			)
			return
		}

	case username != "" && password != "" && orgID != "":
		// Full 3-step flow. A new API key is minted on every Configure call.
		apiKeyName := fmt.Sprintf("terraform-provider-opusdns-%s-%d", p.version, time.Now().UnixNano())
		accessToken, _, _, err = auth.login(ctx, username, password, orgID, apiKeyName, "Auto-generated by terraform-provider-opusdns")
		if err != nil {
			resp.Diagnostics.AddError(
				"OpusDNS Authentication Failed",
				fmt.Sprintf("Failed to obtain bearer token from %s: %s", endpoint, err.Error()),
			)
			return
		}

	case username != "" && password != "":
		// Single-step user-token flow: use the password-grant access_token
		// directly as the bearer token. Org is implied by the JWT `oid` claim.
		accessToken, err = auth.passwordGrant(ctx, username, password)
		if err != nil {
			resp.Diagnostics.AddError(
				"OpusDNS Authentication Failed",
				fmt.Sprintf("password grant against %s failed: %s", endpoint, err.Error()),
			)
			return
		}

	default:
		resp.Diagnostics.AddError(
			"Missing OpusDNS Credentials",
			"Provide one of: `client_secret` + `org_id`; or `username` + `password` + `org_id`; or `username` + `password` alone. "+
				"Equivalent environment variables: OPUSDNS_CLIENT_SECRET + OPUSDNS_ORG_ID, "+
				"or OPUSDNS_USERNAME + OPUSDNS_PASSWORD (+ optional OPUSDNS_ORG_ID).",
		)
		return
	}

	// Build an http.Client whose transport injects `Authorization: Bearer <token>`
	// in place of the SDK's default `X-Api-Key` header.
	httpClient := &http.Client{
		Timeout: opusdns.DefaultTimeout,
		Transport: &bearerTransport{
			token: accessToken,
			base:  http.DefaultTransport,
		},
	}

	opts := []opusdns.Option{
		// APIKey is required by the SDK's Validate() but is stripped by our
		// transport before the request leaves the process. Set it to the bearer
		// token so any code path that inspects it still has a non-empty value.
		opusdns.WithAPIKey(accessToken),
		opusdns.WithAPIEndpoint(endpoint),
		opusdns.WithUserAgent("terraform-provider-opusdns/" + p.version),
		opusdns.WithHTTPClient(httpClient),
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
		NewOrganizationResource,
		NewUserResource,
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
		NewContactDataSource,
		NewContactsDataSource,
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
