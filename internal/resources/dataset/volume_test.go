// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccDataset_volumeUpdate: an in-place update of a VOLUME-type
// truenas_dataset used to send the FILESYSTEM-only enums (aclmode/atime,
// carried as "INHERIT" in state although a volume has no such property), and
// pool.dataset.update rejects them for a VOLUME, so no volume could be
// updated (B-018).
func TestAccDataset_volumeUpdate(t *testing.T) {
	name := importTestName("vol")
	cfg := func(c, sync string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name     = %q
  type     = "VOLUME"
  volsize  = 1073741824
  comments = %q
  sync     = %q
}
`, name, c, sync)
	}
	const addr = "truenas_dataset.test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg("a", "INHERIT"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr(addr, "aclmode"),
					resource.TestCheckNoResourceAttr(addr, "atime"),
				),
			},
			{Config: cfg("a", "INHERIT"), ConfigPlanChecks: emptyPlan()},
			{Config: cfg("b", "INHERIT"), Check: resource.TestCheckResourceAttr(addr, "comments", "b")},
			{Config: cfg("b", "ALWAYS"), Check: resource.TestCheckResourceAttr(addr, "sync", "ALWAYS")},
			{Config: cfg("b", "ALWAYS"), ConfigPlanChecks: emptyPlan()},
		},
	})
}
