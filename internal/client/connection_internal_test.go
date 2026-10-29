// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// Connection-lifecycle tests against an in-process JSON-RPC websocket
// server. These failure modes (a drop in the middle of a job poll, a
// reconnect while every semaphore slot is held, an oversized frame) cannot
// be staged on a real TrueNAS box on demand, so a fake stands in for it.

// fakeError makes the fake server answer with a JSON-RPC error carrying a
// TrueNAS error object (code -32001) or, with protocol set, a bare
// protocol-level error.
type fakeError struct {
	code     int
	errname  string
	reason   string
	protocol bool
}

// fakeRawFrame makes the fake server write this text verbatim, with %d
// replaced by the request id.
type fakeRawFrame string

// fakeServer is a minimal JSON-RPC 2.0 websocket server. handler returns
// (result, drop): drop=true closes the connection instead of answering.
type fakeServer struct {
	srv     *httptest.Server
	mu      sync.Mutex
	conns   []*websocket.Conn
	handler func(connIdx int, method string, params json.RawMessage) (any, bool)
	nconn   atomic.Int32
	// refuse makes the next refuse handshakes fail with 502, the way nginx
	// answers while the middleware restarts.
	refuse atomic.Int32
}

func newFakeServer(t *testing.T, h func(connIdx int, method string, params json.RawMessage) (any, bool)) *fakeServer {
	fs := &fakeServer{handler: h}
	up := websocket.Upgrader{}
	fs.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fs.refuse.Add(-1) >= 0 {
			http.Error(w, "bad gateway", http.StatusBadGateway)
			return
		}
		fs.refuse.Store(0)
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		idx := int(fs.nconn.Add(1)) - 1
		fs.mu.Lock()
		fs.conns = append(fs.conns, c)
		fs.mu.Unlock()
		var wmu sync.Mutex
		for {
			var req struct {
				ID     uint64          `json:"id"`
				Method string          `json:"method"`
				Params json.RawMessage `json:"params"`
			}
			if err := c.ReadJSON(&req); err != nil {
				return
			}
			go func() {
				res, drop := fs.handler(idx, req.Method, req.Params)
				if drop {
					_ = c.Close()
					return
				}
				wmu.Lock()
				defer wmu.Unlock()
				switch v := res.(type) {
				case fakeRawFrame:
					_ = c.WriteMessage(websocket.TextMessage, []byte(fmt.Sprintf(string(v), req.ID)))
				case fakeError:
					e := map[string]any{"code": v.code, "message": v.reason}
					if !v.protocol {
						e = map[string]any{"code": -32001, "message": "Method call error",
							"data": map[string]any{"error": v.code, "errname": v.errname, "reason": v.reason}}
					}
					_ = c.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": e})
				default:
					_ = c.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": res})
				}
			}()
		}
	}))
	t.Cleanup(fs.srv.Close)
	return fs
}

func (fs *fakeServer) url() string { return "ws" + strings.TrimPrefix(fs.srv.URL, "http") }

func (fs *fakeServer) dropAll() {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	for _, c := range fs.conns {
		_ = c.Close()
	}
}

