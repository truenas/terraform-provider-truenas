// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package zvol_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccZvol_basic creates a 64M zvol, checks its attributes, resizes it up
// to 128M in place, imports it by name, and verifies destruction.
func TestAccZvol_basic(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-zv"))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckZvolDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccZvolConfig(name, 67108864, "lz4", "initial comment"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_zvol.test", "name", name),
					resource.TestCheckResourceAttr("truenas_zvol.test", "volsize", "67108864"),
					resource.TestCheckResourceAttr("truenas_zvol.test", "compression", "lz4"),
					resource.TestCheckResourceAttr("truenas_zvol.test", "comments", "initial comment"),
					resource.TestCheckResourceAttrSet("truenas_zvol.test", "pool"),
					resource.TestCheckResourceAttrSet("truenas_zvol.test", "id"),
				),
			},
			// Update in place: resize up 64M -> 128M and change the comment.
			{
				Config: acctest.ProviderConfig() + testAccZvolConfig(name, 134217728, "lz4", "updated comment"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_zvol.test", "volsize", "134217728"),
					resource.TestCheckResourceAttr("truenas_zvol.test", "comments", "updated comment"),
				),
			},
			// Import by zvol name. "sparse" is write-only and never returned
			// by the API, so it can't be verified after import.
			{
				ResourceName:            "truenas_zvol.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"sparse"},
			},
		},
	})
}

func testAccZvolConfig(name string, volsize int64, compression, comments string) string {
	return fmt.Sprintf(`
resource "truenas_zvol" "test" {
  name        = %q
  volsize     = %d
  compression = %q
  comments    = %q
}
`, name, volsize, compression, comments)
}

// zvolQueryResult is the subset of pool.dataset.query fields this test
// package needs to distinguish a VOLUME-type dataset (zvol) from others.
type zvolQueryResult struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

func testAccCheckZvolDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "pool.dataset.query", [][]any{{"id", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking zvol %s: %v", name, err)
		}
		var results []zvolQueryResult
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing pool.dataset.query response: %v", err)
		}
		for _, r := range results {
			if r.Type == "VOLUME" {
				return fmt.Errorf("zvol %s still exists", name)
			}
		}
		return nil
	}
}
