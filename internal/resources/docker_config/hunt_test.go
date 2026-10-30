// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package docker_config_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// All configs below are spellings of .70's original docker.config
// (pools 172.16.0.0/12 /24 + fdd0::/48 /64, cidr_v6 fdd0::/64, no mirrors),
// so the server-side value does not change (no docker restart), except the
// mirror test, which TestHuntDockerConfig_restore undoes.

// TestHuntDockerConfig_poolHostBits, _cidrV6Spelling and _mirrorURL were
// promoted to spelling_test.go (TestAccDockerConfig_*).

func TestHuntDockerConfig_restore(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: acctest.ProviderConfig() + `
resource "truenas_docker_config" "test" {
  enable_image_updates = true
  address_pools = [
    { base = "172.16.0.0/12", size = 24 },
    { base = "fdd0::/48", size = 64 },
  ]
  cidr_v6          = "fdd0::/64"
  registry_mirrors = []
}
`},
		},
	})
}
