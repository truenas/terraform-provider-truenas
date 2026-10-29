// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_auth_test

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// mutualTag is the CHAP group tag for this test, distinct from
// testAccISCSIAuthTag and the end-to-end test's 999.
const mutualTag = 9311

func authCfg(user, body string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_iscsi_auth" "test" {
  tag    = %d
  user   = %q
  secret = "tfaccsecret01"
%s
}
`, mutualTag, user, body)
}

// checkServerAuth asserts a field of the auth entry on the server.
func checkServerAuth(t *testing.T, field string, want any) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, err := strconv.ParseInt(s.RootModule().Resources["truenas_iscsi_auth.test"].Primary.ID, 10, 64)
		if err != nil {
			return err
		}
		if got := acctest.GetInstance(t, "iscsi.auth", id)[field]; got != want {
			return fmt.Errorf("server %s = %v, want %v", field, got, want)
		}
		return nil
	}
}

// TestAccISCSIAuth_disableMutualChap configures mutual CHAP, then removes
// peeruser/peersecret from config to go back to one-way CHAP. peeruser was
// pinned with UseStateForUnknown and peersecret was never cleared, so the
// old peer credentials stayed active (B-104). Removing them now clears both
// on the server.
func TestAccISCSIAuth_disableMutualChap(t *testing.T) {
	user := acctest.RandName("tfaccauthmut")
	oneWay := `  discovery_auth = "CHAP"`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: authCfg(user, `
  peeruser       = "tfaccpeer"
  peersecret     = "tfaccpeersec1"
  discovery_auth = "CHAP_MUTUAL"
`),
				Check: checkServerAuth(t, "peeruser", "tfaccpeer"),
			},
			{
				Config: authCfg(user, oneWay),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_iscsi_auth.test", "peeruser", ""),
					checkServerAuth(t, "peeruser", ""),
					checkServerAuth(t, "peersecret", ""),
					checkServerAuth(t, "discovery_auth", "CHAP"),
				),
			},
			{Config: authCfg(user, oneWay), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{ResourceName: "truenas_iscsi_auth.test", ImportState: true, ImportStateVerify: true, ImportStateVerifyIgnore: []string{"secret", "peersecret"}},
		},
	})
}
