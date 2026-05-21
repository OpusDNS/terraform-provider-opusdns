package provider

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccContactResource_basic(t *testing.T) {
	email := fmt.Sprintf("contact-%d@example.com", rand.Int63())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccContactResourceConfigBasic(email),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("opusdns_contact.test", "id", "opusdns_contact.test", "contact_id"),
					resource.TestCheckResourceAttrSet("opusdns_contact.test", "contact_id"),
					resource.TestCheckResourceAttr("opusdns_contact.test", "first_name", "Jane"),
					resource.TestCheckResourceAttr("opusdns_contact.test", "last_name", "Doe"),
					resource.TestCheckResourceAttr("opusdns_contact.test", "email", email),
					resource.TestCheckResourceAttrSet("opusdns_contact.test", "phone"),
					resource.TestCheckResourceAttr("opusdns_contact.test", "street", "123 Main St"),
					resource.TestCheckResourceAttr("opusdns_contact.test", "city", "New York"),
					resource.TestCheckResourceAttr("opusdns_contact.test", "postal_code", "10001"),
					resource.TestCheckResourceAttr("opusdns_contact.test", "country", "US"),
					resource.TestCheckResourceAttr("opusdns_contact.test", "disclose", "false"),
				),
			},
			{
				ResourceName:      "opusdns_contact.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccContactResource_full(t *testing.T) {
	email := fmt.Sprintf("contact-full-%d@example.com", rand.Int63())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccContactResourceConfigFull(email),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair("opusdns_contact.test", "id", "opusdns_contact.test", "contact_id"),
					resource.TestCheckResourceAttrSet("opusdns_contact.test", "contact_id"),
					resource.TestCheckResourceAttr("opusdns_contact.test", "first_name", "John"),
					resource.TestCheckResourceAttr("opusdns_contact.test", "last_name", "Smith"),
					resource.TestCheckResourceAttr("opusdns_contact.test", "org", "Acme Inc"),
					resource.TestCheckResourceAttr("opusdns_contact.test", "title", "Dr."),
					resource.TestCheckResourceAttr("opusdns_contact.test", "email", email),
					resource.TestCheckResourceAttrSet("opusdns_contact.test", "phone"),
					resource.TestCheckResourceAttrSet("opusdns_contact.test", "fax"),
					resource.TestCheckResourceAttr("opusdns_contact.test", "street", "456 Market St"),
					resource.TestCheckResourceAttr("opusdns_contact.test", "city", "San Francisco"),
					resource.TestCheckResourceAttr("opusdns_contact.test", "state", "CA"),
					resource.TestCheckResourceAttr("opusdns_contact.test", "postal_code", "94105"),
					resource.TestCheckResourceAttr("opusdns_contact.test", "country", "US"),
					resource.TestCheckResourceAttr("opusdns_contact.test", "disclose", "true"),
				),
			},
		},
	})
}

func TestAccContactResource_update(t *testing.T) {
	email1 := fmt.Sprintf("contact-update-%d@example.com", rand.Int63())
	email2 := fmt.Sprintf("contact-update-%d@example.com", rand.Int63())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccContactResourceConfigUpdate("Jane", "Doe", email1),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_contact.test", "first_name", "Jane"),
					resource.TestCheckResourceAttr("opusdns_contact.test", "last_name", "Doe"),
					resource.TestCheckResourceAttr("opusdns_contact.test", "email", email1),
				),
			},
			{
				Config: testAccContactResourceConfigUpdate("Janet", "Roe", email2),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_contact.test", "first_name", "Janet"),
					resource.TestCheckResourceAttr("opusdns_contact.test", "last_name", "Roe"),
					resource.TestCheckResourceAttr("opusdns_contact.test", "email", email2),
				),
			},
		},
	})
}

func testAccContactResourceConfigBasic(email string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_contact" "test" {
  first_name  = "Jane"
  last_name   = "Doe"
  email       = %q
  phone       = "+12125550100"
  street      = "123 Main St"
  city        = "New York"
  postal_code = "10001"
  country     = "US"
}
`, testAccProviderConfig, email)
}

func testAccContactResourceConfigFull(email string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_contact" "test" {
  first_name  = "John"
  last_name   = "Smith"
  org         = "Acme Inc"
  title       = "Dr."
  email       = %q
  phone       = "+14155550100"
  fax         = "+14155550101"
  street      = "456 Market St"
  city        = "San Francisco"
  state       = "CA"
  postal_code = "94105"
  country     = "US"
  disclose    = true
}
`, testAccProviderConfig, email)
}

func testAccContactResourceConfigUpdate(firstName, lastName, email string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_contact" "test" {
  first_name  = %q
  last_name   = %q
  email       = %q
  phone       = "+12125550199"
  street      = "789 Broadway"
  city        = "New York"
  postal_code = "10003"
  country     = "US"
}
`, testAccProviderConfig, firstName, lastName, email)
}
