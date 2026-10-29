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

// advancedFields are the system.advanced fields
// TestAccSystemAdvanced_setAndRestore writes, in HCL order. nvidia is added
// on 26.0+. sed_passwd is never written: it cannot be read back, so the test
// could not restore it (an earlier version sent sed_passwd = "", which clears
// a configured SED password). syslogservers is written only when the box has
// none configured.
var advancedFields = []string{
	"advancedmode", "anonstats", "autotune", "boot_scrub", "consolemenu", "consolemsg", "debugkernel",
	"fqdn_syslog", "kdump_enabled", "kernel_extra_options", "login_banner", "motd", "powerdaemon",
	"sed_user", "serialconsole", "serialport", "serialspeed", "syslog_audit", "sysloglevel", "traceback",
	"uploadcrash",
}

// advancedTestValues are the values step 1 sets. Each one only changes what
// the web UI shows, what gets logged or reported, or the next boot-pool
// scrub interval; nothing that affects booting, the console, power, SED or
// the network. Every other field in advancedFields is written with the box's
// own value. The cleanup writes all original values back.
func advancedTestValues(orig map[string]any) map[string]any {
	flip := func(k string) bool { b, _ := orig[k].(bool); return !b }
	level := "F_NOTICE"
	if orig["sysloglevel"] == level {
		level = "F_INFO"
	}
	scrub := int64(8)
	if n, _ := orig["boot_scrub"].(float64); int64(n) == scrub {
		scrub = 9
	}
	return map[string]any{
		"advancedmode": flip("advancedmode"),
		"anonstats":    flip("anonstats"),
		"boot_scrub":   scrub,
		"fqdn_syslog":  flip("fqdn_syslog"),
		"login_banner": acctest.RandName("tf-acc-banner"),
		"motd":         acctest.RandName("tf-acc-motd"),
		"syslog_audit": flip("syslog_audit"),
		"sysloglevel":  level,
		"traceback":    flip("traceback"),
		"uploadcrash":  flip("uploadcrash"),
	}
}

// readSystemAdvancedOriginal reads the box's current system.advanced
// configuration via acctest.RestoreCall (system.advanced.update is
// job:false, probed live), so the test can restore it exactly afterward.
func readSystemAdvancedOriginal(t *testing.T) map[string]any {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "system.advanced.config")
	if err != nil {
		t.Fatalf("error reading current system advanced config: %v", err)
	}
	var orig map[string]any
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing system.advanced.config response: %v", err)
	}
	return orig
}

// restoreSystemAdvanced writes every field the test sets back to its
// original value via system.advanced.update, through acctest.RestoreCall so
// a t.Cleanup-time stale connection on the long-lived acctest.Client() is
// retried and reconnected. It runs from t.Cleanup, so it restores the box
// even if the Terraform steps fail partway through, and then re-reads the
// configuration to confirm.
func restoreSystemAdvanced(t *testing.T, orig map[string]any, v26 bool) {
	t.Helper()
	fields := append([]string{}, advancedFields...)
	if v26 {
		fields = append(fields, "nvidia")
	}
	p := map[string]any{}
	for _, k := range fields {
		p[k] = orig[k]
	}
	if _, err := acctest.RestoreCall(context.Background(), "system.advanced.update", p); err != nil {
		t.Fatalf("error restoring system advanced config: %v", err)
	}
	after := readSystemAdvancedOriginal(t)
	for _, k := range fields {
		if fmt.Sprint(after[k]) != fmt.Sprint(orig[k]) {
			t.Errorf("restore of %s not confirmed: got %v, want %v", k, after[k], orig[k])
		}
	}
}

// TestAccSystemAdvanced_setAndRestore drives the singleton
// truenas_system_advanced resource through the advanced-settings surface:
// step 1 changes the low-impact fields in advancedTestValues and writes every
// other field with the box's own value, step 2 writes the original values
// back, then it imports. It requires TF_ACC=1 and TRUENAS_DISRUPTIVE=1
// (acctest.DisruptiveCheck), since it mutates the box's live system advanced
// configuration; a t.Cleanup-registered API restore of every written field
// is the safety net if the Terraform steps fail. (An earlier version set
// about twenty fields to fixed values and restored only motd, leaving
// advancedmode, anonstats, traceback, uploadcrash, consolemsg and
// login_banner changed on every box it ran on.)
func TestAccSystemAdvanced_setAndRestore(t *testing.T) {
	acctest.DisruptiveCheck(t)

	// nvidia exists only on TrueNAS 26.0+.
	v26 := acctest.ServerVersionAtLeast(t, 26, 0)
	orig := readSystemAdvancedOriginal(t)
	t.Cleanup(func() { restoreSystemAdvanced(t, orig, v26) })

	test := advancedTestValues(orig)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccSystemAdvancedConfig(orig, test, v26),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_system_advanced.test", "id", "system_advanced"),
					resource.TestCheckResourceAttr("truenas_system_advanced.test", "motd", test["motd"].(string)),
					resource.TestCheckResourceAttr("truenas_system_advanced.test", "login_banner", test["login_banner"].(string)),
					resource.TestCheckResourceAttr("truenas_system_advanced.test", "sysloglevel", test["sysloglevel"].(string)),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccSystemAdvancedConfig(orig, nil, v26),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_system_advanced.test", "motd", fmt.Sprint(orig["motd"])),
					resource.TestCheckResourceAttr("truenas_system_advanced.test", "advancedmode", fmt.Sprint(orig["advancedmode"])),
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

// testAccSystemAdvancedConfig renders every field in advancedFields, taking
// the value from override when present and from the box's original
// configuration otherwise.
func testAccSystemAdvancedConfig(orig, override map[string]any, v26 bool) string {
	v := func(k string) string {
		val, ok := override[k]
		if !ok {
			val = orig[k]
		}
		switch x := val.(type) {
		case string:
			return fmt.Sprintf("%q", x)
		case float64:
			return fmt.Sprintf("%d", int64(x))
		default:
			return fmt.Sprint(x)
		}
	}
	extra := ""
	if v26 {
		extra += "\n  nvidia               = " + v("nvidia")
	}
	if servers, _ := orig["syslogservers"].([]any); len(servers) == 0 {
		extra += "\n  syslogservers        = []"
	}
	return fmt.Sprintf(`
resource "truenas_system_advanced" "test" {
  advancedmode         = %s
  anonstats            = %s
  autotune             = %s
  boot_scrub           = %s
  consolemenu          = %s
  consolemsg           = %s
  debugkernel          = %s
  fqdn_syslog          = %s
  kdump_enabled        = %s
  kernel_extra_options = %s
  login_banner         = %s
  motd                 = %s
  powerdaemon          = %s
  sed_user             = %s
  serialconsole        = %s
  serialport           = %s
  serialspeed          = %s
  syslog_audit         = %s
  sysloglevel          = %s
  traceback            = %s
  uploadcrash          = %s%s
}
`, v("advancedmode"), v("anonstats"), v("autotune"), v("boot_scrub"), v("consolemenu"), v("consolemsg"),
		v("debugkernel"), v("fqdn_syslog"), v("kdump_enabled"), v("kernel_extra_options"), v("login_banner"),
		v("motd"), v("powerdaemon"), v("sed_user"), v("serialconsole"), v("serialport"), v("serialspeed"),
		v("syslog_audit"), v("sysloglevel"), v("traceback"), v("uploadcrash"), extra)
}
