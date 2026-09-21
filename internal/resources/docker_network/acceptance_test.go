// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package docker_network_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// dockerConfigPool is the subset of docker.config this test reads to
// decide whether Docker is configured on the target box.
type dockerConfigPool struct {
	Pool *string `json:"pool"`
}

// isDockerConfigured reads docker.config and reports whether Docker has a
// pool assigned — the same "unconfigured" signal
// TestAccDockerConfig_setAndRestore self-skips on (mail/ups precedent, see
// docker_config's acceptance_test.go).
func isDockerConfigured(t *testing.T) bool {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "docker.config")
	if err != nil {
		t.Fatalf("error reading current docker config: %v", err)
	}
	var cfg dockerConfigPool
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("error parsing docker.config response: %v", err)
	}
	return cfg.Pool != nil && *cfg.Pool != ""
}

// TestAccDockerNetworkDataSource_basic looks up Docker's built-in "bridge"
// network by name through the truenas_docker_network datasource. "bridge"
// (along with "host"/"none") is created by the Docker daemon itself
// whenever Docker is running, independent of which applications (if any)
// are installed, so it's a stable lookup target rather than an
// app-specific network name that could disappear if an app is uninstalled.
//
// SELF-SKIP: docker.network.query returns an empty list when Docker is
// unconfigured (probed live on TrueNAS 25.10, see task-1-report.md) — there
// is no network to look up, and this task's hard rules forbid configuring
// a pool (docker.update's "pool" field) just to make one exist, so the
// test self-skips rather than failing on "not found".
func TestAccDockerNetworkDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}
	acctest.PreCheck(t)

	if !isDockerConfigured(t) {
		t.Skip("Docker is unconfigured on the target box (docker.config \"pool\" is null); no Docker networks exist to look up, and this test never configures a pool itself")
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_docker_network" "test" {
  name = "bridge"
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_docker_network.test", "name", "bridge"),
					resource.TestCheckResourceAttrSet("data.truenas_docker_network.test", "id"),
					resource.TestCheckResourceAttrSet("data.truenas_docker_network.test", "driver"),
					resource.TestCheckResourceAttrSet("data.truenas_docker_network.test", "scope"),
				),
			},
		},
	})
}
