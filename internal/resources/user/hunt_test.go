// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package user_test

// Adversarial bug-hunt test: set every API-accepted user attribute to a
// valid non-default value in one config, apply, assert the read-back
// re-plans empty (the value round-trip), then import. Fixtures (a dataset
// for a real home directory and two groups for primary/secondary
// membership) are created and destroyed by Terraform.

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// huntUserPubKey is a valid, self-contained ed25519 authorized_keys line used
// for the sshpubkey round-trip (generated once; the exact value is immaterial
// so long as it is a well-formed key the server accepts and reads back).
const huntUserPubKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIO1CWF5tER94xKpJAhhDP0lxjR3bLtsKaGvFHj6lEnLq tfhunt"

func TestHuntUser_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)

	username := acctest.RandName("tfhuntuser")
	dsName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfhuntuserhome"))
	gPrimary := acctest.RandName("tfhuntgp")
	gAux := acctest.RandName("tfhuntga")
	v26 := acctest.ServerVersionAtLeast(t, 26, 0)

	cfg := acctest.ProviderConfig() + testHuntUserAllConfig(username, dsName, gPrimary, gAux, v26)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroyed(username),
		Steps: []resource.TestStep{
			{Config: cfg},
			// Value round-trip: the same config must re-plan empty.
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:      "truenas_user.test",
				ImportState:       true,
				ImportStateVerify: true,
				// password is write-only (never in state); password_wo_version
				// is a rotation trigger the API never returns; home_mode is not
				// read back by user.query (see schema.go).
				// On import the API lists every secondary group the user belongs
				// to (incl. ones added out-of-band), in server order; the
				// configured round-trip re-plans empty but raw import differs.
				ImportStateVerifyIgnore: []string{"password", "password_wo_version", "home_mode", "groups"},
			},
		},
	})
}

func testHuntUserAllConfig(username, dsName, gPrimary, gAux string, v26 bool) string {
	webshare := ""
	if v26 {
		webshare = "\n  webshare = true"
	}
	return fmt.Sprintf(`
resource "truenas_dataset" "home" {
  name          = %q
  force_destroy = true # user.create seeds the home with skeleton files
}

resource "truenas_group" "primary" {
  name = %q
}

resource "truenas_group" "aux" {
  name = %q
}

resource "truenas_user" "test" {
  username               = %q
  uid                    = 731337
  full_name              = "Hunt Full Surface"
  email                  = "tfhunt-user@example.com"
  password               = "Tf-Hunt-Passw0rd!"
  password_wo_version    = 1
  password_disabled      = false
  home                   = truenas_dataset.home.mountpoint
  home_mode              = "0750"
  shell                  = "/usr/bin/bash"
  locked                 = false
  smb                    = true
  ssh_password_enabled   = true
  sshpubkey              = %q
  sudo_commands          = ["/usr/bin/id"]
  sudo_commands_nopasswd = ["/usr/bin/whoami"]
  group                  = truenas_group.primary.id
  groups                 = [truenas_group.aux.id]
  group_create           = false%s
}
`, dsName, gPrimary, gAux, username, huntUserPubKey, webshare)
}
