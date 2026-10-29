// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ups_config_test

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

// upsMonPwd reads the monitor password TrueNAS has stored (ups.config
// returns it to an admin session).
func upsMonPwd() (string, error) {
	raw, err := acctest.Client().Call(context.Background(), "ups.config")
	if err != nil {
		return "", err
	}
	var cfg struct {
		MonPwd string `json:"monpwd"`
	}
	err = json.Unmarshal(raw, &cfg)
	return cfg.MonPwd, err
}

func checkUPSMonPwd(want string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		got, err := upsMonPwd()
		if err != nil {
			return err
		}
		if got != want {
			return fmt.Errorf("stored monpwd is %q, want %q", got, want)
		}
		return nil
	}
}

// TestAccUPSConfig_monpwdSentOnlyOnVersionChange: an update that does not
// change monpwd_wo_version does not send monpwd (a changed value is not
// applied); bumping the version sends it. (B-141)
func TestAccUPSConfig_monpwdSentOnlyOnVersionChange(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readUPSConfigOriginal(t)
	if orig.Driver == "" || orig.Port == "" {
		t.Skip("UPS is unconfigured on the target box (empty driver/port); set-and-restore cannot restore the unconfigured state")
	}
	origPwd, err := upsMonPwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		restoreUPSConfig(t, orig)
		// restoreUPSConfig does not carry monpwd; ups.update needs the
		// driver/port pair with it.
		if _, err := acctest.Client().Call(context.Background(), "ups.update", map[string]any{
			"driver": orig.Driver, "port": orig.Port, "monpwd": origPwd,
		}); err != nil {
			t.Errorf("restore monpwd: %v", err)
		}
		if got, err := upsMonPwd(); err != nil || got != origPwd {
			t.Errorf("monpwd not restored: %q, %v", got, err)
		}
	})

	cfg := func(desc, pwd string, ver int) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_ups_config" "test" {
  driver            = %q
  port              = %q
  description       = %q
  monpwd            = %q
  monpwd_wo_version = %d
}
`, orig.Driver, orig.Port, desc, pwd, ver)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		Steps: []resource.TestStep{
			{Config: cfg("tf-acc-wo-1", "tfaccwopwdA", 1), Check: checkUPSMonPwd("tfaccwopwdA")},
			{
				Config: cfg("tf-acc-wo-2", "tfaccwopwdB", 1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_ups_config.test", "description", "tf-acc-wo-2"),
					checkUPSMonPwd("tfaccwopwdA"),
				),
			},
			{Config: cfg("tf-acc-wo-2", "tfaccwopwdB", 2), Check: checkUPSMonPwd("tfaccwopwdB")},
		},
	})
}
