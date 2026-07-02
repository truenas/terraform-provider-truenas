package service_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccService_basic tests enable/disable and start/stop of the NFS service.
// It requires TF_ACC=1 and credentials set via TRUENAS_API_KEY or
// TRUENAS_USERNAME+TRUENAS_PASSWORD.
func TestAccService_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccServiceConfig("nfs", false, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_service.test", "name", "nfs"),
					resource.TestCheckResourceAttr("truenas_service.test", "enabled", "false"),
					resource.TestCheckResourceAttr("truenas_service.test", "running", "false"),
					resource.TestCheckResourceAttr("truenas_service.test", "id", "nfs"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccServiceConfig("nfs", true, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_service.test", "enabled", "true"),
					resource.TestCheckResourceAttr("truenas_service.test", "running", "true"),
				),
			},
			{
				ResourceName:      "truenas_service.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccServiceConfig(name string, enabled, running bool) string {
	return fmt.Sprintf(`
resource "truenas_service" "test" {
  name    = %q
  enabled = %v
  running = %v
}
`, name, enabled, running)
}
