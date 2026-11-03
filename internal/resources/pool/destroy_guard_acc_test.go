// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package pool_test

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccPool_destroyRefusedWhileHoldingData: a pool holding a dataset that
// Terraform does not manage must not be destroyed by `terraform destroy`
// unless force_destroy = true. Until 1.6.2 the provider ran pool.export with
// destroy=true unconditionally.
func TestAccPool_destroyRefusedWhileHoldingData(t *testing.T) {
	disks := testDisks(t, 2)
	name := acctest.RandName("tfaccpoolfd")
	cfg := func(force bool) string {
		return acctest.ProviderConfig() + testAccPoolConfigForce(name, "MIRROR", disks[:2], force)
	}
	ctx := context.Background()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckPoolDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: cfg(false),
				Check: func(s *terraform.State) error {
					// data the configuration knows nothing about
					_, err := acctest.Client().Call(ctx, "pool.dataset.create", map[string]any{"name": name + "/keep"})
					return err
				},
			},
			// terraform destroy must be refused and name what it found
			{
				Config:      cfg(false),
				Destroy:     true,
				ExpectError: regexp.MustCompile(`Destroy pool refused[\s\S]*` + regexp.QuoteMeta(name+"/keep")),
			},
			// still there
			{
				Config: cfg(false),
				Check: func(*terraform.State) error {
					raw, err := acctest.Client().Call(ctx, "pool.dataset.query", []any{[]any{"id", "=", name + "/keep"}})
					if err != nil {
						return err
					}
					if strings.TrimSpace(string(raw)) == "[]" {
						return fmt.Errorf("%s/keep is gone: the refused destroy still deleted data", name)
					}
					return nil
				},
			},
			// opt in, then the harness destroy at the end takes the pool and its contents
			{Config: cfg(true), Check: resource.TestCheckResourceAttr("truenas_pool.test", "force_destroy", "true")},
		},
	})
}

func testAccPoolConfigForce(name, vdevType string, disks []string, force bool) string {
	quoted := make([]string, len(disks))
	for i, d := range disks {
		quoted[i] = fmt.Sprintf("%q", d)
	}
	return fmt.Sprintf(`
resource "truenas_pool" "test" {
  name = %q
  topology = {
    data = [{
      type  = %q
      disks = [%s]
    }]
  }
  force_destroy = %v
}
`, name, vdevType, strings.Join(quoted, ", "), force)
}
