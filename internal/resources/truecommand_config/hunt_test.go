// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package truecommand_config_test

// Adversarial bug-hunt test for TrueCommand config.
//
// Enrolling (enabled = true) needs a reachable TrueCommand instance and a
// real api_key, which the lab does not have. This exercises the
// disabled-state roundtrip: enabled = false, assert an empty re-plan, then
// import. The computed status/remote fields and the write-only api_key are
// ignored on import. The original config is snapshotted and restored.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func snapshotRestore(t *testing.T, ns string, drop ...string) {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), ns+".config")
	if err != nil {
		t.Fatalf("read %s.config: %v", ns, err)
	}
	var orig map[string]any
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatal(err)
	}
	delete(orig, "id")
	for _, d := range drop {
		delete(orig, d)
	}
	t.Cleanup(func() {
		if _, err := acctest.RestoreCall(context.Background(), ns+".update", orig); err != nil {
			t.Errorf("RESTORE %s.update failed: %v", ns, err)
		}
	})
}

func TestHuntTrueCommandConfig_disabled(t *testing.T) {
	acctest.DisruptiveCheck(t)
	snapshotRestore(t, "truecommand", "api_key", "status", "status_reason", "remote_url", "remote_ip_address")
	c := acctest.ProviderConfig() + `
resource "truenas_truecommand_config" "test" {
  enabled = false
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: c},
			{Config: c, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:            "truenas_truecommand_config.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"api_key", "api_key_wo", "api_key_wo_version", "status", "status_reason", "remote_url", "remote_ip_address"},
			},
		},
	})
}