// connectFake connects a client to fs with a plain API-key login.
func connectFake(t *testing.T, fs *fakeServer, opts ...Option) *Client {
	t.Helper()
	c := New(fs.url(), nil, opts...)
	if err := c.Connect(context.Background(), func(ctx context.Context) error { return AuthAPIKey(ctx, c, "1-x") }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// waitDisconnected waits until readLoop has noticed the server side closed
// the connection.
func waitDisconnected(t *testing.T, c *Client) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		c.connMu.Lock()
		gone := c.conn == nil
		c.connMu.Unlock()
		if gone {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("readLoop never observed the closed connection")
}

// shrinkDelays makes CallRead retries and job polls fast for the test.
func shrinkDelays(t *testing.T) {
	t.Helper()
	oldPoll, oldRead := jobPollInterval, readRetryDelays
	jobPollInterval = 20 * time.Millisecond
	readRetryDelays = []time.Duration{20 * time.Millisecond, 50 * time.Millisecond}
	t.Cleanup(func() { jobPollInterval, readRetryDelays = oldPoll, oldRead })
}

// A websocket drop while CallJob polls core.get_jobs must not fail the
// operation: the job keeps running on the server, so CallJob reconnects and
// keeps polling the same job id (B-046).
func TestClient_JobPollSurvivesDrop(t *testing.T) {
	shrinkDelays(t)
	var polls atomic.Int32
	fs := newFakeServer(t, func(_ int, method string, _ json.RawMessage) (any, bool) {
		switch method {
		case "core.get_methods":
			return map[string]any{"thing.create": map[string]any{"job": true}}, false
		case "thing.create":
			return 4242, false
		case "core.get_jobs":
			if polls.Add(1) == 1 {
				return nil, true // transport drop mid-poll
			}
			return []map[string]any{{"id": 4242, "state": "SUCCESS", "result": map[string]any{"id": 7}}}, false
		}
		return true, false
	})
	c := connectFake(t, fs)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	raw, err := c.CallJob(ctx, "thing.create", map[string]any{})
	if err != nil {
		t.Fatalf("CallJob failed after a drop during polling although the job succeeded: %v", err)
	}
	if string(raw) != `{"id":7}` {
		t.Fatalf("result = %s, want the job result {\"id\":7}", raw)
	}
	if n := fs.nconn.Load(); n != 2 {
		t.Fatalf("connections = %d, want 2 (one reconnect)", n)
	}
}

// Re-authentication during a reconnect must not need a semaphore slot: with
// every slot held by a caller waiting on that reconnect it could never get
// one (B-047). The second case is the control with fewer callers than slots.
func TestClient_ReconnectWhenSemaphoreFull(t *testing.T) {
	const slots = 4
	for _, callers := range []int{slots, slots - 1} {
		t.Run(fmt.Sprintf("callers=%d", callers), func(t *testing.T) {
			fs := newFakeServer(t, func(int, string, json.RawMessage) (any, bool) { return true, false })
			c := connectFake(t, fs, WithMaxConcurrentCalls(slots))
			fs.dropAll()
			waitDisconnected(t, c)

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var wg sync.WaitGroup
			errs := make([]error, callers)
			for i := 0; i < callers; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					_, errs[i] = c.Call(ctx, "core.ping")
				}(i)
			}
			wg.Wait()
			for i, e := range errs {
				if e != nil {
					t.Errorf("call %d after a connection drop: %v", i, e)
				}
			}
		})
	}
}

// A connection is published to other callers only after re-authentication
// finished on it; before, a call could be sent on the unauthenticated socket
// and fail with ENOTAUTHENTICATED (B-133).
func TestClient_NoCallBeforeReauth(t *testing.T) {
	var authed sync.Map
	fs := newFakeServer(t, func(connIdx int, method string, _ json.RawMessage) (any, bool) {
		if method == "auth.login_with_api_key" {
			time.Sleep(300 * time.Millisecond)
			authed.Store(connIdx, true)
			return true, false
		}
		if _, ok := authed.Load(connIdx); !ok {
			return fakeError{code: 207, errname: "ENOTAUTHENTICATED", reason: "Not authenticated"}, false
		}
		return "ok", false
	})
	c := connectFake(t, fs)
	fs.dropAll()
	waitDisconnected(t, c)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go func() { _, _ = c.Call(ctx, "core.ping") }() // starts the reconnect; auth takes 300ms
	time.Sleep(100 * time.Millisecond)
	if _, err := c.Call(ctx, "certificate.get_instance", 1); err != nil {
		t.Fatalf("call during re-authentication: %v", err)
	}
}

