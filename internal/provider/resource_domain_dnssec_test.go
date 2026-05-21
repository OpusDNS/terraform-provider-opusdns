package provider

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDomainDNSSECResource_basic(t *testing.T) {
	domainName := testAccDomainName()
	contactKey := fmt.Sprintf("tfacc-%d", rand.Int63())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDomainDNSSECResourceConfig(domainName, contactKey),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_domain.test", "name", domainName),
					resource.TestCheckResourceAttr("opusdns_domain_dnssec.test", "id", domainName),
					resource.TestCheckResourceAttr("opusdns_domain_dnssec.test", "domain_ref", domainName),
					resource.TestCheckResourceAttr("opusdns_domain_dnssec.test", "enabled", "false"),
					resource.TestCheckResourceAttr("opusdns_domain_dnssec.test", "records.0.record_type", "ds_data"),
					resource.TestCheckResourceAttr("opusdns_domain_dnssec.test", "records.0.algorithm", "13"),
					resource.TestCheckResourceAttr("opusdns_domain_dnssec.test", "records.0.digest_type", "2"),
					resource.TestCheckResourceAttr("opusdns_domain_dnssec.test", "records.0.key_tag", "12345"),
					resource.TestCheckResourceAttrSet("opusdns_domain_dnssec.test", "records.0.record_type"),
				),
			},
		},
	})
}

func testAccDomainDNSSECResourceConfig(domainName, contactKey string) string {
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

resource "opusdns_domain_dnssec" "test" {
  domain_ref = opusdns_domain.test.name

  records = [
    {
      record_type = "ds_data"
      algorithm   = 13
      digest      = "0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF"
      digest_type = 2
      key_tag     = 12345
    }
  ]
}
`, testAccProviderConfig, contactKey, domainName)
}
