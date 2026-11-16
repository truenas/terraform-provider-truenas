// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_acl_test

// Issue #67: with recursive = true and uid/gid omitted, an ACL change re-owned
// every file under the path to the root's owner. The plan carries the root's
// current owner from state and the provider sent it back; filesystem.setacl
// then chowns the whole tree. The owner is now sent only when it changes.
// The children are planted and read back over ssh (their owner is not
// reachable through any resource).

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestAccFilesystemAcl_recursiveKeepsChildOwners(t *testing.T) {
	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-facl-own"))
	path := "/mnt/" + datasetName
	child := path + "/sub/child.txt"
	cfg := func(groupPerm, ownerLines string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_filesystem_acl" "test" {
  path = %q
  entries = jsonencode([
    { tag = "owner@", type = "ALLOW", perms = { BASIC = "FULL_CONTROL" }, flags = { BASIC = "INHERIT" } },
    { tag = "group@", type = "ALLOW", perms = { BASIC = %q }, flags = { BASIC = "INHERIT" } },
  ])
%s
  recursive = true
}
`, path, groupPerm, ownerLines)
	}
	owner := func(p string) string { return strings.TrimSpace(acctest.SSH(t, "stat -c %u:%g "+p)) }
	// plant gives sub/ and the file an owner (bin:bin, 2:2) different from the
	// root's (root:root).
	plant := func() {
		acctest.SSH(t, fmt.Sprintf("mkdir -p %[1]s/sub && echo x > %[2]s && chown 2:2 %[1]s/sub %[2]s", path, child))
	}
	expectOwners := func(when, want string) resource.TestCheckFunc {
		return func(*terraform.State) error {
			for _, p := range []string{path + "/sub", child} {
				if got := owner(p); got != want {
					return fmt.Errorf("%s: %s is owned by %s, want %s", when, p, got, want)
				}
			}
			return nil
		}
	}

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			acctest.PreCheck(t)
			c := acctest.Client()
			if _, err := c.Call(context.Background(), "pool.dataset.create", map[string]any{
				"name": datasetName, "acltype": "NFSV4", "aclmode": "PASSTHROUGH",
			}); err != nil {
				t.Fatalf("fixture dataset create failed: %v", err)
			}
			t.Cleanup(func() {
				if _, err := c.Call(context.Background(), "pool.dataset.delete", datasetName, map[string]any{"recursive": true}); err != nil {
					t.Logf("fixture dataset cleanup failed (%s): %v", datasetName, err)
				}
			})
		},
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg("MODIFY", ""), Check: func(*terraform.State) error { plant(); return nil }},
			// ACL-only change, owner omitted: the ACL reaches the children,
			// their owner does not change. (Before the fix: 0:0.)
			{
				Config: cfg("READ", ""),
				Check: resource.ComposeTestCheckFunc(
					expectOwners("after a recursive ACL change with uid/gid omitted", "2:2"),
					testAccCheckACL(path+"/sub", 2, "group@", "READ"),
				),
			},
			// A configured owner change is applied recursively, and only the
			// configured half: gid stays.
			{
				Config: cfg("READ", "  uid = 1"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_filesystem_acl.test", "uid", "1"),
					expectOwners("after uid = 1 recursive", "1:2"),
					func(*terraform.State) error { plant(); return nil },
				),
			},
			// A later ACL-only change with the owner configured but unchanged
			// does not re-own children either.
			{
				Config: cfg("MODIFY", "  uid = 1"),
				Check: resource.ComposeTestCheckFunc(
					expectOwners("after an ACL change with an unchanged configured uid", "2:2"),
					testAccCheckACL(path+"/sub", 2, "group@", "MODIFY"),
				),
			},
			{Config: cfg("MODIFY", "  uid = 1"), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}
