// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package failover_config_test

// Adversarial bug-hunt test for failover config.
//
// failover.update only works on a licensed HA pair. The lab has no HA pair,
// so this test is gated on TRUENAS_HA=1 and skips everywhere else. On HA it
// toggles "disabled" and "master", asserts an empty re-plan, then imports.
// The original config is snapshotted and restored.

import (
	"context"
	"encoding/json"
	"os"
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

func TestHuntFailoverConfig_toggle(t *testing.T) {
	acctest.DisruptiveCheck(t)
	if os.Getenv("TRUENAS_HA") != "1" {
		t.Skip("failover.update requires a licensed HA pair; set TRUENAS_HA=1 on HA hardware")
	}
	snapshotRestore(t, "failover")
	c := acctest.ProviderConfig() + `
resource "truenas_failover_config" "test" {
  disabled = true
  master   = true
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: c},
			{Config: c, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:      "truenas_failover_config.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
