package provider

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccHostResource_basic(t *testing.T) {
	domainName := testAccDomainName()
	contactKey := fmt.Sprintf("tfacc-%d", rand.Int63())
	hostname := fmt.Sprintf("ns1.%s", domainName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccHostResourceConfigBasic(domainName, hostname, contactKey),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("opusdns_host.test", "id"),
					resource.TestCheckResourceAttrSet("opusdns_host.test", "host_id"),
					resource.TestCheckResourceAttr("opusdns_host.test", "hostname", hostname),
					resource.TestCheckResourceAttr("opusdns_host.test", "ip_addresses.#", "1"),
					resource.TestCheckTypeSetElemAttr("opusdns_host.test", "ip_addresses.*", "1.1.1.1"),
					resource.TestCheckResourceAttrSet("opusdns_host.test", "created_on"),
					resource.TestCheckResourceAttrSet("opusdns_host.test", "updated_on"),
				),
			},
		},
	})
}

func TestAccHostResource_updateIPs(t *testing.T) {
	domainName := testAccDomainName()
	contactKey := fmt.Sprintf("tfacc-%d", rand.Int63())
	hostname := fmt.Sprintf("ns1.%s", domainName)

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccHostResourceConfigBasic(domainName, hostname, contactKey),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_host.test", "ip_addresses.#", "1"),
					resource.TestCheckTypeSetElemAttr("opusdns_host.test", "ip_addresses.*", "1.1.1.1"),
				),
			},
			{
				Config: testAccHostResourceConfigUpdate(domainName, hostname, contactKey),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_host.test", "ip_addresses.#", "2"),
					resource.TestCheckTypeSetElemAttr("opusdns_host.test", "ip_addresses.*", "8.8.8.8"),
					resource.TestCheckTypeSetElemAttr("opusdns_host.test", "ip_addresses.*", "2606:4700:4700::1111"),
				),
			},
		},
	})
}

func testAccHostResourceConfigBasic(domainName, _ string, contactKey string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_contact" "test" {
  first_name  = "Terraform"
  last_name   = "Acceptance"
  org         = "OpusDNS"
  email       = "%s@example.com"
  phone       = "+1.2125551234"
  street      = "123 Terraform Street"
  city        = "Exampleville"
  state       = "NY"
  postal_code = "10001"
  country     = "US"
}

resource "opusdns_domain" "test" {
  name         = %q
  period_value = 1
  period_unit  = "y"
  create_zone  = true

  contacts = {
    registrant = [opusdns_contact.test.contact_id]
    admin      = [opusdns_contact.test.contact_id]
    tech       = [opusdns_contact.test.contact_id]
  }
}

resource "opusdns_host" "test" {
  hostname     = format("ns1.%%s", opusdns_domain.test.name)
  ip_addresses = ["1.1.1.1"]
}
`, testAccProviderConfig, contactKey, domainName)
}

func testAccHostResourceConfigUpdate(domainName, _ string, contactKey string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_contact" "test" {
  first_name  = "Terraform"
  last_name   = "Acceptance"
  org         = "OpusDNS"
  email       = "%s@example.com"
  phone       = "+1.2125551234"
  street      = "123 Terraform Street"
  city        = "Exampleville"
  state       = "NY"
  postal_code = "10001"
  country     = "US"
}

resource "opusdns_domain" "test" {
  name         = %q
  period_value = 1
  period_unit  = "y"
  create_zone  = true

  contacts = {
    registrant = [opusdns_contact.test.contact_id]
    admin      = [opusdns_contact.test.contact_id]
    tech       = [opusdns_contact.test.contact_id]
  }
}

resource "opusdns_host" "test" {
  hostname     = format("ns1.%%s", opusdns_domain.test.name)
  ip_addresses = ["8.8.8.8", "2606:4700:4700::1111"]
}
`, testAccProviderConfig, contactKey, domainName)
}
