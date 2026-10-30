// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_host_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

const (
	huntKey1 = "DHHC-1:01:hnvzNuiX9Bi/Z6CwYPoi9KQ3SDHzgRhYGq8n5ZDCDiIPCaWe:"
	huntKey2 = "DHHC-1:01:8//S93VwLZz0kJJv3ENC8FfxlHiZbXkgEWsR0EsWp54I+FSJ:"
)

func huntHostCfg(nqn, body string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_nvmet_host" "test" {
  hostnqn = %q
%s
}
`, nqn, body)
}

// TestHuntNVMetHost_lifecycle: dhgroup/hash set, empty plan, import, unset.
func TestHuntNVMetHost_lifecycle(t *testing.T) {
	nqn := "nqn.2014-08.org.example:" + acctest.RandName("tfhuntblkhostlc")
	full := fmt.Sprintf(`
  description     = "d"
  dhchap_key      = %q
  dhchap_ctrl_key = %q
  dhchap_dhgroup  = "2048-BIT"
  dhchap_hash     = "SHA-512"
`, huntKey1, huntKey2)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: huntHostCfg(nqn, full)},
			{Config: huntHostCfg(nqn, full), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{ResourceName: "truenas_nvmet_host.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"dhchap_key", "dhchap_ctrl_key"}},
			{Config: huntHostCfg(nqn, full), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{Config: huntHostCfg(nqn+"x", full)},
			// Removing the keys now turns DH-CHAP off (B-024), so this is an
			// update, then a no-op.
			{Config: huntHostCfg(nqn+"x", ``)},
			{Config: huntHostCfg(nqn+"x", ``), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}
