package group_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccGroup_basic tests create, update, and import of a local group.
// It requires:
//   - TRUENAS_API_KEY set in the environment (checked by acctest.PreCheck)
//   - TF_ACC=1
func TestAccGroup_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccGroupConfig("tf-acc-testgroup", false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_group.test", "name", "tf-acc-testgroup"),
					resource.TestCheckResourceAttr("truenas_group.test", "smb", "false"),
					resource.TestCheckResourceAttrSet("truenas_group.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_group.test", "gid"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccGroupConfig("tf-acc-testgroup", true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_group.test", "smb", "true"),
				),
			},
			{
				ResourceName:      "truenas_group.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccGroupConfig(name string, smb bool) string {
	return fmt.Sprintf(`
resource "truenas_group" "test" {
  name = %q
  smb  = %v
}
`, name, smb)
}
