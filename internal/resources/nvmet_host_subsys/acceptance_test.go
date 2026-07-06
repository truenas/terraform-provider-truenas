package nvmet_host_subsys_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccNVMetHostSubsys_basic tests create and import of an NVMe-oF
// host/subsystem association. It creates and destroys its own host
// ("nqn.2014-08.org.nvmexpress:uuid:tf-test-host-subsys") and subsystem
// ("tf-test-host-subsys"). It never touches the box's existing LIVE
// host_subsys association (id=1), which serves live storage.
func TestAccNVMetHostSubsys_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccNVMetHostSubsysConfig(),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_nvmet_host_subsys.test", "id"),
					resource.TestCheckResourceAttrPair("truenas_nvmet_host_subsys.test", "host_id", "truenas_nvmet_host.test", "id"),
					resource.TestCheckResourceAttrPair("truenas_nvmet_host_subsys.test", "subsys_id", "truenas_nvmet_subsys.test", "id"),
				),
			},
			{
				ResourceName:      "truenas_nvmet_host_subsys.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccNVMetHostSubsysConfig() string {
	return `
resource "truenas_nvmet_host" "test" {
  hostnqn = "nqn.2014-08.org.nvmexpress:uuid:tf-test-host-subsys"
}

resource "truenas_nvmet_subsys" "test" {
  name = "tf-test-host-subsys"
}

resource "truenas_nvmet_host_subsys" "test" {
  host_id   = truenas_nvmet_host.test.id
  subsys_id = truenas_nvmet_subsys.test.id
}
`
}
