// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package boot_environment_test

// Adversarial bug-hunt tests: create a boot environment cloned from the
// running one, assert an empty re-plan, then import. Terraform destroys the
// clone on cleanup.
//
// activated = true is deliberately not exercised: it changes the next boot,
// and the lab VMs must stay bootable.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// reactivate makes name the next-boot target again. Used to hand the
// activation back to the booted BE so the test clone can be destroyed.
func reactivate(t *testing.T, name string) error {
	t.Helper()
	_, err := acctest.Client().Call(context.Background(), "boot.environment.activate", map[string]any{"id": name})
	if err != nil && strings.Contains(err.Error(), "already activated") {
		return nil
	}
	return err
}

// TestHuntBootEnvironment_activate exercises activated = true: clone the
// booted BE, make the clone the next-boot target, assert the read-back
// re-plans empty, import, then hand activation back to the booted BE so the
// clone can be destroyed. The lab VMs are disposable and the booted BE is
// never touched, so the worst case of a crash mid-test is one extra reboot
// into an identical clone; t.Cleanup re-activates the booted BE regardless.
func TestHuntBootEnvironment_activate(t *testing.T) {
	acctest.DisruptiveCheck(t)
	booted := activeBootEnvironmentName(t)
	t.Cleanup(func() {
		if err := reactivate(t, booted); err != nil {
			t.Errorf("RESTORE: re-activate booted BE %q: %v", booted, err)
		}
	})
	c := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_boot_environment" "test" {
  name      = "tfhuntbeact"
  source    = %q
  keep      = false
  activated = true
}
`, booted)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: c},
			{Config: c, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:            "truenas_boot_environment.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"source"},
			},
			// An activated BE cannot be deleted and activated = false is not
			// expressible (the API has no deactivate), so hand activation back
			// to the booted BE out-of-band. The follow-up plan then wants to
			// re-activate the clone, hence ExpectNonEmptyPlan; destroy follows.
			{
				Config:             c,
				Check:              func(*terraform.State) error { return reactivate(t, booted) },
				ExpectNonEmptyPlan: true,
			},
			// Persist the refreshed activated=false into state: the harness's
			// post-test destroy does not refresh first, and the provider's
			// Delete guard reads activated from state (real `terraform
			// destroy` refreshes by default, so users never see this).
			{RefreshState: true, ExpectNonEmptyPlan: true},
		},
	})
}

// Clone the running BE, keep = true, activated pinned false. Assert the
// read-back re-plans empty, then import. source (the running BE) is required.
func TestHuntBootEnvironment_cloneKeep(t *testing.T) {
	acctest.DisruptiveCheck(t)
	source := activeBootEnvironmentName(t)
	c := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_boot_environment" "test" {
  name      = "tfhuntbe"
  source    = %q
  keep      = true
  activated = false
}
`, source)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: c},
			{Config: c, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:            "truenas_boot_environment.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"source"},
			},
		},
	})
}
