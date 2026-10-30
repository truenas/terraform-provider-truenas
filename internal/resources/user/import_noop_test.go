// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package user_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccUser_importSmbGroups: an smb user that lists explicit groups gets
// builtin_users added by the server. Forgetting the user and importing it again
// with the same configuration must plan a no-op: import drops builtin_users and
// reads group_create back. (B-051)
func TestAccUser_importSmbGroups(t *testing.T) {
	name := acctest.RandName("tfaccuimp")
	grp := fmt.Sprintf(`
resource "truenas_group" "g" {
  name = "%s-g"
}
`, name)
	usr := fmt.Sprintf(`
resource "truenas_user" "test" {
  username     = %q
  full_name    = "Import"
  group_create = true
  password     = "Abcdef123!x"
  smb          = true
  groups       = [truenas_group.g.id]
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps:                    forgetImportNoop("truenas_user.test", name, acctest.ProviderConfig()+grp, usr),
	})
}

// TestAccUser_importSmbGroupsNoGroupCreate: the same with an explicit primary
// group and groups = [], so only the server-added builtin_users can make the
// import plan an update. (B-051)
func TestAccUser_importSmbGroupsNoGroupCreate(t *testing.T) {
	name := acctest.RandName("tfaccuimp")
	grp := fmt.Sprintf(`
resource "truenas_group" "g" {
  name = "%s-g"
}
`, name)
	usr := fmt.Sprintf(`
resource "truenas_user" "test" {
  username  = %q
  full_name = "Import"
  group     = truenas_group.g.id
  password  = "Abcdef123!x"
  smb       = true
  groups    = []
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps:                    forgetImportNoop("truenas_user.test", name, acctest.ProviderConfig()+grp, usr),
	})
}

// TestAccUser_noFullName: user.create requires full_name, so leaving it out is
// a plan-time error rather than a failed apply. (B-052)
func TestAccUser_noFullName(t *testing.T) {
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_user" "test" {
  username     = %q
  group_create = true
  password     = "Abcdef123!x"
  smb          = false
}
`, acctest.RandName("tfaccunofn"))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, PlanOnly: true, ExpectError: regexp.MustCompile(`(?s)full_name.*required`)},
		},
	})
}

// TestAccUser_webshareBelow26: below TrueNAS 26.0 the API has no webshare
// field, so webshare = true is a plan-time error instead of an apply that
// silently drops it and then fails with an inconsistent result. (B-055)
func TestAccUser_webshareBelow26(t *testing.T) {
	acctest.PreCheck(t)
	if acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("needs TrueNAS < 26.0")
	}
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_user" "test" {
  username          = %q
  full_name         = "webshare"
  password_disabled = true
  home              = "/var/empty"
  smb               = false
  group_create      = true
  webshare          = true
}
`, acctest.RandName("tfaccuws"))
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, PlanOnly: true, ExpectError: regexp.MustCompile(`webshare requires TrueNAS 26.0`)},
		},
	})
}

// forgetImportNoop creates base+res, forgets res with a removed block, then
// adopts it again with an import block (id looked up through the data source)
// and requires the import to plan as a no-op, followed by an empty plan.
func forgetImportNoop(addr, username, base, res string) []resource.TestStep {
	removed := fmt.Sprintf(`
removed {
  from = %s
  lifecycle {
    destroy = false
  }
}
`, addr)
	imp := fmt.Sprintf(`
data "truenas_user" "lookup" {
  username = %q
}
import {
  to = %s
  id = tostring(data.truenas_user.lookup.id)
}
`, username, addr)
	return []resource.TestStep{
		{Config: base + res},
		{Config: base + removed},
		{
			Config: base + res + imp,
			ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
				plancheck.ExpectResourceAction(addr, plancheck.ResourceActionNoop),
			}},
		},
		{Config: base + res, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
	}
}

// TestAccUser_sshpubkeyHomeModeSpelling: sshpubkey read with file() ends in
// "\n", which the server strips (B-053), and home_mode = "0700" is the same
// mode the import reads back as "700". Both must apply cleanly, re-plan empty,
// and survive forget + import as a no-op.
func TestAccUser_sshpubkeyHomeModeSpelling(t *testing.T) {
	name := acctest.RandName("tfaccukey")
	base := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "home" {
  name          = "%s/%s"
  force_destroy = true # user.create seeds the home with skeleton files
}
`, acctest.TestPool(), name)
	usr := fmt.Sprintf(`
resource "truenas_user" "test" {
  username     = %q
  full_name    = "Key"
  group_create = true
  password     = "Abcdef123!x"
  smb          = false
  home         = truenas_dataset.home.mountpoint
  home_mode    = "0700"
  sshpubkey    = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIJvH8ZQ0Wd0nZ4V2b3Z0gq3x8X9Y3jv5v4c8x5P1e2Qa tfacc@x\n"
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: append([]resource.TestStep{
			{Config: base + usr, ConfigPlanChecks: resource.ConfigPlanChecks{PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		}, forgetImportNoop("truenas_user.test", name, base, usr)[1:]...),
	})
}