// A response over maxResponseBytes fails only its own call: the connection
// and the other in-flight calls survive, and CallRead does not resend a
// request whose answer cannot fit (B-134). The id is placed after the
// oversized result so the reader has to scan past it.
func TestClient_OversizedResponseFailsOnlyItsCall(t *testing.T) {
	shrinkDelays(t)
	oldMax := maxResponseBytes
	maxResponseBytes = 1 << 20
	t.Cleanup(func() { maxResponseBytes = oldMax })

	big := strings.Repeat("x", 2<<20)
	var bigCalls atomic.Int32
	fs := newFakeServer(t, func(_ int, method string, _ json.RawMessage) (any, bool) {
		switch method {
		case "pool.dataset.query":
			bigCalls.Add(1)
			return fakeRawFrame(`{"jsonrpc":"2.0","result":[{"name":"` + big + `","nested":{"a":[1,2,{"b":null}]}}],"id":%d}`), false
		case "slow.create":
			time.Sleep(300 * time.Millisecond)
			return map[string]any{"id": 1}, false
		}
		return true, false
	})
	c := connectFake(t, fs)
	ctx := context.Background()

	var peerErr error
	done := make(chan struct{})
	go func() { _, peerErr = c.Call(ctx, "slow.create"); close(done) }()
	time.Sleep(50 * time.Millisecond)
	_, err := c.CallRead(ctx, "pool.dataset.query")
	<-done

	if !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("oversized read err = %v, want ErrResponseTooLarge", err)
	}
	if peerErr != nil {
		t.Fatalf("concurrent call failed because of another call's oversized response: %v", peerErr)
	}
	if n := bigCalls.Load(); n != 1 {
		t.Fatalf("oversized read sent %d times, want 1 (not retried)", n)
	}
	if _, err := c.Call(ctx, "core.ping"); err != nil {
		t.Fatalf("call after the oversized response: %v", err)
	}
	if n := fs.nconn.Load(); n != 1 {
		t.Fatalf("connections = %d, want 1 (no reconnect)", n)
	}
}

// The default limit takes an 11 MiB response, which the old 10 MiB
// websocket read limit turned into a dropped connection (B-134).
func TestClient_LargeResponseUnderDefaultLimit(t *testing.T) {
	big := strings.Repeat("x", 11<<20)
	fs := newFakeServer(t, func(_ int, method string, _ json.RawMessage) (any, bool) {
		if method == "pool.dataset.query" {
			return []map[string]any{{"name": big}}, false
		}
		return true, false
	})
	c := connectFake(t, fs)
	raw, err := c.Call(context.Background(), "pool.dataset.query")
	if err != nil {
		t.Fatalf("11 MiB response: %v", err)
	}
	if len(raw) < 11<<20 {
		t.Fatalf("response truncated to %d bytes", len(raw))
	}
}

// Only errors about the object count as not-found. A JSON-RPC protocol
// error such as -32601 "Method does not exist" (a method missing on the
// connected release) must not drop the resource from state (B-135).
func TestClient_IsNotFoundProtocolErrors(t *testing.T) {
	tests := []struct {
		name string
		err  *APIError
		want bool
	}{
		{"method missing", (&rpcError{Code: -32601, Message: "Method does not exist"}).apiError(), false},
		{"invalid params", (&rpcError{Code: -32602, Message: "Invalid params: id not found"}).apiError(), false},
		{"ENOENT errno", &APIError{Code: 2, ErrName: "ENOENT", Message: "Dataset tank/x not found"}, true},
		{"InstanceNotFound", (&rpcError{Code: -32001, Message: "Method call error", Data: &APIError{Code: 22, ErrName: "EINVAL", Message: "[ENOENT] None: Certificate 9 does not exist"}}).apiError(), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNotFound(tt.err); got != tt.want {
				t.Errorf("IsNotFound(%q code %d) = %v, want %v", tt.err.Message, tt.err.Code, got, tt.want)
			}
		})
	}
}

// A method the server marks as not a job returns its result as is, even a
// bare int that equals a live job id; before, CallJob polled that "job" and
// returned the other job's result (B-139).
func TestClient_CallJobNonJobIntNotPolled(t *testing.T) {
	shrinkDelays(t)
	var polls atomic.Int32
	fs := newFakeServer(t, func(_ int, method string, _ json.RawMessage) (any, bool) {
		switch method {
		case "core.get_methods":
			return map[string]any{"user.get_next_uid": map[string]any{"job": false}}, false
		case "user.get_next_uid":
			return 3012, false
		case "core.get_jobs":
			polls.Add(1)
			return []map[string]any{{"id": 3012, "state": "SUCCESS", "result": "someone else's job"}}, false
		}
		return true, false
	})
	c := connectFake(t, fs)
	raw, err := c.CallJob(context.Background(), "user.get_next_uid")
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "3012" || polls.Load() != 0 {
		t.Fatalf("result = %s after %d core.get_jobs polls, want 3012 with no polling", raw, polls.Load())
	}
}

