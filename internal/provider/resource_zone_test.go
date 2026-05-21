package provider

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccZoneResource_basic(t *testing.T) {
	zoneName := fmt.Sprintf("tfacc-%d.test", rand.Int63())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and verify
			{
				Config: testAccZoneResourceConfig(zoneName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_zone.test", "name", zoneName),
					resource.TestCheckResourceAttr("opusdns_zone.test", "id", zoneName),
					resource.TestCheckResourceAttrSet("opusdns_zone.test", "dnssec_status"),
				),
			},
			// ImportState
			{
				ResourceName:      "opusdns_zone.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccZoneResource_subzone(t *testing.T) {
	baseName := fmt.Sprintf("tfacc-%d.test", rand.Int63())
	subName := fmt.Sprintf("sub.%s", baseName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccZoneResourceConfigSubzone(baseName, subName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_zone.base", "name", baseName),
					resource.TestCheckResourceAttr("opusdns_zone.sub", "name", subName),
				),
			},
		},
	})
}

func TestAccZoneResource_dnssec(t *testing.T) {
	zoneName := fmt.Sprintf("tfacc-%d.test", rand.Int63())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with DNSSEC disabled (default)
			{
				Config: testAccZoneResourceConfig(zoneName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_zone.test", "name", zoneName),
				),
			},
			// Enable DNSSEC
			{
				Config: testAccZoneResourceConfigDNSSEC(zoneName, "enabled"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_zone.test", "dnssec_status", "enabled"),
				),
			},
			// Disable DNSSEC
			{
				Config: testAccZoneResourceConfigDNSSEC(zoneName, "disabled"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_zone.test", "dnssec_status", "disabled"),
				),
			},
		},
	})
}

func testAccZoneResourceConfig(name string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_zone" "test" {
  name = %q
}
`, testAccProviderConfig, name)
}

func testAccZoneResourceConfigSubzone(baseName, subName string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_zone" "base" {
  name = %q
}

resource "opusdns_zone" "sub" {
  name = %q
}
`, testAccProviderConfig, baseName, subName)
}

func testAccZoneResourceConfigDNSSEC(name, status string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_zone" "test" {
  name         = %q
  dnssec_status = %q
}
`, testAccProviderConfig, name, status)
}
