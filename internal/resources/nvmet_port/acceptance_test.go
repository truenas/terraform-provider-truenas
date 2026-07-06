package nvmet_port_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccNVMetPort_basic tests create, update, and import of an NVMe-oF
// port. It creates and destroys its own port on trsvcid 14420 and never
// touches the box's existing LIVE port (id=1, TCP 4420 on 192.168.1.68),
// which serves live storage.
func TestAccNVMetPort_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccNVMetPortConfig(14420, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nvmet_port.test", "addr_trtype", "TCP"),
					resource.TestCheckResourceAttr("truenas_nvmet_port.test", "addr_traddr", "192.168.1.68"),
					resource.TestCheckResourceAttr("truenas_nvmet_port.test", "addr_trsvcid", "14420"),
					resource.TestCheckResourceAttr("truenas_nvmet_port.test", "enabled", "false"),
					resource.TestCheckResourceAttrSet("truenas_nvmet_port.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_nvmet_port.test", "index"),
					resource.TestCheckResourceAttrSet("truenas_nvmet_port.test", "addr_adrfam"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccNVMetPortConfig(14420, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nvmet_port.test", "enabled", "true"),
				),
			},
			{
				ResourceName:      "truenas_nvmet_port.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccNVMetPortConfig(trsvcid int, enabled bool) string {
	return fmt.Sprintf(`
resource "truenas_nvmet_port" "test" {
  addr_trtype  = "TCP"
  addr_traddr  = "192.168.1.68"
  addr_trsvcid = %d
  enabled      = %v
}
`, trsvcid, enabled)
}
