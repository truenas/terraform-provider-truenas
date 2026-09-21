// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container_image_test

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccContainerImageDataSource_basic looks up
// "alpine:3.22:amd64:default" through the truenas_container_image
// datasource only. It never writes. Requires TrueNAS 26.0+ (the
// container namespace does not exist on 25.10, confirmed live via
// core.get_methods), so this test self-skips cleanly on any older box.
func TestAccContainerImageDataSource_basic(t *testing.T) {
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
					resource.TestCheckResourceAttrSet("data.truenas_container_image.alpine", "latest_version"),
					resource.TestCheckResourceAttrSet("data.truenas_container_image.alpine", "versions.#"),
				),
			},
		},
	})
}

// TestAccContainerImageDataSource_notFound verifies a clean error
// diagnostic (not a panic/crash) when the requested image name is absent
// from the registry.
func TestAccContainerImageDataSource_notFound(t *testing.T) {
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("truenas_container_image requires TrueNAS 26.0 or later (container namespace absent on 25.10, confirmed live)")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_container_image" "missing" {
  name = "this-image-does-not-exist:0:amd64:default"
}
`,
				ExpectError: regexp.MustCompile(`(?i)not found`),
			},
		},
	})
}
