// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package kerberos_realm_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccKerberosRealm_basic creates a synthetic Kerberos realm, checks its
// attributes, updates its admin_server list in place, imports it by its
// numeric id, and verifies destruction via a live kerberos.realm.query.
//
// The realm is entirely synthetic (TF-ACC-<rand>.LAN, kdc = a reserved
// TEST-NET-1 address that will never respond) and safe against a real
// TrueNAS instance: a throwaway probe (kerberos.realm.create with realm
// "TF-PROBE.LAN", kdc ["192.0.2.88"], immediately deleted) confirmed
// kerberos.realm.create returns in ~200ms without attempting to contact the
// KDC to validate reachability — see task-5-report.md for the full probe
// record.
func TestAccKerberosRealm_basic(t *testing.T) {
	acctest.PreCheck(t)

	realmName := strings.ToUpper(acctest.RandName("tf-acc")) + ".LAN"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckKerberosRealmDestroyed(realmName),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccKerberosRealmConfig(realmName, `["192.0.2.88"]`, `[]`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("truenas_kerberos_realm.test", "id"),
					resource.TestCheckResourceAttr("truenas_kerberos_realm.test", "realm", realmName),
					resource.TestCheckResourceAttr("truenas_kerberos_realm.test", "kdc.#", "1"),
					resource.TestCheckResourceAttr("truenas_kerberos_realm.test", "kdc.0", "192.0.2.88"),
					resource.TestCheckResourceAttr("truenas_kerberos_realm.test", "admin_server.#", "0"),
				),
			},
			// Update in place: set admin_server.
			{
				Config: acctest.ProviderConfig() + testAccKerberosRealmConfig(realmName, `["192.0.2.88"]`, `["192.0.2.89"]`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_kerberos_realm.test", "admin_server.#", "1"),
					resource.TestCheckResourceAttr("truenas_kerberos_realm.test", "admin_server.0", "192.0.2.89"),
				),
			},
			// Import by the realm's numeric id.
			{
				ResourceName:      "truenas_kerberos_realm.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccKerberosRealmConfig(realmName, kdcHCL, adminServerHCL string) string {
	return fmt.Sprintf(`
resource "truenas_kerberos_realm" "test" {
  realm        = %q
  kdc          = %s
  admin_server = %s
}
`, realmName, kdcHCL, adminServerHCL)
}

func testAccCheckKerberosRealmDestroyed(realmName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "kerberos.realm.query", [][]any{{"realm", "=", realmName}})
		if err != nil {
			return fmt.Errorf("error checking Kerberos realm %s: %v", realmName, err)
		}
		var results []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing kerberos.realm.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("Kerberos realm %s still exists", realmName)
		}
		return nil
	}
}
