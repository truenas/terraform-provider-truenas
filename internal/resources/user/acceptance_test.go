// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package user_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccUser_basic creates a local user with a password set, checks its
// attributes, updates full_name and shell in place, imports it by its
// numeric id (ignoring the write-only password), and verifies destruction.
func TestAccUser_basic(t *testing.T) {
	username := acctest.RandName("tf-acc-user")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroyed(username),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccUserConfig(username, "Test User", "/usr/bin/bash"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_user.test", "username", username),
					resource.TestCheckResourceAttr("truenas_user.test", "full_name", "Test User"),
					resource.TestCheckResourceAttr("truenas_user.test", "shell", "/usr/bin/bash"),
					resource.TestCheckResourceAttr("truenas_user.test", "home", "/var/empty"),
					resource.TestCheckResourceAttr("truenas_user.test", "locked", "false"),
					resource.TestCheckResourceAttrSet("truenas_user.test", "id"),
					resource.TestCheckResourceAttrSet("truenas_user.test", "uid"),
				),
			},
			// Update in place: change full_name.
			{
				Config: acctest.ProviderConfig() + testAccUserConfig(username, "Updated Name", "/usr/bin/bash"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_user.test", "full_name", "Updated Name"),
					resource.TestCheckResourceAttr("truenas_user.test", "shell", "/usr/bin/bash"),
				),
			},
			// Import by the user's numeric id. Password is write-only and
			// never returned by the API, so it can't be verified.
			// group_create is read back from the primary group's name.
			{
				ResourceName:            "truenas_user.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password"},
			},
		},
	})
}

// TestAccUser_identityImport creates a local user and re-imports it using
// an import block keyed by resource identity (Terraform 1.12+), rather than
// the legacy `terraform import ID` command.
//
// ImportStateVerify is not supported with plannable import blocks
// (ImportBlockWithID / ImportBlockWithResourceIdentity): terraform-plugin-
// testing v1.16 rejects that combination outright (see
// testStepNewImportState's importStatePreconditions, which returns
// "ImportStateVerify is not supported with plannable import blocks" whenever
// kind.plannable() && step.ImportStateVerify). For a plannable identity
// import the framework instead runs a real `terraform plan` right after the
// import and, unless the step sets ExpectNonEmptyPlan, requires that plan to
// be a no-op; it also automatically compares the pre- and post-import
// resource identity values for equality — that is this kind of step's
// built-in round-trip check.
//
// The post-import plan must be a no-op: group_create is read back from the
// primary group's name, and the other Optional+Computed attributes keep their
// imported values (UseStateForUnknown). (B-051, B-136)
func TestAccUser_identityImport(t *testing.T) {
	username := acctest.RandName("tf-acc-user-ident")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_12_0), // ImportBlockWithResourceIdentity requires Terraform 1.12.0+
		},
		CheckDestroy: testAccCheckUserDestroyed(username),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccUserConfig(username, "Identity Test User", "/usr/bin/bash"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_user.test", "username", username),
					resource.TestCheckResourceAttrSet("truenas_user.test", "id"),
				),
			},
			{
				ResourceName:    "truenas_user.test",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithResourceIdentity,
				ImportPlanChecks: resource.ImportPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("truenas_user.test", plancheck.ResourceActionNoop),
						plancheck.ExpectKnownValue("truenas_user.test", tfjsonpath.New("username"), knownvalue.StringExact(username)),
					},
				},
			},
		},
	})
}

