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
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
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

// TestAccUPSConfig_coverage sets every writable truenas_ups_config attribute
// (including the write-only monpwd / monpwd_wo_version pair) to a non-default
// value, asserts the read-back and the framework's empty-plan check, then
// flips mode to SLAVE, and finally imports. driver and port are set to the
// box's own current values so ups.update stays valid. The original
// configuration is captured first and restored via t.Cleanup (full
// ups.update payload), so the box is left as found even on failure.
//
// monpwd is write-only and cannot be asserted from state; ups.config returns
// it masked, so the test only proves ups.update accepts it.
func TestAccUPSConfig_coverage(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readUPSConfigOriginal(t)
	if orig.Driver == "" || orig.Port == "" {
		t.Skip("UPS is unconfigured on the target box (empty driver/port); set-and-restore cannot restore the unconfigured state")
	}
	t.Cleanup(func() { restoreUPSConfig(t, orig) })

	const extraUsers = "[tfaccextra]\n\tpassword = tfaccpass\n\tupsmon slave"

	full := func(mode string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_ups_config" "test" {
  identifier        = "tfaccups"
  mode              = %q
  remotehost        = "192.0.2.10"
  remoteport        = 3494
  driver            = %q
  port              = %q
  options           = "pollinterval = 10"
  optionsupsd       = "MAXAGE 25"
  description       = "tf-acc-ups-coverage"
  shutdown          = "BATT"
  shutdowntimer     = 90
  shutdowncmd       = "/sbin/poweroff"
  monuser           = "tfaccmon"
  monpwd            = "tfaccmonpass1"
  monpwd_wo_version = 1
  extrausers        = %q
  rmonitor          = true
  powerdown         = true
  hostsync          = 25
  nocommwarntime    = 400
}
`, mode, orig.Driver, orig.Port, extraUsers)
	}

	r := "truenas_ups_config.test"
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		Steps: []resource.TestStep{
			{
				Config: full("MASTER"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(r, "identifier", "tfaccups"),
					resource.TestCheckResourceAttr(r, "mode", "MASTER"),
					resource.TestCheckResourceAttr(r, "remotehost", "192.0.2.10"),
					resource.TestCheckResourceAttr(r, "remoteport", "3494"),
					resource.TestCheckResourceAttr(r, "driver", orig.Driver),
					resource.TestCheckResourceAttr(r, "port", orig.Port),
					resource.TestCheckResourceAttr(r, "options", "pollinterval = 10"),
					resource.TestCheckResourceAttr(r, "optionsupsd", "MAXAGE 25"),
					resource.TestCheckResourceAttr(r, "description", "tf-acc-ups-coverage"),
					resource.TestCheckResourceAttr(r, "shutdown", "BATT"),
					resource.TestCheckResourceAttr(r, "shutdowntimer", "90"),
					resource.TestCheckResourceAttr(r, "shutdowncmd", "/sbin/poweroff"),
					resource.TestCheckResourceAttr(r, "monuser", "tfaccmon"),
					resource.TestCheckResourceAttr(r, "monpwd_wo_version", "1"),
					resource.TestCheckResourceAttr(r, "extrausers", extraUsers),
					resource.TestCheckResourceAttr(r, "rmonitor", "true"),
					resource.TestCheckResourceAttr(r, "powerdown", "true"),
					resource.TestCheckResourceAttr(r, "hostsync", "25"),
					resource.TestCheckResourceAttr(r, "nocommwarntime", "400"),
				),
			},
			{
				Config: full("SLAVE"),
				Check:  resource.TestCheckResourceAttr(r, "mode", "SLAVE"),
			},
			{
				ResourceName:            r,
				ImportState:             true,
				ImportStateId:           "ups_config",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"monpwd", "monpwd_wo_version"},
			},
		},
	})
}
