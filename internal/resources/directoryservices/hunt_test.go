// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package directoryservices_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func huntLDAP(url, extra string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_directoryservices" "test" {
  service_type = "LDAP"
  enable       = true
  %s
  credential = {
    credential_type = "LDAP_PLAIN"
    binddn          = "cn=admin,dc=tftest-ldap,dc=lan"
    bindpw          = %q
  }
  configuration_ldap = {
    server_urls           = [%q]
    basedn                = "dc=tftest-ldap,dc=lan"
    starttls              = false
    validate_certificates = false
  }
}
`, extra, os.Getenv("TFHUNT_LDAP_PW"), url)
}

func huntDSCheck(t *testing.T) {
	acctest.PreCheck(t)
	if os.Getenv("TFHUNT_LDAP_PW") == "" {
		t.Skip("TFHUNT_LDAP_PW not set")
	}
}

var emptyPlan = resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}

// server_urls with a trailing slash.
func TestHuntDS_ldapURLForm(t *testing.T) {
	cfg := huntLDAP("ldap://192.168.1.251/", "")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { huntDSCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg},
			{Config: cfg, ConfigPlanChecks: emptyPlan},
		},
	})
}
