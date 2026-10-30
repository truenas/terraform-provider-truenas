// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ssh_config_test

// Adversarial bug-hunt test: set every API-accepted ssh_config attribute to a
// non-default value at once, assert the read-back re-plans empty, then import.
// ssh_config is a singleton; the box's original config is captured and
// restored in t.Cleanup. SSH management access to this box runs over the
// WebSocket API, not sshd, so touching the SSH fields cannot sever the test's
// own connection — but the test still mirrors the acceptance suite's caution
// and restores everything.

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// huntSSHRestore captures the current ssh.config and registers a t.Cleanup
// that writes it back, so the net effect of the test on the box is a no-op.
func huntSSHRestore(t *testing.T) {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "ssh.config")
	if err != nil {
		t.Fatalf("read ssh.config: %v", err)
	}
	var all map[string]any
	if err := json.Unmarshal(raw, &all); err != nil {
		t.Fatal(err)
	}
	// ssh.config returns the host keys (host_*_key*, privatekey) which
	// ssh.update rejects as "Extra inputs", so echoing the whole config back
	// fails and leaves sshd on the test port. Keep only the writable fields
	// this test changes.
	orig := map[string]any{}
	for _, k := range []string{
		"bindiface", "compression", "kerberosauth", "options", "password_login_groups",
		"passwordauth", "sftp_log_facility", "sftp_log_level", "tcpfwd", "tcpport", "weak_ciphers",
	} {
		if v, ok := all[k]; ok {
			orig[k] = v
		}
	}
	t.Cleanup(func() {
		if _, err := acctest.RestoreCall(context.Background(), "ssh.update", orig); err != nil {
			t.Errorf("RESTORE ssh.update failed: %v", err)
		}
	})
}

// TestHuntSSHConfig_allFields drives every writable ssh_config field to a
// non-default value in one apply.
func TestHuntSSHConfig_allFields(t *testing.T) {
	// Mirror the acceptance suite's gate: a singleton set/restore test that
	// calls ssh.update (restarts sshd, dropping SSH sessions — not the
	// WebSocket API this test uses) runs only on a box explicitly marked
	// disposable via TRUENAS_TEST_SSH_CONFIG.
	if os.Getenv("TRUENAS_TEST_SSH_CONFIG") == "" {
		t.Skip("set TRUENAS_TEST_SSH_CONFIG=1 (on a disposable box) to run the ssh_config all-fields hunt test")
	}
	acctest.PreCheck(t)
	huntSSHRestore(t)

	c := acctest.ProviderConfig() + `
resource "truenas_ssh_config" "test" {
  bindiface             = []
  compression           = true
  kerberosauth          = true
  options               = "MaxAuthTries 3"
  password_login_groups = ["wheel"]
  passwordauth          = true
  sftp_log_facility     = "AUTH"
  sftp_log_level        = "VERBOSE"
  tcpfwd                = true
  tcpport               = 2222
  weak_ciphers          = ["AES128-CBC"]
}
`
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: c,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("truenas_ssh_config.test", "id", "ssh_config"),
					resource.TestCheckResourceAttr("truenas_ssh_config.test", "compression", "true"),
					resource.TestCheckResourceAttr("truenas_ssh_config.test", "tcpport", "2222"),
				),
			},
			{
				Config:           c,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
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
