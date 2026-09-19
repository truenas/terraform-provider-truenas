// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// Live client-behavior tests. The project's testing policy is no mocks:
// every interaction test runs against a real TrueNAS box, gated on the
// acceptance-test env vars (TF_ACC, TRUENAS_ENDPOINT, TRUENAS_API_KEY) and
// skipped otherwise. This file is in package client (not client_test) so
// tests can reach internal tunables (jobPollInterval) and internals
// (c.conn) to keep live runs fast and to stage transport failures against
// the real server.

// liveClient dials the box from the env and authenticates with the API
// key; skips the test when the env is not configured.
func liveClient(t *testing.T) *Client {
	t.Helper()
	endpoint := os.Getenv("TRUENAS_ENDPOINT")
	apiKey := os.Getenv("TRUENAS_API_KEY")
	if os.Getenv("TF_ACC") == "" || endpoint == "" || apiKey == "" {
		t.Skip("Set TF_ACC=1, TRUENAS_ENDPOINT, TRUENAS_API_KEY for live client tests")
	}
	tlsCfg, err := BuildTLSConfig(true, "")
	if err != nil {
		t.Fatal(err)
	}
	c := New(endpoint, tlsCfg)
	if err := c.Connect(context.Background(), func(ctx context.Context) error {
		return AuthAPIKey(ctx, c, apiKey)
	}); err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

// TestLiveCall_Success: a plain synchronous call round-trips.
func TestLiveCall_Success(t *testing.T) {
	c := liveClient(t)
	raw, err := c.Call(context.Background(), "system.version_short")
	if err != nil {
		t.Fatal(err)
	}
	var v string
	if err := json.Unmarshal(raw, &v); err != nil || v == "" {
		t.Fatalf("version_short = %q, %v", v, err)
	}
}

// TestLiveCall_NotFound: the real middleware's InstanceNotFound error maps
// to IsNotFound (errname EINVAL with an "[ENOENT]" reason on the wire).
func TestLiveCall_NotFound(t *testing.T) {
	c := liveClient(t)
	_, err := c.Call(context.Background(), "user.get_instance", 999999)
	if err == nil {
		t.Fatal("expected error for nonexistent user")
	}
	if !IsNotFound(err) {
		t.Fatalf("IsNotFound = false for %v", err)
	}
}

// TestLiveCall_ContextCancel: a cancelled context aborts the call.
func TestLiveCall_ContextCancel(t *testing.T) {
	c := liveClient(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Call(ctx, "system.version_short"); err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

// TestLiveAuth_BadKeyRejected: the real middleware rejects a malformed
// key. One failed login per run — well inside the auth rate limit.
func TestLiveAuth_BadKeyRejected(t *testing.T) {
	endpoint := os.Getenv("TRUENAS_ENDPOINT")
	if os.Getenv("TF_ACC") == "" || endpoint == "" {
		t.Skip("Set TF_ACC=1, TRUENAS_ENDPOINT for live client tests")
	}
	tlsCfg, _ := BuildTLSConfig(true, "")
	c := New(endpoint, tlsCfg)
	err := c.Connect(context.Background(), func(ctx context.Context) error {
		return AuthAPIKey(ctx, c, "1-notavalidkeyatall")
	})
	if err == nil {
		c.Close()
		t.Fatal("server accepted an invalid API key")
	}
}

// TestLiveCallJob_SyncIntBailout: regression coverage for the CallJob
// infinite-poll bug, against the real box. user.get_next_uid is
// synchronous and returns a bare int; CallJob must not mistake it for a
// job id — after maxJobPollMisses empty core.get_jobs polls it returns
// the original result.
func TestLiveCallJob_SyncIntBailout(t *testing.T) {
	c := liveClient(t)

	old := jobPollInterval
	jobPollInterval = 100 * time.Millisecond
	defer func() { jobPollInterval = old }()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	raw, err := c.CallJob(ctx, "user.get_next_uid")
	if err != nil {
		t.Fatalf("CallJob returned error, want sync bailout: %v", err)
	}
	var uid int64
	if err := json.Unmarshal(raw, &uid); err != nil || uid <= 0 {
		t.Fatalf("bailout result = %s (%v), want positive uid", raw, err)
	}
}

// TestLiveCallRead_ReconnectsAfterTransportDrop force-closes the live
// connection out from under the client and asserts CallRead re-dials the
// real box and succeeds. This is the orphan-fix path (read-back after
// create must survive a transport drop) exercised end-to-end.
func TestLiveCallRead_ReconnectsAfterTransportDrop(t *testing.T) {
	c := liveClient(t)

	old := readRetryDelays
	readRetryDelays = []time.Duration{200 * time.Millisecond, 500 * time.Millisecond, time.Second}
	defer func() { readRetryDelays = old }()

	// Kill the transport the way a network blip would.
	c.connMu.Lock()
	c.conn.Close()
	c.connMu.Unlock()

	deadline := time.Now().Add(5 * time.Second)
	for {
		c.connMu.Lock()
		gone := c.conn == nil
		c.connMu.Unlock()
		if gone {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("readLoop never observed the closed connection")
		}
		time.Sleep(10 * time.Millisecond)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	raw, err := c.CallRead(ctx, "system.version_short")
	if err != nil {
		t.Fatalf("CallRead after transport drop: %v", err)
	}
	var v string
	if err := json.Unmarshal(raw, &v); err != nil || v == "" {
		t.Fatalf("reconnected read = %s (%v)", raw, err)
	}
}

// TestLiveReconnect_NoOpWhenAlreadyConnected: reconnect() must not re-dial
// over a live connection.
func TestLiveReconnect_NoOpWhenAlreadyConnected(t *testing.T) {
	c := liveClient(t)

	c.connMu.Lock()
	before := c.conn
	c.connMu.Unlock()

	if err := c.reconnect(context.Background()); err != nil {
		t.Fatalf("reconnect: %v", err)
	}

	c.connMu.Lock()
	after := c.conn
	c.connMu.Unlock()
	if before != after {
		t.Error("reconnect redialed even though a connection was already live")
	}
}

// TestLiveAuthPassword: password login against the real box. Additionally
// gated on TRUENAS_USERNAME+TRUENAS_PASSWORD since most runs use API keys.
func TestLiveAuthPassword(t *testing.T) {
	endpoint := os.Getenv("TRUENAS_ENDPOINT")
	username := os.Getenv("TRUENAS_USERNAME")
	password := os.Getenv("TRUENAS_PASSWORD")
	if os.Getenv("TF_ACC") == "" || endpoint == "" || username == "" || password == "" {
		t.Skip("Set TF_ACC=1, TRUENAS_ENDPOINT, TRUENAS_USERNAME, TRUENAS_PASSWORD for the live password-auth test")
	}
	tlsCfg, _ := BuildTLSConfig(true, "")
	c := New(endpoint, tlsCfg)
	if err := c.Connect(context.Background(), func(ctx context.Context) error {
		return AuthPassword(ctx, c, username, password)
	}); err != nil {
		t.Fatalf("password auth: %v", err)
	}
	defer c.Close()
	if _, err := c.Call(context.Background(), "system.version_short"); err != nil {
		t.Fatalf("authenticated call: %v", err)
	}
}
