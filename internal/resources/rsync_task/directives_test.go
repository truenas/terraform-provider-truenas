// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package rsync_task_test

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func rsyncModuleConfig(ds, extra string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "fixture" {
  name = %q
}

resource "truenas_rsync_task" "test" {
  path           = truenas_dataset.fixture.mountpoint
  user           = "root"
  mode           = "MODULE"
  remotehost     = "192.0.2.10"
  remotemodule   = "tffix3"
  enabled        = false
  validate_rpath = false
  ssh_keyscan    = false
  desc           = "tffix3"
  extra          = %s
}
`, ds, extra)
}

// TestAccRsyncTask_extraWhitespaceRejected: TrueNAS joins extra with spaces
// and splits it again, so an entry with whitespace became several arguments
// and the apply failed (B-043). Such entries are now a plan-time error.
func TestAccRsyncTask_extraWhitespaceRejected(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tffix3shtasks-rs"))
	for _, extra := range []string{`["--rsh=ssh -p 2222"]`, `["--checksum", "--exclude=my file"]`, `["--checksum", ""]`} {
		resource.Test(t, resource.TestCase{
			PreCheck:                 func() { acctest.PreCheck(t) },
			ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
			Steps: []resource.TestStep{{
				Config:      rsyncModuleConfig(ds, extra),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`one\s+non-empty\s+argument\s+without\s+whitespace`),
			}},
		})
	}
}

// TestAccRsyncTask_importPlan: validate_rpath and ssh_keyscan are per-apply
// directives the API never returns, so import followed by plan planned an
// update (B-117). They are write-only now.
func TestAccRsyncTask_importPlan(t *testing.T) {
	ds := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tffix3shtasks-rs"))
	cfg := rsyncModuleConfig(ds, `["--checksum", "--exclude=*.tmp"]`)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		CheckDestroy:             testAccCheckRsyncDatasetDestroyed(ds),
		Steps: []resource.TestStep{
			{Config: cfg, Check: resource.TestCheckNoResourceAttr("truenas_rsync_task.test", "validate_rpath")},
			{Config: cfg, ResourceName: "truenas_rsync_task.test", ImportState: true, ImportStateKind: resource.ImportBlockWithID},
		},
	})
}
