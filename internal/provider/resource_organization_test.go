package provider

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccOrganizationResource_basic(t *testing.T) {
	name := fmt.Sprintf("tfacc-org-%d", rand.Int63())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOrganizationResourceConfig(name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("opusdns_organization.test", "id"),
					resource.TestCheckResourceAttrSet("opusdns_organization.test", "organization_id"),
					resource.TestCheckResourceAttr("opusdns_organization.test", "name", name),
					resource.TestCheckResourceAttrSet("opusdns_organization.test", "parent_organization_id"),
					resource.TestCheckResourceAttrSet("opusdns_organization.test", "status"),
					resource.TestCheckResourceAttrSet("opusdns_organization.test", "created_on"),
				),
			},
		},
	})
}

func TestAccOrganizationResource_update(t *testing.T) {
	initialName := fmt.Sprintf("tfacc-org-%d", rand.Int63())
	updatedName := fmt.Sprintf("tfacc-org-updated-%d", rand.Int63())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOrganizationResourceConfigWithAddress(initialName, "123 Main St", "", "New York", "NY", "10001", "US"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_organization.test", "name", initialName),
					resource.TestCheckResourceAttr("opusdns_organization.test", "address_1", "123 Main St"),
					resource.TestCheckResourceAttr("opusdns_organization.test", "city", "New York"),
					resource.TestCheckResourceAttr("opusdns_organization.test", "state", "NY"),
					resource.TestCheckResourceAttr("opusdns_organization.test", "postal_code", "10001"),
					resource.TestCheckResourceAttr("opusdns_organization.test", "country_code", "US"),
				),
			},
			{
				Config: testAccOrganizationResourceConfigWithAddress(updatedName, "456 Market St", "Suite 100", "San Francisco", "CA", "94105", "US"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_organization.test", "name", updatedName),
					resource.TestCheckResourceAttr("opusdns_organization.test", "address_1", "456 Market St"),
					resource.TestCheckResourceAttr("opusdns_organization.test", "address_2", "Suite 100"),
					resource.TestCheckResourceAttr("opusdns_organization.test", "city", "San Francisco"),
					resource.TestCheckResourceAttr("opusdns_organization.test", "state", "CA"),
					resource.TestCheckResourceAttr("opusdns_organization.test", "postal_code", "94105"),
					resource.TestCheckResourceAttr("opusdns_organization.test", "country_code", "US"),
				),
			},
		},
	})
}

func testAccOrganizationResourceConfig(name string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_organization" "test" {
  name = %q
}
`, testAccProviderConfig, name)
}

func testAccOrganizationResourceConfigWithAddress(name, address1, address2, city, state, postalCode, countryCode string) string {
	address2Line := ""
	if address2 != "" {
		address2Line = fmt.Sprintf("  address_2    = %q\n", address2)
	}

	return fmt.Sprintf(`
%s

resource "opusdns_organization" "test" {
  name         = %q
  address_1    = %q
%s  city         = %q
  state        = %q
  postal_code  = %q
  country_code = %q
}
`, testAccProviderConfig, name, address1, address2Line, city, state, postalCode, countryCode)
}
