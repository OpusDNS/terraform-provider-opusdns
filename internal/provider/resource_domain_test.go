package provider

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDomainResource_basic(t *testing.T) {
	domainLabel := fmt.Sprintf("tfacc-%d", rand.Int63())
	domainName := fmt.Sprintf("%s.test", domainLabel)
	contactKey := fmt.Sprintf("tfacc-%d", rand.Int63())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDomainResourceConfigBasic(domainName, contactKey),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("opusdns_domain.test", "id"),
					resource.TestCheckResourceAttrSet("opusdns_domain.test", "domain_id"),
					resource.TestCheckResourceAttrSet("opusdns_domain.test", "owner_id"),
					resource.TestCheckResourceAttrSet("opusdns_domain.test", "registry_account_id"),
					resource.TestCheckResourceAttr("opusdns_domain.test", "name", domainName),
					resource.TestCheckResourceAttr("opusdns_domain.test", "sld", domainLabel),
					resource.TestCheckResourceAttr("opusdns_domain.test", "tld", "test"),
					resource.TestCheckResourceAttr("opusdns_domain.test", "period_value", "1"),
					resource.TestCheckResourceAttr("opusdns_domain.test", "period_unit", "y"),
					resource.TestCheckResourceAttr("opusdns_domain.test", "create_zone", "true"),
					resource.TestCheckResourceAttr("opusdns_domain.test", "renewal_mode", "renew"),
					resource.TestCheckResourceAttr("opusdns_domain.test", "transfer_lock", "true"),
					resource.TestCheckResourceAttrSet("opusdns_domain.test", "registered_on"),
					resource.TestCheckResourceAttrSet("opusdns_domain.test", "expires_on"),
				),
			},
		},
	})
}

func TestAccDomainResource_updateTransferLock(t *testing.T) {
	domainName := fmt.Sprintf("tfacc-%d.test", rand.Int63())
	contactKey := fmt.Sprintf("tfacc-%d", rand.Int63())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccDomainResourceConfigTransferLock(domainName, contactKey, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_domain.test", "name", domainName),
					resource.TestCheckResourceAttr("opusdns_domain.test", "transfer_lock", "false"),
				),
			},
			{
				Config: testAccDomainResourceConfigTransferLock(domainName, contactKey, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_domain.test", "name", domainName),
					resource.TestCheckResourceAttr("opusdns_domain.test", "transfer_lock", "true"),
				),
			},
		},
	})
}

func testAccDomainResourceConfigBasic(domainName, contactKey string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_contact" "test" {
  first_name  = "Terraform"
  last_name   = "Acceptance"
  org         = "OpusDNS"
  email       = "%s@example.test"
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
`, testAccProviderConfig, contactKey, domainName)
}

func testAccDomainResourceConfigTransferLock(domainName, contactKey string, transferLock bool) string {
	return fmt.Sprintf(`
%s

resource "opusdns_contact" "test" {
  first_name  = "Terraform"
  last_name   = "Acceptance"
  org         = "OpusDNS"
  email       = "%s@example.test"
  phone       = "+1.2125551234"
  street      = "123 Terraform Street"
  city        = "Exampleville"
  state       = "NY"
  postal_code = "10001"
  country     = "US"
}

resource "opusdns_domain" "test" {
  name          = %q
  period_value  = 1
  period_unit   = "y"
  create_zone   = true
  transfer_lock = %t

  contacts = {
    registrant = [opusdns_contact.test.contact_id]
    admin      = [opusdns_contact.test.contact_id]
    tech       = [opusdns_contact.test.contact_id]
  }
}
`, testAccProviderConfig, contactKey, domainName, transferLock)
}
