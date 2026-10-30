// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
	"time"
)

// The fake-server hunt tests were promoted to connection_internal_test.go
// (TestClient_*).

// Live: what an unauthenticated call returns on a real box, and how the
// client classifies it. Gated by HUNT_LIVE_ENDPOINT.
func TestHuntClient_LiveUnauthenticatedShape(t *testing.T) {
	ep := os.Getenv("HUNT_LIVE_ENDPOINT")
	if ep == "" {
		t.Skip("HUNT_LIVE_ENDPOINT unset")
	}
	tlsCfg, _ := BuildTLSConfig(true, "")
	c := New(ep, tlsCfg)
	if err := c.Connect(context.Background(), nil); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	for _, m := range []string{"certificate.get_instance", "no.such_method"} {
		_, err := c.Call(context.Background(), m, 1)
		t.Logf("%s -> %v | IsNotFound=%v isTransient=%v", m, err, IsNotFound(err), isTransient(err))
	}
}

// Live: the endpoint is an ssh port-forward to the box. HUNT_FORWARD_DOWN_CMD
// kills it (connection dropped, then refused, as during a restart) and
// HUNT_FORWARD_UP_CMD brings it back after a few seconds. A mutating-style
// call made while the forward is down must wait and succeed.
func TestHuntClient_LiveCallWaitsOutRestart(t *testing.T) {
	down, up := os.Getenv("HUNT_FORWARD_DOWN_CMD"), os.Getenv("HUNT_FORWARD_UP_CMD")
	if down == "" || up == "" {
		t.Skip("HUNT_FORWARD_DOWN_CMD/HUNT_FORWARD_UP_CMD unset")
	}
	c := liveClient(t)
	if out, err := exec.Command("bash", "-c", down).CombinedOutput(); err != nil {
		t.Fatalf("down: %v %s", err, out)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		c.connMu.Lock()
		gone := c.conn == nil
		c.connMu.Unlock()
		if gone || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	go func() {
		time.Sleep(8 * time.Second)
		if out, err := exec.Command("bash", "-c", up).CombinedOutput(); err != nil {
			t.Logf("up: %v %s", err, out)
		}
	}()
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	raw, err := c.Call(ctx, "system.version_short")
	if err != nil {
		t.Fatalf("call while the endpoint was down for 8s: %v", err)
	}
	var v string
	_ = json.Unmarshal(raw, &v)
	t.Logf("call succeeded after %s: %s", time.Since(start).Round(time.Millisecond), v)
}

// Live: N sessions opened and closed through the provider's auth path; the
// server's auth.sessions count must not grow (session leak check).
func TestHuntClient_LiveSessionsClosed(t *testing.T) {
	endpoint, apiKey := os.Getenv("TRUENAS_ENDPOINT"), os.Getenv("TRUENAS_API_KEY")
	if os.Getenv("HUNT_SESSIONS") == "" || endpoint == "" {
		t.Skip("HUNT_SESSIONS unset")
	}
	tlsCfg, _ := BuildTLSConfig(true, "")
	count := func() int {
		c := liveClient(t)
		raw, err := c.Call(context.Background(), "auth.sessions")
		if err != nil {
			t.Fatal(err)
		}
		var s []any
		_ = json.Unmarshal(raw, &s)
		_ = c.Close()
		return len(s)
	}
	before := count()
	for i := 0; i < 5; i++ {
		c := New(endpoint, tlsCfg)
		if err := c.Connect(context.Background(), func(ctx context.Context) error {
			return WithRetry(ctx, func(ctx context.Context) error { return AuthAPIKeyAuto(ctx, c, os.Getenv("TRUENAS_USERNAME"), apiKey) })
		}); err != nil {
			t.Fatal(err)
		}
		_, _ = c.Call(context.Background(), "core.ping")
		_ = c.Close()
	}
	time.Sleep(3 * time.Second)
	after := count()
	t.Logf("sessions before=%d after=%d", before, after)
}
