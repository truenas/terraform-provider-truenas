// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloudsync_credentials_test

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

func checkCredentialsProvider(key, want string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		id, _ := strconv.ParseInt(s.RootModule().Resources["truenas_cloudsync_credentials.test"].Primary.ID, 10, 64)
		raw, err := acctest.Client().CallRead(context.Background(), "cloudsync.credentials.get_instance", id)
		if err != nil {
			return err
		}
		var got struct {
			Provider map[string]any `json:"provider"`
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			return err
		}
		if got.Provider[key] != want {
			return fmt.Errorf("stored %s = %v, want %q", key, got.Provider[key], want)
		}
		return nil
	}
}

// TestAccCloudsyncCredentials_secretsSentOnlyOnVersionChange:
// cloudsync.credentials.update replaces provider whole, so the secret keys go
// out on every update; with the same provider_secrets_wo_version they are the
// stored values, so a changed overlay is not applied; bumping the version
// sends it. (B-141)
func TestAccCloudsyncCredentials_secretsSentOnlyOnVersionChange(t *testing.T) {
	name := acctest.RandName("tf-acc-credswov")
	cfg := func(accessKey, secret string, version int) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_cloudsync_credentials" "test" {
  name = %q
  provider_config = jsonencode({
    type          = "STORJ_IX"
    access_key_id = %q
  })
  provider_secrets_wo = jsonencode({
    secret_access_key = %q
  })
  provider_secrets_wo_version = %d
}
`, name, accessKey, secret, version)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		CheckDestroy:             testAccCheckCloudsyncCredentialsDestroyed(name),
		Steps: []resource.TestStep{
			{Config: cfg("tfwokey1", "tfwo-secret-a", 1), Check: checkCredentialsProvider("secret_access_key", "tfwo-secret-a")},
			{
				Config: cfg("tfwokey2", "tfwo-secret-b", 1),
				Check: resource.ComposeTestCheckFunc(
					checkCredentialsProvider("access_key_id", "tfwokey2"),
					checkCredentialsProvider("secret_access_key", "tfwo-secret-a"),
				),
			},
			{Config: cfg("tfwokey2", "tfwo-secret-b", 2), Check: checkCredentialsProvider("secret_access_key", "tfwo-secret-b")},
		},
	})
}