// fakeLoginServer answers auth.login_with_api_key from logins in order (the
// last entry repeats) and counts the attempts.
func fakeLoginServer(t *testing.T, logins ...any) (*fakeServer, *atomic.Int32) {
	var n atomic.Int32
	fs := newFakeServer(t, func(_ int, method string, _ json.RawMessage) (any, bool) {
		if method == "auth.login_with_api_key" {
			i := int(n.Add(1)) - 1
			if i >= len(logins) {
				i = len(logins) - 1
			}
			return logins[i], false
		}
		return true, false
	})
	return fs, &n
}

// shrinkAuthBackoff makes WithRetry's waits fast for the test.
func shrinkAuthBackoff(t *testing.T) {
	t.Helper()
	old := authRetryBackoff
	authRetryBackoff = []time.Duration{10 * time.Millisecond, 10 * time.Millisecond, 10 * time.Millisecond, 10 * time.Millisecond}
	t.Cleanup(func() { authRetryBackoff = old })
}

// TrueNAS 25.10 can answer a valid API key with false while the source IP is
// rate limited; WithRetry retries that (v2510 N1). A key that is rejected on
// every attempt still fails, after maxAPIKeyRejectedRetries retries.
func TestClient_APIKeyFalseRetried(t *testing.T) {
	shrinkAuthBackoff(t)
	rateLimited := fakeError{code: 16, errname: "EBUSY", reason: "Rate Limit Exceeded"}

	t.Run("transient false", func(t *testing.T) {
		fs, n := fakeLoginServer(t, rateLimited, false, false, true)
		c := New(fs.url(), nil)
		defer func() { _ = c.Close() }()
		err := c.Connect(context.Background(), func(ctx context.Context) error {
			return WithRetry(ctx, func(ctx context.Context) error { return AuthAPIKey(ctx, c, "1-x") })
		})
		if err != nil {
			t.Fatalf("login with a valid key failed: %v", err)
		}
		if n.Load() != 4 {
			t.Fatalf("login attempts = %d, want 4", n.Load())
		}
	})

	t.Run("invalid key", func(t *testing.T) {
		fs, n := fakeLoginServer(t, false)
		c := New(fs.url(), nil)
		defer func() { _ = c.Close() }()
		err := c.Connect(context.Background(), func(ctx context.Context) error {
			return WithRetry(ctx, func(ctx context.Context) error { return AuthAPIKey(ctx, c, "1-x") })
		})
		if !errors.Is(err, ErrAPIKeyRejected) {
			t.Fatalf("err = %v, want ErrAPIKeyRejected", err)
		}
		if want := int32(1 + maxAPIKeyRejectedRetries); n.Load() != want {
			t.Fatalf("login attempts = %d, want %d", n.Load(), want)
		}
	})
}

