// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package alert_policy_test

// Adversarial bug-hunt test: drive the singleton truenas_alert_policy
// classes object through a non-default per-class override, assert the
// read-back re-plans empty, then import. The box's existing classes are
// captured up front and restored via restoreAlertPolicy in t.Cleanup, so
// the box is left as it was found even if the Terraform steps fail.

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestHuntAlertPolicy_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)

	origClasses := readAlertPolicyOriginal(t)
	t.Cleanup(func() { restoreAlertPolicy(t, origClasses) })

	// classes is the only settable attribute (id is Computed). Layer a
	// non-default override (CRITICAL/IMMEDIATELY) on top of whatever the box
	// already has, so no other class override is reset to its default.
	testClasses := make(map[string]any, len(origClasses)+1)
	for k, v := range origClasses {
		testClasses[k] = v
	}
	testClasses["UPSBatteryLow"] = map[string]any{"level": "CRITICAL", "policy": "IMMEDIATELY"}

	cfg := acctest.ProviderConfig() + testAccAlertPolicyConfig(classesToHCLString(t, testClasses))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_alert_policy.test", "id", "alert_policy"),
					resource.TestCheckResourceAttr("truenas_alert_policy.test", "classes", classesJSONString(t, testClasses)),
				),
			},
			{
				Config:           cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				ResourceName:      "truenas_alert_policy.test",
				ImportState:       true,
				ImportStateId:     "alert_policy",
				ImportStateVerify: true,
			},
		},
	})
}
