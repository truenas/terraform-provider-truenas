// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package tn_connect_config_test

// Adversarial bug-hunt test for the TrueNAS Connect config singleton.
//
// This resource writes EXACTLY ONE field: "enabled" (probed live, tn_connect.
// update on TrueNAS 26.0 accepts nothing else; the richer 25.10 fields are
// deliberately not exposed as writable). Setting "enabled" to true starts a
// real external cloud enrollment the provider's own tests never perform, so
// the only value a value-roundtrip test may safely write is "false". The test
// therefore:
//   - is gated behind TRUENAS_TEST_TNC (a disposable box), and
//   - refuses to run against a box that is already enrolled (enabled=true),
//     to avoid disabling a live enrollment,
// then drives enabled=false, asserts an empty re-plan, and imports. The
// original "enabled" is captured and restored in t.Cleanup (a no-op write of
// false on the only boxes this test will run on).

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntTnConnectConfig_allFields exercises the resource's one writable
// field (enabled=false) end to end. There is no other writable attribute to
// set; every other field is Computed-only.
func TestHuntTnConnectConfig_allFields(t *testing.T) {
	if os.Getenv("TRUENAS_TEST_TNC") == "" {
		t.Skip("set TRUENAS_TEST_TNC=1 (on a disposable, NOT-enrolled box) to run the tn_connect_config all-fields hunt test")
	}
	acctest.PreCheck(t)

	// Refuse to touch a box that is already enrolled: writing enabled=false
	// there would tear down a live TrueNAS Connect enrollment.
	raw, err := acctest.RestoreCall(context.Background(), "tn_connect.config")
	if err != nil {
		t.Fatalf("read tn_connect.config: %v", err)
	}
	var cfg struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Enabled {
		t.Skip("tn_connect is already enabled on this box (a real enrollment): refusing to run the hunt test")
	}
	orig := cfg.Enabled
	t.Cleanup(func() {
		if _, err := acctest.RestoreCall(context.Background(), "tn_connect.update", map[string]any{"enabled": orig}); err != nil {
			t.Errorf("RESTORE tn_connect.update failed: %v", err)
		}
	})

	c := acctest.ProviderConfig() + `
resource "truenas_tn_connect_config" "test" {
  enabled = false
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: c,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_tn_connect_config.test", "id", "tn_connect_config"),
					resource.TestCheckResourceAttr("truenas_tn_connect_config.test", "enabled", "false"),
				),
			},
			{
				Config:           c,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				ResourceName:      "truenas_tn_connect_config.test",
				ImportState:       true,
				ImportStateId:     "tn_connect_config",
				ImportStateVerify: true,
			},
		},
	})
}
