// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package zvol_test

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

// TestAccZvol_basic creates a 64M zvol, checks its attributes, resizes it up
// to 128M in place, imports it by name, and verifies destruction.
func TestAccZvol_basic(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-zv"))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckZvolDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccZvolConfig(name, 67108864, "lz4", "initial comment"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_zvol.test", "name", name),
					resource.TestCheckResourceAttr("truenas_zvol.test", "volsize", "67108864"),
					resource.TestCheckResourceAttr("truenas_zvol.test", "compression", "lz4"),
					resource.TestCheckResourceAttr("truenas_zvol.test", "comments", "initial comment"),
					resource.TestCheckResourceAttrSet("truenas_zvol.test", "pool"),
					resource.TestCheckResourceAttrSet("truenas_zvol.test", "id"),
				),
			},
			// Update in place: resize up 64M -> 128M and change the comment.
			{
				Config: acctest.ProviderConfig() + testAccZvolConfig(name, 134217728, "lz4", "updated comment"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_zvol.test", "volsize", "134217728"),
					resource.TestCheckResourceAttr("truenas_zvol.test", "comments", "updated comment"),
				),
			},
			// Import by zvol name. sparse is read from the refreservation on
			// import (B-109), so it verifies like every other attribute.
			{
				ResourceName:      "truenas_zvol.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccZvolConfig(name string, volsize int64, compression, comments string) string {
	// Full-surface: every writable property set to a non-default value so the
	// post-apply plan and ImportStateVerify prove each round-trips. sparse is
	// read back from the refreservation on import; with it set, the zvol has no
	// reservation, so reservation/refreservation are not set: on a sparse zvol they carry no local
	// value and read back null on 26.0+, so they are not portably testable here.
	return fmt.Sprintf(`
resource "truenas_zvol" "test" {
  name        = %q
  volsize     = %d
  compression = %q
  comments    = %q

  volblocksize             = 16384
  sync                     = "ALWAYS"
  dedup                    = "ON"
  checksum                 = "SHA256"
  readonly                 = "OFF"
  snapdev                  = "VISIBLE"
  copies                   = 2
  sparse                   = true
}
`, name, volsize, compression, comments)
}

// zvolQueryResult is the subset of pool.dataset.query fields this test
// package needs to distinguish a VOLUME-type dataset (zvol) from others.
type zvolQueryResult struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

func testAccCheckZvolDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "pool.dataset.query", [][]any{{"id", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking zvol %s: %v", name, err)
		}
		var results []zvolQueryResult
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing pool.dataset.query response: %v", err)
		}
		for _, r := range results {
			if r.Type == "VOLUME" {
				return fmt.Errorf("zvol %s still exists", name)
			}
		}
		return nil
	}
}

// TestAccZvol_compressionInherit guards GH #38: compression = "inherit" must
// round-trip. The zvol inherits the parent's compression, so a non-source-aware
// read resolved it to the real algorithm (e.g. "lz4") and failed with an
// inconsistent result. It is now reported as "inherit" when not set locally.
func TestAccZvol_compressionInherit(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfacc-zinh"))
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "inh" {
  name        = %q
  volsize     = 67108864
  compression = "inherit"
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckZvolDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check:  resource.TestCheckResourceAttr("truenas_zvol.inh", "compression", "inherit"),
			},
			{
				Config: cfg,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestAccZvol_encryptedGeneratedKey creates a key-based encrypted zvol with a
// generated key, exercising encryption / inherit_encryption /
// encryption_algorithm / encryption_generate_key. Mirrors truenas_dataset.
func TestAccZvol_encryptedGeneratedKey(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfacc-zenc"))
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "enc" {
  name                    = %q
  volsize                 = 67108864
  encryption              = true
  inherit_encryption      = false
  encryption_algorithm    = "AES-256-GCM"
  encryption_generate_key = true
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckZvolDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_zvol.enc", "encrypted", "true"),
					resource.TestCheckResourceAttr("truenas_zvol.enc", "key_format", "HEX"),
				),
			},
		},
	})
}

// TestAccZvol_encryptedPassphrase exercises the write-only encryption_passphrase.
func TestAccZvol_encryptedPassphrase(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfacc-zencp"))
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "encp" {
  name                  = %q
  volsize               = 67108864
  encryption            = true
  inherit_encryption    = false
  encryption_algorithm  = "AES-256-GCM"
  encryption_passphrase = "test-passphrase-1234"
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckZvolDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_zvol.encp", "encrypted", "true"),
					resource.TestCheckResourceAttr("truenas_zvol.encp", "key_format", "PASSPHRASE"),
				),
			},
		},
	})
}

// TestAccZvol_reservations sets reservation and refreservation on a
// non-sparse zvol (a sparse zvol carries no local reservation and reads them
// back null), asserts both read back, and imports.
func TestAccZvol_reservations(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfacc-zres"))
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "res" {
  name           = %q
  volsize        = 67108864
  reservation    = 33554432
  refreservation = 134217728
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckZvolDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_zvol.res", "reservation", "33554432"),
					resource.TestCheckResourceAttr("truenas_zvol.res", "refreservation", "134217728"),
				),
			},
			{
				ResourceName:      "truenas_zvol.res",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestAccZvol_encryptedExplicitKey creates a key-based encrypted zvol from an
// explicit 64-hex-character write-only encryption_key.
func TestAccZvol_encryptedExplicitKey(t *testing.T) {
	name := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tfacc-zenck"))
	cfg := acctest.ProviderConfig() + fmt.Sprintf(`
resource "truenas_zvol" "enck" {
  name                 = %q
  volsize              = 67108864
  encryption           = true
  inherit_encryption   = false
  encryption_algorithm = "AES-256-GCM"
  encryption_key       = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckZvolDestroyed(name),
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_zvol.enck", "encrypted", "true"),
					resource.TestCheckResourceAttr("truenas_zvol.enck", "key_format", "HEX"),
					resource.TestCheckResourceAttr("truenas_zvol.enck", "encryption_algorithm", "AES-256-GCM"),
				),
			},
		},
	})
}
