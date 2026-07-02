package iscsi_extent_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccISCSIExtent_basic tests create, update, and import of an iSCSI extent.
// It requires:
//   - TrueNAS_API_KEY set in the environment (checked by acctest.PreCheck)
//   - A zvol at zvol/tank/iscsi-test-vol to exist on the target TrueNAS host
func TestAccISCSIExtent_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure zvol/tank/iscsi-test-vol exists")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccISCSIExtentConfig("tf-test-extent", false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_extent.test", "name", "tf-test-extent"),
					resource.TestCheckResourceAttr("truenas_iscsi_extent.test", "type", "DISK"),
					resource.TestCheckResourceAttr("truenas_iscsi_extent.test", "enabled", "true"),
					resource.TestCheckResourceAttrSet("truenas_iscsi_extent.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_iscsi_extent.test", "naa"),
					resource.TestCheckResourceAttrSet("truenas_iscsi_extent.test", "serial"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccISCSIExtentConfig("tf-test-extent", true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_extent.test", "ro", "true"),
				),
			},
			{
				ResourceName:      "truenas_iscsi_extent.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccISCSIExtentConfig(name string, ro bool) string {
	return fmt.Sprintf(`
resource "truenas_iscsi_extent" "test" {
  name    = %q
  type    = "DISK"
  disk    = "zvol/tank/iscsi-test-vol"
  enabled = true
  ro      = %v
}
`, name, ro)
}