// TestAccUser_list creates a local user, then runs a `terraform query`
// (list resource) step against truenas_user and asserts the query finds at
// least one result. This exercises internal/listing's StreamCollection path
// end to end against a live box, rather than the stubbed unit tests in
// internal/listing/listing_test.go.
func TestAccUser_list(t *testing.T) {
	username := acctest.RandName("tf-acc-user-list")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_14_0), // list/query support requires Terraform 1.14.0+
		},
		CheckDestroy: testAccCheckUserDestroyed(username),
		Steps: []resource.TestStep{
			{
				// Ensure at least one truenas_user exists (the box's own
				// built-in users would satisfy this too, but don't rely on
				// box-specific state).
				Config: acctest.ProviderConfig() + testAccUserConfig(username, "List Test User", "/usr/bin/bash"),
			},
			{
				// The Query step's config must NOT redeclare the "truenas"
				// provider block: WorkingDir.SetQuery only clears stale
				// *.tfquery.hcl/*.json files, not the *.tf file the prior
				// step's SetConfig left behind (see terraform-plugin-testing
				// v1.16's internal/plugintest/working_dir.go SetQuery, which
				// filters on a ".warioform" extension rather than ".tf").
				// That leftover file's `provider "truenas" {}` block is still
				// present on disk when this step runs, so adding
				// acctest.ProviderConfig() here again produces "Duplicate
				// provider configuration ... A default (non-aliased)
				// provider configuration for \"truenas\" was already given".
				Query: true,
				Config: `
list "truenas_user" "test" {
  provider = truenas
  config {}
}
`,
				QueryResultChecks: []querycheck.QueryResultCheck{
					querycheck.ExpectLengthAtLeast("truenas_user.test", 1),
				},
			},
		},
	})
}

func testAccUserConfig(username, fullName, shell string) string {
	return fmt.Sprintf(`
resource "truenas_user" "test" {
  username          = %q
  full_name         = %q
  password          = "Tf-Acc-Test-Passw0rd!"
  password_disabled = false
  home              = "/var/empty"
  shell             = %q
  smb               = false
  group_create      = true

  email = "tfacc@example.com"
  locked = false
}
`, username, fullName, shell)
}

func testAccCheckUserDestroyed(username string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "user.query", [][]any{{"username", "=", username}})
		if err != nil {
			return fmt.Errorf("error checking user %s: %v", username, err)
		}
		var results []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing user.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("user %s still exists", username)
		}
		return nil
	}
}

// TestAccUser_fullSurface exercises the user attributes that need fixtures:
// group/groups (referenced truenas_group resources), ssh_password_enabled and
// home_mode (a real home directory on a dataset + a login shell), and webshare
// (TrueNAS 26.0+, set only when the target supports it).
func TestAccUser_fullSurface(t *testing.T) {
	username := acctest.RandName("tfaccuserfs")
	dsName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-userhome"))
	gPrimary := acctest.RandName("tfaccgrpp")
	gAux := acctest.RandName("tfaccgrpa")
	v26 := acctest.ServerVersionAtLeast(t, 26, 0)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroyed(username),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccUserFullConfig(username, dsName, gPrimary, gAux, v26),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_user.test", "username", username),
					resource.TestCheckResourceAttr("truenas_user.test", "ssh_password_enabled", "true"),
					resource.TestCheckResourceAttr("truenas_user.test", "password_wo_version", "1"),
					resource.TestCheckResourceAttrSet("truenas_user.test", "group"),
					resource.TestCheckResourceAttr("truenas_user.test", "groups.#", "1"),
				),
			},
			{
				ResourceName:            "truenas_user.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password", "password_wo_version", "home_mode"},
			},
		},
	})
}

func testAccUserFullConfig(username, dsName, gPrimary, gAux string, v26 bool) string {
	webshare := ""
	if v26 {
		webshare = "\n  webshare = false"
	}
	return fmt.Sprintf(`
resource "truenas_dataset" "home" {
  name = %q
}

resource "truenas_group" "primary" {
  name = %q
}

resource "truenas_group" "aux" {
  name = %q
}

resource "truenas_user" "test" {
  username             = %q
  full_name            = "Full Surface"
  password             = "Tf-Acc-Test-Passw0rd!"
  password_wo_version  = 1
  password_disabled    = false
  home                 = truenas_dataset.home.mountpoint
  home_mode            = "0700"
  shell                = "/usr/bin/bash"
  smb                  = true
  group                = truenas_group.primary.id
  groups               = [truenas_group.aux.id]
  ssh_password_enabled = true
  email                = "tfacc-full@example.com"
  locked               = false%s
}
`, dsName, gPrimary, gAux, username, webshare)
}

