// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_permissions_test

// Issue #67 class: with recursive = true, a mode-only change re-owned every
// file under the path to the root's owner, and an owner-only change gave
// every file the directory's mode, because unchanged values carried from
// state were sent back to filesystem.setperm. Only what changes is sent now.
// The children are planted and read back over ssh.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestAccFilesystemPermissions_recursiveChangesOnlyWhatChanged(t *testing.T) {
	acctest.PreCheck(t)
	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-fsperm-rec"))
	path := "/mnt/" + datasetName
	child := path + "/sub/child.txt"
	cfg := func(mode, ownerLines string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "fixture" {
  name          = %q
  acltype       = "posix"
  force_destroy = true # the test plants files
}

resource "truenas_filesystem_permissions" "test" {
  path = truenas_dataset.fixture.mountpoint
  mode = %q
%s
  recursive = true
}
`, datasetName, mode, ownerLines)
	}
	stat := func(p string) string { return strings.TrimSpace(acctest.SSH(t, "stat -c '%u:%g %a' "+p)) }
	// plant: child owned by bin:bin (2:2), mode 640; root is root:root.
	plant := func() {
		acctest.SSH(t, fmt.Sprintf("mkdir -p %[1]s/sub && echo x > %[2]s && chown 2:2 %[1]s/sub %[2]s && chmod 640 %[2]s", path, child))
	}
	expect := func(when, want string) resource.TestCheckFunc {
		return func(*terraform.State) error {
			if got := stat(child); got != want {
				return fmt.Errorf("%s: child is %q, want %q", when, got, want)
			}
			return nil
		}
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg("0755", ""), Check: func(*terraform.State) error { plant(); return nil }},
			// Mode-only change: the mode reaches the child, its owner stays.
			// (Before the fix: 0:0 750.)
			{
				Config: cfg("0750", ""),
				Check: resource.ComposeTestCheckFunc(
					expect("after a recursive mode change with uid/gid omitted", "2:2 750"),
					func(*terraform.State) error { plant(); return nil }, // back to 2:2 640
				),
			},
			// Owner-only change: uid reaches the child; its mode and gid stay.
			// (Before the fix: 1:0 750.)
			{
				Config: cfg("0750", "  uid = 1"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_filesystem_permissions.test", "uid", "1"),
					expect("after a recursive uid change with mode unchanged", "1:2 640"),
				),
			},
			{Config: cfg("0750", "  uid = 1"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}
