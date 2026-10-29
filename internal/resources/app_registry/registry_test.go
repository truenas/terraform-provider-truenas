// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app_registry_test

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// These tests need a reachable registry, because app.registry.create and
// update log in to it (see TestAccAppRegistry_basic). Point
// TRUENAS_TEST_REGISTRY_URI at one the TrueNAS box can reach (e.g. a
// registry:2 app with htpasswd auth at http://127.0.0.1:5000), with
// TRUENAS_TEST_REGISTRY_USERNAME and TRUENAS_TEST_REGISTRY_PASSWORD. The URI
// must not already be used by a registry entry (TrueNAS requires it unique).
func registryEnv(t *testing.T) (uri, user, pass string) {
	t.Helper()
	uri = os.Getenv("TRUENAS_TEST_REGISTRY_URI")
	user = os.Getenv("TRUENAS_TEST_REGISTRY_USERNAME")
	pass = os.Getenv("TRUENAS_TEST_REGISTRY_PASSWORD")
	if uri == "" || user == "" || pass == "" {
		t.Skip("set TRUENAS_TEST_REGISTRY_URI, TRUENAS_TEST_REGISTRY_USERNAME and TRUENAS_TEST_REGISTRY_PASSWORD to run")
	}
	return uri, user, pass
}

func registryConfig(name, uri, user, pass, extra string) string {
	return fmt.Sprintf(`
resource "truenas_app_registry" "test" {
  name                = %q
  uri                 = %q
  username            = %q
  password_wo         = %q
  password_wo_version = 1
%s
}
`, name, uri, user, pass, extra)
}

// TestAccAppRegistry_importURI guards B-122: TrueNAS appends "/" to an http(s)
// registry URI, and import followed by plan with the configured spelling
// (no trailing slash) planned an update.
func TestAccAppRegistry_importURI(t *testing.T) {
	uri, user, pass := registryEnv(t)
	uri = strings.TrimRight(uri, "/")
	name := acctest.RandName("tf-acc-reg")
	cfg := acctest.ProviderConfig() + registryConfig(name, uri, user, pass, `  description = "x"`)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: append([]resource.TestStep{
			{Config: cfg, Check: resource.TestCheckResourceAttr("truenas_app_registry.test", "uri", uri)},
		}, acctest.ImportReapplyNoop("truenas_app_registry.test", cfg, "password_wo_version")...),
	})
}

// TestAccAppRegistry_readonlyRedacted guards B-032: for a READONLY_ADMIN
// session, app.registry returns username as "********", which the data source
// returned and import stored. Run it with TRUENAS_API_KEY set to a
// READONLY_ADMIN user's key and TRUENAS_TEST_READONLY_REGISTRY_NAME/_ID naming
// an existing registry entry (the test only reads).
func TestAccAppRegistry_readonlyRedacted(t *testing.T) {
	name := os.Getenv("TRUENAS_TEST_READONLY_REGISTRY_NAME")
	id := os.Getenv("TRUENAS_TEST_READONLY_REGISTRY_ID")
	if name == "" || id == "" {
		t.Skip("set TRUENAS_TEST_READONLY_REGISTRY_NAME and _ID (and a READONLY_ADMIN TRUENAS_API_KEY) to run")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + fmt.Sprintf(`
data "truenas_app_registry" "d" {
  name = %q
}
`, name),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_app_registry.d", "id", id),
					resource.TestCheckNoResourceAttr("data.truenas_app_registry.d", "username"),
				),
			},
			{
				Config:        acctest.ProviderConfig() + registryConfig(name, "http://unused", "unused", "unused", ""),
				ResourceName:  "truenas_app_registry.test",
				ImportState:   true,
				ImportStateId: id,
				ImportStateCheck: func(s []*terraform.InstanceState) error {
					if got, ok := s[0].Attributes["username"]; ok && got != "" {
						return fmt.Errorf("imported username = %q, want null", got)
					}
					return nil
				},
			},
		},
	})
}
