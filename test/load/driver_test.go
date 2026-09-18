// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package load

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// buildProvider compiles the provider into a temp dir and returns that dir.
func buildProvider(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "terraform-provider-truenas")
	root, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		t.Fatalf("locate repo root: %v", err)
	}
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = string(bytes.TrimSpace(root))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build provider: %v\n%s", err, out)
	}
	return dir
}

// writeDevOverrides writes a CLI config pointing truenas/truenas at binDir and
// returns its path (for TF_CLI_CONFIG_FILE).
func writeDevOverrides(t *testing.T, binDir string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "dev.tfrc")
	body := fmt.Sprintf(`provider_installation {
  dev_overrides { "truenas/truenas" = %q }
  direct {}
}
`, binDir)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write tfrc: %v", err)
	}
	return p
}

type run struct{ dir string }

// terraform runs the terraform binary in r.dir with the given CLI config and
// TF_LOG=INFO captured into r.dir/tf.log. Returns combined stdout+stderr.
func (r run) terraform(t *testing.T, cliCfg string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("terraform", args...)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(),
		"TF_CLI_CONFIG_FILE="+cliCfg,
		"TF_LOG=INFO",
		"TF_LOG_PATH="+filepath.Join(r.dir, "tf.log"),
	)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	return out.String(), err
}
