// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package audit_config_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccAuditConfigDataSource_basic reads the current TrueNAS audit
// configuration through the truenas_audit_config datasource only. It never
// writes: audit retention/quota settings are system-wide configuration, and
// the box's real settings must not be overwritten by a plain acceptance
// test run.
func TestAccAuditConfigDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_audit_config" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_audit_config.test", "id", "audit_config"),
					resource.TestCheckResourceAttrSet("data.truenas_audit_config.test", "retention"),
					resource.TestCheckResourceAttrSet("data.truenas_audit_config.test", "quota_fill_warning"),
					resource.TestCheckResourceAttrSet("data.truenas_audit_config.test", "quota_fill_critical"),
				),
			},
		},
	})
}

// auditConfigOriginal captures the fields TestAccAuditConfig_setAndRestore
// touches.
type auditConfigOriginal struct {
	QuotaFillWarning int64 `json:"quota_fill_warning"`
}

// readAuditConfigOriginal reads the box's current quota_fill_warning via
// audit.config, so the test can restore it exactly afterward.
func readAuditConfigOriginal(t *testing.T) auditConfigOriginal {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "audit.config")
	if err != nil {
		t.Fatalf("error reading current audit config: %v", err)
	}
	var orig auditConfigOriginal
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing audit.config response: %v", err)
	}
	return orig
}

// restoreAuditConfig sends quota_fill_warning back to its original value via
// audit.update. It runs from t.Cleanup, so it restores the box even if the
// Terraform steps themselves fail partway through.
func restoreAuditConfig(t *testing.T, orig auditConfigOriginal) {
	t.Helper()
	if _, err := acctest.RestoreCall(context.Background(), "audit.update", map[string]any{
		"quota_fill_warning": orig.QuotaFillWarning,
	}); err != nil {
		t.Fatalf("error restoring audit config: %v", err)
	}
}

// TestAccAuditConfig_setAndRestore drives the singleton truenas_audit_config
// resource's "quota_fill_warning" field (range 5-80) through a different
// valid value and back to the value read from the box before the test ran,
// then imports it. It requires TF_ACC=1 and TRUENAS_DISRUPTIVE=1
// (acctest.DisruptiveCheck), since it mutates the box's live audit
// configuration; a t.Cleanup-registered API restore is the safety net if
// the Terraform steps fail.
func TestAccAuditConfig_setAndRestore(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readAuditConfigOriginal(t)
	t.Cleanup(func() { restoreAuditConfig(t, orig) })

	// Pick a different valid value (5-80) than what's currently set.
	toggled := int64(60)
	if orig.QuotaFillWarning == toggled {
		toggled = 50
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccAuditConfigConfig(toggled),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_audit_config.test", "id", "audit_config"),
					resource.TestCheckResourceAttr("truenas_audit_config.test", "quota_fill_warning", fmt.Sprintf("%d", toggled)),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccAuditConfigConfig(orig.QuotaFillWarning),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_audit_config.test", "quota_fill_warning", fmt.Sprintf("%d", orig.QuotaFillWarning)),
				),
			},
			{
				ResourceName:      "truenas_audit_config.test",
				ImportState:       true,
				ImportStateId:     "audit_config",
				ImportStateVerify: true,
			},
		},
	})
}

func testAccAuditConfigConfig(quotaFillWarning int64) string {
	return fmt.Sprintf(`
resource "truenas_audit_config" "test" {
  quota_fill_warning = %d
}
`, quotaFillWarning)
}
