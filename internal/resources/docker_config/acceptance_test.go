// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package docker_config_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccDockerConfigDataSource_basic reads the current TrueNAS Docker
// configuration through the truenas_docker_config datasource only. It
// never writes: the Docker pool/dataset underpins every running
// application, and the box's real configuration must not be touched by a
// plain acceptance test run. Runs regardless of whether Docker is
// configured — docker.config always returns a record (pool/dataset null
// when unconfigured).
func TestAccDockerConfigDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_docker_config" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_docker_config.test", "id", "docker_config"),
					resource.TestCheckResourceAttrSet("data.truenas_docker_config.test", "enable_image_updates"),
					resource.TestCheckResourceAttrSet("data.truenas_docker_config.test", "cidr_v6"),
				),
			},
		},
	})
}

// dockerConfigOriginal captures the fields TestAccDockerConfig_setAndRestore
// needs: Pool decides whether Docker is configured at all (the self-skip
// gate), EnableImageUpdates is the cosmetic field the test toggles.
type dockerConfigOriginal struct {
	Pool               *string `json:"pool"`
	EnableImageUpdates bool    `json:"enable_image_updates"`
}

// readDockerConfigOriginal reads the box's current docker.config via
// acctest.RestoreCall (docker.config is a plain sync read, job:false), so
// the test can decide whether to self-skip and can restore the exact
// original value afterward.
func readDockerConfigOriginal(t *testing.T) dockerConfigOriginal {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "docker.config")
	if err != nil {
		t.Fatalf("error reading current docker config: %v", err)
	}
	var orig dockerConfigOriginal
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("error parsing docker.config response: %v", err)
	}
	return orig
}

// restoreDockerConfig sends enable_image_updates back to its original
// value via docker.update. docker.update is job:true (probed live, see
// task-1-report.md), unlike the job:false singleton updates (e.g.
// audit.update) acctest.RestoreCall is built for — RestoreCall's
// underlying CallRead does not poll jobs to completion, so this calls
// acctest.Client().CallJob directly (the same job-aware pattern
// TestAccReplication_basic uses for replication.run) to guarantee the
// restore has actually taken effect before the test returns. It runs from
// t.Cleanup, so it restores the box even if the Terraform steps
// themselves fail partway through.
func restoreDockerConfig(t *testing.T, orig dockerConfigOriginal) {
	t.Helper()
	if _, err := acctest.Client().CallJob(context.Background(), "docker.update", map[string]any{
		"enable_image_updates": orig.EnableImageUpdates,
	}); err != nil {
		t.Fatalf("error restoring docker config: %v", err)
	}
}

// TestAccDockerConfig_setAndRestore drives the singleton
// truenas_docker_config resource's "enable_image_updates" field through
// its opposite value and back to the value read from the box before the
// test ran, then imports it. It requires TF_ACC=1 and
// TRUENAS_DISRUPTIVE=1 (acctest.DisruptiveCheck), since it mutates the
// box's live Docker configuration; a t.Cleanup-registered API restore is
// the safety net if the Terraform steps fail.
//
// SELF-SKIP: if Docker has never been configured on the target box
// (docker.config's "pool" is null), this test skips rather than running —
// mirroring the mail/ups precedent (mail.fromemail=="" /
// ups.driver+port=="" mean "unconfigured, can't restore to that exact
// state"). Here the reasoning is different but the outcome the same:
// setting up Docker requires choosing and committing a real storage pool
// (docker.update's "pool" field), which this task's hard rules forbid
// touching in a committed test, so there is no safe way to make Docker
// configured just to run this toggle test. "pool" itself is NEVER read
// from the plan or written by this test — only enable_image_updates.
func TestAccDockerConfig_setAndRestore(t *testing.T) {
	acctest.DisruptiveCheck(t)

	orig := readDockerConfigOriginal(t)
	if orig.Pool == nil || *orig.Pool == "" {
		t.Skip("Docker is unconfigured on the target box (docker.config \"pool\" is null); enable_image_updates set-and-restore requires Docker to already be configured, and this test never configures a pool itself")
	}
	t.Cleanup(func() { restoreDockerConfig(t, orig) })

	toggled := !orig.EnableImageUpdates

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccDockerConfigConfig(toggled),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_docker_config.test", "id", "docker_config"),
					resource.TestCheckResourceAttr("truenas_docker_config.test", "enable_image_updates", fmt.Sprintf("%t", toggled)),
				),
			},
			{
				Config: acctest.ProviderConfig() + testAccDockerConfigConfig(orig.EnableImageUpdates),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_docker_config.test", "enable_image_updates", fmt.Sprintf("%t", orig.EnableImageUpdates)),
				),
			},
			{
				ResourceName:      "truenas_docker_config.test",
				ImportState:       true,
				ImportStateId:     "docker_config",
				ImportStateVerify: true,
			},
		},
	})
}

func testAccDockerConfigConfig(enableImageUpdates bool) string {
	return fmt.Sprintf(`
resource "truenas_docker_config" "test" {
  enable_image_updates = %t
}
`, enableImageUpdates)
}
