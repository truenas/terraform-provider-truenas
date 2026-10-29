// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package resilver_config_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func resilverRestore(t *testing.T) {
	raw, err := acctest.RestoreCall(context.Background(), "pool.resilver.config")
	if err != nil {
		t.Fatal(err)
	}
	var orig map[string]any
	_ = json.Unmarshal(raw, &orig)
	delete(orig, "id")
	t.Cleanup(func() {
		if _, err := acctest.RestoreCall(context.Background(), "pool.resilver.update", orig); err != nil {
			t.Errorf("restore resilver: %v", err)
		}
	})
}

// TestAccResilverConfig_beginNoLeadingZero guards B-088: "8:00" is accepted
// by the server and stored as "08:00"; the apply must not fail, the re-plan
// must be empty, and so must the plan after a removed + import.
func TestAccResilverConfig_beginNoLeadingZero(t *testing.T) {
	acctest.PreCheck(t)
	acctest.DisruptiveCheck(t)
	resilverRestore(t)
	cfg := acctest.ProviderConfig() + `
resource "truenas_resilver_config" "test" {
  begin = "8:00"
  end   = "17:5"
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, Check: resource.TestCheckResourceAttr("truenas_resilver_config.test", "begin", "8:00")},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{Config: acctest.ProviderConfig() + `
removed {
  from = truenas_resilver_config.test
  lifecycle {
    destroy = false
  }
}
`},
			{Config: cfg, ResourceName: "truenas_resilver_config.test", ImportState: true, ImportStatePersist: true, ImportStateId: "resilver_config"},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}
