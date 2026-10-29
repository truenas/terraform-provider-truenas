// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_initiator_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func initCfg(body string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_iscsi_initiator" "test" {
%s
}
`, body)
}

// checkServerInitiators asserts the server-side initiator list.
func checkServerInitiators(t *testing.T, want ...string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := strconv.ParseInt(s.RootModule().Resources["truenas_iscsi_initiator.test"].Primary.ID, 10, 64)
		if err != nil {
			return err
		}
		inst := acctest.GetInstance(t, "iscsi.initiator", id)
		got, _ := inst["initiators"].([]any)
		if len(got) != len(want) {
			return fmt.Errorf("server initiators = %v, want %v", inst["initiators"], want)
		}
		for i := range want {
			if got[i] != want[i] {
				return fmt.Errorf("server initiators = %v, want %v", inst["initiators"], want)
			}
		}
		return nil
	}
}

// TestAccISCSIInitiator_commentOnlyUpdateKeepsAllowlist: initiators is
// Optional+Computed. A config that stops managing it and changes only the
// comment used to send initiators = [], which opens the group to every
// initiator. The allowlist must be kept; only an explicit [] clears it.
func TestAccISCSIInitiator_commentOnlyUpdateKeepsAllowlist(t *testing.T) {
	c := acctest.RandName("tfaccinit")
	iqn := "iqn.2005-10.org.example:a"
	empty := resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckISCSIInitiatorDestroyed(),
		Steps: []resource.TestStep{
			{Config: initCfg(fmt.Sprintf("  comment = %q\n  initiators = [%q]", c, iqn))},
			{
				Config: initCfg(fmt.Sprintf("  comment = %q", c+"2")),
				Check: resource.ComposeTestCheckFunc(
					checkServerInitiators(t, iqn),
					resource.TestCheckResourceAttr("truenas_iscsi_initiator.test", "initiators.#", "1"),
					resource.TestCheckResourceAttr("truenas_iscsi_initiator.test", "comment", c+"2"),
				),
			},
			{Config: initCfg(fmt.Sprintf("  comment = %q", c+"2")), ConfigPlanChecks: empty},
			// The reverse: manage only initiators; the comment is kept.
			{
				Config: initCfg(fmt.Sprintf("  initiators = [%q, %q]", iqn, iqn+"b")),
				Check: resource.ComposeTestCheckFunc(
					checkServerInitiators(t, iqn, iqn+"b"),
					resource.TestCheckResourceAttr("truenas_iscsi_initiator.test", "comment", c+"2"),
				),
			},
			// An explicit empty list still opens the group.
			{
				Config: initCfg("  initiators = []"),
				Check:  checkServerInitiators(t),
			},
			{Config: initCfg("  initiators = []"), ConfigPlanChecks: empty},
		},
	})
}