// A rate-limited auth.mechanism_choices probe says nothing about SCRAM
// support, so it is not taken as "no SCRAM" (the fallback would send the
// plain key secret to a server that supports SCRAM): SCRAM is attempted
// directly, and the plain login is used only when the server rejects the
// SCRAM mechanism itself (25.10).
func TestClient_MechanismProbeRateLimited(t *testing.T) {
	rateLimited := fakeError{code: 16, errname: "EBUSY", reason: "Rate Limit Exceeded"}
	noSCRAM := fakeError{code: 22, errname: "EINVAL", reason: "[EINVAL] login_data: Input tag 'SCRAM' found using 'mechanism' does not match any of the expected tags"}
	for _, tt := range []struct {
		name      string
		loginEx   any
		wantPlain int32
		wantErr   bool
	}{
		{"server without SCRAM", noSCRAM, 1, false},
		{"SCRAM step rate limited too", rateLimited, 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var plain, scram atomic.Int32
			fs := newFakeServer(t, func(_ int, method string, _ json.RawMessage) (any, bool) {
				switch method {
				case "auth.mechanism_choices":
					return rateLimited, false
				case "auth.login_ex":
					scram.Add(1)
					return tt.loginEx, false
				case "auth.login_with_api_key":
					plain.Add(1)
					return true, false
				}
				return true, false
			})
			c := New(fs.url(), nil)
			defer func() { _ = c.Close() }()
			err := c.Connect(context.Background(), func(ctx context.Context) error { return AuthAPIKeyAuto(ctx, c, "root", "1-x") })
			if (err != nil) != tt.wantErr || (tt.wantErr && !IsRateLimited(err)) {
				t.Fatalf("err = %v, wantErr %v (rate limited)", err, tt.wantErr)
			}
			if scram.Load() != 1 || plain.Load() != tt.wantPlain {
				t.Fatalf("SCRAM attempts = %d, plain logins = %d; want 1, %d", scram.Load(), plain.Load(), tt.wantPlain)
			}
		})
	}
}

// A call sent on a connection whose reader already exited (the server closed
// it between dial and the call) fails at once instead of waiting forever for
// an answer nothing will deliver. Seen live as a 55-minute hang in the auth
// call of a provider Configure while the box was rate limiting.
func TestClient_SendOnClosedConnectionFails(t *testing.T) {
	fs := newFakeServer(t, func(int, string, json.RawMessage) (any, bool) { return true, false })
	c := connectFake(t, fs)
	c.connMu.Lock()
	conn := c.conn
	c.connMu.Unlock()
	fs.dropAll()
	waitDisconnected(t, c)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := c.send(ctx, conn, "core.ping", nil)
	if err == nil || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("send on a closed connection: err = %v, want an immediate transport error", err)
	}
	if !isTransient(err) {
		t.Fatalf("err %v is not transient; CallRead would not retry it", err)
	}
}

