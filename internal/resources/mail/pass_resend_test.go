// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package mail_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// checkMailPass checks the SMTP password TrueNAS has stored (mail.config
// returns it to an admin session).
func checkMailPass(want string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		raw, err := acctest.Client().Call(context.Background(), "mail.config")
		if err != nil {
			return err
		}
		var cfg struct {
			Pass *string `json:"pass"`
		}
		if err := json.Unmarshal(raw, &cfg); err != nil {
			return err
		}
		if cfg.Pass == nil {
			return fmt.Errorf("stored pass is null, want %q", want)
		}
		if *cfg.Pass != want {
			return fmt.Errorf("stored pass is %q, want %q", *cfg.Pass, want)
		}
		return nil
	}
}

// TestAccMail_passSentOnlyOnVersionChange: an update that does not change
// pass_wo_version does not send pass (a changed value is not applied); bumping
// the version sends it. (B-141)
func TestAccMail_passSentOnlyOnVersionChange(t *testing.T) {
	acctest.DisruptiveCheck(t)
	restoreConfigKeys(t, "fromname", "outgoingserver", "port", "security", "smtp", "user", "pass")
	orig := readMailOriginal(t)
	if orig.FromEmail == "" {
		t.Skip("mail is unconfigured on the target box (empty fromemail); set-and-restore cannot restore the unconfigured state")
	}

	cfg := func(fromname, pass string, version int) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_mail" "test" {
  fromname        = %q
  outgoingserver  = "smtp.example.com"
  port            = 587
  security        = "TLS"
  smtp            = false
  user            = "mailuser"
  pass            = %q
  pass_wo_version = %d
}
`, fromname, pass, version)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		Steps: []resource.TestStep{
			{Config: cfg("tf-acc-wo-1", "tf-acc-wo-pass-a", 1), Check: checkMailPass("tf-acc-wo-pass-a")},
			{
				Config: cfg("tf-acc-wo-2", "tf-acc-wo-pass-b", 1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_mail.test", "fromname", "tf-acc-wo-2"),
					checkMailPass("tf-acc-wo-pass-a"),
				),
			},
			{Config: cfg("tf-acc-wo-2", "tf-acc-wo-pass-b", 2), Check: checkMailPass("tf-acc-wo-pass-b")},
		},
	})
}
