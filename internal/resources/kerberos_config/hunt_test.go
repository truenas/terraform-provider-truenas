// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package kerberos_config_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntKerberosConfig_allFields is an adversarial value round-trip for the
// singleton truenas_kerberos_config resource: it sets BOTH settable
// attributes (appdefaults_aux and libdefaults_aux) to valid non-default
// values at once, asserts they read back, that a re-plan is empty, and that
// import round-trips them.
//
// The markers must be well-formed, section-appropriate krb5.conf relations:
// kerberos.update parses them server-side and crashes on a line without "="
// and rejects a key that does not belong to its section (see the note on
// TestAccKerberosConfig_setAndRestore). "no_addresses" is a recognized
// [appdefaults] key; "dns_lookup_kdc" is a recognized [libdefaults] relation.
//
// Singleton, box-wide state: the original appdefaults_aux/libdefaults_aux are
// captured before the run and restored via restoreKerberosConfig from
// t.Cleanup, which is the safety net even if the Terraform steps fail.
func TestHuntKerberosConfig_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readKerberosConfigOriginal(t)
	t.Cleanup(func() { restoreKerberosConfig(t, orig) })

	appMarker := "no_addresses = true"
	if orig.AppdefaultsAux == appMarker {
		appMarker = "no_addresses = false"
	}
	libMarker := "dns_lookup_kdc = true"
	if orig.LibdefaultsAux == libMarker {
		libMarker = "dns_lookup_kdc = false"
	}

	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_kerberos_config" "test" {
  appdefaults_aux = %q
  libdefaults_aux = %q
}
`, appMarker, libMarker)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_kerberos_config.test", "id", "kerberos_config"),
					resource.TestCheckResourceAttr("truenas_kerberos_config.test", "appdefaults_aux", appMarker),
					resource.TestCheckResourceAttr("truenas_kerberos_config.test", "libdefaults_aux", libMarker),
				),
			},
			{
				Config:           cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				ResourceName:      "truenas_kerberos_config.test",
				ImportState:       true,
				ImportStateId:     "kerberos_config",
				ImportStateVerify: true,
			},
		},
	})
}
