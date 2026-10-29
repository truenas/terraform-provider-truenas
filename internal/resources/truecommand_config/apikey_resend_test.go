// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package truecommand_config_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func checkTrueCommandKey(want string) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		raw, err := acctest.Client().Call(context.Background(), "truecommand.config")
		if err != nil {
			return err
		}
		var got struct {
			APIKey *string `json:"api_key"`
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			return err
		}
		if got.APIKey == nil || *got.APIKey != want {
			return fmt.Errorf("stored api_key = %v, want %q", got.APIKey, want)
		}
		return nil
	}
}

// TestAccTrueCommandConfig_apiKeySentOnlyOnVersionChange: toggling enabled
// with a changed api_key_wo and the same api_key_wo_version leaves the stored
// key alone; bumping the version sends the new key. (B-141) Like
// TestAccTrueCommandConfig_EnableStaleStatus it enables TrueCommand with a
// throwaway key (the only other field an update can change), so it needs
// TRUENAS_TEST_TRUECOMMAND_ENABLE=1.
func TestAccTrueCommandConfig_apiKeySentOnlyOnVersionChange(t *testing.T) {
	acctest.DisruptiveCheck(t)
	if os.Getenv("TRUENAS_TEST_TRUECOMMAND_ENABLE") != "1" {
		t.Skip("set TRUENAS_TEST_TRUECOMMAND_ENABLE=1 to let this test enable TrueCommand with a throwaway key")
	}
	restoreTrueCommandOnCleanup(t)
	cfg := func(enabled bool, key string, version int) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_truecommand_config" "test" {
  enabled            = %t
  api_key_wo         = %q
  api_key_wo_version = %d
}
`, enabled, key, version)
	}
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		Steps: []resource.TestStep{
			{Config: cfg(false, "tfwokeyA12345678", 1), Check: checkTrueCommandKey("tfwokeyA12345678")},
			{Config: cfg(true, "tfwokeyB12345678", 1), Check: checkTrueCommandKey("tfwokeyA12345678")},
			{Config: cfg(false, "tfwokeyB12345678", 2), Check: checkTrueCommandKey("tfwokeyB12345678")},
		},
	})
}
