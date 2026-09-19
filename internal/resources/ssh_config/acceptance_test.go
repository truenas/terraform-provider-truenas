// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ssh_config_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccSSHConfigDataSource_basic reads the current TrueNAS SSH
// configuration through the truenas_ssh_config datasource only. It never
// writes: SSH is a system-critical singleton service (management access to
// the box may depend on it), and the box's real SSH configuration must not
// be overwritten by an acceptance test run.
//
// Requires:
//   - TF_ACC=1
//   - TRUENAS_API_KEY (or TRUENAS_USERNAME+TRUENAS_PASSWORD) set for
//     acctest.PreCheck
//   - A running TrueNAS instance
func TestAccSSHConfigDataSource_basic(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped: set TF_ACC=1 and ensure a TrueNAS instance is available")
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + `
data "truenas_ssh_config" "test" {}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.truenas_ssh_config.test", "id", "ssh_config"),
					resource.TestCheckResourceAttrSet("data.truenas_ssh_config.test", "tcpport"),
				),
			},
		},
	})
}

// TestAccSSHConfig_basic is intentionally skipped by default.
// truenas_ssh_config is a SINGLETON resource that manages a system-critical
// service: bringing it under Terraform management mutates the box's actual
// SSH configuration (ssh.update), and a naive create/update/destroy
// acceptance test risks locking out management access to whatever system
// runs it (e.g. by rewriting tcpport, passwordauth, or bindiface).
//
// If this test is ever enabled against a disposable/non-production TrueNAS
// instance, it should:
//  1. Read the current config via the datasource first.
//  2. Only touch "options" (a low-risk, additive string field) in the
//     resource config, driving it through a value and back to the value the
//     datasource observed in step 1, so the net effect on the box is a
//     no-op. Never touch tcpport, passwordauth, bindiface, or kerberosauth
//     in an automated test: changing those on a live system can sever
//     management access.
//  3. Use ImportState with ImportStateId "ssh_config" to verify import
//     normalizes any ID to the fixed singleton ID.
//
// It is gated behind TRUENAS_TEST_SSH_CONFIG (not just TF_ACC): any ssh.update
// restarts sshd (dropping SSH sessions, though not the WebSocket API this test
// uses). It touches only `compression` — a structured bool that cannot malform
// sshd_config and never affects the port, auth, or bind interface — and a
// t.Cleanup restores the box's original value regardless of outcome. Run only
// against a disposable box.
func TestAccSSHConfig_basic(t *testing.T) {
	if os.Getenv("TRUENAS_TEST_SSH_CONFIG") == "" {
		t.Skip("set TRUENAS_TEST_SSH_CONFIG=1 (on a disposable box) to run the ssh_config set/restore test")
	}
	acctest.PreCheck(t)

	// Capture the box's current compression setting and restore it afterward,
	// so the net effect of the test is a no-op.
	orig := currentSSHCompression(t)
	t.Cleanup(func() {
		if _, err := acctest.Client().Call(context.Background(), "ssh.update",
			map[string]any{"compression": orig}); err != nil {
			t.Logf("WARNING: failed to restore ssh compression=%v: %v", orig, err)
		}
	})

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: acctest.ProviderConfig() + testAccSSHConfigCompression(true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_ssh_config.test", "id", "ssh_config"),
					resource.TestCheckResourceAttr("truenas_ssh_config.test", "compression", "true"),
				),
			},
			// Update path: flip it back off in place.
			{
				Config: acctest.ProviderConfig() + testAccSSHConfigCompression(false),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_ssh_config.test", "compression", "false"),
				),
			},
			{
				ResourceName:      "truenas_ssh_config.test",
				ImportState:       true,
				ImportStateId:     "ssh_config",
				ImportStateVerify: true,
			},
		},
	})
}

func testAccSSHConfigCompression(v bool) string {
	return fmt.Sprintf(`
resource "truenas_ssh_config" "test" {
  compression = %v
}
`, v)
}

// currentSSHCompression reads the box's current ssh.config compression bool.
func currentSSHCompression(t *testing.T) bool {
	t.Helper()
	raw, err := acctest.Client().Call(context.Background(), "ssh.config")
	if err != nil {
		t.Fatalf("reading ssh.config: %v", err)
	}
	var cfg struct {
		Compression bool `json:"compression"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parsing ssh.config: %v", err)
	}
	return cfg.Compression
}
