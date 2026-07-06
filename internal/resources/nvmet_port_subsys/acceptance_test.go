package nvmet_port_subsys_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccNVMetPortSubsys_basic tests create and import of an NVMe-oF
// port/subsystem association. It creates and destroys its own port and
// subsystem ("tf-test-port-subsys"). It never touches the box's existing
// LIVE port_subsys association (id=1), which serves live storage.
func TestAccNVMetPortSubsys_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccNVMetPortSubsysConfig(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_nvmet_port_subsys.test", "id"),
					resource.TestCheckResourceAttrPair("truenas_nvmet_port_subsys.test", "port_id", "truenas_nvmet_port.test", "id"),
					resource.TestCheckResourceAttrPair("truenas_nvmet_port_subsys.test", "subsys_id", "truenas_nvmet_subsys.test", "id"),
				),
			},
			{
				ResourceName:      "truenas_nvmet_port_subsys.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccNVMetPortSubsysConfig() string {
	return `
resource "truenas_nvmet_port" "test" {
  addr_trtype  = "TCP"
  addr_traddr  = "127.0.0.1"
  addr_trsvcid = 4423
}

resource "truenas_nvmet_subsys" "test" {
  name = "tf-test-port-subsys"
}

resource "truenas_nvmet_port_subsys" "test" {
  port_id   = truenas_nvmet_port.test.id
  subsys_id = truenas_nvmet_subsys.test.id
}
`
}
