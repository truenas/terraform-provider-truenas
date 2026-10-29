// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package boot_environment_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// B1: source is Required + RequiresReplace and never read back; import leaves
// it null, so the identical config plans destroy + re-clone of the imported BE.
func TestAccBootEnvironment_importSource(t *testing.T) {
	acctest.DisruptiveCheck(t)
	src := activeBootEnvironmentName(t)
	name := "tfhuntbe1"
	cfg := acctest.ProviderConfig() + testAccBootEnvironmentConfig(name, src, false)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: acctest.ProviderConfig() + "\nremoved {\n  from = truenas_boot_environment.test\n  lifecycle {\n    destroy = false\n  }\n}\n"},
			{
				Config: cfg + fmt.Sprintf("\nimport {\n  to = truenas_boot_environment.test\n  id = %q\n}\n", name),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					// source is never returned: recorded in place, never a replacement (fix C1).
					acctest.ExpectNoReplace("truenas_boot_environment.test"),
					acctest.ExpectOnlyAttrChanges("truenas_boot_environment.test", "source"),
				}},
			},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}
