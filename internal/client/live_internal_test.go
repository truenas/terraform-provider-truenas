// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
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
	// Wrap auth in WithRetry (as the provider and acctest helper do): running
	// the full live suite back-to-back with other packages opens many logins
	// and can trip TrueNAS's ~20-logins/minute cap (code 16), which would
	// otherwise fail these tests spuriously.
	if err := c.Connect(context.Background(), func(ctx context.Context) error {
		return WithRetry(ctx, func(ctx context.Context) error { return AuthAPIKey(ctx, c, apiKey) })
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
// key, and WithRetry reports it as ErrAPIKeyRejected after its few retries
// of a false login. Three failed logins per run — well inside the auth rate
// limit.
func TestLiveAuth_BadKeyRejected(t *testing.T) {
	endpoint := os.Getenv("TRUENAS_ENDPOINT")
	if os.Getenv("TF_ACC") == "" || endpoint == "" {
		t.Skip("Set TF_ACC=1, TRUENAS_ENDPOINT for live client tests")
	}
	tlsCfg, _ := BuildTLSConfig(true, "")
	c := New(endpoint, tlsCfg)
	err := c.Connect(context.Background(), func(ctx context.Context) error {
		return WithRetry(ctx, func(ctx context.Context) error { return AuthAPIKey(ctx, c, "1-notavalidkeyatall") })
	})
	if err == nil {
		c.Close()
		t.Fatal("server accepted an invalid API key")
	}
	if !errors.Is(err, ErrAPIKeyRejected) {
		t.Fatalf("err = %v, want ErrAPIKeyRejected", err)
	}
	t.Logf("rejected: %v", err)
}

// TestLiveCallJob_SyncIntBailout: regression coverage for the CallJob
// infinite-poll bug, against the real box. user.get_next_uid is
// synchronous and returns a bare int; CallJob must not mistake it for a
// job id — the server's method metadata says it is not a job, so the
// result comes back unpolled.
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

// TestLiveCallJob_PollResumesAfterDrop starts a real job (core.bulk of
// core.ping), kills the connection, and asserts the poll reconnects and
// finds the same job from the new session (B-046).
func TestLiveCallJob_PollResumesAfterDrop(t *testing.T) {
	c := liveClient(t)

	oldPoll, oldRead := jobPollInterval, readRetryDelays
	jobPollInterval = 100 * time.Millisecond
	readRetryDelays = []time.Duration{200 * time.Millisecond, 500 * time.Millisecond, time.Second}
	defer func() { jobPollInterval, readRetryDelays = oldPoll, oldRead }()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if isJob, known := c.methodIsJob(ctx, "core.bulk"); !known || !isJob {
		t.Fatalf("methodIsJob(core.bulk) = %v, %v; want true, true", isJob, known)
	}
	raw, err := c.Call(ctx, "core.bulk", "core.ping", []any{[]any{}, []any{}})
	if err != nil {
		t.Fatal(err)
	}
	var jobID int64
	if err := json.Unmarshal(raw, &jobID); err != nil || jobID == 0 {
		t.Fatalf("core.bulk returned %s, want a job id", raw)
	}

	c.connMu.Lock()
	c.conn.Close()
	c.connMu.Unlock()

	res, err := c.waitJob(ctx, jobID, raw, true)
	if err != nil {
		t.Fatalf("polling job %d after a connection drop: %v", jobID, err)
	}
	var results []map[string]any
	if err := json.Unmarshal(res, &results); err != nil || len(results) != 2 {
		t.Fatalf("job result = %s (%v), want two core.ping results", res, err)
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

// TestLiveCall_ReconnectsAfterTransportDrop force-closes the live connection
// and asserts a plain Call (not CallRead) re-dials and succeeds. Call gained a
// pre-send reconnect in v1.5.12: when the socket dropped and nothing has been
// sent for the call yet, re-dialing and sending once is side-effect-safe, so an
// idle-closed session no longer surfaces "not connected" on the next operation.
func TestLiveCall_ReconnectsAfterTransportDrop(t *testing.T) {
	c := liveClient(t)

	// Kill the transport the way an idle close / network blip would.
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
	raw, err := c.Call(ctx, "system.version_short")
	if err != nil {
		t.Fatalf("Call after transport drop: %v", err)
	}
	var v string
	if err := json.Unmarshal(raw, &v); err != nil || v == "" {
		t.Fatalf("reconnected call = %s (%v)", raw, err)
	}
}

// TestLiveKeepalive_KeepsSessionAlive asserts the keepalive (v1.5.12) holds the
// session open across an idle span longer than the server's idle-close window
// (observed ~45-60s with no traffic). It makes NO calls during the idle, so
// only the keepalive goroutine can keep c.conn live. Slow by nature (the server
// timeout cannot be shrunk), so it runs only under the live gate.
func TestLiveKeepalive_KeepsSessionAlive(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping slow keepalive idle test under -short")
	}
	c := liveClient(t)

	// Idle well past the observed idle-close window; keepalive pings at
	// keepaliveInterval (30s) should keep the connection up throughout.
	for elapsed := keepaliveInterval; elapsed <= 75*time.Second; elapsed += keepaliveInterval {
		time.Sleep(keepaliveInterval)
		c.connMu.Lock()
		live := c.conn != nil
		c.connMu.Unlock()
		if !live {
			t.Fatalf("connection dropped after ~%s idle despite keepalive", elapsed)
		}
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

// TestLiveAuth_LoginBurstRetriesFalse opens N sessions back to back and
// logs in on each through the provider's API-key path. Past ~20 logins a
// minute the server rate-limits; TrueNAS 25.10 then answers some valid-key
// logins with a bare false, which used to fail as "invalid API key".
//
// With LOGIN_BURST_RAW unset every session authenticates through WithRetry
// (as the provider does) and must come up. With LOGIN_BURST_RAW=1 the first
// login of each session is sent without any backoff, the way a hammering
// client would (rate-limited sessions are skipped), so the bucket stays
// tripped; a session whose valid-key login came back false must then be
// recovered by WithRetry. It deliberately trips the rate limit, so it runs
// only with LOGIN_BURST=<sessions> set.
func TestLiveAuth_LoginBurstRetriesFalse(t *testing.T) {
	endpoint := os.Getenv("TRUENAS_ENDPOINT")
	apiKey := os.Getenv("TRUENAS_API_KEY")
	username := os.Getenv("TRUENAS_USERNAME")
	n, _ := strconv.Atoi(os.Getenv("LOGIN_BURST"))
	raw := os.Getenv("LOGIN_BURST_RAW") != ""
	if os.Getenv("TF_ACC") == "" || endpoint == "" || apiKey == "" || n <= 0 {
		t.Skip("Set TF_ACC=1, TRUENAS_ENDPOINT, TRUENAS_API_KEY and LOGIN_BURST=<sessions> to run")
	}
	tlsCfg, err := BuildTLSConfig(true, "")
	if err != nil {
		t.Fatal(err)
	}
	var falses, limited, recovered, failed int
	for i := 0; i < n; i++ {
		c := New(endpoint, tlsCfg)
		ctx := context.Background()
		if err := c.Connect(ctx, nil); err != nil {
			t.Fatal(err)
		}
		start := time.Now()
		var outcomes []string
		login := func(ctx context.Context) error {
			err := AuthAPIKeyAuto(ctx, c, username, apiKey)
			switch {
			case err == nil:
				outcomes = append(outcomes, "ok")
			case errors.Is(err, ErrAPIKeyRejected):
				falses++
				outcomes = append(outcomes, "false")
			case IsRateLimited(err):
				limited++
				outcomes = append(outcomes, "EBUSY")
			default:
				outcomes = append(outcomes, err.Error())
			}
			return err
		}
		if raw {
			err = login(ctx)
			if errors.Is(err, ErrAPIKeyRejected) {
				err = WithRetry(ctx, login)
				if err == nil {
					recovered++
				}
			} else if IsRateLimited(err) {
				err = nil
			}
		} else {
			err = WithRetry(ctx, login)
		}
		_ = c.Close()
		t.Logf("session %d: %v in %s", i, outcomes, time.Since(start).Round(time.Millisecond))
		if err != nil {
			failed++
			t.Errorf("session %d failed: %v", i, err)
		}
	}
	t.Logf("%d sessions: %d failed; valid-key logins answered false %d times (%d sessions recovered by WithRetry), rate limited %d times",
		n, failed, falses, recovered, limited)
}
