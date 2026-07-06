package ups_config_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccUPSConfigDataSource_basic reads the current TrueNAS UPS
// configuration through the truenas_ups_config datasource only. It never
// writes: UPS is a system-critical power management singleton, and the
// box's real UPS configuration must not be overwritten by an acceptance
// test run.
//
// Requires:
//   - TF_ACC=1
//   - TRUENAS_API_KEY (or TRUENAS_USERNAME+TRUENAS_PASSWORD) set for
//     acctest.PreCheck
//   - A running TrueNAS SCALE instance
func TestAccUPSConfigDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_ups_config" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_ups_config.test", "id", "ups_config"),
					resource.TestCheckResourceAttrSet("data.truenas_ups_config.test", "mode"),
				),
			},
		},
	})
}

// TestAccUPSConfig_basic is intentionally skipped by default.
// truenas_ups_config is a SINGLETON resource that manages system-critical
// power management configuration: bringing it under Terraform management
// mutates the box's actual UPS configuration (ups.update), and a naive
// create/update/destroy acceptance test risks disrupting UPS monitoring on
// whatever system runs it.
//
// If this test is ever enabled against a disposable/non-production TrueNAS
// instance, it should:
//  1. Read the current config via the datasource first.
//  2. Only touch "description" (low-risk, additive string field) in the
//     resource config, driving it through a value and back to the value the
//     datasource observed in step 1, so the net effect on the box is a
//     no-op.
//  3. Use ImportStateVerifyIgnore: []string{"monpwd"} on the import step,
//     since ups.config never returns a usable value for it.
func TestAccUPSConfig_basic(t *testing.T) {
	t.Skip("truenas_ups_config manages system-critical power management configuration; skipped to avoid mutating the target box's UPS configuration. See comment on TestAccUPSConfig_basic for how to safely enable this against a disposable instance.")
}
