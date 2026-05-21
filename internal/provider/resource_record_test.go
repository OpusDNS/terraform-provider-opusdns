package provider

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRecordResource_A(t *testing.T) {
	zoneName := fmt.Sprintf("tfacc-%d.test", rand.Int63())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create A record
			{
				Config: testAccRecordResourceConfig_A(zoneName, "www", "192.0.2.1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_record.test", "zone_name", zoneName),
					resource.TestCheckResourceAttr("opusdns_record.test", "name", "www"),
					resource.TestCheckResourceAttr("opusdns_record.test", "type", "A"),
					resource.TestCheckResourceAttr("opusdns_record.test", "ttl", "60"),
					resource.TestCheckResourceAttr("opusdns_record.test", "records.#", "1"),
					resource.TestCheckResourceAttr("opusdns_record.test", "records.0", "192.0.2.1"),
				),
			},
			// ImportState
			{
				ResourceName:      "opusdns_record.test",
				ImportState:       true,
				ImportStateId:     fmt.Sprintf("%s/www/A", zoneName),
				ImportStateVerify: true,
			},
			// Update: change IP
			{
				Config: testAccRecordResourceConfig_A(zoneName, "www", "192.0.2.2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_record.test", "records.0", "192.0.2.2"),
				),
			},
		},
	})
}

func TestAccRecordResource_AAAA(t *testing.T) {
	zoneName := fmt.Sprintf("tfacc-%d.test", rand.Int63())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRecordResourceConfig_AAAA(zoneName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_record.test", "type", "AAAA"),
					resource.TestCheckResourceAttr("opusdns_record.test", "records.0", "2001:db8::1"),
				),
			},
		},
	})
}

func TestAccRecordResource_CNAME(t *testing.T) {
	zoneName := fmt.Sprintf("tfacc-%d.test", rand.Int63())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRecordResourceConfig_CNAME(zoneName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_record.test", "name", "alias"),
					resource.TestCheckResourceAttr("opusdns_record.test", "type", "CNAME"),
					resource.TestCheckResourceAttr("opusdns_record.test", "records.0", "www.example.com"),
				),
			},
		},
	})
}

func TestAccRecordResource_MX(t *testing.T) {
	zoneName := fmt.Sprintf("tfacc-%d.test", rand.Int63())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRecordResourceConfig_MX(zoneName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_record.test", "name", "@"),
					resource.TestCheckResourceAttr("opusdns_record.test", "type", "MX"),
					resource.TestCheckResourceAttr("opusdns_record.test", "records.#", "2"),
				),
			},
		},
	})
}

func TestAccRecordResource_TXT(t *testing.T) {
	zoneName := fmt.Sprintf("tfacc-%d.test", rand.Int63())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRecordResourceConfig_TXT(zoneName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_record.test", "type", "TXT"),
					resource.TestCheckResourceAttr("opusdns_record.test", "records.0", "v=spf1 include:_spf.example.com ~all"),
				),
			},
		},
	})
}

func TestAccRecordResource_multiRecord(t *testing.T) {
	zoneName := fmt.Sprintf("tfacc-%d.test", rand.Int63())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRecordResourceConfig_multiA(zoneName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_record.test", "records.#", "3"),
				),
			},
		},
	})
}

func TestAccRecordResource_updateTTL(t *testing.T) {
	zoneName := fmt.Sprintf("tfacc-%d.test", rand.Int63())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with default TTL
			{
				Config: testAccRecordResourceConfig_A(zoneName, "ttltest", "192.0.2.1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_record.test", "ttl", "60"),
				),
			},
			// Update TTL to 3600
			{
				Config: testAccRecordResourceConfig_customTTL(zoneName, "ttltest", "192.0.2.1", 3600),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_record.test", "ttl", "3600"),
				),
			},
		},
	})
}

