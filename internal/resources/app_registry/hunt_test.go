// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app_registry_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// Requires a reachable authenticated registry; TFHUNT_REG_URI points at it
// (a registry:2 container with htpasswd on the target box, account
// TRUENAS_TEST_REGISTRY_USERNAME / _PASSWORD — app.registry.create logs in).
func huntReg(uri, extra string) string {
	u := ""
	if uri != "" {
		u = fmt.Sprintf("  uri = %q\n", uri)
	}
	return fmt.Sprintf(`
resource "truenas_app_registry" "test" {
  name                = "tfhuntreg"
  username            = %q
  password_wo         = %q
  password_wo_version = 1
%s%s
}
`, os.Getenv("TRUENAS_TEST_REGISTRY_USERNAME"), os.Getenv("TRUENAS_TEST_REGISTRY_PASSWORD"), u, extra)
}

func TestHuntAppRegistry_lifecycle(t *testing.T) {
	uri := os.Getenv("TFHUNT_REG_URI")
	if uri == "" {
		t.Skip("TFHUNT_REG_URI unset")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// description = "" (zero value)
			{Config: acctest.ProviderConfig() + huntReg(uri, `  description = ""`)},
			// change one attr
			{Config: acctest.ProviderConfig() + huntReg(uri, `  description = "x"`)},
			// set -> unset
			{Config: acctest.ProviderConfig() + huntReg(uri, ``)},
			// remove uri from config: Optional+Computed keeps old value
			{Config: acctest.ProviderConfig() + huntReg("", ``),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				Config:                  acctest.ProviderConfig() + huntReg(uri, ``),
				ResourceName:            "truenas_app_registry.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password_wo", "password_wo_version"},
			},
		},
	})
}

// Pattern 3: username is Secret[str] in the API, redacted for READONLY_ADMIN
// callers. With a full-admin key the data source and import must return the
// real username of the pre-existing entry TRUENAS_RO_REGISTRY / TFHUNT_REG_ID
// (TestAccAppRegistry_readonlyRedacted covers the read-only key).
func TestHuntAppRegistry_readonlyRedacted(t *testing.T) {
	name, id := os.Getenv("TRUENAS_RO_REGISTRY"), os.Getenv("TFHUNT_REG_ID")
	if name == "" || id == "" {
		t.Skip("TRUENAS_RO_REGISTRY / TFHUNT_REG_ID unset")
	}
	want := os.Getenv("TRUENAS_TEST_REGISTRY_USERNAME")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: acctest.ProviderConfig() + fmt.Sprintf(`
data "truenas_app_registry" "d" {
  name = %q
}
`, name), Check: resource.TestCheckResourceAttr("data.truenas_app_registry.d", "username", want)},
			{Config: acctest.ProviderConfig() + huntReg("http://127.0.0.1:9322/", ""),
				ResourceName: "truenas_app_registry.test", ImportState: true,
				ImportStateId: id,
				ImportStateCheck: func(s []*terraform.InstanceState) error {
					if got := s[0].Attributes["username"]; got != want {
						return fmt.Errorf("imported username = %q, want %q", got, want)
					}
					return nil
				}},
		},
	})
}
