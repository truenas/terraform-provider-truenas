// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloudsync_credentials_test

// Adversarial bug-hunt test: set every API-accepted attribute of
// truenas_cloudsync_credentials to a non-default value, assert the read-back
// re-plans empty, then import. Credentials are stored, not validated against
// a real endpoint at create time, so dummy provider_config values are
// lab-safe. The secret is supplied through the write-only
// provider_secrets_wo overlay, so it never reaches state.

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestHuntCloudsyncCredentials_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)

	name := acctest.RandName("tfhunt-creds")

	// name, provider_config (type + access_key_id), the write-only
	// provider_secrets_wo overlay, and its version.
	cfg := acctest.ProviderConfig() + testAccCloudsyncCredentialsWOConfig(name, "tfhunt-secret-1", 1)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCloudsyncCredentialsDestroyed(name),
		Steps: []resource.TestStep{
			{Config: cfg},
			{
				Config:           cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				ResourceName:            "truenas_cloudsync_credentials.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"provider_config", "provider_secrets_wo", "provider_secrets_wo_version"},
			},
		},
	})
}
