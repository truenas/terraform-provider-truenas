// Copyright TrueNAS 2026
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

// applyDestroy generates a config of n uniquely-named datasets, applies at the
// given parallelism, then destroys them synchronously before returning — so a
// sustained loop does not accumulate datasets and concurrent callers never
// collide on names. It marks the test failed with t.Errorf (safe to call from
// a goroutine) on any surfaced error or rate-limit; it never calls t.Fatalf.
func applyDestroy(t *testing.T, cliCfg string, n, parallelism int) {
	t.Helper()
	r := run{dir: t.TempDir()}
	prefix := acctest.RandName("tf-load")
	cfg := GenerateConfig(acctest.TestPool(), prefix, n, false)
	if err := os.WriteFile(filepath.Join(r.dir, "main.tf"), []byte(cfg), 0o644); err != nil {
		t.Errorf("write config: %v", err)
		return
	}
	// Destroy exactly this run's own Terraform state before returning. Scoped
	// to this call, so it never touches another goroutine's datasets (unlike
	// the broad tf-load- Sweep, which is a test-level backstop only).
	defer func() {
		_, _ = r.terraform(t, cliCfg, "destroy", "-auto-approve", "-parallelism="+strconv.Itoa(parallelism))
	}()
	out, err := r.terraform(t, cliCfg, "apply", "-auto-approve", "-parallelism="+strconv.Itoa(parallelism))
	if err != nil {
		t.Errorf("apply failed: %v\n%s", err, out)
		return
	}
	if strings.Contains(out, "Rate Limit Exceeded") {
		t.Errorf("rate-limit surfaced to user during apply:\n%s", out)
	}
}

func TestLoad_TerraformApply(t *testing.T) {
	loadtest.LoadCheck(t)
	binDir := buildProvider(t)
	cliCfg := writeDevOverrides(t, binDir)
	t.Cleanup(func() {
		if c := acctest.Client(); c != nil {
			_, _ = loadtest.Sweep(context.Background(), c, acctest.TestPool())
		}
	})
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
	if c := acctest.Client(); c != nil {
		_, _ = loadtest.Sweep(context.Background(), c, acctest.TestPool())
	}
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

// applyMixed writes a mixed config of n-per-type resources, applies it, then
// re-applies a variant (an in-place update), then destroys — exercising
// Create, Read, and Update across many resource types under load. Failures use
// t.Errorf (never t.Fatalf) to stay goroutine-safe if a caller parallelises.
func applyMixed(t *testing.T, cliCfg string, n, parallelism int) {
	t.Helper()
	r := run{dir: t.TempDir()}
	write := func(variant int) error {
		cfg := GenerateMixedConfig(acctest.TestPool(), n, variant)
		return os.WriteFile(filepath.Join(r.dir, "main.tf"), []byte(cfg), 0o644)
	}
	if err := write(0); err != nil {
		t.Errorf("write config: %v", err)
		return
	}
	defer func() {
		_, _ = r.terraform(t, cliCfg, "destroy", "-auto-approve", "-parallelism="+strconv.Itoa(parallelism))
	}()
	out, err := r.terraform(t, cliCfg, "apply", "-auto-approve", "-parallelism="+strconv.Itoa(parallelism))
	if err != nil {
		t.Errorf("apply(create) failed: %v\n%s", err, out)
		return
	}
	if strings.Contains(out, "Rate Limit Exceeded") || strings.Contains(out, "concurrent calls") {
		t.Errorf("rate/concurrency limit surfaced on create:\n%s", out)
		return
	}
	if err := write(1); err != nil {
		t.Errorf("write update config: %v", err)
		return
	}
	out, err = r.terraform(t, cliCfg, "apply", "-auto-approve", "-parallelism="+strconv.Itoa(parallelism))
	if err != nil {
		t.Errorf("apply(update) failed: %v\n%s", err, out)
		return
	}
	if strings.Contains(out, "Rate Limit Exceeded") || strings.Contains(out, "concurrent calls") {
		t.Errorf("rate/concurrency limit surfaced on update:\n%s", out)
	}
}

// TestLoad_MixedResources drives a spread of eight resource types through
// apply -> in-place update -> destroy at high parallelism.
func TestLoad_MixedResources(t *testing.T) {
	loadtest.LoadCheck(t)
	binDir := buildProvider(t)
	cliCfg := writeDevOverrides(t, binDir)
	t.Cleanup(func() {
		if c := acctest.Client(); c != nil {
			_, _ = loadtest.Sweep(context.Background(), c, acctest.TestPool())
		}
	})
	applyMixed(t, cliCfg, envInt("LOAD_MIXED_PER_TYPE", 10), envInt("LOAD_PARALLELISM", 20))
}
