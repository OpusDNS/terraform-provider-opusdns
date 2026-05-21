package provider

import (
	"fmt"
	"math/rand"
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

// randomName generates a unique name for test resources to avoid collisions
// when tests run in parallel. The prefix helps identify which test created
// the resource for debugging.
func randomName(prefix string) string {
	return fmt.Sprintf("tfacc-%s-%d", prefix, rand.Int63())
}

// testAccProviderConfig returns the provider configuration block that all
// acceptance test configs should include. Credentials are read from environment
// variables (set via GitHub Secrets in CI).
const testAccProviderConfig = `
provider "opusdns" {}
`
