// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package filesystem_permissions_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func permDS() string {
	return fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfacc-st3-perm"))
}

func permCfg(ds, body string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "fx" {
  name    = %q
  acltype = "posix"
}
resource "truenas_filesystem_permissions" "test" {
  path = truenas_dataset.fx.mountpoint
  %s
}
`, ds, body)
}

func emptyPlan() resource.ConfigPlanChecks {
	return resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
}

// mode = "755" equals the "0755" read back; a spelling-only change plans
// nothing. (B-091)
func TestAccFilesystemPermissions_threeDigitMode(t *testing.T) {
	ds := permDS()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: permCfg(ds, `mode = "755"`), Check: resource.TestCheckResourceAttr("truenas_filesystem_permissions.test", "mode", "755")},
			{Config: permCfg(ds, `mode = "755"`), ConfigPlanChecks: emptyPlan()},
			{Config: permCfg(ds, `mode = "0755"`), ConfigPlanChecks: emptyPlan()},
		},
	})
}

// uid/gid = -1 ("leave unchanged" to setperm, read back as the real id) is
// rejected at plan time. (B-091)
func TestAccFilesystemPermissions_uidMinusOneRejected(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: permCfg(permDS(), "mode = \"0750\"\n  uid = -1"), PlanOnly: true, ExpectError: regexp.MustCompile(`uid`)},
			{Config: permCfg(permDS(), "mode = \"0750\"\n  gid = -1"), PlanOnly: true, ExpectError: regexp.MustCompile(`gid`)},
		},
	})
}

// recursive/traverse are write-only: an import against a configuration that
// sets them plans no change (it used to plan an update that re-ran a recursive
// setperm), and the recursive flag still reaches setperm. (B-115)
func TestAccFilesystemPermissions_recursiveWriteOnly(t *testing.T) {
	ds := permDS()
	body := "mode      = \"0750\"\n  uid       = 0\n  gid       = 0\n  recursive = true\n  traverse  = true"
	cfg := permCfg(ds, body)
	sub := "/mnt/" + ds + "/sub"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: permCfg(ds, "mode = \"0755\"")},
			{
				PreConfig: func() {
					if _, err := acctest.Client().Call(t.Context(), "filesystem.mkdir", map[string]any{"path": sub}); err != nil {
						t.Fatalf("mkdir %s: %v", sub, err)
					}
				},
				Config: cfg,
				Check: func(s *terraform.State) error {
					raw, err := acctest.Client().CallRead(t.Context(), "filesystem.stat", sub)
					if err != nil {
						return err
					}
					if !regexp.MustCompile(`"mode":\s*16872\b`).Match(raw) {
						return fmt.Errorf("recursive setperm did not reach %s (want mode 0750): %s", sub, raw)
					}
					return resource.TestCheckNoResourceAttr("truenas_filesystem_permissions.test", "recursive")(s)
				},
			},
			{Config: acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "fx" {
  name    = %q
  acltype = "posix"
}
removed {
  from = truenas_filesystem_permissions.test
  lifecycle {
    destroy = false
  }
}
`, ds)},
			{
				Config: cfg + `
import {
  to = truenas_filesystem_permissions.test
  id = truenas_dataset.fx.mountpoint
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_filesystem_permissions.test", plancheck.ResourceActionNoop),
				}},
			},
		},
	})
}
