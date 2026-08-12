// Copyright iXsystems, Inc. 2026
// SPDX-License-Identifier: MPL-2.0

package load

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

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

// TestLoad_ConcurrentApplies runs LOAD_CONCURRENT_APPLIES independent applies
// at once, stacking auth pressure like multiple CI pipelines. Every run must
// converge and destroy cleanly.
func TestLoad_ConcurrentApplies(t *testing.T) {
	loadtest.LoadCheck(t)
	binDir := buildProvider(t)
	cliCfg := writeDevOverrides(t, binDir)
	k := envInt("LOAD_CONCURRENT_APPLIES", 6)
	perRun := envInt("LOAD_RESOURCES", 150) / k
	if perRun < 1 {
		perRun = 1
	}
	var wg sync.WaitGroup
	for i := 0; i < k; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			applyDestroy(t, cliCfg, perRun, envInt("LOAD_PARALLELISM", 20))
		}()
	}
	wg.Wait()
}

// TestLoad_Sustained loops apply/destroy for LOAD_DURATION and asserts no
// leaked tf-load- objects remain at the end.
func TestLoad_Sustained(t *testing.T) {
	loadtest.LoadCheck(t)
	binDir := buildProvider(t)
	cliCfg := writeDevOverrides(t, binDir)
	dur, err := time.ParseDuration(os.Getenv("LOAD_DURATION"))
	if err != nil {
		dur = 10 * time.Minute
	}
	deadline := time.Now().Add(dur)
	iter := 0
	for time.Now().Before(deadline) {
		iter++
		applyDestroy(t, cliCfg, 20, envInt("LOAD_PARALLELISM", 20))
	}
	t.Logf("sustained: completed %d apply/destroy iterations over %s", iter, dur)
	c := acctest.Client()
	left, err := loadtest.Sweep(context.Background(), c, acctest.TestPool())
	if err != nil {
		t.Fatalf("final sweep: %v", err)
	}
	if left > 0 {
		t.Fatalf("%d tf-load- objects leaked after sustained run", left)
	}
}
