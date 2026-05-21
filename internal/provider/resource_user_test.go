package provider

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccUserResource_basic(t *testing.T) {
	username := fmt.Sprintf("tfaccuser%d", rand.Int63())
	email := fmt.Sprintf("tfacc-%d@test.opusdns.dev", rand.Int63())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccUserResourceConfig(username, email, "Terraform", "Acceptance", "en-US"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("opusdns_user.test", "id"),
					resource.TestCheckResourceAttrSet("opusdns_user.test", "user_id"),
					resource.TestCheckResourceAttr("opusdns_user.test", "username", username),
					resource.TestCheckResourceAttr("opusdns_user.test", "email", email),
					resource.TestCheckResourceAttr("opusdns_user.test", "first_name", "Terraform"),
					resource.TestCheckResourceAttr("opusdns_user.test", "last_name", "Acceptance"),
					resource.TestCheckResourceAttr("opusdns_user.test", "locale", "en-US"),
					resource.TestCheckResourceAttrSet("opusdns_user.test", "organization_id"),
					resource.TestCheckResourceAttrSet("opusdns_user.test", "status"),
				),
			},
		},
	})
}

func testAccUserResourceConfig(username, email, firstName, lastName, locale string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_user" "test" {
  username   = %q
  email      = %q
  first_name = %q
  last_name  = %q
  locale     = %q
}
`, testAccProviderConfig, username, email, firstName, lastName, locale)
}
