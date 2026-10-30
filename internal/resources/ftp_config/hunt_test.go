// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ftp_config_test

// Adversarial bug-hunt tests: apply every API-accepted value and assert an
// empty re-plan, then import. snapshotRestore is defined in anonpath_test.go.
//
// TestHuntFTPConfig_allFields covers the non-TLS surface;
// TestHuntFTPConfig_tlsPath covers tls + every tls_opt_* flag + tls_policy
// using the box's own truenas_default certificate (id 1, present on every
// TrueNAS install), so no certificate fixture is needed.

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// Every non-TLS bool and limit at a non-default value, octal masks, empty
// string fields. Assert the read-back re-plans empty, then import.
func TestHuntFTPConfig_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)
	snapshotRestore(t, "ftp")
	c := acctest.ProviderConfig() + `
resource "truenas_ftp_config" "test" {
  port               = 2121
  clients            = 20
  ipconnections      = 4
  loginattempt       = 2
  timeout            = 500
  timeout_notransfer = 200
  passiveportsmin    = 40000
  passiveportsmax    = 40100
  onlyanonymous      = false
  onlylocal          = false
  anonpath           = ""
  fxp                = true
  resume             = true
  defaultroot        = true
  ident              = true
  reversedns         = true
  masqaddress        = ""
  banner             = "hunt-banner"
  options            = ""
  dirmask            = "027"
  filemask           = "027"
  localuserbw        = 0
  localuserdlbw      = 0
  tls                = false
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: c},
			{Config: c, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:      "truenas_ftp_config.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TLS surface: tls on, tls_policy set, every tls_opt_* flag on, bound to the
// box's truenas_default certificate (id 1). Assert the read-back re-plans
// empty, then import. snapshotRestore puts tls back off and the cert to null.
func TestHuntFTPConfig_tlsPath(t *testing.T) {
	acctest.DisruptiveCheck(t)
	snapshotRestore(t, "ftp")
	c := acctest.ProviderConfig() + `
resource "truenas_ftp_config" "test" {
  tls                                 = true
  ssltls_certificate                  = 1
  tls_policy                          = "on"
  tls_opt_allow_client_renegotiations = true
  tls_opt_allow_dot_login             = true
  tls_opt_allow_per_user              = true
  tls_opt_common_name_required        = true
  tls_opt_dns_name_required           = true
  tls_opt_enable_diags                = true
  tls_opt_export_cert_data            = true
  tls_opt_ip_address_required         = true
  tls_opt_no_empty_fragments          = true
  tls_opt_no_session_reuse_required   = true
  tls_opt_stdenvvars                  = true
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: c},
			{Config: c, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			{
				ResourceName:      "truenas_ftp_config.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
