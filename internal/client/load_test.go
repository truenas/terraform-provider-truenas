// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package client_test

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/truenas/terraform-provider-truenas/internal/acctest"
	"github.com/truenas/terraform-provider-truenas/internal/client"
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

func stamp() string { return time.Now().UTC().Format("20060102T150405Z") }

// connectOnce builds a fresh Client and authenticates once (one login),
// recording rate-limit/retry evidence into m. WithRetry absorbs auth throttle.
func connectOnce(ctx context.Context, m *loadtest.Metrics) error {
	tlsCfg, _ := client.BuildTLSConfig(true, "")
	c := client.New(acctest.Endpoint(), tlsCfg)
	user := os.Getenv("TRUENAS_USERNAME")
	key := os.Getenv("TRUENAS_API_KEY")
	m.RecordAttempt()
	err := c.Connect(ctx, func(ctx context.Context) error {
		return client.WithRetry(ctx, func(ctx context.Context) error {
			e := client.AuthAPIKeyAuto(ctx, c, user, key)
			if e != nil && client.IsRateLimited(e) {
				m.RecordRateLimit()
				m.RecordRetry(0) // backoff is internal to WithRetry; count the event
			}
			return e
		})
	})
	if err == nil {
		_ = c.Close()
	}
	return err
}

// TestLoad_AuthBurst launches LOAD_AUTH_CONNS concurrent logins to cross the
// ~20/min auth throttle and asserts every one eventually authenticates.
func TestLoad_AuthBurst(t *testing.T) {
	loadtest.LoadCheck(t)
	n := envInt("LOAD_AUTH_CONNS", 40)
	m := loadtest.NewMetrics()
	start := time.Now()

	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = connectOnce(context.Background(), m)
		}(i)
	}
	wg.Wait()
	wall := time.Since(start)

	failed := 0
	for _, e := range errs {
		if e != nil {
			failed++
			t.Errorf("connection failed: %v", e)
		}
	}
	_, _, _ = loadtest.Report("results", loadtest.ReportInput{
		Name: "authburst", Stamp: stamp(), Wall: wall, Snap: m.Snapshot(),
		Notes: map[string]string{
			"conns": strconv.Itoa(n), "failed": strconv.Itoa(failed),
		},
	})
	if failed > 0 {
		t.Fatalf("%d/%d logins never succeeded despite retry — auth rate-limit not absorbed", failed, n)
	}
}

// TestLoad_CallSaturation fires concurrent ops across all three call paths on
// one connection. CallRead should self-heal; Call/CallJob rate-limits are
// RECORDED, not failed — that is the finding this test produces.
func TestLoad_CallSaturation(t *testing.T) {
	loadtest.LoadCheck(t)
	ctx := context.Background()
	conc := envInt("LOAD_CALL_CONCURRENCY", 50)
	pool := acctest.TestPool()

	tlsCfg, _ := client.BuildTLSConfig(true, "")
	c := client.New(acctest.Endpoint(), tlsCfg)
	if err := c.Connect(ctx, func(ctx context.Context) error {
		return client.AuthAPIKeyAuto(ctx, c, os.Getenv("TRUENAS_USERNAME"), os.Getenv("TRUENAS_API_KEY"))
	}); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		if n, err := loadtest.Sweep(context.Background(), c, pool); err != nil {
			t.Logf("sweep: %v (deleted %d)", err, n)
		}
		c.Close()
	})

	// Pre-create one dataset for the snapshot (Call) path.
	base := fmt.Sprintf("%s/%s", pool, acctest.RandName("tf-load"))
	if _, err := c.Call(ctx, "pool.dataset.create", map[string]any{"name": base}); err != nil {
		t.Fatalf("precreate dataset: %v", err)
	}

	m := loadtest.NewMetrics()
	start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < conc; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m.RecordAttempt()
			var err error
			var class string
			switch i % 3 {
			case 0:
				_, err = c.CallRead(ctx, "system.version_short")
				class = "callread"
			case 1:
				_, err = c.Call(ctx, "pool.snapshot.create", map[string]any{"dataset": base, "name": acctest.RandName("s")})
				class = "call"
			case 2:
				ds := fmt.Sprintf("%s/%s", pool, acctest.RandName("tf-load"))
				if _, e := c.Call(ctx, "pool.dataset.create", map[string]any{"name": ds}); e == nil {
					_, err = c.CallJob(ctx, "pool.dataset.delete", ds)
				} else {
					err = e
				}
				class = "calljob"
			}
			if err != nil {
				if client.IsRateLimited(err) {
					m.RecordRateLimit()
				}
				m.RecordFailureSample(class, err.Error())
			}
		}(i)
	}
	wg.Wait()
	wall := time.Since(start)

	snap := m.Snapshot()
	_, _, _ = loadtest.Report("results", loadtest.ReportInput{
		Name: "callsaturation", Stamp: stamp(), Wall: wall, Snap: snap,
		Notes: map[string]string{
			"concurrency": strconv.Itoa(conc),
			"finding":     "non-zero call/calljob failures indicate write/job rate-limit is surfaced, not retried",
		},
	})
	// CallRead must NOT be among the surfaced failures — it has retry.
	if snap.Failures["callread"] > 0 {
		t.Fatalf("CallRead surfaced %d failures under load — its retry path regressed", snap.Failures["callread"])
	}
	t.Logf("call/calljob surfaced failures: call=%d calljob=%d (recorded finding, see results/)",
		snap.Failures["call"], snap.Failures["calljob"])
}

