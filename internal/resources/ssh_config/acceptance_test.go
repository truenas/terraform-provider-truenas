package ssh_config_test

import (
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
func TestAccSSHConfig_basic(t *testing.T) {
	t.Skip("truenas_ssh_config manages a system-critical service; skipped to avoid mutating the target box's SSH configuration and risking loss of management access. See comment on TestAccSSHConfig_basic for how to safely enable this against a disposable instance.")
}
