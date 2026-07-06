package tunable_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccTunable_basic tests create, update, and import of a sysctl tunable.
// It requires:
//   - TRUENAS_API_KEY set in the environment (checked by acctest.PreCheck)
//   - TF_ACC=1
func TestAccTunable_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccTunableConfig("kernel.threads-max", "100000", "tf-acc test tunable"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_tunable.test", "var", "kernel.threads-max"),
					resource.TestCheckResourceAttr("truenas_tunable.test", "value", "100000"),
					resource.TestCheckResourceAttr("truenas_tunable.test", "comment", "tf-acc test tunable"),
					resource.TestCheckResourceAttrSet("truenas_tunable.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_tunable.test", "type"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccTunableConfig("kernel.threads-max", "200000", "tf-acc test tunable updated"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_tunable.test", "value", "200000"),
					resource.TestCheckResourceAttr("truenas_tunable.test", "comment", "tf-acc test tunable updated"),
				),
			},
			{
				ResourceName:            "truenas_tunable.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"update_initramfs"},
			},
		},
	})
}

func testAccTunableConfig(varName, value, comment string) string {
	return fmt.Sprintf(`
resource "truenas_tunable" "test" {
  var     = %q
  value   = %q
  comment = %q
}
`, varName, value, comment)
}
