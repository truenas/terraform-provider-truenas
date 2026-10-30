// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_permissions_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntFilesystemPermissions_allFields is an adversarial value round-trip: it
// sets every writable attribute of truenas_filesystem_permissions to a
// non-default value (mode, uid, gid from fixture user/group, recursive, traverse)
// and proves each round-trips by (1) applying, (2) re-planning the identical
// config and asserting an empty plan, and (3) importing by path with
// ImportStateVerify.
//
// It reuses the acceptance config builder (testAccFilesystemPermissionsConfig),
// which creates a truenas_dataset fixture plus a fixture user and group and sets
// the permissions resource on the dataset's mountpoint with recursive = true and
// traverse = true. recursive and traverse are write-only (never stored in state),
// so — as in the acceptance import step — they need no ImportStateVerify ignore.
func TestHuntFilesystemPermissions_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)
	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfhunt-fsperm-ds"))
	path := "/mnt/" + datasetName
	username := acctest.RandName("tfhunt-fsperm-user")
	groupName := acctest.RandName("tfhunt-fsperm-grp")
	cfg := acctest.ProviderConfig() + testAccFilesystemPermissionsConfig(datasetName, username, groupName, "0775", true)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_filesystem_permissions.test", "path", path),
					resource.TestCheckResourceAttr("truenas_filesystem_permissions.test", "id", path),
					resource.TestCheckResourceAttr("truenas_filesystem_permissions.test", "mode", "0775"),
					resource.TestCheckResourceAttrPair("truenas_filesystem_permissions.test", "uid", "truenas_user.fixture", "uid"),
					resource.TestCheckResourceAttrPair("truenas_filesystem_permissions.test", "gid", "truenas_group.fixture", "gid"),
					testAccCheckFSStat(path, "0775"),
				),
			},
			{
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:      "truenas_filesystem_permissions.test",
				ImportState:       true,
				ImportStateId:     path,
				ImportStateVerify: true,
			},
		},
	})
}
