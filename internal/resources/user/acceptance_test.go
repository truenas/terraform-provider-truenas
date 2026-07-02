package user_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccUser_basic tests create, update, and import of a local user.
// It requires:
//   - TRUENAS_API_KEY set in the environment (checked by acctest.PreCheck)
//   - TF_ACC=1
func TestAccUser_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccUserConfig("tf-acc-testuser", "Test User", false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_user.test", "username", "tf-acc-testuser"),
					resource.TestCheckResourceAttr("truenas_user.test", "full_name", "Test User"),
					resource.TestCheckResourceAttr("truenas_user.test", "locked", "false"),
					resource.TestCheckResourceAttrSet("truenas_user.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_user.test", "uid"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccUserConfig("tf-acc-testuser", "Updated Name", true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_user.test", "full_name", "Updated Name"),
					resource.TestCheckResourceAttr("truenas_user.test", "locked", "true"),
				),
			},
			{
				ResourceName:            "truenas_user.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password"},
			},
		},
	})
}

func testAccUserConfig(username, fullName string, locked bool) string {
	return fmt.Sprintf(`
resource "truenas_user" "test" {
  username          = %q
  full_name         = %q
  locked            = %v
  password_disabled = true
  smb               = false
}
`, username, fullName, locked)
}
