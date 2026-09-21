// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_permissions_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccFilesystemPermissions_basic creates a dataset fixture plus a
// fixture user and group (for uid/gid), sets mode 0750 + those uid/gid via
// truenas_filesystem_permissions, stat-verifies the result server-side,
// updates the mode to 0770 in place, imports by path (ignoring the
// apply-time-only recursive/traverse options), then removes ONLY the
// filesystem_permissions resource from the config (destroying it while
// leaving the dataset/user/group fixtures standing) and asserts — this is
// the documented Delete semantic — that the dataset still exists AND its
// mode is STILL 0770: destroy does not revert anything, it only forgets
// Terraform state.
func TestAccFilesystemPermissions_basic(t *testing.T) {
	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-fsperm-ds"))
	path := "/mnt/" + datasetName
	username := acctest.RandName("tf-acc-fsperm-user")
	groupName := acctest.RandName("tf-acc-fsperm-grp")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccFilesystemPermissionsConfig(datasetName, username, groupName, "0750", true /* includeFSPerm */),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_filesystem_permissions.test", "path", path),
					resource.TestCheckResourceAttr("truenas_filesystem_permissions.test", "id", path),
					resource.TestCheckResourceAttr("truenas_filesystem_permissions.test", "mode", "0750"),
					resource.TestCheckResourceAttrPair("truenas_filesystem_permissions.test", "uid", "truenas_user.fixture", "uid"),
					resource.TestCheckResourceAttrPair("truenas_filesystem_permissions.test", "gid", "truenas_group.fixture", "gid"),
					testAccCheckFSStat(path, "0750"),
				),
			},
			// Update mode in place; uid/gid unchanged.
			{
				Config: acctest.ProviderConfig() + testAccFilesystemPermissionsConfig(datasetName, username, groupName, "0770", true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_filesystem_permissions.test", "mode", "0770"),
					testAccCheckFSStat(path, "0770"),
				),
			},
			// Import by path.
			{
				ResourceName:            "truenas_filesystem_permissions.test",
				ImportState:             true,
				ImportStateId:           path,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"recursive", "traverse"},
			},
			// Remove ONLY truenas_filesystem_permissions from the config:
			// this destroys just that resource (Delete = state-only +
			// warning) while the dataset/user/group fixtures remain, so
			// the documented Delete semantic can be checked directly
			// server-side before the framework's final full teardown.
			{
				Config: acctest.ProviderConfig() + testAccFilesystemPermissionsConfig(datasetName, username, groupName, "0770", false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckDatasetStillExists(datasetName),
					testAccCheckFSStat(path, "0770"),
				),
			},
		},
	})
}

func testAccFilesystemPermissionsConfig(datasetName, username, groupName, mode string, includeFSPerm bool) string {
	fixtures := fmt.Sprintf(`
resource "truenas_dataset" "fixture" {
  name = %q
}

resource "truenas_user" "fixture" {
  username          = %q
  full_name         = "TF Acc Fsperm User"
  password          = "Tf-Acc-Test-Passw0rd!"
  password_disabled = false
  home              = "/var/empty"
  shell             = "/usr/bin/bash"
  smb               = false
  group_create      = true
}

resource "truenas_group" "fixture" {
  name = %q
}
`, datasetName, username, groupName)

	if !includeFSPerm {
		return fixtures
	}

	return fixtures + fmt.Sprintf(`
resource "truenas_filesystem_permissions" "test" {
  path = truenas_dataset.fixture.mountpoint
  mode = %q
  uid  = truenas_user.fixture.uid
  gid  = truenas_group.fixture.gid
}
`, mode)
}

// fsStatSummary is the subset of filesystem.stat's response this test
// package needs directly.
type fsStatSummary struct {
	Mode int64 `json:"mode"`
}

// testAccCheckFSStat calls filesystem.stat directly (bypassing Terraform
// state entirely) and asserts the path's permission bits equal wantMode.
func testAccCheckFSStat(path, wantMode string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "filesystem.stat", path)
		if err != nil {
			return fmt.Errorf("error calling filesystem.stat for %s: %v", path, err)
		}
		var stat fsStatSummary
		if err := json.Unmarshal(raw, &stat); err != nil {
			return fmt.Errorf("error parsing filesystem.stat response: %v", err)
		}
		gotMode := fmt.Sprintf("%04o", stat.Mode&0o7777)
		if gotMode != wantMode {
			return fmt.Errorf("filesystem.stat(%s) mode = %s, want %s", path, gotMode, wantMode)
		}
		return nil
	}
}

// testAccCheckDatasetStillExists calls pool.dataset.get_instance directly
// and asserts the dataset fixture was NOT destroyed alongside
// truenas_filesystem_permissions (it wasn't removed from this test step's
// config).
func testAccCheckDatasetStillExists(datasetName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		if _, err := c.Call(context.Background(), "pool.dataset.get_instance", datasetName); err != nil {
			return fmt.Errorf("dataset %s should still exist after truenas_filesystem_permissions destroy, but pool.dataset.get_instance failed: %v", datasetName, err)
		}
		return nil
	}
}
