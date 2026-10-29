// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cronjob_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccCronjob_dataSourceAmbiguous: two jobs with the same description;
// the lookup silently returned the first (B-131). It is now an error.
func TestAccCronjob_dataSourceAmbiguous(t *testing.T) {
	desc := acctest.RandName("tffix3shtasks-cron")
	one := testAccCronjobConfig(desc)
	two := strings.Replace(one, `resource "truenas_cronjob" "test"`, `resource "truenas_cronjob" "other"`, 1)
	base := acctest.ProviderConfig() + one + two
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: base},
			{
				Config: base + `
data "truenas_cronjob" "d" {
  description = truenas_cronjob.test.description
  depends_on  = [truenas_cronjob.other]
}
`,
				ExpectError: regexp.MustCompile(`Ambiguous lookup`),
			},
		},
	})
}
