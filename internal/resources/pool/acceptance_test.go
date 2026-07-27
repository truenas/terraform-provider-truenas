// Copyright (c) iXsystems, Inc.
// SPDX-License-Identifier: MPL-2.0

package pool_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccPoolDataSource_basic reads the acceptance-test pool
// (acctest.TestPool(), default "tank") via the truenas_pool data source and
// checks that the name round-trips and a status is reported. It does not
// create or destroy any pool.
func TestAccPoolDataSource_basic(t *testing.T) {
	poolName := acctest.TestPool()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccPoolDataSourceConfig(poolName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_pool.t", "name", poolName),
					resource.TestCheckResourceAttrSet("data.truenas_pool.t", "status"),
				),
			},
		},
	})
}

func testAccPoolDataSourceConfig(name string) string {
	return fmt.Sprintf(`
data "truenas_pool" "t" {
  name = %q
}
`, name)
}

// TestAccPool_basic would exercise full create/update/import/destroy of a
// truenas_pool resource, but pool.create requires dedicated spare disks that
// are not safe to assume are present (or blank) on any given target box.
// This test documents the intended config shape (matching the schema in
// schema.go: topology.data is a list of {type, disks}; name and topology
// force replacement; autotrim/guid/status/healthy/path/size/free/allocated
// are computed) so it can be run manually against a box with known-blank
// disks by removing the t.Skip call and setting real disk device names.
func TestAccPool_basic(t *testing.T) {
	acctest.PreCheck(t)
	t.Skip("pool creation requires dedicated spare disks; run manually against a box with blank disks")

	name := acctest.RandName("tf-acc-pool")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPoolDestroyed(name),
		Steps: []resource.TestStep{
			{
				// Replace with real blank disk device names before running manually.
				Config: acctest.ProviderConfig() + testAccPoolConfig(name, []string{"REPLACE_ME_DISK1", "REPLACE_ME_DISK2"}, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_pool.test", "name", name),
					resource.TestCheckResourceAttr("truenas_pool.test", "topology.data.0.type", "MIRROR"),
					resource.TestCheckResourceAttrSet("truenas_pool.test", "guid"),
					resource.TestCheckResourceAttrSet("truenas_pool.test", "status"),
					resource.TestCheckResourceAttrSet("truenas_pool.test", "healthy"),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccPoolConfig(name, []string{"REPLACE_ME_DISK1", "REPLACE_ME_DISK2"}, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_pool.test", "autotrim", "true"),
				),
			},
			{
				ResourceName:      "truenas_pool.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccPoolConfig(name string, disks []string, autotrim bool) string {
	quoted := make([]string, len(disks))
	for i, d := range disks {
		quoted[i] = fmt.Sprintf("%q", d)
	}
	return fmt.Sprintf(`
resource "truenas_pool" "test" {
  name = %q
  topology = {
    data = [{
      type  = "MIRROR"
      disks = [%s]
    }]
  }
  autotrim = %v
}
`, name, strings.Join(quoted, ", "), autotrim)
}

// poolSummary is the subset of pool.query fields this test package needs
// directly (outside of the provider's own resource code).
type poolSummary struct {
	ID int64 `json:"id"`
}

func testAccCheckPoolDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "pool.query", [][]any{{"name", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking pool %s: %v", name, err)
		}
		var results []poolSummary
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing pool.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("pool %s still exists", name)
		}
		return nil
	}
}
