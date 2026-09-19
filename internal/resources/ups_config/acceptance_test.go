// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ups_config_test

import (
	"context"
	"encoding/json"
	"fmt"
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
//   - A running TrueNAS instance
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

// upsConfigOriginal captures every writable, non-secret field returned by
// ups.config, not just the cosmetic "description" field the test drives:
// ups.update requires several of these (at minimum port and driver) on
// every call, so the restore call needs the full set to succeed.
// complete_identifier is server-derived and excluded (never accepted by
// ups.update); monpwd is a secret and is never read back by ups.config.
type upsConfigOriginal struct {
	Identifier     string  `json:"identifier"`
	Mode           string  `json:"mode"`
	RemoteHost     string  `json:"remotehost"`
	RemotePort     int64   `json:"remoteport"`
	Driver         string  `json:"driver"`
	Port           string  `json:"port"`
	Options        string  `json:"options"`
	OptionsUPSD    string  `json:"optionsupsd"`
	Description    string  `json:"description"`
	Shutdown       string  `json:"shutdown"`
	ShutdownTimer  int64   `json:"shutdowntimer"`
	ShutdownCmd    *string `json:"shutdowncmd"`
	MonUser        string  `json:"monuser"`
	ExtraUsers     string  `json:"extrausers"`
	RMonitor       bool    `json:"rmonitor"`
	PowerDown      bool    `json:"powerdown"`
	HostSync       int64   `json:"hostsync"`
	NoCommWarnTime *int64  `json:"nocommwarntime"`
}

// readUPSConfigOriginal reads the box's current UPS configuration via
// ups.config, so the test can restore it exactly afterward.
func readUPSConfigOriginal(t *testing.T) upsConfigOriginal {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "ups.config")
	if err != nil {
		t.Fatalf("error reading current ups config: %v", err)
	}
	var orig upsConfigOriginal
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing ups.config response: %v", err)
	}
	return orig
}

// restoreUPSConfig sends the full original configuration back via
// ups.update (ups.update requires fields such as port and driver on every
// call, so sending only "description" back would fail the same way the
// live bug did). It runs from t.Cleanup, so it restores the box even if the
// Terraform steps themselves fail partway through.
func restoreUPSConfig(t *testing.T, orig upsConfigOriginal) {
	t.Helper()
	payload := map[string]any{
		"identifier":    orig.Identifier,
		"mode":          orig.Mode,
		"remotehost":    orig.RemoteHost,
		"remoteport":    orig.RemotePort,
		"driver":        orig.Driver,
		"port":          orig.Port,
		"options":       orig.Options,
		"optionsupsd":   orig.OptionsUPSD,
		"description":   orig.Description,
		"shutdown":      orig.Shutdown,
		"shutdowntimer": orig.ShutdownTimer,
		"monuser":       orig.MonUser,
		"extrausers":    orig.ExtraUsers,
		"rmonitor":      orig.RMonitor,
		"powerdown":     orig.PowerDown,
		"hostsync":      orig.HostSync,
	}
	if orig.ShutdownCmd != nil {
		payload["shutdowncmd"] = *orig.ShutdownCmd
	} else {
		payload["shutdowncmd"] = nil
	}
	if orig.NoCommWarnTime != nil {
		payload["nocommwarntime"] = *orig.NoCommWarnTime
	} else {
		payload["nocommwarntime"] = nil
	}
	if _, err := acctest.Client().Call(context.Background(), "ups.update", payload); err != nil {
		t.Fatalf("error restoring ups configuration: %v", err)
	}
}

// TestAccUPSConfig_setAndRestore drives the singleton truenas_ups_config
// resource's "description" field (a cosmetic, low-risk descriptive string)
// through a test value and back to the value read from the box before the
// test ran, then imports it. It requires TF_ACC=1 and TRUENAS_DISRUPTIVE=1
// (acctest.DisruptiveCheck), since it mutates the box's live UPS
// configuration; a t.Cleanup-registered API restore is the safety net if the
// Terraform steps fail.
func TestAccUPSConfig_setAndRestore(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readUPSConfigOriginal(t)
	// ups.update requires driver and port on every call. On a box where UPS
	// was never configured (empty driver/port), any update we make could not
	// be restored to the unconfigured state — skip rather than leave residue.
	if orig.Driver == "" || orig.Port == "" {
		t.Skip("UPS is unconfigured on the target box (empty driver/port); set-and-restore cannot restore the unconfigured state")
	}
	t.Cleanup(func() { restoreUPSConfig(t, orig) })

	testValue := acctest.RandName("tf-acc-description")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccUPSConfigConfig(testValue),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_ups_config.test", "id", "ups_config"),
					resource.TestCheckResourceAttr("truenas_ups_config.test", "description", testValue),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccUPSConfigConfig(orig.Description),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_ups_config.test", "description", orig.Description),
				),
			},
			{
				ResourceName:            "truenas_ups_config.test",
				ImportState:             true,
				ImportStateId:           "ups_config",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"monpwd"},
			},
		},
	})
}

func testAccUPSConfigConfig(description string) string {
	return fmt.Sprintf(`
resource "truenas_ups_config" "test" {
  description = %q
}
`, description)
}
