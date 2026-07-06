package iscsi_auth_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccISCSIAuth_basic tests create, update, and import of an iSCSI CHAP
// auth entry.
// It requires:
//   - TrueNAS_API_KEY set in the environment (checked by acctest.PreCheck)
//   - A running TrueNAS SCALE instance
func TestAccISCSIAuth_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccISCSIAuthConfig(1, "tf-acc-chapuser", "tf-acc-chapsecret1"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_auth.test", "user", "tf-acc-chapuser"),
					resource.TestCheckResourceAttr("truenas_iscsi_auth.test", "tag", "1"),
					resource.TestCheckResourceAttrSet("truenas_iscsi_auth.test", "id"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccISCSIAuthConfig(1, "tf-acc-chapuser-updated", "tf-acc-chapsecret2"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_auth.test", "user", "tf-acc-chapuser-updated"),
				),
			},
			{
				ResourceName:            "truenas_iscsi_auth.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"secret", "peersecret"},
			},
		},
	})
}

func testAccISCSIAuthConfig(tag int, user, secret string) string {
	return fmt.Sprintf(`
resource "truenas_iscsi_auth" "test" {
  tag    = %d
  user   = %q
  secret = %q
}
`, tag, user, secret)
}
