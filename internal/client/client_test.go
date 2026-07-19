package client_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// echoServer starts a test WebSocket server that handles DDP connect + echoes method calls.
func echoServer(t *testing.T, handler func(conn *websocket.Conn, msg map[string]any)) *httptest.Server {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Logf("upgrade: %v", err)
			return
		}
		defer conn.Close()

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

func wsURL(srv *httptest.Server) string {
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

func TestCall_Success(t *testing.T) {
	srv := echoServer(t, func(conn *websocket.Conn, msg map[string]any) {
		conn.WriteJSON(map[string]any{
			"id":     msg["id"],
			"result": "hello",
		})
	})
	defer srv.Close()

	c := client.New(wsURL(srv)+"/api/current", nil)
	if err := c.Connect(context.Background(), nil); err != nil {
		t.Fatal(err)
	}

	raw, err := c.Call(context.Background(), "test.method")
	if err != nil {
		t.Fatal(err)
	}
	var s string
	json.Unmarshal(raw, &s)
	if s != "hello" {
		t.Errorf("got %q want %q", s, "hello")
	}
}

func TestCall_APIError(t *testing.T) {
	srv := echoServer(t, func(conn *websocket.Conn, msg map[string]any) {
		conn.WriteJSON(map[string]any{
			"id": msg["id"],
			"error": map[string]any{
				"code":    -32001,
				"message": "Method call error",
				"data":    map[string]any{"error": 2, "reason": "Dataset tank/missing not found"},
			},
		})
	})
	defer srv.Close()

	c := client.New(wsURL(srv)+"/api/current", nil)
	c.Connect(context.Background(), nil)

	_, err := c.Call(context.Background(), "pool.dataset.get_instance", "tank/missing")
	if err == nil {
		t.Fatal("expected error")
	}
	if !client.IsNotFound(err) {
		t.Errorf("IsNotFound=false for %v", err)
	}
}

func TestCall_ContextCancel(t *testing.T) {
	srv := echoServer(t, func(conn *websocket.Conn, msg map[string]any) {
		// never respond — let ctx cancel fire
	})
	defer srv.Close()

	c := client.New(wsURL(srv)+"/api/current", nil)
	c.Connect(context.Background(), nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := c.Call(ctx, "test.method")
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestIsNotFound(t *testing.T) {
	err := &client.APIError{Code: 2, Message: "Dataset tank/x not found"}
	if !client.IsNotFound(err) {
		t.Error("expected IsNotFound=true")
	}
	notFoundMsg := &client.APIError{Code: 22, Message: "tank/x: object does not exist"}
	if !client.IsNotFound(notFoundMsg) {
		t.Error("expected IsNotFound=true for 'does not exist'")
	}
}

// TestIsRateLimited is a regression test for a live acceptance failure:
// "auth.login_with_api_key: truenas API error (code 16): [EBUSY] Rate
// Limit Exceeded". It covers the *APIError code-16 path, a message-based
// fallback, wrapped errors (fmt.Errorf %w, as AuthAPIKey/AuthPassword
// produce), and negative cases that must not be misclassified as
// rate-limited.
func TestIsRateLimited(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"code 16 APIError", &client.APIError{Code: 16, Message: "[EBUSY] Rate Limit Exceeded"}, true},
		{"code 16 with unrelated message", &client.APIError{Code: 16, Message: "something else"}, true},
		{"message-only rate limit, different code", &client.APIError{Code: 22, Message: "Rate limit exceeded, try again later"}, true},
		{"wrapped APIError via fmt.Errorf %w", fmt.Errorf("auth.login_with_api_key: %w", &client.APIError{Code: 16, Message: "[EBUSY] Rate Limit Exceeded"}), true},
		{"not found, unrelated code", &client.APIError{Code: 2, Message: "Dataset tank/x not found"}, false},
		{"generic connection error", fmt.Errorf("websocket dial wss://host: connection refused"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := client.IsRateLimited(tt.err); got != tt.want {
				t.Errorf("IsRateLimited(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// Ensure Client works with ws:// (not wss://) in tests
func TestNew_WSS_Rejection(t *testing.T) {
	// ws:// should be allowed in non-production (tests don't enforce wss)
	c := client.New("ws://localhost/api/current", nil)
	if c == nil {
		t.Error("New returned nil")
	}
}

func TestAuthAPIKey_Success(t *testing.T) {
	srv := echoServer(t, func(conn *websocket.Conn, msg map[string]any) {
		if msg["method"] == "auth.login_with_api_key" {
			conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": msg["id"], "result": true})
		}
	})
	defer srv.Close()

	c := client.New(wsURL(srv)+"/api/current", nil)
	c.Connect(context.Background(), nil)
	if err := client.AuthAPIKey(context.Background(), c, "1-testkey"); err != nil {
		t.Fatal(err)
	}
}

func TestAuthAPIKey_Failure(t *testing.T) {
	srv := echoServer(t, func(conn *websocket.Conn, msg map[string]any) {
		if msg["method"] == "auth.login_with_api_key" {
			conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": msg["id"], "result": false})
		}
	})
	defer srv.Close()

	c := client.New(wsURL(srv)+"/api/current", nil)
	c.Connect(context.Background(), nil)
	if err := client.AuthAPIKey(context.Background(), c, "bad"); err == nil {
		t.Fatal("expected error")
	}
}

func TestAuthPassword_Success(t *testing.T) {
	srv := echoServer(t, func(conn *websocket.Conn, msg map[string]any) {
		if msg["method"] == "auth.login" {
			conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": msg["id"], "result": true})
		}
	})
	defer srv.Close()

	c := client.New(wsURL(srv)+"/api/current", nil)
	c.Connect(context.Background(), nil)
	if err := client.AuthPassword(context.Background(), c, "root", "pass"); err != nil {
		t.Fatal(err)
	}
}

func TestCallJob_Success(t *testing.T) {
	jobDone := make(chan struct{})
	srv := echoServer(t, func(conn *websocket.Conn, msg map[string]any) {
		switch msg["method"] {
		case "pool.scrub":
			// Return a job ID
			conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": msg["id"], "result": float64(42)})
		case "core.get_jobs":
			select {
			case <-jobDone:
				conn.WriteJSON(map[string]any{
					"id": msg["id"],
					"result": []any{map[string]any{
						"id":     float64(42),
						"state":  "SUCCESS",
						"result": "done",
					}},
				})
			default:
				conn.WriteJSON(map[string]any{
					"id": msg["id"],
					"result": []any{map[string]any{
						"id":    float64(42),
						"state": "RUNNING",
					}},
				})
				close(jobDone)
			}
		}
	})
	defer srv.Close()

	c := client.New(wsURL(srv)+"/api/current", nil)
	c.Connect(context.Background(), nil)

	raw, err := c.CallJob(context.Background(), "pool.scrub", "tank")
	if err != nil {
		t.Fatal(err)
	}
	var result string
	json.Unmarshal(raw, &result)
	if result != "done" {
		t.Errorf("got %q want %q", result, "done")
	}
}

func TestCallJob_Failure(t *testing.T) {
	srv := echoServer(t, func(conn *websocket.Conn, msg map[string]any) {
		switch msg["method"] {
		case "pool.scrub":
			conn.WriteJSON(map[string]any{"jsonrpc": "2.0", "id": msg["id"], "result": float64(99)})
		case "core.get_jobs":
			conn.WriteJSON(map[string]any{
				"id": msg["id"],
				"result": []any{map[string]any{
					"id":    float64(99),
					"state": "FAILED",
					"error": "disk error",
				}},
			})
		}
	})
	defer srv.Close()

	c := client.New(wsURL(srv)+"/api/current", nil)
	c.Connect(context.Background(), nil)

	_, err := c.CallJob(context.Background(), "pool.scrub", "tank")
	if err == nil {
		t.Fatal("expected error for failed job")
	}
	if !strings.Contains(err.Error(), "disk error") {
		t.Errorf("expected job error message, got: %v", err)
	}
}
