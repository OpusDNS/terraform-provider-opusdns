package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccZoneDataSource_basic(t *testing.T) {
	zoneName := testAccDomainName()

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccZoneDataSourceConfigBasic(zoneName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.opusdns_zone.test", "name", "opusdns_zone.test", "name"),
					resource.TestCheckResourceAttrPair("data.opusdns_zone.test", "id", "opusdns_zone.test", "id"),
					resource.TestCheckResourceAttrPair("data.opusdns_zone.test", "dnssec_status", "opusdns_zone.test", "dnssec_status"),
				),
			},
		},
	})
}

func TestAccZonesDataSource_basic(t *testing.T) {
	zoneNameOne := testAccDomainName()
	zoneNameTwo := testAccDomainName()

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccZonesDataSourceConfigBasic(zoneNameOne, zoneNameTwo),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.opusdns_zones.test", "id", "zones"),
					resource.TestCheckResourceAttrSet("data.opusdns_zones.test", "zones.0.name"),
					resource.TestCheckResourceAttrSet("data.opusdns_zones.test", "zones.0.dnssec_status"),
				),
			},
		},
	})
}

func testAccZoneDataSourceConfigBasic(zoneName string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_zone" "test" {
  name = %q
}

data "opusdns_zone" "test" {
  name = opusdns_zone.test.name
}
`, testAccProviderConfig, zoneName)
}

func testAccZonesDataSourceConfigBasic(zoneNameOne, zoneNameTwo string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_zone" "one" {
  name = %q
}

resource "opusdns_zone" "two" {
  name = %q
}

data "opusdns_zones" "test" {
  depends_on = [opusdns_zone.one, opusdns_zone.two]
}
`, testAccProviderConfig, zoneNameOne, zoneNameTwo)
}
