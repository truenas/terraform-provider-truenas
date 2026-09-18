// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package webshare_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccWebshare_basic creates a dataset fixture and a Webshare share on
// its mountpoint, checks its attributes, updates "enabled" in place (this
// resource's equivalent of smb/nfs's "comment" cosmetic update field —
// sharing.webshare has no comment field at all, probed live), imports it by
// its numeric id, and verifies destruction of both the share and the
// dataset fixture.
//
// Requires TrueNAS 26.0+: the sharing.webshare namespace does not
// exist on earlier releases (probed live — TrueNAS 25.10 exposes 0
// webshare.*/sharing.webshare.* methods via core.get_methods), so this test
// self-skips cleanly via acctest.ServerVersionAtLeast on any older box,
// matching this resource's own version-gate diagnostic (see resource.go's
// checkVersion / model.go's versionGateDiagnostics).
//
// SAFETY: uses a RandName-suffixed dataset + share name and creates/
// destroys only its own objects (never touches any pre-existing share),
// matching the smb/nfs/container Tier-1 convention.
func TestAccWebshare_basic(t *testing.T) {
	if !acctest.ServerVersionAtLeast(t, 26, 0) {
		t.Skip("truenas_webshare requires TrueNAS 26.0 or later (sharing.webshare namespace absent on 25.10, confirmed live)")
	}

	datasetName := fmt.Sprintf("%s/%s", acctest.TestPool(), acctest.RandName("tf-acc-ds-webshare"))
	shareName := acctest.RandName("tf-acc-webshare")

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		CheckDestroy: resource.ComposeAggregateTestCheckFunc(
			testAccCheckWebshareDestroyed(shareName),
			testAccCheckWebshareDatasetDestroyed(datasetName),
		),
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccWebshareConfig(datasetName, shareName, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_webshare.test", "name", shareName),
					resource.TestCheckResourceAttr("truenas_webshare.test", "enabled", "true"),
					resource.TestCheckResourceAttr("truenas_webshare.test", "is_home_base", "false"),
					resource.TestCheckResourceAttrSet("truenas_webshare.test", "path"),
					resource.TestCheckResourceAttrSet("truenas_webshare.test", "id"),
				),
			},
			// Update in place: toggle enabled (this resource's cosmetic
			// equivalent of smb/nfs's "comment" — see the design doc's
			// probe evidence that sharing.webshare has no comment field).
			{
				Config: acctest.ProviderConfig() + testAccWebshareConfig(datasetName, shareName, false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_webshare.test", "enabled", "false"),
				),
			},
			// Import by the share's numeric id.
			{
				ResourceName:      "truenas_webshare.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccWebshareConfig(datasetName, shareName string, enabled bool) string {
	return fmt.Sprintf(`
resource "truenas_dataset" "test" {
  name = %q
}

resource "truenas_webshare" "test" {
  path    = truenas_dataset.test.mountpoint
  name    = %q
  enabled = %v
}
`, datasetName, shareName, enabled)
}

func testAccCheckWebshareDestroyed(shareName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "sharing.webshare.query", [][]any{{"name", "=", shareName}})
		if err != nil {
			return fmt.Errorf("error checking Webshare share %s: %v", shareName, err)
		}
		var results []struct {
			ID int64 `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing sharing.webshare.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("Webshare share %s still exists", shareName)
		}
		return nil
	}
}

func testAccCheckWebshareDatasetDestroyed(name string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		c := acctest.Client()
		raw, err := c.Call(context.Background(), "pool.dataset.query", [][]any{{"id", "=", name}})
		if err != nil {
			return fmt.Errorf("error checking dataset %s: %v", name, err)
		}
		var results []struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(raw, &results); err != nil {
			return fmt.Errorf("error parsing pool.dataset.query response: %v", err)
		}
		if len(results) > 0 {
			return fmt.Errorf("dataset %s still exists", name)
		}
		return nil
	}
}
