package provider

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccParkingResource_basic(t *testing.T) {
	domainName := testAccDomainName()
	contactKey := fmt.Sprintf("tfacc-%d", rand.Int63())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccParkingResourceConfigBasic(domainName, contactKey),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("opusdns_parking.test", "id"),
					resource.TestCheckResourceAttrSet("opusdns_parking.test", "parking_id"),
					resource.TestCheckResourceAttr("opusdns_parking.test", "domain", domainName),
					resource.TestCheckResourceAttr("opusdns_parking.test", "enabled", "false"),
					resource.TestCheckResourceAttrSet("opusdns_parking.test", "created_on"),
					resource.TestCheckResourceAttrSet("opusdns_parking.test", "updated_on"),
				),
			},
		},
	})
}

func testAccParkingResourceConfigBasic(domainName, contactKey string) string {
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

resource "opusdns_parking" "test" {
  domain  = opusdns_domain.test.name
  enabled = false
}
`, testAccProviderConfig, contactKey, domainName)
}
