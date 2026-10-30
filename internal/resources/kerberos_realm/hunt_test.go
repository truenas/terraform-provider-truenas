// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package kerberos_realm_test

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntKerberosRealm_allFields is an adversarial value round-trip: it sets
// every API-accepted attribute of truenas_kerberos_realm to a valid
// non-default value (a multi-element kdc list, a non-empty admin_server, a
// non-empty kpasswd_server, and primary_kdc), then asserts (1) the apply
// succeeds, (2) a re-plan of the identical config is empty, and (3) import by
// numeric id round-trips every attribute.
//
// The realm is entirely synthetic (TFHUNT-<rand>.LAN, with TEST-NET-1 /
// 192.0.2.x reserved addresses that never respond) and lab-safe: as the
// acceptance test documents, kerberos.realm.create stores the realm without
// contacting any KDC.
func TestHuntKerberosRealm_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)

	realmName := strings.ToUpper(acctest.RandName("tfhunt")) + ".LAN"
	// Reuse the acceptance config helper: it already wires primary_kdc
	// (192.0.2.90) and kpasswd_server (["192.0.2.91"]); here we also give kdc
	// two entries and admin_server one, so every settable field is non-default.
	cfg := acctest.ProviderConfig() + testAccKerberosRealmConfig(realmName, `["192.0.2.88", "192.0.2.92"]`, `["192.0.2.89"]`)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckKerberosRealmDestroyed(realmName),
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_kerberos_realm.test", "id"),
					resource.TestCheckResourceAttr("truenas_kerberos_realm.test", "realm", realmName),
					resource.TestCheckResourceAttr("truenas_kerberos_realm.test", "kdc.#", "2"),
					resource.TestCheckResourceAttr("truenas_kerberos_realm.test", "kdc.0", "192.0.2.88"),
					resource.TestCheckResourceAttr("truenas_kerberos_realm.test", "kdc.1", "192.0.2.92"),
					resource.TestCheckResourceAttr("truenas_kerberos_realm.test", "admin_server.#", "1"),
					resource.TestCheckResourceAttr("truenas_kerberos_realm.test", "admin_server.0", "192.0.2.89"),
					resource.TestCheckResourceAttr("truenas_kerberos_realm.test", "primary_kdc", "192.0.2.90"),
					resource.TestCheckResourceAttr("truenas_kerberos_realm.test", "kpasswd_server.#", "1"),
					resource.TestCheckResourceAttr("truenas_kerberos_realm.test", "kpasswd_server.0", "192.0.2.91"),
				),
			},
			// Re-plan of the identical config must be empty (no drift).
			{
				Config:           cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
			{
				ResourceName:      "truenas_kerberos_realm.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
