// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset_test

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestHuntDataset_allFields is an adversarial value round-trip: it sets every
// writable attribute a plain FILESYSTEM dataset accepts to a non-default value
// in one config, then proves each one round-trips by (1) applying, (2)
// re-planning the identical config and asserting an empty plan, and (3)
// importing with ImportStateVerify.
//
// It extends the acceptance full-surface config (testAccDatasetConfig) with the
// attributes that config omits: readonly (a source-aware ZFS enum) and
// force_destroy (a state-only delete option). It also uses more adversarial
// values than the acceptance test — aclmode RESTRICTED, checksum SHA512, copies
// 3, recordsize 1M. The VOLUME-only attributes (type, volsize, volblocksize) and
// the encryption cluster (encryption*, share_type) are exercised by truenas_zvol
// and the dataset encryption acceptance tests respectively, so they are left out.
//
// force_destroy is Optional+Computed with a false default and is reset to false
// on import (it is a state-only flag the API never returns), so it is the one
// attribute ImportStateVerify must ignore.
func TestHuntDataset_allFields(t *testing.T) {
	acctest.DisruptiveCheck(t)
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfhunt-ds"))
	cfg := acctest.ProviderConfig() + huntDatasetAllFieldsConfig(name)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckDatasetDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_dataset.hunt", "name", name),
					resource.TestCheckResourceAttr("truenas_dataset.hunt", "compression", "zstd"),
					resource.TestCheckResourceAttr("truenas_dataset.hunt", "aclmode", "RESTRICTED"),
					resource.TestCheckResourceAttr("truenas_dataset.hunt", "acltype", "nfsv4"),
					resource.TestCheckResourceAttr("truenas_dataset.hunt", "atime", "OFF"),
					resource.TestCheckResourceAttr("truenas_dataset.hunt", "exec", "OFF"),
					resource.TestCheckResourceAttr("truenas_dataset.hunt", "readonly", "ON"),
					resource.TestCheckResourceAttr("truenas_dataset.hunt", "checksum", "SHA512"),
					resource.TestCheckResourceAttr("truenas_dataset.hunt", "copies", "3"),
					resource.TestCheckResourceAttr("truenas_dataset.hunt", "dedup", "ON"),
					resource.TestCheckResourceAttr("truenas_dataset.hunt", "quota", "2147483648"),
					resource.TestCheckResourceAttr("truenas_dataset.hunt", "refquota", "1073741824"),
					resource.TestCheckResourceAttr("truenas_dataset.hunt", "reservation", "10485760"),
					resource.TestCheckResourceAttr("truenas_dataset.hunt", "refreservation", "10485760"),
					resource.TestCheckResourceAttr("truenas_dataset.hunt", "recordsize", "1M"),
					resource.TestCheckResourceAttr("truenas_dataset.hunt", "snapdir", "VISIBLE"),
					resource.TestCheckResourceAttr("truenas_dataset.hunt", "sync", "ALWAYS"),
				),
			},
			{
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:            "truenas_dataset.hunt",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"force_destroy"},
			},
		},
	})
}

// huntDatasetAllFieldsConfig sets every writable FILESYSTEM attribute to a
// non-default value. acltype is lowercase-canonical (nfsv4); aclmode/checksum/
// sync are uppercase. recordsize is case-insensitive and stored uppercase (1M).
func huntDatasetAllFieldsConfig(name string) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "hunt" {
  name        = %q
  compression = "zstd"
  comments    = "hunt all fields"

  aclmode                  = "RESTRICTED"
  acltype                  = "nfsv4"
  atime                    = "OFF"
  exec                     = "OFF"
  readonly                 = "ON"
  checksum                 = "SHA512"
  copies                   = 3
  dedup                    = "ON"
  quota                    = 2147483648
  refquota                 = 1073741824
  reservation              = 10485760
  refreservation           = 10485760
  recordsize               = "1M"
  snapdir                  = "VISIBLE"
  special_small_block_size = 0
  sync                     = "ALWAYS"
  force_destroy            = true
}
`, name)
}

// Hunt F7: GZIP-6 is ZFS's default gzip level; 28.0 stores it as "gzip" and
// 25.10/26.0 rejected the alias. It must create, read back, and re-plan empty
// on every release.
func TestHuntDataset_compressionGzip6Alias(t *testing.T) {
	name := acctest.TestPool() + "/" + acctest.RandName("tfhuntgz6")
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_dataset" "gz" {
  name        = %q
  compression = "GZIP-6"
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{Config: cfg, Check: resource.TestCheckResourceAttr("truenas_dataset.gz", "compression", "GZIP-6")},
			{Config: cfg, ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
		},
	})
}
