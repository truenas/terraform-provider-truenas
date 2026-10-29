// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloudsync_credentials_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func expectEmptyPlan() resource.ConfigPlanChecks {
	return resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}
}

func credURLTest(t *testing.T, name, body string, importStep bool) {
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_cloudsync_credentials" "test" {
  name = %q
%s
}
`, name, body)
	steps := []resource.TestStep{
		{Config: cfg},
		{Config: cfg, ConfigPlanChecks: expectEmptyPlan()},
	}
	if importStep {
		steps = append(steps, resource.TestStep{Config: cfg, ResourceName: "truenas_cloudsync_credentials.test", ImportState: true, ImportStateKind: resource.ImportBlockWithID})
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_11_0)},
		CheckDestroy:             testAccCheckCloudsyncCredentialsDestroyed(name),
		Steps:                    steps,
	})
}

// TestAccCloudsyncCredentials_httpURLNoSlash: the server stores the HTTP url
// with a trailing slash, a perpetual diff (B-085). Import plans no change
// (B-119).
func TestAccCloudsyncCredentials_httpURLNoSlash(t *testing.T) {
	credURLTest(t, acctest.RandName("tffix3shtasks-cred"), `
  provider_config = jsonencode({
    type = "HTTP"
    url  = "https://example.com"
  })
`, true)
}

// TestAccCloudsyncCredentials_s3EndpointDefaultPort: the server drops an
// explicit default port and lower-cases the host of the S3 endpoint (B-085).
func TestAccCloudsyncCredentials_s3EndpointDefaultPort(t *testing.T) {
	credURLTest(t, acctest.RandName("tffix3shtasks-cred"), `
  provider_config = jsonencode({
    type              = "S3"
    access_key_id     = "AKIATFFIX3"
    secret_access_key = "tffix3"
    endpoint          = "https://S3.Example.com:443/"
  })
`, true)
}

// TestAccCloudsyncCredentials_webdavURLNoSlashWO: the same through the
// write-only overlay (B-085). No import step: provider_secrets_wo_version
// cannot be read back, so an import plans one update for it (by design).
func TestAccCloudsyncCredentials_webdavURLNoSlashWO(t *testing.T) {
	credURLTest(t, acctest.RandName("tffix3shtasks-cred"), `
  provider_config = jsonencode({
    type   = "WEBDAV"
    url    = "https://dav.example.com"
    vendor = "OTHER"
    user   = "tffix3"
  })
  provider_secrets_wo = jsonencode({
    pass = "tffix3"
  })
  provider_secrets_wo_version = 1
`, false)
}

// TestAccCloudsyncCredentials_readOnlyKey: a READONLY_ADMIN session sees the
// secret keys as "********"; the data source returned the mask as data
// (B-035), and a refresh would plan to overwrite the real keys with it. The
// object is created with the admin key; the read-only step's configuration
// has only the read-only provider (the in-process test provider serves every
// provider block from one instance); the last step switches back so destroy
// runs as admin.
func TestAccCloudsyncCredentials_readOnlyKey(t *testing.T) {
	key, user := os.Getenv("TRUENAS_RO_API_KEY"), os.Getenv("TRUENAS_RO_USERNAME")
	if key == "" || user == "" {
		t.Skip("TRUENAS_RO_API_KEY and TRUENAS_RO_USERNAME (a READONLY_ADMIN API key) are not set")
	}
	ro := fmt.Sprintf(`
provider "truenas" {
  endpoint = %q
  insecure = true
  api_key  = %q
  username = %q
}
`, acctest.Endpoint(), key, user)
	name := acctest.RandName("tffix3shtasks-cred")
	res := fmt.Sprintf(`
resource "truenas_cloudsync_credentials" "test" {
  name = %q
  provider_config = jsonencode({
    type              = "S3"
    access_key_id     = "AKIATFFIX3"
    secret_access_key = "tffix3-secret"
  })
}
`, name)
	ds := fmt.Sprintf(`
data "truenas_cloudsync_credentials" "ro" {
  name = %q
}
`, name)
	noMask := func(addr string) resource.TestCheckFunc {
		return func(s *terraform.State) error {
			rs := s.RootModule().Resources[addr]
			if rs == nil {
				return fmt.Errorf("%s not in state", addr)
			}
			for k, v := range rs.Primary.Attributes {
				if strings.Contains(v, "********") {
					return fmt.Errorf("%s.%s holds the redaction mask: %q", addr, k, v)
				}
			}
			return nil
		}
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCloudsyncCredentialsDestroyed(name),
		Steps: []resource.TestStep{
			{Config: acctest.ProviderConfig() + res},
			{
				Config: ro + res + ds,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_cloudsync_credentials.test", plancheck.ResourceActionNoop),
				}},
				Check: resource.ComposeTestCheckFunc(
					noMask("data.truenas_cloudsync_credentials.ro"),
					noMask("truenas_cloudsync_credentials.test"),
				),
			},
			{Config: acctest.ProviderConfig() + res},
		},
	})
}
