package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccEmailForwardResource_basic(t *testing.T) {
	zoneName := testAccDomainName()
	forwardTo := fmt.Sprintf("admin@%s", zoneName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccEmailForwardResourceConfigBasic(zoneName, forwardTo),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("opusdns_email_forward.test", "id", "opusdns_email_forward.test", "email_forward_id"),
					resource.TestCheckResourceAttrSet("opusdns_email_forward.test", "email_forward_id"),
					resource.TestCheckResourceAttr("opusdns_email_forward.test", "enabled", "true"),
					resource.TestCheckResourceAttr("opusdns_email_forward.test", "aliases.#", "1"),
					resource.TestCheckResourceAttrSet("opusdns_email_forward.test", "aliases.0.alias_id"),
					resource.TestCheckResourceAttr("opusdns_email_forward.test", "aliases.0.alias", "info"),
					resource.TestCheckResourceAttr("opusdns_email_forward.test", "aliases.0.forward_to.#", "1"),
					resource.TestCheckResourceAttr("opusdns_email_forward.test", "aliases.0.forward_to.0", forwardTo),
				),
			},
		},
	})
}

func testAccEmailForwardResourceConfigBasic(zoneName, forwardTo string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_zone" "test" {
  name = %q
}

resource "opusdns_email_forward" "test" {
  hostname = opusdns_zone.test.name

  aliases = [
    {
      alias      = "info"
      forward_to = [%q]
    }
  ]
}
`, testAccProviderConfig, zoneName, forwardTo)
}
