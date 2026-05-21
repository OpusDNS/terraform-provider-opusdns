package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccOrganizationIPRestrictionResource_basic(t *testing.T) {
	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOrganizationIPRestrictionResourceConfig("0.0.0.0/0"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("opusdns_organization_ip_restriction.test", "id"),
					resource.TestCheckResourceAttrSet("opusdns_organization_ip_restriction.test", "ip_restriction_id"),
					resource.TestCheckResourceAttrSet("opusdns_organization_ip_restriction.test", "organization_id"),
					resource.TestCheckResourceAttr("opusdns_organization_ip_restriction.test", "ip_network", "0.0.0.0/0"),
					resource.TestCheckResourceAttrSet("opusdns_organization_ip_restriction.test", "created_on"),
				),
			},
		},
	})
}

func testAccOrganizationIPRestrictionResourceConfig(ipNetwork string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_organization_ip_restriction" "test" {
  ip_network      = %q
}
`, testAccProviderConfig, ipNetwork)
}