// When the server closes a new connection before authentication finished
// (seen on busy boxes under login rate limiting), Connect re-dials instead
// of failing provider configuration or hanging.
func TestClient_RedialWhenClosedDuringAuth(t *testing.T) {
	old := dialRetryDelays
	dialRetryDelays = []time.Duration{10 * time.Millisecond, 10 * time.Millisecond}
	t.Cleanup(func() { dialRetryDelays = old })

	fs := newFakeServer(t, func(connIdx int, method string, _ json.RawMessage) (any, bool) {
		if method == "auth.login_with_api_key" && connIdx == 0 {
			return nil, true // close the first connection under the login
		}
		return true, false
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c := New(fs.url(), nil)
	defer func() { _ = c.Close() }()
	if err := c.Connect(ctx, func(ctx context.Context) error {
		return WithRetry(ctx, func(ctx context.Context) error { return AuthAPIKey(ctx, c, "1-x") })
	}); err != nil {
		t.Fatalf("Connect after the server closed the first connection during auth: %v", err)
	}
	if n := fs.nconn.Load(); n != 2 {
		t.Fatalf("connections = %d, want 2", n)
	}
	if _, err := c.Call(ctx, "core.ping"); err != nil {
		t.Fatal(err)
	}
}

// A reconnect whose re-authentication is rate limited, on a server that
// then closes the connection, must end: the retry of the login used to wait
// forever (it re-entered reconnect while the outer reconnect held its lock,
// or waited on a connection without a reader). Reported as a VM destroy on
// .70 that never returned after "Rate Limit Exceeded".
func TestClient_RateLimitedReauthDoesNotHang(t *testing.T) {
	shrinkAuthBackoff(t)
	oldDial := dialRetryDelays
	dialRetryDelays = []time.Duration{10 * time.Millisecond, 10 * time.Millisecond, 10 * time.Millisecond}
	t.Cleanup(func() { dialRetryDelays = oldDial })

	var logins atomic.Int32
	var fs *fakeServer
	fs = newFakeServer(t, func(connIdx int, method string, _ json.RawMessage) (any, bool) {
		if method != "auth.login_with_api_key" {
			return true, false
		}
		n := logins.Add(1)
		if n == 1 {
			return true, false // initial Connect
		}
		if n <= 3 {
			// Re-auth is rate limited, and the server closes the
			// connection right after answering.
			go func() { time.Sleep(20 * time.Millisecond); fs.dropAll() }()
			return fakeError{code: 16, errname: "EBUSY", reason: "Rate Limit Exceeded"}, false
		}
		return true, false
	})
	c := New(fs.url(), nil)
	t.Cleanup(func() { _ = c.Close() })
	if err := c.Connect(context.Background(), func(ctx context.Context) error {
		return WithRetry(ctx, func(ctx context.Context) error { return AuthAPIKey(ctx, c, "1-x") })
	}); err != nil {
		t.Fatal(err)
	}
	fs.dropAll()
	waitDisconnected(t, c)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := c.Call(ctx, "vm.delete", 1)
	if errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("call hung through a rate-limited re-authentication: %v", err)
	}
	if err != nil {
		t.Fatalf("call after a rate-limited re-authentication: %v", err)
	}
}

// After the connection drops, a call that finds the server unreachable for a
// while (nginx answering 502 during a middleware restart) waits for it to come
// back instead of failing with "not connected"; nothing has been sent yet, so
// this is safe even for a mutating call. Reads retry the same way.
func TestClient_CallWaitsOutServerRestart(t *testing.T) {
	shrinkDelays(t)
	old := reconnectRetryDelays
	reconnectRetryDelays = []time.Duration{20 * time.Millisecond, 20 * time.Millisecond, 20 * time.Millisecond, 20 * time.Millisecond}
	t.Cleanup(func() { reconnectRetryDelays = old })

	fs := newFakeServer(t, func(_ int, method string, _ json.RawMessage) (any, bool) {
		if method == "auth.login_with_api_key" {
			return true, false
		}
		return map[string]any{"id": 1}, false
	})
	c := connectFake(t, fs)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for _, call := range []struct {
		name string
		fn   func() (json.RawMessage, error)
	}{
		{"Call", func() (json.RawMessage, error) { return c.Call(ctx, "pool.dataset.create", map[string]any{}) }},
		{"CallRead", func() (json.RawMessage, error) { return c.CallRead(ctx, "pool.dataset.query") }},
	} {
		fs.refuse.Store(3)
		fs.dropAll()
		waitDisconnected(t, c)
		if _, err := call.fn(); err != nil {
			t.Fatalf("%s during a server restart: %v", call.name, err)
		}
	}
	if fs.refuse.Load() != 0 {
		t.Fatalf("refused handshakes left = %d, want 0", fs.refuse.Load())
	}
}

// Connect retries a failed websocket open briefly (a busy nginx timing out or
// dropping the handshake) instead of failing provider configuration at once.
func TestClient_ConnectRetriesFailedOpen(t *testing.T) {
	old := dialRetryDelays
	dialRetryDelays = []time.Duration{10 * time.Millisecond, 10 * time.Millisecond, 10 * time.Millisecond}
	t.Cleanup(func() { dialRetryDelays = old })

	fs := newFakeServer(t, func(int, string, json.RawMessage) (any, bool) { return true, false })
	fs.refuse.Store(2)
	c := connectFake(t, fs)
	if _, err := c.Call(context.Background(), "core.ping"); err != nil {
		t.Fatal(err)
	}

	fs.refuse.Store(100)
	c2 := New(fs.url(), nil)
	err := c2.Connect(context.Background(), nil)
	if !errors.Is(err, errDial) {
		t.Fatalf("Connect to a server that stays unreachable: err = %v, want errDial", err)
	}
}

// The keepalive pings only a live connection of a client in use: it never
// re-dials a dropped connection, and it goes quiet once the client has been
// unused for keepaliveIdleLimit. Before, every abandoned client (in-process
// acceptance tests create one per Terraform command) kept a server session
// forever and logged in again after any drop, exhausting the login rate
// limit for everyone.
func TestClient_KeepaliveLeavesAbandonedClientsAlone(t *testing.T) {
	oldInt, oldIdle := keepaliveInterval, keepaliveIdleLimit
	keepaliveInterval, keepaliveIdleLimit = 20*time.Millisecond, 200*time.Millisecond
	t.Cleanup(func() { keepaliveInterval, keepaliveIdleLimit = oldInt, oldIdle })

	var pings atomic.Int32
	fs := newFakeServer(t, func(_ int, method string, _ json.RawMessage) (any, bool) {
		if method == "core.ping" {
			pings.Add(1)
		}
		return true, false
	})
	c := connectFake(t, fs)

	time.Sleep(100 * time.Millisecond)
	if pings.Load() == 0 {
		t.Fatal("keepalive did not ping a client in use")
	}
	time.Sleep(300 * time.Millisecond) // past keepaliveIdleLimit
	idle := pings.Load()
	time.Sleep(200 * time.Millisecond)
	if n := pings.Load(); n != idle {
		t.Fatalf("keepalive pinged an idle client %d more times", n-idle)
	}

	// In use again, then the connection drops: no re-dial by the keepalive.
	if _, err := c.Call(context.Background(), "core.ping"); err != nil {
		t.Fatal(err)
	}
	fs.dropAll()
	waitDisconnected(t, c)
	time.Sleep(150 * time.Millisecond)
	if n := fs.nconn.Load(); n != 1 {
		t.Fatalf("connections = %d, want 1 (keepalive must not re-dial)", n)
	}
}

// A call whose context has no deadline still ends when the server never
// answers it (seen live: provider Configure stuck 11+ minutes in
// auth.mechanism_choices). If the connection still answers a ping it is
// kept; if it does not, it is dropped so the next call re-dials. A login
// that is never answered fails over to a fresh connection.
func TestClient_CallTimeout(t *testing.T) {
	oldCall, oldAuth, oldPing, oldDial := callTimeout, authCallTimeout, pingTimeout, dialRetryDelays
	callTimeout, authCallTimeout, pingTimeout = 150*time.Millisecond, 150*time.Millisecond, 100*time.Millisecond
	dialRetryDelays = []time.Duration{10 * time.Millisecond, 10 * time.Millisecond}
	t.Cleanup(func() {
		callTimeout, authCallTimeout, pingTimeout, dialRetryDelays = oldCall, oldAuth, oldPing, oldDial
	})

	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	var deadConn atomic.Int32
	deadConn.Store(-1)
	fs := newFakeServer(t, func(connIdx int, method string, _ json.RawMessage) (any, bool) {
		hang := method == "slow.method" ||
			int32(connIdx) == deadConn.Load() ||
			(connIdx == 0 && method == "auth.login_with_api_key")
		if hang {
			<-release
			return nil, true
		}
		return true, false
	})

	// The first connection's login is never answered: Connect moves on to a
	// second connection.
	c := connectFake(t, fs)
	if n := fs.nconn.Load(); n != 2 {
		t.Fatalf("connections after an unanswered login = %d, want 2", n)
	}
	ctx := context.Background()

	// One unanswered call on a live connection: only that call fails.
	start := time.Now()
	if _, err := c.Call(ctx, "slow.method"); err == nil || !strings.Contains(err.Error(), "no response") {
		t.Fatalf("unanswered call: err = %v, want a timeout", err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("unanswered call took %s", d)
	}
	if _, err := c.Call(ctx, "core.ping"); err != nil || fs.nconn.Load() != 2 {
		t.Fatalf("live connection after one slow call: err=%v connections=%d, want kept", err, fs.nconn.Load())
	}

	// The connection stops answering altogether: it is dropped and the
	// next call re-dials.
	deadConn.Store(1)
	if _, err := c.Call(ctx, "core.ping"); err == nil {
		t.Fatal("call on a dead connection succeeded")
	}
	waitDisconnected(t, c)
	if _, err := c.Call(ctx, "core.ping"); err != nil {
		t.Fatalf("call after the dead connection was dropped: %v", err)
	}
	if n := fs.nconn.Load(); n != 3 {
		t.Fatalf("connections = %d, want 3", n)
	}
}