func TestAccRecordResource_NS(t *testing.T) {
	zoneName := fmt.Sprintf("tfacc-%d.test", rand.Int63())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRecordResourceConfig_NS(zoneName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_record.test", "type", "NS"),
					resource.TestCheckResourceAttr("opusdns_record.test", "records.#", "2"),
				),
			},
		},
	})
}

func TestAccRecordResource_SRV(t *testing.T) {
	zoneName := fmt.Sprintf("tfacc-%d.test", rand.Int63())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRecordResourceConfig_SRV(zoneName),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_record.test", "type", "SRV"),
					resource.TestCheckResourceAttr("opusdns_record.test", "name", "_sip._tcp"),
				),
			},
		},
	})
}

// --- Config helpers ---

func testAccRecordResourceConfig_A(zoneName, name, ip string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_zone" "test" {
  name = %q
}

resource "opusdns_record" "test" {
  zone_name = opusdns_zone.test.name
  name      = %q
  type      = "A"
  records   = [%q]
}
`, testAccProviderConfig, zoneName, name, ip)
}

func testAccRecordResourceConfig_customTTL(zoneName, name, ip string, ttl int) string {
	return fmt.Sprintf(`
%s

resource "opusdns_zone" "test" {
  name = %q
}

resource "opusdns_record" "test" {
  zone_name = opusdns_zone.test.name
  name      = %q
  type      = "A"
  ttl       = %d
  records   = [%q]
}
`, testAccProviderConfig, zoneName, name, ttl, ip)
}

func testAccRecordResourceConfig_AAAA(zoneName string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_zone" "test" {
  name = %q
}

resource "opusdns_record" "test" {
  zone_name = opusdns_zone.test.name
  name      = "ipv6"
  type      = "AAAA"
  records   = ["2001:db8::1"]
}
`, testAccProviderConfig, zoneName)
}

func testAccRecordResourceConfig_CNAME(zoneName string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_zone" "test" {
  name = %q
}

resource "opusdns_record" "test" {
  zone_name = opusdns_zone.test.name
  name      = "alias"
  type      = "CNAME"
  records   = ["www.example.com"]
}
`, testAccProviderConfig, zoneName)
}

func testAccRecordResourceConfig_MX(zoneName string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_zone" "test" {
  name = %q
}

resource "opusdns_record" "test" {
  zone_name = opusdns_zone.test.name
  name      = "@"
  type      = "MX"
  records   = ["10 mail1.example.com", "20 mail2.example.com"]
}
`, testAccProviderConfig, zoneName)
}

func testAccRecordResourceConfig_TXT(zoneName string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_zone" "test" {
  name = %q
}

resource "opusdns_record" "test" {
  zone_name = opusdns_zone.test.name
  name      = "@"
  type      = "TXT"
  records   = ["v=spf1 include:_spf.example.com ~all"]
}
`, testAccProviderConfig, zoneName)
}

func testAccRecordResourceConfig_multiA(zoneName string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_zone" "test" {
  name = %q
}

resource "opusdns_record" "test" {
  zone_name = opusdns_zone.test.name
  name      = "multi"
  type      = "A"
  records   = ["192.0.2.1", "192.0.2.2", "192.0.2.3"]
}
`, testAccProviderConfig, zoneName)
}

func testAccRecordResourceConfig_NS(zoneName string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_zone" "test" {
  name = %q
}

resource "opusdns_record" "test" {
  zone_name = opusdns_zone.test.name
  name      = "delegated"
  type      = "NS"
  records   = ["ns1.example.com", "ns2.example.com"]
}
`, testAccProviderConfig, zoneName)
}

func testAccRecordResourceConfig_SRV(zoneName string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_zone" "test" {
  name = %q
}

resource "opusdns_record" "test" {
  zone_name = opusdns_zone.test.name
  name      = "_sip._tcp"
  type      = "SRV"
  records   = ["10 60 5060 sip.example.com"]
}
`, testAccProviderConfig, zoneName)
}