// TestAccUser_updateOmittedGroupsPreserved is the live #43 regression (v1.5.13):
// updating a user while `groups` is unmanaged (omitted from config, so unknown)
// must NOT clear the user's secondary groups. Pre-fix, basePayload coerced the
// unknown list to [] and the update wiped them — which on an imported admin
// stripped builtin_administrators and broke its API key. Step 1 manages one aux
// group; step 2 omits `groups` and changes full_name; the aux group must
// survive on the server.
func TestAccUser_updateOmittedGroupsPreserved(t *testing.T) {
	username := acctest.RandName("tfaccgrpkeep")
	gPrimary := acctest.RandName("tfaccgkp")
	gAux := acctest.RandName("tfaccgka")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroyed(username),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccUserGroupsConfig(username, gPrimary, gAux, "First Name", true),
				Check:  resource.TestCheckResourceAttr("truenas_user.test", "groups.#", "1"),
			},
			{
				// groups omitted (unmanaged) + a real change: aux must survive.
				Config: acctest.ProviderConfig() + testAccUserGroupsConfig(username, gPrimary, gAux, "Second Name", false),
				Check:  testAccCheckUserHasGroup(username, gAux),
			},
		},
	})
}

func testAccUserGroupsConfig(username, gPrimary, gAux, fullName string, withGroups bool) string {
	groupsLine := ""
	if withGroups {
		groupsLine = "\n  groups    = [truenas_group.aux.id]"
	}
	return fmt.Sprintf(`
resource "truenas_group" "primary" { name = %q }
resource "truenas_group" "aux"     { name = %q }

resource "truenas_user" "test" {
  username  = %q
  full_name = %q
  password  = "Tf-Acc-Test-Passw0rd!"
  home      = "/var/empty"
  shell     = "/usr/bin/bash"
  group     = truenas_group.primary.id%s
}
`, gPrimary, gAux, username, fullName, groupsLine)
}

// testAccCheckUserHasGroup verifies the user is still a member of the named
// secondary group on the server (not just in Terraform state).
func testAccCheckUserHasGroup(username, groupName string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		c := acctest.Client()
		ctx := context.Background()
		graw, err := c.Call(ctx, "group.query", [][]any{{"name", "=", groupName}})
		if err != nil {
			return fmt.Errorf("group.query: %w", err)
		}
		var groups []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(graw, &groups); err != nil || len(groups) == 0 {
			return fmt.Errorf("aux group %q not found (%v)", groupName, err)
		}
		auxID := groups[0].ID
		uraw, err := c.Call(ctx, "user.query", [][]any{{"username", "=", username}})
		if err != nil {
			return fmt.Errorf("user.query: %w", err)
		}
		var users []struct {
			Groups []int64 `json:"groups"`
		}
		if err := json.Unmarshal(uraw, &users); err != nil || len(users) == 0 {
			return fmt.Errorf("user %q not found (%v)", username, err)
		}
		for _, g := range users[0].Groups {
			if g == auxID {
				return nil
			}
		}
		return fmt.Errorf("user %q lost its secondary group %d after an update that omitted groups (regression)", username, auxID)
	}
}

// TestAccUser_identityAfterUpdate verifies the int64-id identity pattern is set
// after an in-place update (issue #20): truenas_user's Create/Update key the
// identity on plan.ID.ValueInt64(). Gated to Terraform 1.12+.
func TestAccUser_identityAfterUpdate(t *testing.T) {
	username := acctest.RandName("tfaccuserident")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_12_0)},
		CheckDestroy:             testAccCheckUserDestroyed(username),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccUserConfig(username, "Ident One", "/usr/bin/bash"),
			},
			{
				Config: acctest.ProviderConfig() + testAccUserConfig(username, "Ident Two", "/usr/bin/bash"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectIdentityValueMatchesState("truenas_user.test", tfjsonpath.New("id")),
				},
			},
		},
	})
}

// TestAccUserDataSource_basic reads an existing user through the data source.
// Regression for #23: the data source model carried a write-only home_mode
// field absent from its schema, so every read failed with "Struct defines
// fields not found in object: home_mode". There was no data source test before.
func TestAccUserDataSource_basic(t *testing.T) {
	username := acctest.RandName("tfaccuserds")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckUserDestroyed(username),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccUserConfig(username, "DS Source", "/usr/bin/bash") + `
data "truenas_user" "by_name" {
  username   = truenas_user.test.username
  depends_on = [truenas_user.test]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_user.by_name", "username", username),
					resource.TestCheckResourceAttr("data.truenas_user.by_name", "full_name", "DS Source"),
					resource.TestCheckResourceAttrSet("data.truenas_user.by_name", "id"),
					resource.TestCheckResourceAttrSet("data.truenas_user.by_name", "uid"),
				),
			},
		},
	})
}
