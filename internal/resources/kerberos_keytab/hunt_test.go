// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package kerberos_keytab_test

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntKerberosKeytab_allFields is an adversarial value round-trip. The
// schema's two secret-carrying inputs (file and file_wo) are mutually
// exclusive — exactly one may be set — so the "all fields" surface here is
// name + file, the path whose value IS stored in state and read back on
// refresh (file_wo is write-only and never read back; it has its own
// coverage in TestAccKerberosKeytab_fileWO / the file_resend test). It sets
// name and the base64 keytab from TRUENAS_DS_KEYTAB_B64, asserts the apply
// succeeds, that a re-plan is empty, and imports.
//
// Import never reads the keytab into state (it cannot tell whether the config
// uses file or file_wo — B-172), so "file" is in ImportStateVerifyIgnore,
// matching the acceptance test.
func TestHuntKerberosKeytab_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)
	fileB64 := testAccKeytabPreCheck(t)

	name := acctest.RandName("tfhunt-keytab")
	cfg := acctest.ProviderConfig() + testAccKerberosKeytabConfig(name, fileB64)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckKerberosKeytabDestroyed,
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_kerberos_keytab.test", "id"),
					resource.TestCheckResourceAttr("truenas_kerberos_keytab.test", "name", name),
					resource.TestCheckResourceAttr("truenas_kerberos_keytab.test", "file", fileB64),
				),
			},
			{
				Config:           cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				ResourceName:            "truenas_kerberos_keytab.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"file"},
			},
		},
	})
}
