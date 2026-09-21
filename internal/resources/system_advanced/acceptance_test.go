// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package system_advanced_test

import (
	"context"
	"encoding/json"
	"fmt"
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
//   - A running TrueNAS instance
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

// systemAdvancedOriginal captures the one field
// TestAccSystemAdvanced_setAndRestore touches.
type systemAdvancedOriginal struct {
	Motd string `json:"motd"`
}

// readSystemAdvancedOriginal reads the box's current motd via
// acctest.RestoreCall (system.advanced.update is job:false, probed live),
// so the test can restore it exactly afterward.
func readSystemAdvancedOriginal(t *testing.T) systemAdvancedOriginal {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "system.advanced.config")
	if err != nil {
		t.Fatalf("error reading current system advanced config: %v", err)
	}
	var orig systemAdvancedOriginal
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing system.advanced.config response: %v", err)
	}
	return orig
}

// restoreSystemAdvanced sends only motd back to its original value via
// system.advanced.update, through acctest.RestoreCall so a t.Cleanup-time
// stale connection on the long-lived acctest.Client() (see RestoreCall's
// doc comment) is retried and reconnected rather than failing the whole
// test with "not connected". It runs from t.Cleanup, so it restores the box
// even if the Terraform steps themselves fail partway through.
// syslogservers, serialconsole, serialport, serialspeed, consolemenu,
// sed_user, and sed_passwd are never touched by this test.
func restoreSystemAdvanced(t *testing.T, orig systemAdvancedOriginal) {
	t.Helper()
	if _, err := acctest.RestoreCall(context.Background(), "system.advanced.update", map[string]any{
		"motd": orig.Motd,
	}); err != nil {
		t.Fatalf("error restoring system advanced motd: %v", err)
	}
}

// TestAccSystemAdvanced_setAndRestore drives the singleton
// truenas_system_advanced resource's "motd" field (a cosmetic
// message-of-the-day string) through a test value and back to the value
// read from the box before the test ran, then imports it. It requires
// TF_ACC=1 and TRUENAS_DISRUPTIVE=1 (acctest.DisruptiveCheck), since it
// mutates the box's live system advanced configuration; a
// t.Cleanup-registered API restore is the safety net if the Terraform steps
// fail. syslogservers, serialconsole, serialport, serialspeed, consolemenu,
// sed_user, and sed_passwd are never touched: changing those on a live
// system can disrupt console/serial access or SED unlock behavior.
func TestAccSystemAdvanced_setAndRestore(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readSystemAdvancedOriginal(t)
	t.Cleanup(func() { restoreSystemAdvanced(t, orig) })

	testValue := acctest.RandName("tf-acc-motd")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccSystemAdvancedConfig(testValue),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_system_advanced.test", "id", "system_advanced"),
					resource.TestCheckResourceAttr("truenas_system_advanced.test", "motd", testValue),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccSystemAdvancedConfig(orig.Motd),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_system_advanced.test", "motd", orig.Motd),
				),
			},
			{
				ResourceName:            "truenas_system_advanced.test",
				ImportState:             true,
				ImportStateId:           "system_advanced",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"sed_passwd"},
			},
		},
	})
}

func testAccSystemAdvancedConfig(motd string) string {
	return fmt.Sprintf(`
resource "truenas_system_advanced" "test" {
  motd = %q
}
`, motd)
}
