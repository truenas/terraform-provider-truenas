// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package zvol_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntZvol_allFields is an adversarial value round-trip for a non-sparse
// (thick) zvol: it sets every writable attribute to a non-default value in one
// config, then proves each round-trips by (1) applying, (2) re-planning the
// identical config and asserting an empty plan, and (3) importing with
// ImportStateVerify.
//
// A single config cannot cover both the sparse and the reservation paths: a
// sparse zvol carries no refreservation, so reservation/refreservation read back
// null. This test takes the thick path (sparse = false) so reservation and
// refreservation are set and read back, extending the acceptance full-surface
// config (which is sparse and omits them). The encryption cluster is covered by
// the zvol encryption acceptance tests.
//
// force_destroy is Optional+Computed with a false default and is reset to false
// on import, so it is the one attribute ImportStateVerify must ignore.
func TestHuntZvol_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfhunt-zv"))
	cfg := acctest.ProviderConfig() + huntZvolAllFieldsConfig(name)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckZvolDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_zvol.hunt", "name", name),
					resource.TestCheckResourceAttr("truenas_zvol.hunt", "volsize", "134217728"),
					resource.TestCheckResourceAttr("truenas_zvol.hunt", "volblocksize", "16384"),
					resource.TestCheckResourceAttr("truenas_zvol.hunt", "compression", "zstd"),
					resource.TestCheckResourceAttr("truenas_zvol.hunt", "sync", "ALWAYS"),
					resource.TestCheckResourceAttr("truenas_zvol.hunt", "dedup", "ON"),
					resource.TestCheckResourceAttr("truenas_zvol.hunt", "checksum", "SHA512"),
					resource.TestCheckResourceAttr("truenas_zvol.hunt", "readonly", "ON"),
					resource.TestCheckResourceAttr("truenas_zvol.hunt", "snapdev", "VISIBLE"),
					resource.TestCheckResourceAttr("truenas_zvol.hunt", "copies", "2"),
					resource.TestCheckResourceAttr("truenas_zvol.hunt", "sparse", "false"),
					resource.TestCheckResourceAttr("truenas_zvol.hunt", "reservation", "33554432"),
					resource.TestCheckResourceAttr("truenas_zvol.hunt", "refreservation", "134217728"),
					resource.TestCheckResourceAttr("truenas_zvol.hunt", "comments", "hunt all fields"),
				),
			},
			{
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:            "truenas_zvol.hunt",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"force_destroy"},
			},
		},
	})
}

// huntZvolAllFieldsConfig sets every writable attribute to a non-default value.
// checksum/sync/dedup/readonly/snapdev are uppercase enums; a strong checksum
// (SHA512) is required alongside dedup = ON. sparse = false keeps the zvol thick
// so reservation and refreservation carry local values and read back.
func huntZvolAllFieldsConfig(name string) string {
	return fmt.Sprintf(`
resource "truenas_zvol" "hunt" {
  name        = %q
  volsize     = 134217728
  compression = "zstd"
  comments    = "hunt all fields"

  volblocksize   = 16384
  sync           = "ALWAYS"
  dedup          = "ON"
  checksum       = "SHA512"
  readonly       = "ON"
  snapdev        = "VISIBLE"
  copies         = 2
  sparse         = false
  reservation    = 33554432
  refreservation = 134217728
  force_destroy  = true
}
`, name)
}

// Hunt F7 (zvol variant): GZIP-6 must create, read back and re-plan empty.
func TestHuntZvol_compressionGzip6Alias(t *testing.T) {
	name := acctest.TestPool() + "/" + acctest.RandName("tfhuntzgz6")
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "gz" {
  name        = %q
  volsize     = 16777216
  sparse      = true
  compression = "GZIP-6"
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, Check: resource.TestCheckResourceAttr("truenas_zvol.gz", "compression", "GZIP-6")},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}
