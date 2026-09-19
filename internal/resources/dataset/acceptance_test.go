// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccDataset_basic creates a ZFS dataset, checks its computed
// attributes, updates a mutable field (comments), imports it by name, and
// verifies destruction.
func TestAccDataset_basic(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-ds"))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDatasetDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccDatasetConfig(name, "lz4", "initial comment"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_dataset.test", "name", name),
					resource.TestCheckResourceAttr("truenas_dataset.test", "compression", "lz4"),
					resource.TestCheckResourceAttr("truenas_dataset.test", "comments", "initial comment"),
					resource.TestCheckResourceAttrSet("truenas_dataset.test", "mountpoint"),
					resource.TestCheckResourceAttrSet("truenas_dataset.test", "pool"),
					resource.TestCheckResourceAttrSet("truenas_dataset.test", "id"),
				),
			},
			// Update a mutable field in place (comments); compression left
			// unchanged to keep this a pure in-place-update step.
			{
				Config: acctest.ProviderConfig() + testAccDatasetConfig(name, "lz4", "updated comment"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_dataset.test", "comments", "updated comment"),
				),
			},
			// Import by dataset name.
			{
				ResourceName:      "truenas_dataset.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccDatasetConfig(name, compression, comments string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name        = %q
  compression = %q
  comments    = %q
}
`, name, compression, comments)
}

// datasetSummary is the subset of pool.dataset.query fields this test
// package needs directly (outside of the provider's own resource code).
type datasetSummary struct {
	ID string `json:"id"`
}

func testAccCheckDatasetDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "pool.dataset.query", [][]any{{"id", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking dataset %s: %v", name, err)
		}
		var results []datasetSummary
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing pool.dataset.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("dataset %s still exists", name)
		}
		return nil
	}
}
