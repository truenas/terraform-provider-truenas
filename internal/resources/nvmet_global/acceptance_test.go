package nvmet_global_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccNVMeTGlobalDataSource_basic reads the current TrueNAS NVMe-oF global
// configuration through the truenas_nvmet_global datasource only. It never
// writes: the NVMe-oF global configuration serves live storage, and the
// box's real configuration must not be overwritten by an acceptance test
// run.
//
// Requires:
//   - TF_ACC=1
//   - TRUENAS_API_KEY (or TRUENAS_USERNAME+TRUENAS_PASSWORD) set for
//     acctest.PreCheck
//   - A running TrueNAS SCALE instance
func TestAccNVMeTGlobalDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_nvmet_global" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_nvmet_global.test", "id", "nvmet_global"),
					resource.TestCheckResourceAttrSet("data.truenas_nvmet_global.test", "basenqn"),
				),
			},
		},
	})
}

// TestAccNVMeTGlobal_basic is intentionally skipped by default.
// truenas_nvmet_global is a SINGLETON resource that serves live storage: the
// box this suite runs against serves LIVE NVMe-oF storage, and bringing this
// configuration under Terraform management mutates the box's actual
// NVMe-oF global configuration (nvmet.global.update). A naive
// create/update/destroy acceptance test risks disrupting active NVMe-oF
// connections or discovery on that system.
//
// If this test is ever enabled against a disposable/non-production TrueNAS
// instance, it should:
//  1. Read the current config via the datasource first.
//  2. Only touch a low-risk boolean toggle (e.g. "xport_referral") in the
//     resource config, driving it through a value and back to the value the
//     datasource observed in step 1, so the net effect on the box is a
//     no-op. Never touch basenqn, ana, kernel, or rdma in an automated test:
//     changing those on a live system can disrupt active NVMe-oF connections
//     or discovery.
//  3. Use ImportState with ImportStateId "nvmet_global" to verify import
//     normalizes any ID to the fixed singleton ID.
func TestAccNVMeTGlobal_basic(t *testing.T) {
	t.Skip("truenas_nvmet_global serves LIVE NVMe-oF storage; skipped to avoid mutating the target box's NVMe-oF configuration. See comment on TestAccNVMeTGlobal_basic for how to safely enable this against a disposable instance.")
}
