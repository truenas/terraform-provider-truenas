// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

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

// APIError is a TrueNAS API error. On the JSON-RPC 2.0 wire
// (/api/current) this object arrives as the `data` member of the JSON-RPC
// error: {"error": {"code": -32001, "message": "...", "data": {"error":
// <errno>, "errname": "EINVAL", "reason": "<string>", "trace": ...}}}.
// Protocol-level errors with no data member (e.g. -32601 "Method does not
// exist") are mapped onto the same struct with the JSON-RPC code and
// message.
type APIError struct {
	Code    int    `json:"error"`
	ErrName string `json:"errname,omitempty"`
	Message string `json:"reason"`
	Trace   any    `json:"trace,omitempty"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("truenas API error (code %d): %s", e.Code, e.Message)
}

// IsNotFound returns true when the error means the resource does not exist.
// Note the middleware's InstanceNotFound errors report errname EINVAL with
// an "[ENOENT] ..." reason, so the reason text is checked as well as the
// errno.
func IsNotFound(err error) bool {
	e, ok := err.(*APIError)
	if !ok {
		return false
	}
	msg := strings.ToLower(e.Message)
	return e.Code == 2 ||
		e.ErrName == "ENOENT" ||
		strings.Contains(msg, "[enoent]") ||
		strings.Contains(msg, "not found") ||
		strings.Contains(msg, "does not exist")
}

// rpcRequest is a JSON-RPC 2.0 request frame.
type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	ID      uint64 `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

// rpcError is the JSON-RPC 2.0 error member. Data carries the TrueNAS
// error object when the failure came from the called method.
type rpcError struct {
	Code    int       `json:"code"`
	Message string    `json:"message"`
	Data    *APIError `json:"data,omitempty"`
}

// rpcResponse is a JSON-RPC 2.0 response or server-push notification
// frame. Responses carry an ID; notifications (e.g. collection_update)
// carry a method and no ID.
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *uint64         `json:"id"`
	Method  string          `json:"method,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// apiError flattens a JSON-RPC error into the *APIError callers match on:
// the TrueNAS data object when present, else a synthesized APIError from
// the protocol-level code/message.
func (e *rpcError) apiError() *APIError {
	if e.Data != nil {
		return e.Data
	}
	return &APIError{Code: e.Code, Message: e.Message}
}

type callResult struct {
	result json.RawMessage
	err    error
}

type pendingCall struct {
	ch chan callResult
}

// Client is a TrueNAS JSON-RPC 2.0 WebSocket API client (the /api/current
// endpoint) safe for concurrent use.
type Client struct {
	endpoint  string
	tlsConfig *tls.Config

	connMu sync.Mutex
	conn   *websocket.Conn
	authFn func(ctx context.Context) error // stored by Connect; replayed by reconnect

	// reconnectMu serializes reconnect attempts. connMu alone isn't enough
	// because dial() does network I/O while holding it only briefly (to set
	// c.conn); without this, two concurrent CallRead retries could both see
	// conn==nil and race to open two connections.
	reconnectMu sync.Mutex

	writeMu sync.Mutex

	// sem bounds the number of in-flight concurrent RPCs. TrueNAS rejects
	// calls beyond a per-connection cap (~20) with JSON-RPC error -32000
	// ("Maximum number of concurrent calls exceeded"); acquiring a slot in
	// Call makes excess calls queue instead of failing. Sized below the
	// server cap for headroom; override with WithMaxConcurrentCalls.
	sem chan struct{}

	pendingMu sync.Mutex
	pending   map[uint64]*pendingCall

	seq atomic.Uint64

	// versionMu guards version, fetched lazily by ServerVersion.
	versionMu sync.Mutex
	version   string

	// userAgent is sent as the User-Agent header on the WebSocket
	// handshake, identifying the provider (and its version) to the
	// TrueNAS middleware. Set via WithUserAgent; defaults to a "dev"
	// build identifier when unset.
	userAgent string
}

// userAgentBase and userAgentURL compose the default and
// WithUserAgent-derived User-Agent header values.
const (
	userAgentBase = "terraform-provider-truenas"
	userAgentURL  = "https://github.com/truenas/terraform-provider-truenas"
)

// defaultMaxConcurrentCalls is the in-flight RPC ceiling used when
// WithMaxConcurrentCalls is not given. It sits below TrueNAS's per-connection
// cap of 20 so a little headroom remains for any out-of-band call.
const defaultMaxConcurrentCalls = 16

// Option configures optional Client behavior at construction time.
type Option func(*Client)

// WithMaxConcurrentCalls caps the number of in-flight concurrent RPCs the
// client will have outstanding at once. Calls beyond the cap block until a
// slot frees, rather than being rejected by TrueNAS with error -32000. A
// value < 1 is ignored (the default is kept).
func WithMaxConcurrentCalls(n int) Option {
	return func(c *Client) {
		if n >= 1 {
			c.sem = make(chan struct{}, n)
		}
	}
}

// WithUserAgent sets the User-Agent header sent on the WebSocket handshake
// to identify this provider (and its version) to the TrueNAS middleware.
// An empty version yields the default "dev" identifier.
func WithUserAgent(version string) Option {
	if version == "" {
		version = "dev"
	}
	ua := fmt.Sprintf("%s/%s (+%s)", userAgentBase, version, userAgentURL)
	return func(c *Client) {
		c.userAgent = ua
	}
}

// New creates a Client. Call Connect() before use. By default the client
// identifies itself with a "dev" User-Agent; pass WithUserAgent to set the
// real provider version.
func New(endpoint string, tlsCfg *tls.Config, opts ...Option) *Client {
	c := &Client{
		endpoint:  endpoint,
		tlsConfig: tlsCfg,
		pending:   make(map[uint64]*pendingCall),
		userAgent: fmt.Sprintf("%s/dev (+%s)", userAgentBase, userAgentURL),
	}
	for _, opt := range opts {
		opt(c)
	}
	if c.sem == nil {
		c.sem = make(chan struct{}, defaultMaxConcurrentCalls)
	}
	return c
}

// Connect dials the WebSocket, then calls authenticateFn (if non-nil).
// authenticateFn is retained so a later reconnect (see reconnect) can replay
// the same auth path after a transport failure.
func (c *Client) Connect(ctx context.Context, authenticateFn func(ctx context.Context) error) error {
	c.connMu.Lock()
	c.authFn = authenticateFn
	c.connMu.Unlock()
	return c.dial(ctx)
}

// dial opens the websocket connection and (if an authFn was stored by
// Connect) authenticates. Unlike the legacy /websocket endpoint, JSON-RPC
// needs no protocol handshake — the first frame can be a call. Shared by
// Connect and reconnect so there is exactly one implementation of the
// connect path.
func (c *Client) dial(ctx context.Context) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		TLSClientConfig:  c.tlsConfig,
		ReadBufferSize:   65536,
		WriteBufferSize:  65536,
	}

	header := http.Header{}
	header.Set("User-Agent", c.userAgent)

	conn, _, err := dialer.DialContext(ctx, c.endpoint, header)
	if err != nil {
		return fmt.Errorf("websocket dial %s: %w", c.endpoint, err)
	}
	conn.SetReadLimit(10 * 1024 * 1024)

	c.connMu.Lock()
	c.conn = conn
	authFn := c.authFn
	c.connMu.Unlock()

	go c.readLoop(conn)

	if authFn != nil {
		if err := authFn(ctx); err != nil {
			_ = c.Close()
			return err
		}
	}

	return nil
}

// reconnect re-dials the connection using the same connect+auth path as
// Connect, but only when there is no live connection (conn == nil, e.g.
// after readLoop observed a transport failure). It is a no-op if a
// connection already exists. Concurrent callers serialize on reconnectMu so
// at most one dial happens per outage; the connMu check below runs both
// before and effectively after that serialization point, so a caller that
// waited behind another reconnect() will see the fresh conn and skip
// dialing again.
func (c *Client) reconnect(ctx context.Context) error {
	c.reconnectMu.Lock()
	defer c.reconnectMu.Unlock()

	c.connMu.Lock()
	alreadyConnected := c.conn != nil
	c.connMu.Unlock()
	if alreadyConnected {
		return nil
	}

	return c.dial(ctx)
}

func (c *Client) readLoop(conn *websocket.Conn) {
	for {
		var msg rpcResponse
		if err := conn.ReadJSON(&msg); err != nil {
			c.failPending(fmt.Errorf("connection closed: %w", err))
			c.connMu.Lock()
			if c.conn == conn {
				c.conn = nil
			}
			c.connMu.Unlock()
			return
		}
		// Server-push notifications (collection_update etc.) carry no ID.
		if msg.ID == nil {
			continue
		}
		c.pendingMu.Lock()
		p, ok := c.pending[*msg.ID]
		if ok {
			delete(c.pending, *msg.ID)
		}
		c.pendingMu.Unlock()
		if ok {
			if msg.Error != nil {
				p.ch <- callResult{err: msg.Error.apiError()}
			} else {
				p.ch <- callResult{result: msg.Result}
			}
		}
	}
}

// failPending fails every in-flight call after a transport loss. The
// deliberate wrapping as &APIError{Code: 0} (no real TrueNAS error carries
// code 0) is what CallRead's isTransient uses to recognize a mid-flight
// transport failure as retryable.
func (c *Client) failPending(err error) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	for _, p := range c.pending {
		p.ch <- callResult{err: &APIError{Message: err.Error()}}
	}
	c.pending = make(map[uint64]*pendingCall)
}

// Call invokes a TrueNAS method and returns the raw JSON result.
func (c *Client) Call(ctx context.Context, method string, params ...any) (json.RawMessage, error) {
	// Bound in-flight concurrent RPCs (see the sem field): acquire a slot or
	// wait, so calls beyond the server's per-connection cap queue instead of
	// failing with -32000. Released on every return path via defer.
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	id := c.seq.Add(1)
	p := &pendingCall{ch: make(chan callResult, 1)}

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

	if params == nil {
		params = []any{}
	}
	c.writeMu.Lock()
	err := conn.WriteJSON(rpcRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
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
	case res := <-p.ch:
		return res.result, res.err
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