// TestLoad_JobSaturation pre-creates LOAD_JOB_COUNT datasets, then deletes
// them all concurrently via CallJob — each submits a delete job and polls
// core.get_jobs, so this holds many jobs open at once and stresses the
// job+poll path against the client's concurrency semaphore. Any surfaced
// error (especially -32000) is a failure.
func TestLoad_JobSaturation(t *testing.T) {
	loadtest.LoadCheck(t)
	ctx := context.Background()
	count := envInt("LOAD_JOB_COUNT", 30)
	pool := acctest.TestPool()

	tlsCfg, _ := client.BuildTLSConfig(true, "")
	c := client.New(acctest.Endpoint(), tlsCfg)
	if err := c.Connect(ctx, func(ctx context.Context) error {
		return client.AuthAPIKeyAuto(ctx, c, os.Getenv("TRUENAS_USERNAME"), os.Getenv("TRUENAS_API_KEY"))
	}); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		if n, err := loadtest.Sweep(context.Background(), c, pool); err != nil {
			t.Logf("sweep: %v (deleted %d)", err, n)
		}
		c.Close()
	})

	// Pre-create the datasets to delete. dataset id == its name, so we delete
	// by the name we created.
	names := make([]string, 0, count)
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("%s/%s", pool, acctest.RandName("tf-load"))
		if _, err := c.Call(ctx, "pool.dataset.create", map[string]any{"name": name}); err != nil {
			t.Fatalf("precreate dataset %d: %v", i, err)
		}
		names = append(names, name)
	}

	m := loadtest.NewMetrics()
	start := time.Now()
	var wg sync.WaitGroup
	for _, name := range names {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			m.RecordAttempt()
			if _, err := c.CallJob(ctx, "pool.dataset.delete", name,
				map[string]any{"recursive": true, "force": true}); err != nil {
				if client.IsRateLimited(err) {
					m.RecordRateLimit()
				}
				m.RecordFailureSample("calljob", err.Error())
			}
		}(name)
	}
	wg.Wait()
	wall := time.Since(start)

	snap := m.Snapshot()
	_, _, _ = loadtest.Report("results", loadtest.ReportInput{
		Name: "jobsaturation", Stamp: stamp(), Wall: wall, Snap: snap,
		Notes: map[string]string{"jobs": strconv.Itoa(count)},
	})
	if snap.Failures["calljob"] > 0 {
		t.Fatalf("%d/%d concurrent delete jobs surfaced errors (see results/) — sample: %s",
			snap.Failures["calljob"], count, snap.FailureSamples["calljob"])
	}
}

// TestLoadSweep is the standalone entry point behind `make loadtest-sweep`: it
// connects and deletes any tf-load- datasets left behind by a crashed or
// killed load run. Its name deliberately lacks the TestLoad_ prefix so the
// `-run TestLoad_` make targets do not sweep as a side effect.
func TestLoadSweep(t *testing.T) {
	loadtest.LoadCheck(t)
	ctx := context.Background()
	tlsCfg, _ := client.BuildTLSConfig(true, "")
	c := client.New(acctest.Endpoint(), tlsCfg)
	if err := c.Connect(ctx, func(ctx context.Context) error {
		return client.AuthAPIKeyAuto(ctx, c, os.Getenv("TRUENAS_USERNAME"), os.Getenv("TRUENAS_API_KEY"))
	}); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer c.Close()
	n, err := loadtest.Sweep(ctx, c, acctest.TestPool())
	if err != nil {
		t.Fatalf("sweep: %v", err)
	}
	t.Logf("swept %d tf-load- datasets", n)
}
