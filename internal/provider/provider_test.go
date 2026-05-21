package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/opusdns/opusdns-go-client/opusdns"
)

// testAccProtoV6ProviderFactories are used to instantiate a provider during
// acceptance testing. The factory function will be invoked for every Terraform
// CLI command executed to create a provider server to which the CLI can
// reattach.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"opusdns": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck validates the required environment variables are set so
// acceptance tests can run. Call this in every TestAcc function.
func testAccPreCheck(t *testing.T) {
	t.Helper()

	if v := os.Getenv("OPUSDNS_API_KEY"); v == "" {
		t.Fatal("OPUSDNS_API_KEY must be set for acceptance tests")
	}
	if v := os.Getenv("OPUSDNS_API_ENDPOINT"); v == "" {
		t.Fatal("OPUSDNS_API_ENDPOINT must be set for acceptance tests")
	}
}

// testAccProviderConfig returns the provider configuration block that all
// acceptance test configs should include. Credentials are read from environment
// variables (set via GitHub Secrets in CI).
const testAccProviderConfig = `
provider "opusdns" {}
`

func TestLoadProviderConfigPrefersProviderValues(t *testing.T) {
	t.Parallel()

	config := loadProviderConfig(
		OpusDNSProviderModel{
			APIKey:      types.StringValue("provider-key"),
			APIEndpoint: types.StringValue("https://provider.example"),
		},
		func(string) string {
			return "ignored"
		},
	)

	if config.APIKey != "provider-key" {
		t.Fatalf("expected provider api key to win, got %q", config.APIKey)
	}
	if config.APIEndpoint != "https://provider.example" {
		t.Fatalf("expected provider endpoint to win, got %q", config.APIEndpoint)
	}
}

func TestLoadProviderConfigFallsBackToEnvironment(t *testing.T) {
	t.Parallel()

	env := map[string]string{
		"OPUSDNS_API_KEY":      "env-key",
		"OPUSDNS_API_ENDPOINT": "https://env.example",
	}

	config := loadProviderConfig(OpusDNSProviderModel{}, func(key string) string {
		return env[key]
	})

	if config.APIKey != "env-key" {
		t.Fatalf("expected environment api key, got %q", config.APIKey)
	}
	if config.APIEndpoint != "https://env.example" {
		t.Fatalf("expected environment endpoint, got %q", config.APIEndpoint)
	}
}

func TestLoadProviderConfigUsesDefaultEndpoint(t *testing.T) {
	t.Parallel()

	config := loadProviderConfig(
		OpusDNSProviderModel{
			APIKey: types.StringValue("provider-key"),
		},
		func(string) string {
			return ""
		},
	)

	if config.APIKey != "provider-key" {
		t.Fatalf("expected provider api key, got %q", config.APIKey)
	}
	if config.APIEndpoint != opusdns.DefaultAPIEndpoint {
		t.Fatalf("expected default endpoint %q, got %q", opusdns.DefaultAPIEndpoint, config.APIEndpoint)
	}
}
