// Copyright iXsystems, Inc. 2026
// SPDX-License-Identifier: MPL-2.0

package load

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/truenas/terraform-provider-truenas/internal/acctest"
	"github.com/truenas/terraform-provider-truenas/internal/loadtest"
)

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// applyDestroy generates a config of n datasets, applies at the given
// parallelism, then destroys. Fails on any error surfaced to the user, and
// on any "Rate Limit Exceeded" in Terraform's output. Returns the apply log.
func applyDestroy(t *testing.T, cliCfg string, n, parallelism int) {
	t.Helper()
	r := run{dir: t.TempDir()}
	cfg := GenerateConfig(acctest.TestPool(), n, false)
	if err := os.WriteFile(filepath.Join(r.dir, "main.tf"), []byte(cfg), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	// dev_overrides: no init needed.
	out, err := r.terraform(t, cliCfg, "apply", "-auto-approve",
		"-parallelism="+strconv.Itoa(parallelism))
	t.Cleanup(func() {
		_, _ = r.terraform(t, cliCfg, "destroy", "-auto-approve", "-parallelism="+strconv.Itoa(parallelism))
		if c := acctest.Client(); c != nil {
			_, _ = loadtest.Sweep(context.Background(), c, acctest.TestPool())
		}
	})
	if err != nil {
		t.Fatalf("apply failed: %v\n%s", err, out)
	}
	if strings.Contains(out, "Rate Limit Exceeded") {
		t.Fatalf("rate-limit surfaced to user during apply:\n%s", out)
	}
}

func TestLoad_TerraformApply(t *testing.T) {
	loadtest.LoadCheck(t)
	binDir := buildProvider(t)
	cliCfg := writeDevOverrides(t, binDir)
	applyDestroy(t, cliCfg, envInt("LOAD_RESOURCES", 150), envInt("LOAD_PARALLELISM", 20))
}
