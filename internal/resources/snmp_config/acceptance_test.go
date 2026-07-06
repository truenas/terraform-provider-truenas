package snmp_config_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccSNMPConfigDataSource_basic reads the current TrueNAS SNMP
// configuration through the truenas_snmp_config datasource only. It never
// writes: SNMP is a system-critical monitoring singleton, and the box's
// real SNMP configuration must not be overwritten by an acceptance test run.
//
// Requires:
//   - TF_ACC=1
//   - TRUENAS_API_KEY (or TRUENAS_USERNAME+TRUENAS_PASSWORD) set for
//     acctest.PreCheck
//   - A running TrueNAS SCALE instance
func TestAccSNMPConfigDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_snmp_config" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_snmp_config.test", "id", "snmp_config"),
					resource.TestCheckResourceAttrSet("data.truenas_snmp_config.test", "community"),
				),
			},
		},
	})
}

// TestAccSNMPConfig_basic is intentionally skipped by default.
// truenas_snmp_config is a SINGLETON resource that manages system-critical
// monitoring configuration: bringing it under Terraform management mutates
// the box's actual SNMP configuration (snmp.update), and a naive
// create/update/destroy acceptance test risks disrupting monitoring on
// whatever system runs it.
//
// If this test is ever enabled against a disposable/non-production TrueNAS
// instance, it should:
//  1. Read the current config via the datasource first.
//  2. Only touch "location" or "contact" (low-risk, additive string fields)
//     in the resource config, driving them through a value and back to the
//     value the datasource observed in step 1, so the net effect on the box
//     is a no-op.
//  3. Use ImportStateVerifyIgnore: []string{"v3_password", "v3_privpassphrase"}
//     on the import step, since snmp.config never returns usable values for
//     either.
func TestAccSNMPConfig_basic(t *testing.T) {
	t.Skip("truenas_snmp_config manages system-critical monitoring configuration; skipped to avoid mutating the target box's SNMP configuration. See comment on TestAccSNMPConfig_basic for how to safely enable this against a disposable instance.")
}
