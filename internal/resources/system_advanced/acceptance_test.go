package system_advanced_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccSystemAdvancedDataSource_basic reads the current TrueNAS system
// advanced configuration through the truenas_system_advanced datasource
// only. It never writes: this configuration controls syslog forwarding and
// serial/console behavior, and the box's real configuration must not be
// overwritten by an acceptance test run.
//
// Requires:
//   - TF_ACC=1
//   - TRUENAS_API_KEY (or TRUENAS_USERNAME+TRUENAS_PASSWORD) set for
//     acctest.PreCheck
//   - A running TrueNAS SCALE instance
func TestAccSystemAdvancedDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_system_advanced" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_system_advanced.test", "id", "system_advanced"),
					resource.TestCheckResourceAttrSet("data.truenas_system_advanced.test", "sysloglevel"),
				),
			},
		},
	})
}

// TestAccSystemAdvanced_basic is intentionally skipped by default.
// truenas_system_advanced is a SINGLETON resource that controls live syslog
// and console configuration: the box this suite runs against is managed
// live, and bringing this configuration under Terraform management mutates
// the box's actual system advanced configuration (system.advanced.update).
// A naive create/update/destroy acceptance test risks changing
// syslogservers, serialconsole, consolemenu, or sed_user on a live system.
//
// If this test is ever enabled against a disposable/non-production TrueNAS
// instance, it should:
//  1. Read the current config via the datasource first.
//  2. Only touch a low-risk field (e.g. "motd") in the resource config,
//     driving it through a value and back to the value the datasource
//     observed in step 1, so the net effect on the box is a no-op. Never
//     touch syslogservers, serialconsole, serialport, serialspeed,
//     consolemenu, sed_user, or sed_passwd in an automated test: changing
//     those on a live system can disrupt console/serial access or SED
//     unlock behavior.
//  3. Use ImportState with ImportStateId "system_advanced" to verify
//     import normalizes any ID to the fixed singleton ID.
func TestAccSystemAdvanced_basic(t *testing.T) {
	t.Skip("truenas_system_advanced controls LIVE syslog/console configuration; skipped to avoid disrupting the target box. See comment on TestAccSystemAdvanced_basic for how to safely enable this against a disposable instance.")
}
