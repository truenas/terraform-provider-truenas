package nvmet_subsys_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccNVMetSubsys_basic tests create, update, and import of an NVMe-oF
// subsystem. It creates and destroys its own subsystem ("tf-test-subsys")
// and never touches the box's existing LIVE subsystem (id=1,
// name="proxmox-test"), which serves live storage.
func TestAccNVMetSubsys_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccNVMetSubsysConfig("tf-test-subsys", false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nvmet_subsys.test", "name", "tf-test-subsys"),
					resource.TestCheckResourceAttr("truenas_nvmet_subsys.test", "allow_any_host", "false"),
					resource.TestCheckResourceAttrSet("truenas_nvmet_subsys.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_nvmet_subsys.test", "subnqn"),
					resource.TestCheckResourceAttrSet("truenas_nvmet_subsys.test", "serial"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccNVMetSubsysConfig("tf-test-subsys", true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nvmet_subsys.test", "allow_any_host", "true"),
				),
			},
			{
				ResourceName:      "truenas_nvmet_subsys.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccNVMetSubsysConfig(name string, allowAnyHost bool) string {
	return fmt.Sprintf(`
resource "truenas_nvmet_subsys" "test" {
  name           = %q
  allow_any_host = %v
}
`, name, allowAnyHost)
}
