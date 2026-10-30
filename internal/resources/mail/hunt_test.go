// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package mail_test

// Adversarial bug-hunt test: apply every API-accepted value and assert an
// empty re-plan, then import. restoreConfigKeys is defined in restore_test.go.

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntMail_allFields sets every writable mail.config field to a valid
// non-default value, including the write-only pass / pass_wo_version pair,
// asserts the read-back re-plans empty, then imports. smtp stays false so the
// box never attempts an authenticated SMTP connection; the SMTP server is a
// documentation hostname that is only stored, never contacted. The write-only
// password is not in state and pass_wo_version is a local rotation trigger
// never read back, so both are ignored on import.
func TestHuntMail_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)
	// Restore every field the configuration below writes (pass is a secret and
	// is not returned by mail.config, so it is left unchanged on restore).
	restoreConfigKeys(t, "fromemail", "fromname", "outgoingserver", "port", "security", "smtp", "user")

	c := acctest.ProviderConfig() + `
resource "truenas_mail" "test" {
  fromemail       = "hunt@example.com"
  fromname        = "Hunt Mailer"
  outgoingserver  = "smtp.example.com"
  port            = 2525
  security        = "SSL"
  smtp            = false
  user            = "huntuser"
  pass            = "huntpassword"
  pass_wo_version = 1
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		Steps: []resource.TestStep{
			{Config: c},
			{Config: c, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:            "truenas_mail.test",
				ImportState:             true,
				ImportStateId:           "mail",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"pass", "pass_wo_version"},
			},
		},
	})
}
