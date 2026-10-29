// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package kerberos_realm_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccKerberosRealm_primaryKDCUnset: removing primary_kdc from config plans
// an update that clears it (the API only clears it with null). (B-112)
func TestAccKerberosRealm_primaryKDCUnset(t *testing.T) {
	realm := strings.ToUpper(acctest.RandName("tf-acc-pkdc")) + ".LAN"
	cfg := func(pk string) string {
		return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_kerberos_realm" "test" {
  realm = %q
  kdc   = ["kdc1.tfacc.lan"]
  %s
}
`, realm, pk)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg(`primary_kdc = "kdc1.tfacc.lan"`)},
			{
				Config: cfg(""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_kerberos_realm.test", plancheck.ResourceActionUpdate),
				}},
				Check: resource.TestCheckNoResourceAttr("truenas_kerberos_realm.test", "primary_kdc"),
			},
			{Config: cfg(""), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}
