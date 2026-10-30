// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container_image_test

// Adversarial bug-hunt test for the DATASOURCE-ONLY truenas_container_image:
// container_image has no managed resource (the upstream LXC image registry
// is a pure lookup — see datasource.go), so this reads the datasource and
// asserts that every computed attribute it derives from
// container.image.query_registry is populated. Modeled on
// TestAccContainerImageDataSource_basic.

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestHuntContainerImage_datasource(t *testing.T) {
	// INFRA GATE: the container namespace (and thus
	// container.image.query_registry) does not exist before TrueNAS 26.0
	// (confirmed live via core.get_methods on 25.10), so self-skip cleanly on
	// any older box, mirroring TestAccContainerImageDataSource_basic.
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("truenas_container_image requires TrueNAS 26.0 or later (container namespace absent on 25.10, confirmed live)")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_container_image" "alpine" {
  name = "alpine:3.22:amd64:default"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_container_image.alpine", "id", "alpine:3.22:amd64:default"),
					resource.TestCheckResourceAttr("data.truenas_container_image.alpine", "name", "alpine:3.22:amd64:default"),
					resource.TestCheckResourceAttrSet("data.truenas_container_image.alpine", "latest_version"),
					resource.TestCheckResourceAttrSet("data.truenas_container_image.alpine", "versions.#"),
					// The registry lists at least one build for this image; the
					// first element must be populated (oldest-to-newest order).
					resource.TestCheckResourceAttrSet("data.truenas_container_image.alpine", "versions.0"),
				),
			},
		},
	})
}
