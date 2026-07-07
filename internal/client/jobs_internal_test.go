package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// echoServerInternal is a trimmed copy of the echoServer test helper in
// client_test.go. It lives here (in package client, not client_test)
// because this file needs to reach the unexported jobPollInterval var to
// keep the test fast.
func echoServerInternal(t *testing.T, handler func(conn *websocket.Conn, msg map[string]any)) *httptest.Server {
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

// TestCallJob_NoJobBailout is a regression test for a live acceptance
// failure: group.create (job:false, sync) returns a bare int (the new
// group's id), which CallJob mistook for a job id and polled
// core.get_jobs for forever, since a matching job entry never appears.
//
// This drives CallJob against a method that returns int(7) and a
// core.get_jobs handler that always reports zero matching entries, and
// asserts CallJob bails out after maxJobPollMisses misses and returns the
// original raw result (7) with a nil error, instead of polling forever.
func TestCallJob_NoJobBailout(t *testing.T) {
	old := jobPollInterval
	jobPollInterval = 10 * time.Millisecond
	defer func() { jobPollInterval = old }()

	srv := echoServerInternal(t, func(conn *websocket.Conn, msg map[string]any) {
		switch msg["method"] {
		case "group.create":
			conn.WriteJSON(map[string]any{"id": msg["id"], "msg": "result", "result": float64(7)})
		case "core.get_jobs":
			// No job ever matches — group.create wasn't actually a job.
			conn.WriteJSON(map[string]any{"id": msg["id"], "msg": "result", "result": []any{}})
		}
	})
	defer srv.Close()

	c := New("ws"+srv.URL[len("http"):]+"/websocket", nil)
	if err := c.Connect(context.Background(), nil); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	raw, err := c.CallJob(ctx, "group.create", map[string]any{"name": "testgroup"})
	if err != nil {
		t.Fatalf("CallJob returned error, want nil (sync bailout): %v", err)
	}

	var id int64
	if err := json.Unmarshal(raw, &id); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if id != 7 {
		t.Errorf("got id %d, want 7", id)
	}
}
