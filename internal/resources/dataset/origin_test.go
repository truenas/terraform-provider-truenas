// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset_test

// origin: a dataset cloned from a snapshot (pool.snapshot.clone). The golden
// holds a file with known bytes before the snapshot; the clone must contain
// it, carry its own properties, read its origin back on import and plan
// empty afterwards.

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestAccDataset_originClone(t *testing.T) {
	acctest.PreCheck(t)
	golden := acctest.TestPool() + "/" + acctest.RandName("tfaccgoldds")
	clone := acctest.TestPool() + "/" + acctest.RandName("tfacccloneds")
	content := "golden file " + acctest.RandName("payload") + "\n"
	goldenCfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "golden" {
  name          = %q
  force_destroy = true # holds the file and the snapshot
}
`, golden)
	snapCfg := goldenCfg + `
resource "truenas_snapshot" "g" {
  dataset = truenas_dataset.golden.name
  name    = "golden"
}
`
	cloneCfg := snapCfg + fmt.Sprintf(`
resource "truenas_dataset" "clone" {
  name          = %q
  origin        = truenas_snapshot.g.id
  comments      = "cloned from golden"
  compression   = "zstd"
  force_destroy = true # holds the file
}
`, clone)
	c := acctest.Client()
	ctx := context.Background()
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: goldenCfg, Check: func(*terraform.State) error {
				_, err := c.Call(ctx, "filesystem.file_receive", "/mnt/"+golden+"/keep.txt",
					base64.StdEncoding.EncodeToString([]byte(content)), map[string]any{"mode": 0o644})
				return err
			}},
			{Config: snapCfg},
			{
				Config: cloneCfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_dataset.clone", plancheck.ResourceActionCreate),
				}},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_dataset.clone", "origin", golden+"@golden"),
					resource.TestCheckResourceAttr("truenas_dataset.clone", "comments", "cloned from golden"),
					resource.TestCheckResourceAttr("truenas_dataset.clone", "compression", "zstd"),
					resource.TestCheckResourceAttr("truenas_dataset.clone", "mountpoint", "/mnt/"+clone),
					resource.TestCheckNoResourceAttr("truenas_dataset.golden", "origin"),
					func(*terraform.State) error {
						if got := acctest.SSH(t, "cat /mnt/"+clone+"/keep.txt"); got != content {
							return fmt.Errorf("clone file content %q, want the golden's %q", got, content)
						}
						return nil
					},
				),
			},
			{Config: cloneCfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{Config: cloneCfg, ResourceName: "truenas_dataset.clone", ImportState: true, ImportStateId: clone,
				ImportStateVerify: true, ImportStateVerifyIgnore: []string{"force_destroy"}},
			// Changing origin replaces the clone (create-only).
			{
				Config: snapCfg + fmt.Sprintf(`
resource "truenas_snapshot" "g2" {
  dataset = truenas_dataset.golden.name
  name    = "golden2"
}
resource "truenas_dataset" "clone" {
  name          = %q
  origin        = truenas_snapshot.g2.id
  comments      = "cloned from golden"
  compression   = "zstd"
  force_destroy = true
}
`, clone),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_dataset.clone", plancheck.ResourceActionReplace),
				}},
				Check: resource.TestCheckResourceAttr("truenas_dataset.clone", "origin", golden+"@golden2"),
			},
		},
	})
}

func TestAccDataset_originRejectsEncryption(t *testing.T) {
	acctest.PreCheck(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "c" {
  name                    = %q
  origin                  = "%s/nothing@snap"
  encryption              = true
  encryption_generate_key = true
}
`, acctest.TestPool()+"/"+acctest.RandName("tfaccoenc"), acctest.TestPool()),
			PlanOnly:    true,
			ExpectError: regexp.MustCompile(`origin cannot be combined with encryption settings`),
		}},
	})
}
