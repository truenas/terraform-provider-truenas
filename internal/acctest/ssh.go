// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// SSH runs a shell command as root on the acceptance-test box over ssh and
// returns its stdout. Some data-safety checks have no API path: reading a
// file's bytes back (filesystem.get is a download job) or writing and
// checksumming blocks of a zvol. The user is TRUENAS_SSH_USER (default root)
// and the host comes from TRUENAS_ENDPOINT; keys must already be set up
// (BatchMode, no prompts). The test is skipped, not failed, when ssh cannot
// reach the box, with the ssh error in the reason so the runner reports it as
// an unbuilt fixture.
func SSH(t *testing.T, command string) string {
	t.Helper()
	out, err := sshRun(command)
	if err != nil {
		if strings.Contains(err.Error(), "ssh unreachable") {
			t.Skipf("ssh to the test box is not set up (%v); needed to check file/block contents", err)
		}
		t.Fatalf("ssh %q: %v", command, err)
	}
	return out
}

// sshRun is SSH without the testing.T: an "ssh unreachable" error means the
// connection itself failed (exit 255), any other error is the command's.
func sshRun(command string) (string, error) {
	user := os.Getenv("TRUENAS_SSH_USER")
	if user == "" {
		user = "root"
	}
	args := []string{"-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=no", "-o", "ConnectTimeout=10",
		user + "@" + EndpointHost(), command}
	cmd := exec.Command("ssh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()
	var err error
	select {
	case err = <-done:
	case <-time.After(2 * time.Minute):
		_ = cmd.Process.Kill()
		return "", fmt.Errorf("ssh timed out")
	}
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 255 {
			return "", fmt.Errorf("ssh unreachable: %s", strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
