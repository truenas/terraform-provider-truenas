// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package lxc_config_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccLXCConfig_bridgeAuto guards B-081: "[AUTO]" is the only key
// lxc.bridge_choices offers by default, and the server stores it as null, so
// bridge = "[AUTO]" failed the apply. Import followed by plan must be empty
// too. The original bridge is restored from t.Cleanup.
func TestAccLXCConfig_bridgeAuto(t *testing.T) {
	acctest.DisruptiveCheck(t)
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("truenas_lxc_config requires TrueNAS 26.0 or later")
	}
	raw, err := acctest.RestoreCall(context.Background(), "lxc.config")
	if err != nil {
		t.Fatalf("read lxc.config: %v", err)
	}
	var orig struct {
		Bridge *string `json:"bridge"`
	}
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("parse lxc.config: %v", err)
	}
	t.Cleanup(func() {
		if _, err := acctest.RestoreCall(context.Background(), "lxc.update", map[string]any{"bridge": orig.Bridge}); err != nil {
			t.Errorf("restore lxc bridge: %v", err)
			return
		}
		raw, err := acctest.RestoreCall(context.Background(), "lxc.config")
		var got struct {
			Bridge *string `json:"bridge"`
		}
		if err != nil || json.Unmarshal(raw, &got) != nil || !strPtrEqual(got.Bridge, orig.Bridge) {
			t.Errorf("lxc bridge not restored: got %s, want %s (err %v)", fmtStrPtr(got.Bridge), fmtStrPtr(orig.Bridge), err)
		}
	})

	cfg := acctest.ProviderConfig() + `
resource "truenas_lxc_config" "test" {
  bridge = "[AUTO]"
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: append([]resource.TestStep{
			{Config: cfg, Check: resource.TestCheckResourceAttr("truenas_lxc_config.test", "bridge", "[AUTO]")},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{
				PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		}, acctest.ImportReapplyNoop("truenas_lxc_config.test", cfg)...),
	})
}
