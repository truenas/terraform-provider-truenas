// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_auth_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

const huntAuthTag = 9311

func huntAuthCfg(user, body string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_iscsi_auth" "test" {
  tag    = %d
  user   = %q
  secret = "tfhuntsecret01"
%s
}
`, huntAuthTag, user, body)
}

// TestHuntISCSIAuth_lifecycle: empty plan, import, rotate via version, unset
// discovery_auth.
func TestHuntISCSIAuth_lifecycle(t *testing.T) {
	user := acctest.RandName("tfhuntblkauthlc")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: huntAuthCfg(user, `  discovery_auth = "CHAP"`)},
			{Config: huntAuthCfg(user, `  discovery_auth = "CHAP"`), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{ResourceName: "truenas_iscsi_auth.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"secret", "peersecret"}},
			{Config: huntAuthCfg(user, `  discovery_auth = "CHAP"`), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{Config: huntAuthCfg(user, ``), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}
