package provider

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTagResource_basic(t *testing.T) {
	label := fmt.Sprintf("tfacc-tag-%d", rand.Int63())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTagResourceConfig(label, "DOMAIN", "color-1", "Created by Terraform acceptance tests"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("opusdns_tag.test", "id"),
					resource.TestCheckResourceAttrSet("opusdns_tag.test", "tag_id"),
					resource.TestCheckResourceAttr("opusdns_tag.test", "label", label),
					resource.TestCheckResourceAttr("opusdns_tag.test", "type", "DOMAIN"),
					resource.TestCheckResourceAttr("opusdns_tag.test", "color", "color-1"),
					resource.TestCheckResourceAttr("opusdns_tag.test", "description", "Created by Terraform acceptance tests"),
				),
			},
			{
				ResourceName:      "opusdns_tag.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccTagResource_update(t *testing.T) {
	initialLabel := fmt.Sprintf("tfacc-tag-%d", rand.Int63())
	updatedLabel := fmt.Sprintf("tfacc-tag-updated-%d", rand.Int63())

	resource.ParallelTest(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccTagResourceConfig(initialLabel, "DOMAIN", "color-1", "Initial Terraform acceptance test tag"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_tag.test", "label", initialLabel),
					resource.TestCheckResourceAttr("opusdns_tag.test", "color", "color-1"),
				),
			},
			{
				Config: testAccTagResourceConfig(updatedLabel, "DOMAIN", "color-2", "Initial Terraform acceptance test tag"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("opusdns_tag.test", "label", updatedLabel),
					resource.TestCheckResourceAttr("opusdns_tag.test", "color", "color-2"),
				),
			},
		},
	})
}

func testAccTagResourceConfig(label, tagType, color, description string) string {
	return fmt.Sprintf(`
%s

resource "opusdns_tag" "test" {
  label       = %q
  type        = %q
  color       = %q
  description = %q
}
`, testAccProviderConfig, label, tagType, color, description)
}
