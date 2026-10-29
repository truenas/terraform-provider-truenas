// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// olderVersion is the registry build before latest_version.
const olderVersion = "data.truenas_container_image.alpine.versions[length(data.truenas_container_image.alpine.versions)-2]"

// TestAccContainer_imageVersionBump pins the image contract (B-004).
// container.update cannot change a container's image (26.0, 28.0), so a new
// image version still plans a replacement; the documented latest_version
// pattern therefore pairs it with lifecycle { ignore_changes = [image] },
// under which a new registry build plans nothing.
func TestAccContainer_imageVersionBump(t *testing.T) {
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("containers require TrueNAS 26.0+")
	}
	name := "tffixct2"
	addr := "truenas_container.test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckContainerDestroyed(name),
		Steps: []resource.TestStep{
			{Config: acctest.ProviderConfig() + containerConfig(name, olderVersion, ``)},
			{
				Config:             acctest.ProviderConfig() + containerConfig(name, latest, ``),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPreRefresh: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction(addr, plancheck.ResourceActionReplace),
					},
				},
			},
			{
				Config: acctest.ProviderConfig() + containerConfig(name, latest, `
  lifecycle {
    ignore_changes = [image]
  }
`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}
