// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package replication_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccReplication_pstOrder: the server sorts periodic_snapshot_tasks, so
// a config listing them in another order failed the apply (B-086). Import
// plans no change either.
func TestAccReplication_pstOrder(t *testing.T) {
	name := acctest.RandName("tffix3shtasks-repl")
	cfg := acctest.ProviderConfig() + replConfig(name, `
  auto             = true
  retention_policy = "SOURCE"
  periodic_snapshot_tasks = [truenas_periodic_snapshot_task.feed_b.id, truenas_periodic_snapshot_task.feed_a.id]
`)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckReplicationDestroyed(name),
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{Config: cfg, ResourceName: "truenas_replication_task.test", ImportState: true, ImportStateKind: resource.ImportBlockWithID},
		},
	})
}
