// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package twofactor_auth_test

// Adversarial bug-hunt test: apply every safely-settable API-accepted value
// and assert an empty re-plan, then import.
//
// SAFETY: "enabled" is deliberately NOT set. Flipping system-wide 2FA can lock
// out password-based logins for users who have not enrolled a TOTP secret;
// the resource's own contract and the committed acceptance test never flip it.
// Leaving it unset keeps it null in req.Config, so updatePayload (config-
// driven) omits it and the box's enable/disable state is untouched. The
// remaining writable fields — window and services.ssh — do not break logins
// on their own (services.ssh only takes effect once 2FA is enabled). This
// mirrors the acceptance test's never-flip-enabled approach while additionally
// exercising services.ssh, so no env-gated t.Skip is needed.

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// huntTwoFactorOriginal captures the two fields the hunt test writes, so both
// can be restored (the package's restoreTwoFactorAuth restores only window).
type huntTwoFactorOriginal struct {
	Enabled  bool  `json:"enabled"`
	Window   int64 `json:"window"`
	Services struct {
		SSH bool `json:"ssh"`
	} `json:"services"`
}

// TestHuntTwoFactorAuth_enabled flips the one field _allFields leaves alone:
// enabled = true (with services.ssh = true and a non-default window), asserts
// the read-back re-plans empty, then imports. Global 2FA only gates password
// logins — the provider's API-key session and key-based SSH are unaffected —
// and the lab boxes are disposable, so the original enabled/window/ssh are
// captured and restored via t.Cleanup rather than leaving the field untested.
func TestHuntTwoFactorAuth_enabled(t *testing.T) {
	acctest.DisruptiveCheck(t)

	raw, err := acctest.RestoreCall(context.Background(), "auth.twofactor.config")
	if err != nil {
		t.Fatalf("read auth.twofactor.config: %v", err)
	}
	var orig huntTwoFactorOriginal
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("parse auth.twofactor.config: %v", err)
	}
	t.Cleanup(func() {
		if _, err := acctest.RestoreCall(context.Background(), "auth.twofactor.update", map[string]any{
			"enabled":  orig.Enabled,
			"window":   orig.Window,
			"services": map[string]any{"ssh": orig.Services.SSH},
		}); err != nil {
			t.Errorf("RESTORE auth.twofactor (enabled=%v): %v", orig.Enabled, err)
		}
	})

	c := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_twofactor_auth" "test" {
  enabled = true
  window  = %d
  services = {
    ssh = true
  }
}
`, orig.Window+15)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: c},
			{Config: c, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:      "truenas_twofactor_auth.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestHuntTwoFactorAuth_allFields sets window and services.ssh to non-default
// values, asserts the read-back re-plans empty, then imports. enabled is left
// at the box's value (see SAFETY above). The original window and services.ssh
// are captured first and restored via t.Cleanup.
func TestHuntTwoFactorAuth_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)

	raw, err := acctest.RestoreCall(context.Background(), "auth.twofactor.config")
	if err != nil {
		t.Fatalf("read auth.twofactor.config: %v", err)
	}
	var orig huntTwoFactorOriginal
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("parse auth.twofactor.config: %v", err)
	}
	t.Cleanup(func() {
		if _, err := acctest.RestoreCall(context.Background(), "auth.twofactor.update", map[string]any{
			"window":   orig.Window,
			"services": map[string]any{"ssh": orig.Services.SSH},
		}); err != nil {
			t.Errorf("restore auth.twofactor config: %v", err)
		}
	})

	window := orig.Window + 30
	ssh := !orig.Services.SSH

	c := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_twofactor_auth" "test" {
  window = %d
  services = {
    ssh = %t
  }
}
`, window, ssh)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: c},
			{Config: c, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:      "truenas_twofactor_auth.test",
				ImportState:       true,
				ImportStateId:     "twofactor_auth",
				ImportStateVerify: true,
			},
		},
	})
}
