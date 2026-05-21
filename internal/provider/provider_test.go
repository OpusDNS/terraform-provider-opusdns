package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
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

	if v := os.Getenv("OPUSDNS_CLIENT_SECRET"); v == "" {
		t.Fatal("OPUSDNS_CLIENT_SECRET must be set for acceptance tests")
	}
	if v := os.Getenv("OPUSDNS_ORG_ID"); v == "" {
		t.Fatal("OPUSDNS_ORG_ID must be set for acceptance tests")
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
