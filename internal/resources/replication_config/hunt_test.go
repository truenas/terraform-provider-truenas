// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package replication_config_test

// Adversarial bug-hunt test: apply every API-accepted value and assert an
// empty re-plan, then import. readReplicationConfigOriginal /
// restoreReplicationConfig are defined in acceptance_test.go.

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntReplicationConfig_allFields sets the sole writable field
// max_parallel_replication_tasks to a non-default positive value (a positive
// value is used rather than the 0 "unlimited" sentinel so the round-trip is
// unambiguous), asserts the read-back re-plans empty, then imports. The
// original value is captured first and restored via t.Cleanup.
func TestHuntReplicationConfig_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readReplicationConfigOriginal(t)
	t.Cleanup(func() { restoreReplicationConfig(t, orig) })

	var value int64 = 3
	if orig.MaxParallelReplicationTasks != nil && *orig.MaxParallelReplicationTasks == value {
		value = 4
	}

	c := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_replication_config" "test" {
  max_parallel_replication_tasks = %d
}
`, value)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: c},
			{Config: c, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:      "truenas_replication_config.test",
				ImportState:       true,
				ImportStateId:     "replication_config",
				ImportStateVerify: true,
			},
		},
	})
}
