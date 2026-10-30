// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package enclosure_test

// Adversarial bug-hunt test for the DATASOURCE-ONLY truenas_enclosure:
// enclosure has no managed resource (enclosures are fixed physical/virtual
// hardware, never created or destroyed via the API — label management lives
// in truenas_enclosure_label), so this reads the datasource and asserts that
// every key computed attribute it derives from enclosure2.query is
// populated. Modeled on TestAccEnclosureDataSource_basic; reuses that file's
// firstEnclosureID helper and testAccEnclosureDataSourceConfig builder.

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestHuntEnclosure_datasource(t *testing.T) {
	// INFRA GATE: enclosures need real enclosure hardware / an Enterprise HA
	// license. acctest.HACheck requires TRUENAS_HA=1, a matching
	// TRUENAS_HA_ALLOWED_ENDPOINT guard, and a live failover.licensed probe;
	// firstEnclosureID then self-skips if the box reports no enclosures. A VM
	// with no enclosure hardware never runs this. Mirrors
	// TestAccEnclosureDataSource_basic exactly.
	acctest.HACheck(t)
	id := firstEnclosureID(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccEnclosureDataSourceConfig(id),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_enclosure.test", "id", id),
					resource.TestCheckResourceAttrSet("data.truenas_enclosure.test", "name"),
					resource.TestCheckResourceAttrSet("data.truenas_enclosure.test", "label"),
					resource.TestCheckResourceAttrSet("data.truenas_enclosure.test", "model"),
					resource.TestCheckResourceAttrSet("data.truenas_enclosure.test", "vendor"),
					resource.TestCheckResourceAttrSet("data.truenas_enclosure.test", "product"),
					resource.TestCheckResourceAttrSet("data.truenas_enclosure.test", "controller"),
					resource.TestCheckResourceAttrSet("data.truenas_enclosure.test", "status.#"),
				),
			},
		},
	})
}
