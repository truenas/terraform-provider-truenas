// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_global_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntNVMeTGlobal_basenqn (.70 only): change basenqn only, empty plan,
// import, drop from config, restore. t.Cleanup restores basenqn + xport_referral.
func TestHuntNVMeTGlobal_basenqn(t *testing.T) {
	acctest.DisruptiveCheck(t)
	if acctest.EndpointHost() != "192.168.1.70" {
		t.Skip("writes NVMe-oF globals; .70 only")
	}
	raw, err := acctest.Client().Call(context.Background(), "nvmet.global.config")
	if err != nil {
		t.Fatal(err)
	}
	var orig struct {
		Basenqn       string `json:"basenqn"`
		XportReferral bool   `json:"xport_referral"`
	}
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := acctest.RestoreCall(context.Background(), "nvmet.global.update", map[string]any{
			"basenqn": orig.Basenqn, "xport_referral": orig.XportReferral,
		}); err != nil {
			t.Fatalf("restore nvmet global: %v", err)
		}
	})
	set := acctest.ProviderConfig() + `
resource "truenas_nvmet_global" "test" {
  basenqn = "nqn.2014-08.org.tfhunt"
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: set},
			{Config: set, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{ResourceName: "truenas_nvmet_global.test", ImportState: true, ImportStateId: "nvmet_global", ImportStateVerify: true},
			{Config: set, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{Config: acctest.ProviderConfig() + `resource "truenas_nvmet_global" "test" {}`, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{Config: acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_nvmet_global" "test" {
  basenqn = %q
}
`, orig.Basenqn)},
		},
	})
}
