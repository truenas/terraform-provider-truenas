package mail_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccMailDataSource_basic reads the current TrueNAS mail configuration
// through the truenas_mail datasource only. It never writes: mail is a
// system-critical singleton, and the box's real mail settings must not be
// overwritten by an acceptance test run.
//
// Requires:
//   - TF_ACC=1
//   - TRUENAS_API_KEY (or TRUENAS_USERNAME+TRUENAS_PASSWORD) set for
//     acctest.PreCheck
//   - A running TrueNAS SCALE instance
func TestAccMailDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_mail" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_mail.test", "id", "mail"),
					resource.TestCheckResourceAttrSet("data.truenas_mail.test", "fromemail"),
					resource.TestCheckResourceAttrSet("data.truenas_mail.test", "outgoingserver"),
					resource.TestCheckResourceAttrSet("data.truenas_mail.test", "security"),
				),
			},
		},
	})
}

// TestAccMail_basic is intentionally skipped by default. truenas_mail is a
// SINGLETON resource: bringing it under Terraform management mutates the
// box's actual mail configuration (mail.update), and a naive
// create/update/destroy acceptance test would leave the target system's
// mail settings altered by whatever the test's Destroy step does (or, per
// this resource's Delete semantics, simply abandoned in whatever state the
// last Update left them in).
//
// If this test is ever enabled against a disposable/throwaway TrueNAS
// instance, it should:
//  1. Read the current config via the datasource first.
//  2. Only touch "fromname" (a cosmetic, low-risk field) in the resource
//     config, driving it through a value and back to the value the
//     datasource observed in step 1, so the net effect on the box is a
//     no-op.
//  3. Use ImportStateVerifyIgnore: []string{"pass"} on the import step,
//     since mail.config never returns the password.
func TestAccMail_basic(t *testing.T) {
	t.Skip("truenas_mail is a system-critical singleton; skipped to avoid mutating the target box's mail configuration. See comment on TestAccMail_basic for how to safely enable this against a disposable instance.")
}
