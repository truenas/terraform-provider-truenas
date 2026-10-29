// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package alert_service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func checkAlertServiceURL(want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, _ := strconv.ParseInt(s.RootModule().Resources["truenas_alert_service.test"].Primary.ID, 10, 64)
		raw, err := acctest.Client().CallRead(context.Background(), "alertservice.get_instance", id)
		if err != nil {
			return err
		}
		var got struct {
			Attributes map[string]any `json:"attributes"`
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			return err
		}
		if got.Attributes["url"] != want {
			return fmt.Errorf("stored url = %v, want %q", got.Attributes["url"], want)
		}
		return nil
	}
}

// TestAccAlertService_secretsSentOnlyOnVersionChange: alertservice.update
// replaces attributes whole, so the secret keys go out on every update; with
// the same attributes_secrets_wo_version they are the stored values, so a
// changed overlay is not applied; bumping the version sends it. (B-141)
func TestAccAlertService_secretsSentOnlyOnVersionChange(t *testing.T) {
	name := acctest.RandName("tf-acc-alertwov")
	cfg := func(level, url string, version int) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_alert_service" "test" {
  name    = %q
  level   = %q
  enabled = false
  attributes = jsonencode({
    type = "Slack"
  })
  attributes_secrets_wo         = jsonencode({ url = %q })
  attributes_secrets_wo_version = %d
}
`, name, level, url, version)
	}
	const a, b = "https://hooks.slack.example/tfwo/a", "https://hooks.slack.example/tfwo/b"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		CheckDestroy:             testAccCheckAlertServiceDestroyed(name),
		Steps: []resource.TestStep{
			{Config: cfg("WARNING", a, 1), Check: checkAlertServiceURL(a)},
			{
				Config: cfg("ERROR", b, 1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_alert_service.test", "level", "ERROR"),
					checkAlertServiceURL(a),
				),
			},
			{Config: cfg("ERROR", b, 2), Check: checkAlertServiceURL(b)},
		},
	})
}
