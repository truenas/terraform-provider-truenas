// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package certificate_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccCertificate_trustedStoreUnset: removing add_to_trusted_store = true
// from the configuration kept the certificate in the trusted store (B-140);
// it now resets to the default, false.
func TestAccCertificate_trustedStoreUnset(t *testing.T) {
	name := acctest.RandName("tffix3shtasks-trust")
	certPEM, keyPEM := genSelfSigned(t, "tffix3-trust.example.com")
	base := testAccCertificateImportedConfig(name, certPEM, keyPEM)
	trusted := acctest.ProviderConfig() + base[:len(base)-2] + "  add_to_trusted_store = true\n}\n"
	unset := acctest.ProviderConfig() + base
	n := "truenas_certificate.test"
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckCertificateDestroyed(name, ""),
		Steps: []resource.TestStep{
			{Config: trusted, Check: resource.TestCheckResourceAttr(n, "add_to_trusted_store", "true")},
			{
				Config: unset,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(n, plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(n, "add_to_trusted_store", "false"),
					testAccCheckCertificateTrusted(name, false),
				),
			},
			{Config: unset, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}

func testAccCheckCertificateTrusted(name string, want bool) resource.TestCheckFunc {
	return func(_ *terraform.State) error {
		raw, err := acctest.Client().Call(context.Background(), "certificate.query", [][]any{{"name", "=", name}})
		if err != nil {
			return err
		}
		var rows []struct {
			Trusted bool `json:"add_to_trusted_store"`
		}
		if err := json.Unmarshal(raw, &rows); err != nil {
			return err
		}
		if len(rows) != 1 || rows[0].Trusted != want {
			return fmt.Errorf("server certificate %s: %+v, want add_to_trusted_store = %v", name, rows, want)
		}
		return nil
	}
}
