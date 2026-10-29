// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package group_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

var emptyPlan = resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}

// twoUsers configures users a and b, b created after a (so b has the higher id).
func twoUsers(name string) string {
	return fmt.Sprintf(`
resource "truenas_user" "a" {
  username     = "%[1]s-a"
  full_name    = "a"
  group_create = true
  password     = "Abcdef123!x"
  smb          = false
}
resource "truenas_user" "b" {
  username     = "%[1]s-b"
  full_name    = "b"
  group_create = true
  password     = "Abcdef123!x"
  smb          = false
  depends_on   = [truenas_user.a]
}
`, name)
}

// TestAccGroup_usersOrder: group.users comes back in the server's set
// iteration order. A configured order the server does not keep must still
// apply cleanly and re-plan empty. (B-057)
func TestAccGroup_usersOrder(t *testing.T) {
	name := acctest.RandName("tfaccgord")
	cfg := acctest.ProviderConfig() + twoUsers(name) + fmt.Sprintf(`
resource "truenas_group" "test" {
  name  = "%s-g"
  users = [truenas_user.b.id, truenas_user.a.id]
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, ConfigPlanChecks: emptyPlan},
		},
	})
}

// primaryMemberConfig: a group with users = [] and a user whose PRIMARY group
// is that group, which group.query reports as a member.
func primaryMemberConfig(name string) string {
	return fmt.Sprintf(`
resource "truenas_group" "test" {
  name  = "%[1]s-g"
  users = []
}
resource "truenas_user" "p" {
  username  = "%[1]s-p"
  full_name = "p"
  group     = truenas_group.test.id
  password  = "Abcdef123!x"
  smb       = false
}
`, name)
}

// TestAccGroup_primaryMemberDrift: the primary-group member is not an
// auxiliary member and must not show as drift against users = []. (B-058)
func TestAccGroup_primaryMemberDrift(t *testing.T) {
	cfg := acctest.ProviderConfig() + primaryMemberConfig(acctest.RandName("tfaccgprim"))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, ConfigPlanChecks: emptyPlan},
		},
	})
}

// TestAccGroup_importPrimaryMember: forgetting and re-importing the group from
// TestAccGroup_primaryMemberDrift plans a no-op; import has no configured list
// to filter against, so it drops primary-group members itself. (B-058)
func TestAccGroup_importPrimaryMember(t *testing.T) {
	name := acctest.RandName("tfaccgprim")
	lookup := fmt.Sprintf(`
data "truenas_group" "lookup" {
  name = "%s-g"
}
`, name)
	user := func(group string) string {
		return fmt.Sprintf(`
resource "truenas_user" "p" {
  username  = "%s-p"
  full_name = "p"
  group     = %s
  password  = "Abcdef123!x"
  smb       = false
}
`, name, group)
	}
	grp := fmt.Sprintf(`
resource "truenas_group" "test" {
  name  = "%s-g"
  users = []
}
`, name)
	full := acctest.ProviderConfig() + grp + user("truenas_group.test.id")
	forgotten := acctest.ProviderConfig() + lookup + user("data.truenas_group.lookup.id") + `
removed {
  from = truenas_group.test
  lifecycle {
    destroy = false
  }
}
`
	imported := full + lookup + `
import {
  to = truenas_group.test
  id = tostring(data.truenas_group.lookup.id)
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: full},
			{Config: forgotten},
			{
				Config: imported,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_group.test", plancheck.ResourceActionNoop),
				}},
			},
			{Config: full, ConfigPlanChecks: emptyPlan},
		},
	})
}

// TestAccGroup_smbDefault: smb left unset takes the server default (true)
// instead of being sent as false. (B-108)
func TestAccGroup_smbDefault(t *testing.T) {
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_group" "test" {
  name = "%s-g"
}
`, acctest.RandName("tfaccgsmb"))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, Check: resource.TestCheckResourceAttr("truenas_group.test", "smb", "true")},
			{Config: cfg, ConfigPlanChecks: emptyPlan},
		},
	})
}

// TestAccGroup_updateKeepsUnmanaged: smb and sudo_commands left out of config
// must survive an update to another attribute (users) instead of being
// overwritten with false/[]. (B-037)
func TestAccGroup_updateKeepsUnmanaged(t *testing.T) {
	name := acctest.RandName("tfaccgkeep")
	step1 := acctest.ProviderConfig() + twoUsers(name) + fmt.Sprintf(`
resource "truenas_group" "test" {
  name          = "%s-g"
  smb           = true
  sudo_commands = ["/usr/bin/ls"]
}
`, name)
	step2 := acctest.ProviderConfig() + twoUsers(name) + fmt.Sprintf(`
resource "truenas_group" "test" {
  name  = "%s-g"
  users = [truenas_user.a.id]
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: step1},
			{Config: step2, Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("truenas_group.test", "smb", "true"),
				resource.TestCheckResourceAttr("truenas_group.test", "sudo_commands.#", "1"),
				resource.TestCheckResourceAttr("truenas_group.test", "sudo_commands.0", "/usr/bin/ls"),
			)},
			{Config: step2, ConfigPlanChecks: emptyPlan},
		},
	})
}
