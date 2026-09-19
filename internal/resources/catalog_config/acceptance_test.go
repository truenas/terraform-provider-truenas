// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package catalog_config_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccCatalogConfigDataSource_basic reads the current TrueNAS app catalog
// configuration through the truenas_catalog_config datasource only. It never
// writes. Runs regardless of whether Docker/apps are configured —
// catalog.config and catalog.trains both succeeded live on a TrueNAS 25.10 box
// with Docker/apps never configured (see task-3-report.md), unlike
// docker.config's pool/dataset which read back null there.
func TestAccCatalogConfigDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_catalog_config" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_catalog_config.test", "id", "catalog_config"),
					resource.TestCheckResourceAttrSet("data.truenas_catalog_config.test", "label"),
					resource.TestCheckResourceAttrSet("data.truenas_catalog_config.test", "location"),
					resource.TestCheckResourceAttrSet("data.truenas_catalog_config.test", "preferred_trains.#"),
					resource.TestCheckResourceAttrSet("data.truenas_catalog_config.test", "trains.#"),
				),
			},
		},
	})
}

// catalogConfigOriginal captures preferred_trains as read from catalog.config
// before TestAccCatalogConfig_setAndRestore runs.
type catalogConfigOriginal struct {
	PreferredTrains []string `json:"preferred_trains"`
}

// readCatalogConfigOriginal reads the box's current catalog.config via
// acctest.RestoreCall (job:false, probed live on both TrueNAS 25.10 and 26.0 —
// see task-3-report.md), so the test can decide which train to toggle and
// restore the exact original list afterward. Returns ok=false (rather than
// failing the test outright) when the read itself errors, so the caller can
// self-skip — mirroring the mail/ups precedent — even though in practice
// catalog.config succeeded on every probed box, including a TrueNAS 25.10 VM
// with Docker/apps completely unconfigured.
func readCatalogConfigOriginal(t *testing.T) (catalogConfigOriginal, bool) {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "catalog.config")
	if err != nil {
		return catalogConfigOriginal{}, false
	}
	var orig catalogConfigOriginal
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing catalog.config response: %v", err)
	}
	return orig, true
}

// restoreCatalogConfig sends preferred_trains back to its original value via
// catalog.update. catalog.update is job:false (probed live on both TrueNAS
// 25.10 and 26.0 — see task-3-report.md), so — unlike docker_config's
// CallJob-based restore — the plain acctest.RestoreCall (which wraps a
// job-unaware CallRead) is sufficient here.
func restoreCatalogConfig(t *testing.T, orig catalogConfigOriginal) {
	t.Helper()
	if _, err := acctest.RestoreCall(context.Background(), "catalog.update", map[string]any{
		"preferred_trains": orig.PreferredTrains,
	}); err != nil {
		t.Fatalf("error restoring catalog config: %v", err)
	}
}

// containsTrain reports whether name is present in trains.
func containsTrain(trains []string, name string) bool {
	for _, tr := range trains {
		if tr == name {
			return true
		}
	}
	return false
}

// TestAccCatalogConfig_setAndRestore drives the singleton
// truenas_catalog_config resource's "preferred_trains" field through one
// train removed (if "community" is currently preferred — the observed state
// on both probed boxes) or added (if it is absent), then back to the value
// read from the box before the test ran, then imports it. It requires
// TF_ACC=1 and TRUENAS_DISRUPTIVE=1 (acctest.DisruptiveCheck), since it
// mutates the box's live catalog preferences; a t.Cleanup-registered API
// restore is the safety net if the Terraform steps fail.
//
// SELF-SKIP: if catalog.config itself errors when read, this test skips
// rather than running — mirroring the mail/ups precedent. This is a
// defensive fallback rather than something either probed box actually hit:
// catalog.config AND catalog.update both succeeded live on TrueNAS 25.10 even
// with Docker/apps completely unconfigured there (decisive probe — a
// preferred_trains no-op update round-tripped cleanly; see task-3-report.md
// for the full transcript), unlike docker.update, which requires a
// configured pool. preferred_trains is cosmetic app-catalog metadata (it
// only affects which trains are shown/preferred when browsing the catalog
// UI; it does not install, remove, start, or stop anything), so this test
// is judged safe to run on whichever box TRUENAS_ENDPOINT points at,
// production-serving 26.0 included — see task-3-report.md for the full
// judgment.
func TestAccCatalogConfig_setAndRestore(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig, ok := readCatalogConfigOriginal(t)
	if !ok {
		t.Skip("catalog.config could not be read on the target box; preferred_trains set-and-restore requires a readable catalog configuration")
	}
	t.Cleanup(func() { restoreCatalogConfig(t, orig) })

	var toggled []string
	if containsTrain(orig.PreferredTrains, "community") {
		for _, tr := range orig.PreferredTrains {
			if tr != "community" {
				toggled = append(toggled, tr)
			}
		}
	} else {
		toggled = append(append([]string{}, orig.PreferredTrains...), "community")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccCatalogConfigConfig(toggled),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_catalog_config.test", "id", "catalog_config"),
					resource.TestCheckResourceAttr("truenas_catalog_config.test", "preferred_trains.#", fmt.Sprintf("%d", len(toggled))),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccCatalogConfigConfig(orig.PreferredTrains),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_catalog_config.test", "preferred_trains.#", fmt.Sprintf("%d", len(orig.PreferredTrains))),
				),
			},
			{
				ResourceName:      "truenas_catalog_config.test",
				ImportState:       true,
				ImportStateId:     "catalog_config",
				ImportStateVerify: true,
			},
		},
	})
}

func testAccCatalogConfigConfig(trains []string) string {
	quoted := make([]string, len(trains))
	for i, tr := range trains {
		quoted[i] = fmt.Sprintf("%q", tr)
	}
	return fmt.Sprintf(`
resource "truenas_catalog_config" "test" {
  preferred_trains = [%s]
}
`, strings.Join(quoted, ", "))
}
