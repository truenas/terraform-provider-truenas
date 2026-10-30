// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package group_test

// Adversarial bug-hunt test: set every API-accepted group attribute to a
// valid non-default value (a fixed gid, smb on, a real member user, and both
// sudo-command lists), apply, assert an empty re-plan, then import. The
// member user fixture is created and destroyed by Terraform.

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestHuntGroup_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)

	name := acctest.RandName("tfhuntgrp")
	username := acctest.RandName("tfhuntgm")

	cfg := acctest.ProviderConfig() + testHuntGroupAllConfig(name, username)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckGroupDestroyed(name),
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:      "truenas_group.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testHuntGroupAllConfig(name, username string) string {
	return fmt.Sprintf(`
resource "truenas_user" "member" {
  username          = %q
  full_name         = "TF Hunt Group Member"
  password          = "Tf-Hunt-Passw0rd!"
  password_disabled = false
  home              = "/var/empty"
  shell             = "/usr/bin/bash"
  smb               = false
  group_create      = true
}

resource "truenas_group" "test" {
  name                   = %q
  gid                    = 731338
  smb                    = true
  users                  = [truenas_user.member.id]
  sudo_commands          = ["/usr/bin/id"]
  sudo_commands_nopasswd = ["/usr/bin/whoami"]
}
`, username, name)
}
