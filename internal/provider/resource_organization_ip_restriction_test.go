package provider

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccOrganizationIPRestrictionResource_basic(t *testing.T) {
	orgName := fmt.Sprintf("tfacc-ip-org-%d", rand.Int63())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOrganizationIPRestrictionResourceConfig(orgName, "0.0.0.0/0"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("opusdns_organization_ip_restriction.test", "id"),
					resource.TestCheckResourceAttrSet("opusdns_organization_ip_restriction.test", "ip_restriction_id"),
					resource.TestCheckResourceAttrPair("opusdns_organization_ip_restriction.test", "organization_id", "opusdns_organization.test", "organization_id"),
					resource.TestCheckResourceAttr("opusdns_organization_ip_restriction.test", "ip_network", "0.0.0.0/0"),
					resource.TestCheckResourceAttrSet("opusdns_organization_ip_restriction.test", "created_on"),
				),
			},
		},
	})
}

func testAccOrganizationIPRestrictionResourceConfig(orgName, ipNetwork string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_organization" "test" {
  name = %q
}

resource "opusdns_organization_ip_restriction" "test" {
  organization_id = opusdns_organization.test.organization_id
  ip_network      = %q
}
`, testAccProviderConfig, orgName, ipNetwork)
}
