// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package alert_service_test

// Adversarial bug-hunt test: set every API-accepted attribute of
// truenas_alert_service to a non-default value, assert the read-back
// re-plans empty, then import. The service is a disabled Slack target whose
// webhook url is supplied only through the write-only attributes_secrets_wo
// overlay, so nothing is ever sent and no secret reaches state — lab-safe.

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestHuntAlertService_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)

	name := acctest.RandName("tfhunt-alertsvc")
	const secretURL = "https://hooks.slack.com/services/T000/B000/tfhuntsecret"

	// Reuse the acceptance WO builder: name, level, enabled=false, Slack
	// attributes (type only), and the write-only secret overlay + version.
	cfg := acctest.ProviderConfig() + testAccAlertServiceWOConfig(name, secretURL)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckAlertServiceDestroyed(name),
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				Config:           cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				ResourceName:            "truenas_alert_service.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"attributes", "attributes_secrets_wo", "attributes_secrets_wo_version"},
			},
		},
	})
}
