// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package twofactor_auth_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccTwoFactorAuthDataSource_basic reads the current TrueNAS two-factor
// authentication configuration through the truenas_twofactor_auth
// datasource only. It never writes: 2FA is system-wide security
// configuration, and the box's real settings must not be overwritten by a
// plain acceptance test run.
func TestAccTwoFactorAuthDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_twofactor_auth" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_twofactor_auth.test", "id", "twofactor_auth"),
					resource.TestCheckResourceAttrSet("data.truenas_twofactor_auth.test", "enabled"),
					resource.TestCheckResourceAttrSet("data.truenas_twofactor_auth.test", "window"),
					resource.TestCheckResourceAttrSet("data.truenas_twofactor_auth.test", "services.ssh"),
				),
			},
		},
	})
}

// twoFactorAuthOriginal captures the field
// TestAccTwoFactorAuth_setAndRestore touches.
type twoFactorAuthOriginal struct {
	Window int64 `json:"window"`
}

// readTwoFactorAuthOriginal reads the box's current window via
// auth.twofactor.config, so the test can restore it exactly afterward.
func readTwoFactorAuthOriginal(t *testing.T) twoFactorAuthOriginal {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "auth.twofactor.config")
	if err != nil {
		t.Fatalf("error reading current two-factor auth config: %v", err)
	}
	var orig twoFactorAuthOriginal
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing auth.twofactor.config response: %v", err)
	}
	return orig
}

// restoreTwoFactorAuth sends window back to its original value via
// auth.twofactor.update. It runs from t.Cleanup, so it restores the box
// even if the Terraform steps themselves fail partway through. It
// deliberately sends ONLY "window" — never "enabled" — matching this
// resource's safety contract that the committed acceptance test must never
// flip system-wide 2FA on or off.
func restoreTwoFactorAuth(t *testing.T, orig twoFactorAuthOriginal) {
	t.Helper()
	if _, err := acctest.RestoreCall(context.Background(), "auth.twofactor.update", map[string]any{
		"window": orig.Window,
	}); err != nil {
		t.Fatalf("error restoring two-factor auth config: %v", err)
	}
}

// TestAccTwoFactorAuth_setAndRestore drives the singleton
// truenas_twofactor_auth resource's "window" field through a different
// valid value and back to the value read from the box before the test ran,
// then imports it. It requires TF_ACC=1 and TRUENAS_DISRUPTIVE=1
// (acctest.DisruptiveCheck), since it mutates the box's live two-factor
// authentication configuration; a t.Cleanup-registered API restore is the
// safety net if the Terraform steps fail.
//
// SAFETY: this test — and the resource's updatePayload — must NEVER set
// "enabled". The generated config below intentionally omits it, and
// updatePayload sources inclusion from req.Config (not req.Plan): a field
// is only ever included in the auth.twofactor.update payload when the
// user's HCL explicitly sets it, so an omitted "enabled" attribute stays
// null in config and is correctly left out of every request this test's
// steps issue, leaving the box's system-wide 2FA enable/disable state
// untouched by this test. (Separately, a one-off scripted SAFETY
// VERIFICATION documented in task-3-report.md confirmed live that enabling
// 2FA does not break the provider's own API-key authentication — that
// check is not part of this committed test, since flipping "enabled" here
// would violate the never-flip-enabled contract this test exists to
// enforce.)
func TestAccTwoFactorAuth_setAndRestore(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readTwoFactorAuthOriginal(t)
	t.Cleanup(func() { restoreTwoFactorAuth(t, orig) })

	// Pick a different valid value (>= 0) than what's currently set.
	toggled := orig.Window + 30

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccTwoFactorAuthConfig(toggled),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_twofactor_auth.test", "id", "twofactor_auth"),
					resource.TestCheckResourceAttr("truenas_twofactor_auth.test", "window", fmt.Sprintf("%d", toggled)),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccTwoFactorAuthConfig(orig.Window),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_twofactor_auth.test", "window", fmt.Sprintf("%d", orig.Window)),
				),
			},
			{
				ResourceName:      "truenas_twofactor_auth.test",
				ImportState:       true,
				ImportStateId:     "twofactor_auth",
				ImportStateVerify: true,
			},
		},
	})
}

// testAccTwoFactorAuthConfig sets ONLY "window" — "enabled" and "services"
// are deliberately omitted from the HCL, so they stay null in req.Config
// and updatePayload (config-driven) never includes them, per this
// resource's never-flip-enabled safety contract.
func testAccTwoFactorAuthConfig(window int64) string {
	return fmt.Sprintf(`
resource "truenas_twofactor_auth" "test" {
  window = %d
}
`, window)
}
