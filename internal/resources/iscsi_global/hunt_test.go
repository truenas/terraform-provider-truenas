// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_global_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntISCSIGlobal_lifecycle (.70 only; iSCSI service stopped there):
// change basename + listen_port + threshold, empty plan, import, clear
// threshold with 0, then drop attrs from config. t.Cleanup restores all.
func TestHuntISCSIGlobal_lifecycle(t *testing.T) {
	acctest.DisruptiveCheck(t)
	if acctest.EndpointHost() != "192.168.1.70" {
		t.Skip("writes iSCSI globals; .70 only")
	}
	orig := readISCSIGlobalOriginal(t)
	t.Cleanup(func() { restoreISCSIGlobal(t, orig) })
	full := acctest.ProviderConfig() + `
resource "truenas_iscsi_global" "test" {
  basename             = "iqn.2005-10.org.tfhunt.ctl"
  listen_port          = 3262
  pool_avail_threshold = 50
  isns_servers         = []
}
`
	zero := acctest.ProviderConfig() + `
resource "truenas_iscsi_global" "test" {
  basename             = "iqn.2005-10.org.tfhunt.ctl"
  pool_avail_threshold = 0
}
`
	bare := acctest.ProviderConfig() + `
resource "truenas_iscsi_global" "test" {
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: full},
			{Config: full, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{ResourceName: "truenas_iscsi_global.test", ImportState: true, ImportStateId: "iscsi_global", ImportStateVerify: true},
			{Config: full, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{Config: zero, Check: resource.TestCheckResourceAttr("truenas_iscsi_global.test", "pool_avail_threshold", "0")},
			{Config: bare, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{Config: acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_iscsi_global" "test" {
  basename    = %q
  listen_port = %d
}
`, orig.Basename, orig.ListenPort)},
		},
	})
}
