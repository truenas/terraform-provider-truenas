package client_test

import (
	"context"
	"encoding/json"
	"net"
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

		// DDP handshake
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

func wsURL(srv *httptest.Server) string {
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

func TestCall_Success(t *testing.T) {
	srv := echoServer(t, func(conn *websocket.Conn, msg map[string]any) {
		conn.WriteJSON(map[string]any{
			"id":     msg["id"],
			"msg":    "result",
			"result": "hello",
		})
	})
	defer srv.Close()

	c := client.New(wsURL(srv)+"/websocket", nil)
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
			"id":  msg["id"],
			"msg": "result",
			"error": map[string]any{
				"code":    2,
				"message": "Dataset tank/missing not found",
			},
		})
	})
	defer srv.Close()

	c := client.New(wsURL(srv)+"/websocket", nil)
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

	c := client.New(wsURL(srv)+"/websocket", nil)
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

// Ensure Client works with ws:// (not wss://) in tests
func TestNew_WSS_Rejection(t *testing.T) {
	// ws:// should be allowed in non-production (tests don't enforce wss)
	c := client.New("ws://localhost/websocket", nil)
	if c == nil {
		t.Error("New returned nil")
	}
}

var _ net.Listener = (*net.TCPListener)(nil) // import check
