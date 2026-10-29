// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_subsys_test

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func subsysCfg(name, body string) string {
	return acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_nvmet_subsys" "test" {
  name = %q
%s
}
`, name, body)
}

// TestAccNVMetSubsys_ieeeOUIAllDigits covers ieee_oui values the server
// returns as a JSON number. The DB column is INTEGER, so an all-digit value
// comes back as a number; a strict string decode failed create after the
// subsystem existed (orphaning it) and broke refresh and the data source.
//
//  1. An all-digit value in config is rejected at plan time: the kernel
//     reads it as decimal and the server drops leading zeros, so "005056"
//     would set OUI 0x0013c0, not VMware's 0x005056.
//  2. 0x-hex round-trips unchanged through the resource and data source.
//  3. An all-digit value set outside Terraform refreshes without error, in
//     both the resource and the data source.
func TestAccNVMetSubsys_ieeeOUIAllDigits(t *testing.T) {
	name := acctest.RandName("tffixoui")
	ds := fmt.Sprintf(`
data "truenas_nvmet_subsys" "test" {
  name       = %q
  depends_on = [truenas_nvmet_subsys.test]
}
`, name)
	var id int64
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckNVMetSubsysDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config:      subsysCfg(name, `  ieee_oui = "005056"`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`0x prefix`),
			},
			{
				Config: subsysCfg(name, `  ieee_oui = "0x005056"`) + ds,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nvmet_subsys.test", "ieee_oui", "0x005056"),
					resource.TestCheckResourceAttr("data.truenas_nvmet_subsys.test", "ieee_oui", "0x005056"),
					func(s *terraform.State) error {
						var err error
						id, err = strconv.ParseInt(s.RootModule().Resources["truenas_nvmet_subsys.test"].Primary.ID, 10, 64)
						return err
					},
				),
			},
			{
				PreConfig: func() {
					if _, err := acctest.Client().Call(context.Background(), "nvmet.subsys.update", id,
						map[string]any{"ieee_oui": "123456"}); err != nil {
						t.Fatalf("set all-digit ieee_oui outside Terraform: %v", err)
					}
				},
				Config: subsysCfg(name, ``) + ds,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("truenas_nvmet_subsys.test", plancheck.ResourceActionNoop),
					},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_nvmet_subsys.test", "ieee_oui", "123456"),
					resource.TestCheckResourceAttr("data.truenas_nvmet_subsys.test", "ieee_oui", "123456"),
				),
			},
		},
	})
}
