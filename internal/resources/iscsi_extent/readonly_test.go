// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_extent_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// checkZvolReadonly asserts the zvol's ZFS readonly value and whether it is
// set locally (TrueNAS sets it locally for every DISK extent).
func checkZvolReadonly(t *testing.T, zvol, value string, local bool) resource.TestCheckFunc {
	return func(*terraform.State) error {
		inst := acctest.GetInstance(t, "pool.dataset", zvol)
		got, _ := acctest.ServerPath(inst, "readonly.value")
		src, _ := acctest.ServerPath(inst, "readonly.source")
		if got != value || (src == "LOCAL") != local {
			return fmt.Errorf("zvol %s readonly = %v (source %v), want %s (local=%v)", zvol, got, src, value, local)
		}
		return nil
	}
}

// TestAccISCSIExtent_roResetOnDestroy covers B-040. ro = true makes TrueNAS
// set readonly=on on the backing zvol, and the server's extent delete does
// not undo it, so the zvol stayed read-only after the extent was gone. The
// provider now resets readonly to inherit when it destroys an extent with
// ro = true.
func TestAccISCSIExtent_roResetOnDestroy(t *testing.T) {
	base := acctest.RandName("tfaccextro")
	zvol := fmt.Sprintf("%s/%s", acctest.TestPool(), base)
	zv := fmt.Sprintf(`
resource "truenas_zvol" "a" {
  name    = %q
  volsize = 67108864
}
`, zvol)
	withExtent := acctest.ProviderConfig() + zv + fmt.Sprintf(`
resource "truenas_iscsi_extent" "test" {
  name = %q
  type = "DISK"
  disk = "zvol/${truenas_zvol.a.name}"
  ro   = true
}
`, base)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: withExtent, Check: checkZvolReadonly(t, zvol, "ON", true)},
			{Config: withExtent, ConfigPlanChecks: emptyPlan()},
			// Destroy the extent, keep the zvol.
			{Config: acctest.ProviderConfig() + zv, Check: checkZvolReadonly(t, zvol, "OFF", false)},
			{Config: acctest.ProviderConfig() + zv, ConfigPlanChecks: emptyPlan()},
		},
	})
}
