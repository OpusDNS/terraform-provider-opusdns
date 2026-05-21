package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRecordDataSource_basic(t *testing.T) {
	zoneName := testAccDomainName()

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRecordDataSourceConfigBasic(zoneName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.opusdns_record.test", "zone_name", "opusdns_record.test", "zone_name"),
					resource.TestCheckResourceAttrPair("data.opusdns_record.test", "name", "opusdns_record.test", "name"),
					resource.TestCheckResourceAttrPair("data.opusdns_record.test", "type", "opusdns_record.test", "type"),
					resource.TestCheckResourceAttrPair("data.opusdns_record.test", "id", "opusdns_record.test", "id"),
					resource.TestCheckResourceAttrPair("data.opusdns_record.test", "ttl", "opusdns_record.test", "ttl"),
					resource.TestCheckResourceAttrPair("data.opusdns_record.test", "records.#", "opusdns_record.test", "records.#"),
					resource.TestCheckResourceAttrPair("data.opusdns_record.test", "records.0", "opusdns_record.test", "records.0"),
				),
			},
		},
	})
}

func TestAccRecordsDataSource_basic(t *testing.T) {
	zoneName := testAccDomainName()

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRecordsDataSourceConfigBasic(zoneName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.opusdns_records.test", "zone_name", "opusdns_record.www", "zone_name"),
					resource.TestCheckResourceAttr("data.opusdns_records.test", "records.#", "1"),
					resource.TestCheckResourceAttrPair("data.opusdns_records.test", "records.0.name", "opusdns_record.www", "name"),
					resource.TestCheckResourceAttrPair("data.opusdns_records.test", "records.0.type", "opusdns_record.www", "type"),
					resource.TestCheckResourceAttrPair("data.opusdns_records.test", "records.0.ttl", "opusdns_record.www", "ttl"),
					resource.TestCheckResourceAttrPair("data.opusdns_records.test", "records.0.records.#", "opusdns_record.www", "records.#"),
					resource.TestCheckResourceAttrPair("data.opusdns_records.test", "records.0.records.0", "opusdns_record.www", "records.0"),
				),
			},
		},
	})
}

func testAccRecordDataSourceConfigBasic(zoneName string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_zone" "test" {
  name = %q
}

resource "opusdns_record" "test" {
  zone_name = opusdns_zone.test.name
  name      = "www"
  type      = "A"
  records   = ["192.0.2.1"]
}

data "opusdns_record" "test" {
  zone_name = opusdns_zone.test.name
  name      = opusdns_record.test.name
  type      = opusdns_record.test.type
}
`, testAccProviderConfig, zoneName)
}

func testAccRecordsDataSourceConfigBasic(zoneName string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_zone" "test" {
  name = %q
}

resource "opusdns_record" "www" {
  zone_name = opusdns_zone.test.name
  name      = "www"
  type      = "A"
  records   = ["192.0.2.1"]
}

resource "opusdns_record" "api" {
  zone_name = opusdns_zone.test.name
  name      = "api"
  type      = "A"
  records   = ["192.0.2.2"]
}

data "opusdns_records" "test" {
  zone_name = opusdns_zone.test.name
  name      = opusdns_record.www.name
  type      = opusdns_record.www.type
}
`, testAccProviderConfig, zoneName)
}
