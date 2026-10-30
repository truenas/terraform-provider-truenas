// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package system_general_test

// Adversarial bug-hunt test: set every SAFE system_general attribute to a
// non-default value at once, assert the read-back re-plans empty, then import.
// system_general is a singleton controlling the management UI; the box's
// original values are captured and restored in t.Cleanup.
//
// Deliberately NOT set (they govern how the box is reached over the network,
// and changing them can cut off management access): ui_address, ui_v6address,
// ui_allowlist, ui_certificate, ui_port, ui_httpsport, ui_httpsprotocols.
// updatePayload sends only the fields present in config, so those keep their
// current values and management access cannot be severed.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// huntGeneralFields are the fields TestHuntSystemGeneral_allFields sets and
// restores. None affect how the box is reached over the network.
var huntGeneralFields = []string{
	// ds_auth is not set: enabling it needs an Enterprise license
	// ("[EINVAL] general_settings.ds_auth: Directory services authentication
	// for UI and API access requires an Enterprise license").
	"kbdmap", "timezone", "ui_consolemsg",
	"ui_httpsredirect", "ui_x_frame_options", "usage_collection",
}

// huntGeneralCaptureRestore reads the current values of huntGeneralFields and
// registers a t.Cleanup that writes them back (restarting the web server so a
// restored ui_x_frame_options / ui_httpsredirect takes effect). usage_collection
// goes back to null when the administrator had never explicitly set it, so the
// restore does not record an opt-in.
func huntGeneralCaptureRestore(t *testing.T) map[string]any {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "system.general.config")
	if err != nil {
		t.Fatalf("read system.general.config: %v", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	orig := map[string]any{}
	for _, k := range huntGeneralFields {
		orig[k] = cfg[k]
	}
	usageWasSet, _ := cfg["usage_collection_is_set"].(bool)

	t.Cleanup(func() {
		p := map[string]any{"ui_restart_delay": 1}
		for _, k := range huntGeneralFields {
			if k == "usage_collection" {
				continue
			}
			p[k] = orig[k]
		}
		if usageWasSet {
			p["usage_collection"] = orig["usage_collection"]
		} else {
			p["usage_collection"] = nil
		}
		if _, err := acctest.RestoreCall(context.Background(), "system.general.update", p); err != nil {
			t.Errorf("RESTORE system.general.update failed: %v", err)
			return
		}
		time.Sleep(12 * time.Second) // let the web server restart
	})
	return orig
}

// TestHuntSystemGeneral_allFields drives every safe writable field to a
// non-default value in one apply.
func TestHuntSystemGeneral_allFields(t *testing.T) {
	if os.Getenv("TRUENAS_TEST_SYSTEM_GENERAL") == "" {
		t.Skip("set TRUENAS_TEST_SYSTEM_GENERAL=1 (on a disposable box) to run the system_general all-fields hunt test")
	}
	acctest.PreCheck(t)

	orig := huntGeneralCaptureRestore(t)

	flip := func(k string) bool {
		b, _ := orig[k].(bool)
		return !b
	}
	kbdmap := "de"
	if orig["kbdmap"] == "de" {
		kbdmap = "us"
	}
	tz := "UTC"
	if orig["timezone"] == "UTC" {
		tz = "America/New_York"
	}
	xfo := "DENY"
	if orig["ui_x_frame_options"] == "DENY" {
		xfo = "SAMEORIGIN"
	}

	c := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_system_general" "test" {
  kbdmap             = %q
  timezone           = %q
  ui_consolemsg      = %t
  ui_httpsredirect   = %t
  ui_x_frame_options = %q
  usage_collection   = %t
}
`, kbdmap, tz, flip("ui_consolemsg"), flip("ui_httpsredirect"), xfo, flip("usage_collection"))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: c,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_system_general.test", "id", "system_general"),
					resource.TestCheckResourceAttr("truenas_system_general.test", "kbdmap", kbdmap),
					resource.TestCheckResourceAttr("truenas_system_general.test", "timezone", tz),
					resource.TestCheckResourceAttr("truenas_system_general.test", "ui_x_frame_options", xfo),
				),
			},
			{
				Config:           c,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				ResourceName:      "truenas_system_general.test",
				ImportState:       true,
				ImportStateId:     "system_general",
				ImportStateVerify: true,
			},
		},
	})
}
