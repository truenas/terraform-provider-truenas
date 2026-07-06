package nvmet_host_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccNVMetHost_basic tests create, update, and import of an NVMe-oF
// host (initiator). It requires:
//   - TrueNAS_API_KEY set in the environment (checked by acctest.PreCheck)
//   - A running TrueNAS SCALE instance
func TestAccNVMetHost_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccNVMetHostConfig(
					"nqn.2014-08.org.nvmexpress:uuid:tf-acc-host", "tf-acc host", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nvmet_host.test", "hostnqn", "nqn.2014-08.org.nvmexpress:uuid:tf-acc-host"),
					resource.TestCheckResourceAttr("truenas_nvmet_host.test", "description", "tf-acc host"),
					resource.TestCheckResourceAttrSet("truenas_nvmet_host.test", "id"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccNVMetHostConfig(
					"nqn.2014-08.org.nvmexpress:uuid:tf-acc-host", "tf-acc host updated", "tf-acc-dhchap-key-1"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nvmet_host.test", "description", "tf-acc host updated"),
				),
			},
			{
				// hostnqn is mutable in place (no RequiresReplace): verify
				// that changing it updates the existing resource rather than
				// forcing a create/destroy.
				Config: acctest.ProviderConfig() + testAccNVMetHostConfig(
					"nqn.2014-08.org.nvmexpress:uuid:tf-acc-host-renamed", "tf-acc host updated", "tf-acc-dhchap-key-1"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nvmet_host.test", "hostnqn", "nqn.2014-08.org.nvmexpress:uuid:tf-acc-host-renamed"),
				),
			},
			{
				ResourceName:            "truenas_nvmet_host.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"dhchap_key", "dhchap_ctrl_key"},
			},
		},
	})
}

func testAccNVMetHostConfig(hostnqn, description, dhchapKey string) string {
	if dhchapKey == "" {
		return fmt.Sprintf(`
resource "truenas_nvmet_host" "test" {
  hostnqn     = %q
  description = %q
}
`, hostnqn, description)
	}
	return fmt.Sprintf(`
resource "truenas_nvmet_host" "test" {
  hostnqn     = %q
  description = %q
  dhchap_key  = %q
}
`, hostnqn, description, dhchapKey)
}
