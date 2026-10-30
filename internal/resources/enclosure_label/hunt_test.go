// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package enclosure_label_test

// Adversarial bug-hunt test: set the enclosure's one user-settable field
// ("label") to a fresh non-default value, assert the read-back re-plans
// empty, then import and verify. The enclosure's original label is captured
// before the run and restored both by the resource's own restore-on-destroy
// contract and by a t.Cleanup safety net. Reuses acceptance_test.go's
// firstEnclosureID, readEnclosureLabel, restoreEnclosureLabel helpers and
// testAccEnclosureLabelConfig builder.
//
// Fields: "id" selects which existing enclosure to manage (RequiresReplace,
// not an adversarial value), "name" is Computed/read-back. "label" is the
// only API-accepted writable attribute, so it is the whole value round trip.

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestHuntEnclosureLabel_allFields(t *testing.T) {
	// INFRA GATE: enclosure_label mutates real enclosure hardware, which
	// needs real/virtual enclosure hardware or an Enterprise HA license.
	// DisruptiveCheck requires TRUENAS_DISRUPTIVE=1 (this is a write test
	// that mutates box state — house rule for hunt tests); HACheck requires
	// TRUENAS_HA=1, the endpoint guard, and a live failover.licensed probe;
	// firstEnclosureID then self-skips if the box reports no enclosures.
	acctest.DisruptiveCheck(t)
	acctest.HACheck(t)

	id := firstEnclosureID(t)
	orig := readEnclosureLabel(t, id)
	// Safety net: restore the original label if the Terraform run (or the
	// resource's own restore-on-destroy) fails to complete.
	t.Cleanup(func() { restoreEnclosureLabel(t, id, orig) })

	newLabel := acctest.RandName("tfhunt-enclosure")
	cfg := acctest.ProviderConfig() + testAccEnclosureLabelConfig(id, newLabel)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_enclosure_label.test", "id", id),
					resource.TestCheckResourceAttr("truenas_enclosure_label.test", "label", newLabel),
					resource.TestCheckResourceAttrSet("truenas_enclosure_label.test", "name"),
				),
			},
			// Adversarial re-plan: the value we set must read back unchanged.
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:      "truenas_enclosure_label.test",
				ImportState:       true,
				ImportStateId:     id,
				ImportStateVerify: true,
			},
		},
	})

	// Independent live confirmation that destroy restored the original label.
	if got := readEnclosureLabel(t, id); got != orig {
		t.Errorf("after Terraform destroy, enclosure %q label = %q, want restored original %q", id, got, orig)
	}
}
