package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// echoServerInternalRead is a trimmed copy of the echoServer test helper in
// client_test.go / echoServerInternal in jobs_internal_test.go. It lives
// here (package client, not client_test) because these tests need to reach
// the unexported readRetryDelays var to keep them fast.
func echoServerInternalRead(t *testing.T, handler func(conn *websocket.Conn, msg map[string]any)) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade: %v", err)
			return
		}
		defer conn.Close()

		var hello map[string]any
		if err := conn.ReadJSON(&hello); err != nil || hello["msg"] != "connect" {
			t.Logf("expected connect, got: %v", hello)
			return
		}
		conn.WriteJSON(map[string]any{"msg": "connected", "session": "test"})

		for {
			var msg map[string]any
			if err := conn.ReadJSON(&msg); err != nil {
				return
			}
			handler(conn, msg)
		}
	}))
	return srv
}

func TestIsTransient(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"rate-limit APIError code 16", &APIError{Code: 16, Message: "[EBUSY] Rate Limit Exceeded"}, true},
		{"rate-limit APIError by message only", &APIError{Code: 22, Message: "Rate limit exceeded, try again"}, true},
		{"EINVAL APIError", &APIError{Code: 22, Message: "[EINVAL] Invalid value"}, false},
		{"not-found APIError (code 2)", &APIError{Code: 2, Message: "Dataset tank/x not found"}, false},
		{"plain transport error", fmt.Errorf("connection closed: websocket: close 1006"), true},
		{"not connected", fmt.Errorf("not connected"), true},
		{"failPending-wrapped connection-closed (Code 0 APIError)", &APIError{Code: 0, Message: "connection closed: websocket: close 1006 (abnormal closure): unexpected EOF"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTransient(tt.err); got != tt.want {
				t.Errorf("isTransient(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// TestCallRead_RetriesOnRateLimitThenSucceeds is the orphan-fix scenario:
// a read-back after create fails twice with a rate-limit rejection, then
// succeeds. CallRead must retry (not immediately propagate the error) and
// return the eventual success.
func TestCallRead_RetriesOnRateLimitThenSucceeds(t *testing.T) {
	old := readRetryDelays
	readRetryDelays = []time.Duration{5 * time.Millisecond, 5 * time.Millisecond, 5 * time.Millisecond}
	defer func() { readRetryDelays = old }()

	var calls atomic.Int32
	srv := echoServerInternalRead(t, func(conn *websocket.Conn, msg map[string]any) {
		n := calls.Add(1)
		if n <= 2 {
			conn.WriteJSON(map[string]any{
				"id":  msg["id"],
				"msg": "result",
				"error": map[string]any{
					"error":  16,
					"reason": "[EBUSY] Rate Limit Exceeded",
				},
			})
			return
		}
		conn.WriteJSON(map[string]any{"id": msg["id"], "msg": "result", "result": "ok"})
	})
	defer srv.Close()

	c := New("ws"+srv.URL[len("http"):]+"/websocket", nil)
	if err := c.Connect(context.Background(), nil); err != nil {
		t.Fatal(err)
	}

	raw, err := c.CallRead(context.Background(), "user.get_instance", 1)
	if err != nil {
		t.Fatalf("CallRead: %v", err)
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	if s != "ok" {
		t.Errorf("got %q want %q", s, "ok")
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("server saw %d calls, want 3 (2 failures + 1 success)", got)
	}
}

// TestCallRead_NonTransientFailsImmediately asserts that a non-transient
// APIError (EINVAL) is returned as-is on the first attempt, with no retry —
// otherwise IsNotFound/other error-classification semantics for reads would
// change under this fix.
func TestCallRead_NonTransientFailsImmediately(t *testing.T) {
	old := readRetryDelays
	readRetryDelays = []time.Duration{5 * time.Millisecond, 5 * time.Millisecond, 5 * time.Millisecond}
	defer func() { readRetryDelays = old }()

	var calls atomic.Int32
	srv := echoServerInternalRead(t, func(conn *websocket.Conn, msg map[string]any) {
		calls.Add(1)
		conn.WriteJSON(map[string]any{
			"id":  msg["id"],
			"msg": "result",
			"error": map[string]any{
				"error":  22,
				"reason": "[EINVAL] Invalid value",
			},
		})
	})
	defer srv.Close()

	c := New("ws"+srv.URL[len("http"):]+"/websocket", nil)
	if err := c.Connect(context.Background(), nil); err != nil {
		t.Fatal(err)
	}

	_, err := c.CallRead(context.Background(), "user.get_instance", 1)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("server saw %d calls, want 1 (no retry on non-transient error)", got)
	}
}

// TestReconnect_RedialsWhenDisconnected is a focused unit test on the
// reconnect() path (used by CallRead between retry attempts after a
// transport failure). It closes the connection server-side to simulate a
// dropped transport, waits for readLoop to observe the failure and nil out
// c.conn, then calls reconnect() directly and asserts it establishes a new,
// working connection to the same endpoint using the handshake path stored
// by Connect. (End-to-end coverage of CallRead driving a reconnect after a
// transport failure — as opposed to this direct call to reconnect() — needs
// an echo server that behaves differently per accepted TCP connection; that
// is exercised in TestCallRead_ReconnectsAfterTransportFailure below.)
func TestReconnect_RedialsWhenDisconnected(t *testing.T) {
	srv := echoServerInternalRead(t, func(conn *websocket.Conn, msg map[string]any) {
		conn.WriteJSON(map[string]any{"id": msg["id"], "msg": "result", "result": "ok"})
	})
	defer srv.Close()

	c := New("ws"+srv.URL[len("http"):]+"/websocket", nil)
	if err := c.Connect(context.Background(), nil); err != nil {
		t.Fatal(err)
	}

	// Simulate a transport failure: forcibly close the live connection out
	// from under the client, then give readLoop a moment to observe the
	// read error and nil out c.conn (same effect a dropped TCP connection
	// would have).
	c.connMu.Lock()
	c.conn.Close()
	c.connMu.Unlock()

	deadline := time.Now().Add(2 * time.Second)
	for {
		c.connMu.Lock()
		nilled := c.conn == nil
		c.connMu.Unlock()
		if nilled {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for readLoop to observe the closed connection")
		}
		time.Sleep(5 * time.Millisecond)
	}

	if err := c.reconnect(context.Background()); err != nil {
		t.Fatalf("reconnect: %v", err)
	}

	raw, err := c.Call(context.Background(), "user.get_instance", 1)
	if err != nil {
		t.Fatalf("Call after reconnect: %v", err)
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	if s != "ok" {
		t.Errorf("got %q want %q", s, "ok")
	}
}

// TestReconnect_NoOpWhenAlreadyConnected asserts reconnect() does nothing
// (no re-dial) when a connection is already live.
func TestReconnect_NoOpWhenAlreadyConnected(t *testing.T) {
	srv := echoServerInternalRead(t, func(conn *websocket.Conn, msg map[string]any) {
		conn.WriteJSON(map[string]any{"id": msg["id"], "msg": "result", "result": "ok"})
	})
	defer srv.Close()

	c := New("ws"+srv.URL[len("http"):]+"/websocket", nil)
	if err := c.Connect(context.Background(), nil); err != nil {
		t.Fatal(err)
	}

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

// TestCallRead_ReconnectsAfterTransportFailure drives CallRead end-to-end
// through a transport failure: the first accepted connection is dropped
// without a response (simulating a dead transport), and CallRead must
// reconnect (a second TCP connection lands on the same test server) and
// succeed on retry.
func TestCallRead_ReconnectsAfterTransportFailure(t *testing.T) {
	old := readRetryDelays
	readRetryDelays = []time.Duration{5 * time.Millisecond, 5 * time.Millisecond, 5 * time.Millisecond}
	defer func() { readRetryDelays = old }()

	var connCount atomic.Int32
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		n := connCount.Add(1)

		var hello map[string]any
		if err := conn.ReadJSON(&hello); err != nil || hello["msg"] != "connect" {
			return
		}
		conn.WriteJSON(map[string]any{"msg": "connected", "session": "test"})

		if n == 1 {
			// First connection: read the client's read call, then drop
			// dead without responding — readLoop will see this as
			// "connection closed" (a transient, non-APIError failure).
			var msg map[string]any
			conn.ReadJSON(&msg)
			return
		}

		// Second (reconnected) connection: serve normally.
		for {
			var msg map[string]any
			if err := conn.ReadJSON(&msg); err != nil {
				return
			}
			conn.WriteJSON(map[string]any{"id": msg["id"], "msg": "result", "result": "ok"})
		}
	}))
	defer srv.Close()

	c := New("ws"+srv.URL[len("http"):]+"/websocket", nil)
	if err := c.Connect(context.Background(), nil); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	raw, err := c.CallRead(ctx, "user.get_instance", 1)
	if err != nil {
		t.Fatalf("CallRead: %v", err)
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	if s != "ok" {
		t.Errorf("got %q want %q", s, "ok")
	}
	if got := connCount.Load(); got != 2 {
		t.Errorf("server accepted %d connections, want 2 (initial + reconnect)", got)
	}
}
