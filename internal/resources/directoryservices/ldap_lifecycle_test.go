// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package directoryservices_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

var dsEmptyPlan = resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}

// ldapLifecycleConfig is testAccDirectoryServicesLDAPConfig with optional
// search bases, for the second-apply and import tests below.
func ldapLifecycleConfig(serverURL, baseDN, bindDN, bindPW string, timeout int, baseUser string) string {
	sb := ""
	if baseUser != "" {
		sb = fmt.Sprintf("\n    search_bases          = { base_user = %q }", baseUser)
	}
	return fmt.Sprintf(`
resource "truenas_directoryservices" "test" {
  service_type = "LDAP"
  enable       = true
  timeout      = %d

  credential = {
    credential_type = "LDAP_PLAIN"
    binddn          = %q
    bindpw          = %q
  }

  configuration_ldap = {
    server_urls           = [%q]
    basedn                = %q
    starttls              = false
    validate_certificates = false%s
  }
}
`, timeout, bindDN, bindPW, serverURL, baseDN, sb)
}

// TestAccDirectoryServices_ldapChangeWhileEnabled: a configuration_ldap change
// on an enabled bind is outside what TrueNAS allows while enabled, so the
// provider disables, applies the change, and re-enables. A second change
// exercises the same path again. (B-049)
func TestAccDirectoryServices_ldapChangeWhileEnabled(t *testing.T) {
	serverURL, baseDN, bindDN, bindPW := dsPreCheckLDAP(t)
	base := acctest.ProviderConfig() + ldapLifecycleConfig(serverURL, baseDN, bindDN, bindPW, 20, "")
	people := "ou=People," + baseDN
	changed := acctest.ProviderConfig() + ldapLifecycleConfig(serverURL, baseDN, bindDN, bindPW, 20, people)
	changed2 := acctest.ProviderConfig() + ldapLifecycleConfig(serverURL, baseDN, bindDN, bindPW, 20, baseDN)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDirectoryServicesDisabledWithType("LDAP"),
		Steps: []resource.TestStep{
			{Config: base},
			{Config: changed, Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("truenas_directoryservices.test", "configuration_ldap.search_bases.base_user", people),
				testAccCheckDirectoryServicesStatus("HEALTHY"),
			)},
			{Config: changed, ConfigPlanChecks: dsEmptyPlan},
			{Config: changed2, Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("truenas_directoryservices.test", "configuration_ldap.search_bases.base_user", baseDN),
				testAccCheckDirectoryServicesStatus("HEALTHY"),
			)},
			{Config: changed2, ConfigPlanChecks: dsEmptyPlan},
		},
	})
}

// TestAccDirectoryServices_ldapImport: forgetting the bind and importing it
// again with the identical configuration plans a no-op (no rebind job): import
// reads the non-secret part of the LDAP credential. (B-050)
func TestAccDirectoryServices_ldapImport(t *testing.T) {
	serverURL, baseDN, bindDN, bindPW := dsPreCheckLDAP(t)
	// While the bind is forgotten (between the removed and import steps) a
	// failure skips CheckDestroy and would leave it enabled; disable it here.
	t.Cleanup(func() {
		if _, err := acctest.Client().CallJob(context.Background(), "directoryservices.update", map[string]any{"enable": false}); err != nil {
			t.Logf("cleanup: disable directory services: %v", err)
		}
	})
	cfg := acctest.ProviderConfig() + ldapLifecycleConfig(serverURL, baseDN, bindDN, bindPW, 20, "")
	removed := acctest.ProviderConfig() + `
removed {
  from = truenas_directoryservices.test
  lifecycle {
    destroy = false
  }
}
`
	imp := `
import {
  to = truenas_directoryservices.test
  id = "directoryservices"
}
`
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDirectoryServicesDisabledWithType("LDAP"),
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, ConfigPlanChecks: dsEmptyPlan},
			{Config: removed},
			{
				Config: cfg + imp,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_directoryservices.test", plancheck.ResourceActionNoop),
				}},
			},
			{Config: cfg, ConfigPlanChecks: dsEmptyPlan},
		},
	})
}
