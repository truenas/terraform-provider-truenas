package nvmet_namespace_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccNVMetNamespace_basic tests create, update, and import of an
// NVMe-oF namespace. It creates and destroys its own subsystem
// ("tf-test-ns-subsys") and namespace, and requires a pre-existing zvol at
// zvol/tank/nvmet-test-vol on the target TrueNAS host. It never touches the
// box's existing LIVE subsystem/namespace (id=1), which serves live
// storage.
func TestAccNVMetNamespace_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure zvol/tank/nvmet-test-vol exists")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccNVMetNamespaceConfig(false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nvmet_namespace.test", "device_path", "zvol/tank/nvmet-test-vol"),
					resource.TestCheckResourceAttr("truenas_nvmet_namespace.test", "device_type", "ZVOL"),
					resource.TestCheckResourceAttr("truenas_nvmet_namespace.test", "enabled", "false"),
					resource.TestCheckResourceAttrSet("truenas_nvmet_namespace.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_nvmet_namespace.test", "nsid"),
					resource.TestCheckResourceAttrSet("truenas_nvmet_namespace.test", "device_nguid"),
					resource.TestCheckResourceAttrSet("truenas_nvmet_namespace.test", "device_uuid"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccNVMetNamespaceConfig(true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nvmet_namespace.test", "enabled", "true"),
				),
			},
			{
				ResourceName:      "truenas_nvmet_namespace.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccNVMetNamespaceConfig(enabled bool) string {
	return fmt.Sprintf(`
resource "truenas_nvmet_subsys" "test" {
  name = "tf-test-ns-subsys"
}

resource "truenas_nvmet_namespace" "test" {
  subsys_id   = truenas_nvmet_subsys.test.id
  device_path = "zvol/tank/nvmet-test-vol"
  device_type = "ZVOL"
  enabled     = %v
}
`, enabled)
}
