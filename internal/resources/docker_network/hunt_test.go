// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package docker_network_test

// Adversarial bug-hunt test for the DATASOURCE-ONLY truenas_docker_network:
// docker_network has no managed resource (Docker networks are created and
// destroyed by Docker itself, not by this provider — see datasource.go), so
// this reads the datasource and asserts that every computed attribute it
// derives from docker.network.query is populated. Modeled on
// TestAccDockerNetworkDataSource_basic; reuses that file's isDockerConfigured
// self-skip helper.

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestHuntDockerNetwork_datasource(t *testing.T) {
	// INFRA GATE: docker.network.query returns an empty list when Docker is
	// unconfigured (no apps pool assigned, probed live). There is then no
	// network to look up, and this test never configures a pool itself, so
	// it self-skips — exactly as TestAccDockerNetworkDataSource_basic does.
	acctest.PreCheck(t)
	if !isDockerConfigured(t) {
		t.Skip("Docker is unconfigured on the target box (docker.config \"pool\" is null); no Docker networks exist to look up, and this test never configures a pool itself")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// "bridge" is created by the Docker daemon itself whenever
				// Docker runs, independent of installed apps — a stable lookup
				// target that cannot disappear when an app is uninstalled.
				Config: acctest.ProviderConfig() + `
data "truenas_docker_network" "test" {
  name = "bridge"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_docker_network.test", "name", "bridge"),
					resource.TestCheckResourceAttrSet("data.truenas_docker_network.test", "id"),
					resource.TestCheckResourceAttrSet("data.truenas_docker_network.test", "driver"),
					resource.TestCheckResourceAttrSet("data.truenas_docker_network.test", "scope"),
					resource.TestCheckResourceAttrSet("data.truenas_docker_network.test", "short_id"),
					resource.TestCheckResourceAttrSet("data.truenas_docker_network.test", "created"),
					resource.TestCheckResourceAttrSet("data.truenas_docker_network.test", "ipam.driver"),
				),
			},
		},
	})
}
