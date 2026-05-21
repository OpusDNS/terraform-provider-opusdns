package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDomainForwardResource_basic(t *testing.T) {
	zoneName := testAccDomainName()

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDomainForwardResourceConfig(zoneName, "/"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("opusdns_domain_forward.test", "id", "opusdns_domain_forward.test", "hostname"),
					resource.TestCheckResourceAttr("opusdns_domain_forward.test", "enabled", "true"),
					resource.TestCheckResourceAttr("opusdns_domain_forward.test", "https.#", "1"),
					resource.TestCheckResourceAttr("opusdns_domain_forward.test", "https.0.request_path", "/"),
					resource.TestCheckResourceAttr("opusdns_domain_forward.test", "https.0.target_protocol", "https"),
					resource.TestCheckResourceAttrSet("opusdns_domain_forward.test", "https.0.target_hostname"),
					resource.TestCheckResourceAttr("opusdns_domain_forward.test", "https.0.target_path", "/"),
					resource.TestCheckResourceAttr("opusdns_domain_forward.test", "https.0.redirect_code", "301"),
				),
			},
		},
	})
}

func TestAccDomainForwardResource_update(t *testing.T) {
	zoneName := testAccDomainName()

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDomainForwardResourceConfig(zoneName, "/"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_domain_forward.test", "https.0.target_path", "/"),
				),
			},
			{
				Config: testAccDomainForwardResourceConfig(zoneName, "/updated"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_domain_forward.test", "https.0.target_path", "/updated"),
					resource.TestCheckResourceAttr("opusdns_domain_forward.test", "https.0.redirect_code", "301"),
				),
			},
		},
	})
}

func testAccDomainForwardResourceConfig(zoneName, targetPath string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_zone" "test" {
  name = %q
}

resource "opusdns_domain_forward" "test" {
  hostname = format("www.%s", opusdns_zone.test.name)

  https = [
    {
      request_path    = "/"
      target_protocol = "https"
      target_hostname = "example.net"
      target_path     = %q
      redirect_code   = 301
    }
  ]
}
`, testAccProviderConfig, zoneName, "%s", targetPath)
}
