// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package privilege_test

// Adversarial bug-hunt test: set every API-accepted privilege attribute that
// is valid on a box without a joined directory service (name, a local group
// via local_groups, multiple roles, web_shell on), apply, assert an empty
// re-plan, then import. ds_groups is intentionally omitted: it references
// directory-service group GIDs that do not exist on a standalone lab box, so
// setting it would be rejected rather than exercise a round-trip.

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestHuntPrivilege_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)

	groupName := acctest.RandName("tfhuntprivgrp")
	privName := acctest.RandName("tfhuntpriv")

	cfg := acctest.ProviderConfig() + testHuntPrivilegeAllConfig(groupName, privName)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckPrivilegeDestroyed(privName),
			testAccCheckGroupDestroyed(groupName),
		),
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:      "truenas_privilege.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testHuntPrivilegeAllConfig(groupName, privName string) string {
	return fmt.Sprintf(`
resource "truenas_group" "fixture" {
  name = %q
  smb  = false
}

resource "truenas_privilege" "test" {
  name         = %q
  local_groups = [truenas_group.fixture.gid]
  roles        = ["READONLY_ADMIN", "SHARING_READ"]
  web_shell    = true
}
`, groupName, privName)
}
