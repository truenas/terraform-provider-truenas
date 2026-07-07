package mail_test

import (
	"context"
	"encoding/json"
	"fmt"
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

// mailOriginal captures the one field TestAccMail_setAndRestore touches.
type mailOriginal struct {
	FromName string `json:"fromname"`
}

// readMailOriginal reads the box's current fromname via mail.config, so the
// test can restore it exactly afterward.
func readMailOriginal(t *testing.T) mailOriginal {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "mail.config")
	if err != nil {
		t.Fatalf("error reading current mail config: %v", err)
	}
	var orig mailOriginal
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing mail.config response: %v", err)
	}
	return orig
}

// restoreMail sends only fromname back to its original value via
// mail.update. It runs from t.Cleanup, so it restores the box even if the
// Terraform steps themselves fail partway through.
func restoreMail(t *testing.T, orig mailOriginal) {
	t.Helper()
	if _, err := acctest.Client().Call(context.Background(), "mail.update", map[string]any{
		"fromname": orig.FromName,
	}); err != nil {
		t.Fatalf("error restoring mail fromname: %v", err)
	}
}

// TestAccMail_setAndRestore drives the singleton truenas_mail resource's
// "fromname" field (a cosmetic, low-risk display name) through a test value
// and back to the value read from the box before the test ran, then imports
// it. It requires TF_ACC=1 and TRUENAS_DISRUPTIVE=1
// (acctest.DisruptiveCheck), since it mutates the box's live mail
// configuration; a t.Cleanup-registered API restore is the safety net if the
// Terraform steps fail.
func TestAccMail_setAndRestore(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readMailOriginal(t)
	t.Cleanup(func() { restoreMail(t, orig) })

	testValue := acctest.RandName("tf-acc")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccMailConfig(testValue),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_mail.test", "id", "mail"),
					resource.TestCheckResourceAttr("truenas_mail.test", "fromname", testValue),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccMailConfig(orig.FromName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_mail.test", "fromname", orig.FromName),
				),
			},
			{
				ResourceName:            "truenas_mail.test",
				ImportState:             true,
				ImportStateId:           "mail",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"pass"},
			},
		},
	})
}

func testAccMailConfig(fromname string) string {
	return fmt.Sprintf(`
resource "truenas_mail" "test" {
  fromname = %q
}
`, fromname)
}
