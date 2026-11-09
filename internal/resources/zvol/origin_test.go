// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package zvol_test

// origin: a zvol cloned from a golden snapshot (pool.snapshot.clone). The
// golden gets a signature over ssh before the snapshot; the clone must read
// the same bytes, grow to its own volsize, read its origin back on import and
// plan empty afterwards. Destroy order: the clone (references the snapshot)
// goes before the snapshot, the snapshot before the golden.

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

func TestAccZvol_originClone(t *testing.T) {
	acctest.PreCheck(t)
	golden := acctest.TestPool() + "/" + acctest.RandName("tfaccgold")
	clone := acctest.TestPool() + "/" + acctest.RandName("tfaccclone")
	const ddRange = "bs=1M skip=2 count=1"
	// cfg has the golden, its snapshot and (when withClone) the clone, which
	// is twice the golden's size, thick, and takes its origin from the
	// snapshot resource's id ("dataset@name").
	cfg := func(withClone bool, cloneExtra string) string {
		s := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "golden" {
  name          = %q
  volsize       = 16777216
  sparse        = true
  force_destroy = true # holds the signature and the snapshot
}
resource "truenas_snapshot" "g" {
  dataset = truenas_zvol.golden.name
  name    = "golden"
}
`, golden)
		if withClone {
			s += fmt.Sprintf(`
resource "truenas_zvol" "clone" {
  name          = %q
  origin        = truenas_snapshot.g.id
  volsize       = 33554432
  sparse        = false
  comments      = "cloned from golden"
  force_destroy = true # shares the signature blocks
%s}
`, clone, cloneExtra)
		}
		return s
	}
	var sum string
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Golden + snapshot first; the snapshot resource is created after
			// the signature is written? No: write the signature, then take the
			// snapshot in a second step so the snapshot holds the bytes.
			{Config: acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "golden" {
  name          = %q
  volsize       = 16777216
  sparse        = true
  force_destroy = true
}
`, golden), Check: func(*terraform.State) error {
				acctest.SSH(t, "dd if=/dev/urandom of=/dev/zvol/"+golden+" bs=1M seek=2 count=1 conv=fsync status=none")
				sum = strings.TrimSpace(acctest.SSH(t, "dd if=/dev/zvol/"+golden+" "+ddRange+" status=none | sha256sum"))
				if len(sum) < 64 {
					return fmt.Errorf("could not checksum the golden: %q", sum)
				}
				return nil
			}},
			{Config: cfg(false, "")},
			{
				Config: cfg(true, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("truenas_zvol.clone", plancheck.ResourceActionCreate),
				}},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_zvol.clone", "origin", golden+"@golden"),
					resource.TestCheckResourceAttr("truenas_zvol.clone", "volsize", "33554432"),
					resource.TestCheckResourceAttr("truenas_zvol.clone", "refreservation", "33554432"),
					resource.TestCheckResourceAttr("truenas_zvol.clone", "sparse", "false"),
					resource.TestCheckResourceAttr("truenas_zvol.clone", "comments", "cloned from golden"),
					resource.TestCheckNoResourceAttr("truenas_zvol.golden", "origin"),
					func(s *terraform.State) error {
						got := strings.TrimSpace(acctest.SSH(t, "dd if=/dev/zvol/"+clone+" "+ddRange+" status=none | sha256sum"))
						if got != sum {
							return fmt.Errorf("clone block checksum %s, want the golden's %s", got, sum)
						}
						return nil
					},
				),
			},
			{Config: cfg(true, ""), ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}},
			// Import by name reads origin back, so the config plans nothing.
			{Config: cfg(true, ""), ResourceName: "truenas_zvol.clone", ImportState: true, ImportStateId: clone,
				ImportStateVerify: true, ImportStateVerifyIgnore: []string{"force_destroy"}},
			// Writing to the clone diverges it; the golden keeps its bytes.
			{Config: cfg(true, ""), Check: func(*terraform.State) error {
				acctest.SSH(t, "dd if=/dev/zero of=/dev/zvol/"+clone+" bs=1M seek=2 count=1 conv=fsync status=none")
				got := strings.TrimSpace(acctest.SSH(t, "dd if=/dev/zvol/"+golden+" "+ddRange+" status=none | sha256sum"))
				if got != sum {
					return fmt.Errorf("golden changed after writing to the clone: %s, want %s", got, sum)
				}
				return nil
			}},
			// A different origin or volblocksize cannot be applied in place:
			// origin replaces; a volblocksize unlike the origin's is refused.
			{
				Config:      cfg(true, "  volblocksize  = 65536\n"),
				ExpectError: regexp.MustCompile(`volblocksize cannot differ from the origin's`),
			},
			// Back to the clean clone config, then the harness destroys clone
			// -> snapshot -> golden.
			{Config: cfg(true, "")},
		},
	})
}

// TestAccZvol_originRejectsEncryption: origin and the encryption inputs are
// mutually exclusive at plan time.
func TestAccZvol_originRejectsEncryption(t *testing.T) {
	acctest.PreCheck(t)
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "c" {
  name       = %q
  origin     = "%s/nothing@snap"
  volsize    = 16777216
  encryption = true
}
`, acctest.TestPool()+"/"+acctest.RandName("tfaccoenc"), acctest.TestPool()),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`origin cannot be combined with encryption settings`),
			},
			{
				Config: acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "c" {
  name    = %q
  origin  = "not-a-snapshot"
  volsize = 16777216
}
`, acctest.TestPool()+"/"+acctest.RandName("tfaccobad")),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`origin must name a snapshot`),
			},
		},
	})
}
