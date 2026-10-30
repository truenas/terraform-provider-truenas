// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset_test

// Hunt finding F8: renaming a dataset in config is a replacement (Terraform
// does not allow the resource identity — the name — to change on an update),
// and the destroy half used to take the data with it. Destroying a dataset
// that still holds files must now be refused unless force_destroy is set, so
// a rename, a removed resource or a pool move cannot lose data silently.

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

func TestAccDataset_renameRefusedWhileNotEmpty(t *testing.T) {
	acctest.PreCheck(t)
	base := acctest.TestPool() + "/" + acctest.RandName("tfaccren")
	renamed := base + "-renamed"
	cfg := func(name string, force bool) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "d" {
  name          = %q
  comments      = "rename test"
  force_destroy = %t
}
`, name, force)
	}
	c := acctest.Client()
	ctx := context.Background()
	plant := func(*terraform.State) error {
		_, err := c.Call(ctx, "filesystem.file_receive", "/mnt/"+base+"/keep.txt",
			base64.StdEncoding.EncodeToString([]byte("precious")), map[string]any{"mode": 0o644})
		return err
	}
	stillThere := func(*terraform.State) error {
		if _, err := c.Call(ctx, "filesystem.stat", "/mnt/"+base+"/keep.txt"); err != nil {
			return fmt.Errorf("the file did not survive the refused rename: %w", err)
		}
		return nil
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg(base, false), Check: plant},
			// The rename plans a replacement, and the destroy half must refuse
			// because the dataset holds a file — nothing is lost.
			{
				Config: cfg(renamed, false),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_dataset.d", plancheck.ResourceActionReplace),
				}},
				ExpectError: regexp.MustCompile(`not empty`),
			},
			{Config: cfg(base, false), Check: stillThere, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			// A plain destroy is refused the same way.
			{Config: cfg(base, false), Destroy: true, ExpectError: regexp.MustCompile(`not empty`)},
			// force_destroy makes the final destroy succeed.
			{Config: cfg(base, true)},
		},
	})
}
