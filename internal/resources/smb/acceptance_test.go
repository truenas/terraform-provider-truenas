package smb_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccSMBShare_basic tests create, update, and import of an SMB share.
// It requires:
//   - TrueNAS_API_KEY set in the environment (checked by acctest.PreCheck)
//   - The path /mnt/tank/smb-test-share to exist on the target TrueNAS host
func TestAccSMBShare_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure /mnt/tank/smb-test-share exists")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccSMBShareConfig("smb-test-share", false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_smb_share.test", "name", "smb-test-share"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "path", "/mnt/tank/smb-test-share"),
					resource.TestCheckResourceAttr("truenas_smb_share.test", "enabled", "true"),
					resource.TestCheckResourceAttrSet("truenas_smb_share.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_smb_share.test", "vuid"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccSMBShareConfig("smb-test-share", true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_smb_share.test", "ro", "true"),
				),
			},
			{
				ResourceName:      "truenas_smb_share.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccSMBShareConfig(name string, ro bool) string {
	return fmt.Sprintf(`
resource "truenas_smb_share" "test" {
  path    = "/mnt/tank/smb-test-share"
  name    = %q
  enabled = true
  ro      = %v
}
`, name, ro)
}
