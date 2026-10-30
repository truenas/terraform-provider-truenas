// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package rsync_task_test

// Adversarial bug-hunt test: set every API-accepted attribute of
// truenas_rsync_task to a non-default value, assert the read-back re-plans
// empty, then import. It reuses testAccRsyncTaskCoverageConfig (MODULE mode,
// TEST-NET-1 host 192.0.2.10, enabled=false, validate_rpath=false), so
// nothing is ever contacted — lab-safe. ssh_credentials is left unset (it
// needs a keychain SSH credential and a reachable host in SSH mode).

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestHuntRsyncTask_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)

	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfhunt-rsync-ds"))
	cfg := acctest.ProviderConfig() + testAccRsyncTaskCoverageConfig(datasetName)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckRsyncTaskDestroyed(datasetName),
			testAccCheckRsyncDatasetDestroyed(datasetName),
		),
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				Config:           cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				ResourceName:      "truenas_rsync_task.test",
				ImportState:       true,
				ImportStateVerify: true,
				// validate_rpath/ssh_keyscan are write-only directives the API
				// never returns.
				ImportStateVerifyIgnore: []string{"validate_rpath", "ssh_keyscan"},
			},
		},
	})
}
