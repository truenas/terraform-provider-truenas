package client

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// APIError is a TrueNAS WebSocket API error.
type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Trace   any    `json:"trace,omitempty"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("truenas API error (code %d): %s", e.Code, e.Message)
}

// IsNotFound returns true when the error means the resource does not exist.
func IsNotFound(err error) bool {
	e, ok := err.(*APIError)
	if !ok {
		return false
	}
	msg := strings.ToLower(e.Message)
	return e.Code == 2 ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "does not exist")
}

type wsMsg struct {
	ID     string          `json:"id,omitempty"`
	Msg    string          `json:"msg"`
	Method string          `json:"method,omitempty"`
	Params []any           `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *APIError       `json:"error,omitempty"`
}

type pendingCall struct {
	ch chan *wsMsg
}

// Client is a TrueNAS WebSocket API client safe for concurrent use.
type Client struct {
	endpoint  string
	tlsConfig *tls.Config

	connMu  sync.Mutex
	conn    *websocket.Conn

	writeMu sync.Mutex

	pendingMu sync.Mutex
	pending   map[string]*pendingCall

	seq atomic.Uint64
}

// New creates a Client. Call Connect() before use.
func New(endpoint string, tlsCfg *tls.Config) *Client {
	return &Client{
		endpoint:  endpoint,
		tlsConfig: tlsCfg,
		pending:   make(map[string]*pendingCall),
	}
}

// Connect dials the WebSocket, performs the DDP handshake, then calls authenticateFn (if non-nil).
func (c *Client) Connect(ctx context.Context, authenticateFn func(ctx context.Context) error) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		TLSClientConfig:  c.tlsConfig,
		ReadBufferSize:   65536,
		WriteBufferSize:  65536,
	}

	conn, _, err := dialer.DialContext(ctx, c.endpoint, http.Header{})
	if err != nil {
		return fmt.Errorf("websocket dial %s: %w", c.endpoint, err)
	}
	conn.SetReadLimit(10 * 1024 * 1024)

	// DDP handshake
	if err := conn.WriteJSON(map[string]any{"msg": "connect", "version": "1", "support": []string{"1"}}); err != nil {
		conn.Close()
		return fmt.Errorf("DDP connect write: %w", err)
	}
	var hello wsMsg
	if err := conn.ReadJSON(&hello); err != nil || hello.Msg != "connected" {
		conn.Close()
		return fmt.Errorf("DDP handshake failed: got msg=%q", hello.Msg)
	}

	c.connMu.Lock()
	c.conn = conn
	c.connMu.Unlock()

	go c.readLoop(conn)

	if authenticateFn != nil {
		if err := authenticateFn(ctx); err != nil {
			c.Close()
			return err
		}
	}

	return nil
}

func (c *Client) readLoop(conn *websocket.Conn) {
	for {
		var msg wsMsg
		if err := conn.ReadJSON(&msg); err != nil {
			c.failPending(fmt.Errorf("connection closed: %w", err))
			c.connMu.Lock()
			if c.conn == conn {
				c.conn = nil
			}
			c.connMu.Unlock()
			return
		}
		if msg.ID == "" {
			continue
		}
		c.pendingMu.Lock()
		p, ok := c.pending[msg.ID]
		if ok {
			delete(c.pending, msg.ID)
		}
		c.pendingMu.Unlock()
		if ok {
			p.ch <- &msg
		}
	}
}

func (c *Client) failPending(err error) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	for _, p := range c.pending {
		p.ch <- &wsMsg{Error: &APIError{Message: err.Error()}}
	}
	c.pending = make(map[string]*pendingCall)
}

// Call invokes a TrueNAS method and returns the raw JSON result.
func (c *Client) Call(ctx context.Context, method string, params ...any) (json.RawMessage, error) {
	id := fmt.Sprintf("%d", c.seq.Add(1))
	p := &pendingCall{ch: make(chan *wsMsg, 1)}

	c.pendingMu.Lock()
	c.pending[id] = p
	c.pendingMu.Unlock()

	c.connMu.Lock()
	conn := c.conn
	c.connMu.Unlock()
	if conn == nil {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("not connected")
	}

	c.writeMu.Lock()
	err := conn.WriteJSON(map[string]any{
		"id":     id,
		"msg":    "method",
		"method": method,
		"params": params,
	})
	c.writeMu.Unlock()

	if err != nil {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("write %s: %w", method, err)
	}

	select {
	case <-ctx.Done():
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, ctx.Err()
	case msg := <-p.ch:
		if msg.Error != nil {
			return nil, msg.Error
		}
		return msg.Result, nil
	}
}

// Close shuts down the WebSocket connection.
func (c *Client) Close() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}
