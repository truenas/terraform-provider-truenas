// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
//
// JSON-RPC protocol errors (negative codes, e.g. -32601 "Method does not
// exist" when the connected release lacks a method, or -32602 invalid
// params) say nothing about the object and never count as not-found:
// matching their text would let a Read silently drop a resource from state
// after, for example, a rollback to an older release.
func IsNotFound(err error) bool {
	e, ok := err.(*APIError)
	if !ok {
		return false
	}
	if e.Code < 0 {
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
	ch   chan callResult
	conn *wsConn // the connection the request was written on
}

// wsConn is one websocket connection. closed is set (under
// Client.pendingMu) when its readLoop has exited and failed the calls
// pending on it; send checks it while registering a call, so a call can
// never wait on a connection that no longer has a reader.
type wsConn struct {
	*websocket.Conn
	closed bool
}

// ErrResponseTooLarge is returned by a call whose response exceeded
// maxResponseBytes. Only that call fails: the connection stays up and
// other in-flight calls are unaffected. It is not transient (CallRead does
// not retry it), since resending the same request returns the same
// response.
var ErrResponseTooLarge = errors.New("response exceeds the client's size limit")

// maxResponseBytes bounds a single response held in memory. Large reads are
// legitimate (pool.dataset.query on a pool with thousands of datasets runs
// to tens of MiB), so the bound is generous; it exists to keep a runaway
// response from exhausting memory. A package variable so tests can shrink it.
var maxResponseBytes int64 = 256 << 20

// authConnKey marks the context of an authFn run by dial. Calls made with
// that context go straight to the connection being authenticated: they skip
// the in-flight semaphore (whose slots may all be held by callers waiting on
// this very reconnect) and the published c.conn (which is not set until auth
// succeeds).
type authConnKey struct{}

// Client is a TrueNAS JSON-RPC 2.0 WebSocket API client (the /api/current
// endpoint) safe for concurrent use.
type Client struct {
	endpoint  string
	tlsConfig *tls.Config

	connMu sync.Mutex
	conn   *wsConn
	authFn func(ctx context.Context) error // stored by Connect; replayed by reconnect

	// reconnectMu serializes reconnect attempts. connMu alone isn't enough
	// because dial() does network I/O while holding it only briefly (to set
	// c.conn); without this, two concurrent CallRead retries could both see
	// conn==nil and race to open two connections.
	reconnectMu sync.Mutex

	writeMu sync.Mutex

	// fsMutationMu serializes filesystem.setperm / filesystem.setacl. The
	// TrueNAS 25.10 middleware silently drops some of these when they run
	// concurrently — parallel setperm jobs report SUCCESS but leave paths at
	// their old mode/owner (issue #45, reproduced: 6/12 concurrent setperm no-op,
	// 12/12 when serialized). The client is the per-run shared object, so one
	// mutex here serializes these mutations across every resource in a single
	// Terraform apply. Reads (filesystem.stat/getacl) are not affected.
	fsMutationMu sync.Mutex

	// sem bounds the number of in-flight concurrent RPCs. TrueNAS rejects
	// calls beyond a per-connection cap (~20) with JSON-RPC error -32000
	// ("Maximum number of concurrent calls exceeded"); acquiring a slot in
	// Call makes excess calls queue instead of failing. Sized below the
	// server cap for headroom; override with WithMaxConcurrentCalls.
	sem chan struct{}

	pendingMu sync.Mutex
	pending   map[uint64]*pendingCall

	// jobMethodsMu guards jobMethods: service -> method -> "runs as a job",
	// from core.get_methods (see methodIsJob).
	jobMethodsMu sync.Mutex
	jobMethods   map[string]map[string]bool

	seq atomic.Uint64

	// versionMu guards version, fetched lazily by ServerVersion.
	versionMu sync.Mutex
	version   string

	// userAgent is sent as the User-Agent header on the WebSocket
	// handshake, identifying the provider (and its version) to the
	// TrueNAS middleware. Set via WithUserAgent; defaults to a "dev"
	// build identifier when unset.
	userAgent string

	// keepalive proactively pings the server so it does not idle-close the
	// session. TrueNAS drops a websocket with no RPC traffic after ~45-60s
	// (measured); a quiet stretch in a run — slow plan, think-time between
	// applies, a lull during job polling — would otherwise leave the next call
	// to reconnect. Started once by Connect, stopped by Close.
	keepaliveStop chan struct{}
	keepaliveOnce sync.Once
	lastUse       atomic.Int64 // UnixNano of the last Connect or Call
	// keepaliveEvery/keepaliveIdle are keepaliveInterval/keepaliveIdleLimit,
	// captured by New.
	keepaliveEvery time.Duration
	keepaliveIdle  time.Duration

	stopOnce sync.Once
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
		endpoint:       endpoint,
		tlsConfig:      tlsCfg,
		pending:        make(map[uint64]*pendingCall),
		userAgent:      fmt.Sprintf("%s/dev (+%s)", userAgentBase, userAgentURL),
		keepaliveStop:  make(chan struct{}),
		keepaliveEvery: keepaliveInterval,
		keepaliveIdle:  keepaliveIdleLimit,
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
	c.lastUse.Store(time.Now().UnixNano())
	// A failed websocket open (handshake timeout, EOF from a busy nginx) is
	// retried briefly; a server that stays unreachable still fails
	// configuration within ~20s.
	for attempt := 0; ; attempt++ {
		err := c.dial(ctx)
		if err == nil {
			break
		}
		if !errors.Is(err, errDial) || attempt >= len(dialRetryDelays) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(jitteredBackoff(dialRetryDelays[attempt])):
		}
	}
	c.startKeepalive()
	return nil
}

// keepaliveInterval is how often the keepalive pings. It is well under the
// observed ~45-60s server idle-close window so the session never idles out.
// A package variable so tests can shrink it.
var keepaliveInterval = 30 * time.Second

// keepaliveIdleLimit stops the keepalive for a client nobody has called for
// this long. A provider process is busy from Configure to exit (job polls
// run every 2s), so this only matters for clients that were abandoned
// without Close: in-process acceptance tests configure a new provider, and
// so a new client, for every Terraform command. Kept warm forever, those
// abandoned clients held one server session each and, after any connection
// drop, all logged in again at once and exhausted the login rate limit. A
// package variable so tests can shrink it.
var keepaliveIdleLimit = 5 * time.Minute

// startKeepalive launches the single keepalive goroutine (idempotent). Each
// tick issues a best-effort core.ping on the live connection, which resets
// the server's idle timer. It never reconnects (a dropped connection is
// re-dialed by the next real call) and stops pinging once the client has
// been unused for keepaliveIdleLimit, so an abandoned client lets the server
// close its session. Errors are ignored — a real call surfaces any genuine
// fault; a ping must not.
func (c *Client) startKeepalive() {
	c.keepaliveOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(c.keepaliveEvery)
			defer ticker.Stop()
			for {
				select {
				case <-c.keepaliveStop:
					return
				case <-ticker.C:
					c.keepalivePing()
				}
			}
		}()
	})
}

// keepalivePing pings the current connection if there is one and the client
// is in use. It does not count as use.
func (c *Client) keepalivePing() {
	if time.Since(time.Unix(0, c.lastUse.Load())) > c.keepaliveIdle {
		return
	}
	c.connMu.Lock()
	conn := c.conn
	c.connMu.Unlock()
	if conn == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return
	}
	_, _ = c.send(ctx, conn, "core.ping", nil)
}

// dial opens the websocket connection and (if an authFn was stored by
// Connect) authenticates. Unlike the legacy /websocket endpoint, JSON-RPC
// needs no protocol handshake — the first frame can be a call. Shared by
// Connect and reconnect so there is exactly one implementation of the
// connect path.
func (c *Client) dial(ctx context.Context) error {
	for attempt := 0; ; attempt++ {
		err := c.dialOnce(ctx)
		if err == nil || !errors.Is(err, errClosedDuringAuth) || attempt >= len(dialRetryDelays) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(jitteredBackoff(dialRetryDelays[attempt])):
		}
	}
}

// errClosedDuringAuth marks a dial whose connection the server closed, or
// stopped answering, before authentication finished. A busy server (seen
// under login rate limiting) does this; dial retries it on a fresh
// connection after dialRetryDelays.
var errClosedDuringAuth = errors.New("connection closed during authentication")

// errDial marks a failure to open the websocket at all (connection refused,
// failed handshake), as opposed to an authentication failure on an open one.
var errDial = errors.New("cannot open connection")

// reconnectRetryDelays are the waits between re-dial attempts when Call
// finds the connection gone and the server unreachable. They span about a
// minute, enough for a middleware/nginx restart. Connect uses the shorter
// dialRetryDelays: a box that is down at configuration time fails quickly.
// A package variable so tests can shrink it.
var reconnectRetryDelays = []time.Duration{2 * time.Second, 5 * time.Second, 10 * time.Second, 20 * time.Second, 30 * time.Second}

// dialRetryDelays are the waits before re-dialing after errClosedDuringAuth,
// and before Connect retries a failed websocket open (errDial). A package
// variable so tests can shrink it.
var dialRetryDelays = []time.Duration{2 * time.Second, 5 * time.Second, 10 * time.Second}

// dialOnce makes one connect+auth attempt (see dial).
func (c *Client) dialOnce(ctx context.Context) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
		TLSClientConfig:  c.tlsConfig,
		ReadBufferSize:   65536,
		WriteBufferSize:  65536,
	}

	header := http.Header{}
	header.Set("User-Agent", c.userAgent)

	ws, _, err := dialer.DialContext(ctx, c.endpoint, header)
	if err != nil {
		return fmt.Errorf("websocket dial %s: %w: %w", c.endpoint, errDial, err)
	}
	// No websocket-level read limit: readLoop enforces maxResponseBytes
	// itself so an oversized response fails only its own call instead of
	// tearing down the connection.
	ws.SetReadLimit(0)
	conn := &wsConn{Conn: ws}

	c.connMu.Lock()
	authFn := c.authFn
	c.connMu.Unlock()

	done := make(chan struct{})
	go c.readLoop(conn, done)

	// Authenticate BEFORE publishing the connection in c.conn: other
	// goroutines only ever send on an authenticated socket. The auth calls
	// reach this connection through the context (see authConnKey).
	if authFn != nil {
		if err := authFn(context.WithValue(ctx, authConnKey{}, conn)); err != nil {
			// Checked before closing it ourselves: did the connection drop
			// under the auth call? failPending marks it closed before it
			// fails the call, so this is reliable.
			c.pendingMu.Lock()
			dropped := conn.closed
			c.pendingMu.Unlock()
			_ = conn.Close()
			if dropped || errors.Is(err, errNoResponse) {
				return fmt.Errorf("websocket %s: %w: %v", c.endpoint, errClosedDuringAuth, err)
			}
			return err
		}
	}

	c.connMu.Lock()
	defer c.connMu.Unlock()
	select {
	case <-done:
		return fmt.Errorf("websocket %s: %w", c.endpoint, errClosedDuringAuth)
	default:
	}
	c.conn = conn
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

// readLoop reads responses from conn until it fails, then fails the calls
// pending on conn, unpublishes it, and closes done (under connMu, so dial can
// tell whether the connection died before it was published).
func (c *Client) readLoop(conn *wsConn, done chan struct{}) {
	err := c.readMessages(conn)
	c.failPending(conn, fmt.Errorf("connection closed: %w", err))
	c.connMu.Lock()
	if c.conn == conn {
		c.conn = nil
	}
	close(done)
	c.connMu.Unlock()
}

// readMessages dispatches responses until a transport or protocol error,
// which it returns.
func (c *Client) readMessages(conn *wsConn) error {
	for {
		_, r, err := conn.NextReader()
		if err != nil {
			return err
		}
		buf, err := io.ReadAll(io.LimitReader(r, maxResponseBytes+1))
		if err != nil {
			return err
		}
		if int64(len(buf)) > maxResponseBytes {
			// Fail only the call this response answers. Its id may come
			// after the oversized result, so scan the rest of the message
			// for it without keeping the values.
			id, ok := scanResponseID(io.MultiReader(bytes.NewReader(buf), r))
			if !ok {
				return fmt.Errorf("oversized response (> %d bytes) without a readable id", maxResponseBytes)
			}
			c.deliver(id, callResult{err: fmt.Errorf("%w (%d bytes)", ErrResponseTooLarge, maxResponseBytes)})
			continue
		}
		var msg rpcResponse
		if err := json.Unmarshal(buf, &msg); err != nil {
			return err
		}
		// Server-push notifications (collection_update etc.) carry no ID.
		if msg.ID == nil {
			continue
		}
		if msg.Error != nil {
			c.deliver(*msg.ID, callResult{err: msg.Error.apiError()})
		} else {
			c.deliver(*msg.ID, callResult{result: msg.Result})
		}
	}
}

// deliver hands res to the pending call with this id, if it is still
// waiting.
func (c *Client) deliver(id uint64, res callResult) {
	c.pendingMu.Lock()
	p, ok := c.pending[id]
	if ok {
		delete(c.pending, id)
	}
	c.pendingMu.Unlock()
	if ok {
		p.ch <- res
	}
}

// scanResponseID walks one JSON-RPC response object token by token and
// returns its top-level "id", without holding the other values in memory.
func scanResponseID(r io.Reader) (uint64, bool) {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return 0, false
	}
	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return 0, false
		}
		if key, _ := t.(string); key == "id" {
			var id uint64
			if err := dec.Decode(&id); err != nil {
				return 0, false
			}
			return id, true
		}
		if err := skipJSONValue(dec); err != nil {
			return 0, false
		}
	}
	return 0, false
}

// skipJSONValue consumes one complete value from dec.
func skipJSONValue(dec *json.Decoder) error {
	depth := 0
	for {
		t, err := dec.Token()
		if err != nil {
			return err
		}
		switch t {
		case json.Delim('{'), json.Delim('['):
			depth++
		case json.Delim('}'), json.Delim(']'):
			depth--
		}
		if depth == 0 {
			return nil
		}
	}
}

// failPending fails every call in flight on conn after a transport loss.
// Calls on other connections (a newer connection being authenticated, say)
// are untouched. The deliberate wrapping as &APIError{Code: 0} (no real
// TrueNAS error carries code 0) is what CallRead's isTransient uses to
// recognize a mid-flight transport failure as retryable.
func (c *Client) failPending(conn *wsConn, err error) {
	c.pendingMu.Lock()
	defer c.pendingMu.Unlock()
	conn.closed = true
	for id, p := range c.pending {
		if p.conn != conn {
			continue
		}
		p.ch <- callResult{err: &APIError{Message: err.Error()}}
		delete(c.pending, id)
	}
}

// Call invokes a TrueNAS method and returns the raw JSON result.
func (c *Client) Call(ctx context.Context, method string, params ...any) (json.RawMessage, error) {
	// An authFn run by dial: send on the connection being authenticated,
	// without a semaphore slot (see authConnKey).
	if conn, ok := ctx.Value(authConnKey{}).(*wsConn); ok {
		return c.send(ctx, conn, method, params)
	}

	c.lastUse.Store(time.Now().UnixNano())

	// Bound in-flight concurrent RPCs (see the sem field): acquire a slot or
	// wait, so calls beyond the server's per-connection cap queue instead of
	// failing with -32000. Released on every return path via defer.
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	c.connMu.Lock()
	conn := c.conn
	c.connMu.Unlock()
	if conn == nil {
		// The connection dropped (readLoop nils c.conn on transport loss) and
		// nothing has been sent for THIS call yet, so re-dialing and sending
		// once cannot duplicate a side effect. Reconnect before giving up, so a
		// call that lands just after an idle drop survives it instead of failing
		// with "not connected". A mid-flight drop — the write in send, or a
		// connection lost while awaiting the response (failPending) — is
		// deliberately NOT reconnected-and-retried here: that request may already
		// have taken effect, and silently resending it could double-apply a
		// mutating call. (Idempotent reads layer their own retry in CallRead.)
		//
		// While the server cannot be reached at all (a middleware or nginx
		// restart refuses or fails the websocket handshake for a while) the
		// re-dial is retried after reconnectRetryDelays: still nothing has
		// been sent, so waiting is safe for any call.
		for attempt := 0; ; attempt++ {
			rErr := c.reconnect(ctx)
			c.connMu.Lock()
			conn = c.conn
			c.connMu.Unlock()
			if conn != nil {
				break
			}
			if rErr == nil {
				return nil, fmt.Errorf("not connected")
			}
			if !errors.Is(rErr, errDial) || attempt >= len(reconnectRetryDelays) {
				return nil, fmt.Errorf("not connected: %v", rErr)
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(reconnectRetryDelays[attempt]):
			}
		}
	}
	return c.send(ctx, conn, method, params)
}

// send writes one request on conn and waits for its response.
func (c *Client) send(ctx context.Context, conn *wsConn, method string, params []any) (json.RawMessage, error) {
	id := c.seq.Add(1)
	p := &pendingCall{ch: make(chan callResult, 1), conn: conn}

	c.pendingMu.Lock()
	if conn.closed {
		// The reader is gone: nothing would ever answer this call. Nothing
		// has been sent, so this is the same as "not connected".
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("write %s: connection closed", method)
	}
	c.pending[id] = p
	c.pendingMu.Unlock()

	if params == nil {
		params = []any{}
	}
	c.writeMu.Lock()
	_ = conn.SetWriteDeadline(time.Now().Add(writeTimeout))
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

	// A caller without a deadline (Terraform's contexts usually have none)
	// still gets an answer or an error: a server that never answers, or a
	// connection that died without the reader noticing, must not hang the
	// run.
	var timeout <-chan time.Time
	limit := callTimeout
	if _, auth := ctx.Value(authConnKey{}).(*wsConn); auth {
		limit = authCallTimeout
	}
	if _, has := ctx.Deadline(); !has {
		t := time.NewTimer(limit)
		defer t.Stop()
		timeout = t.C
	}

	select {
	case <-ctx.Done():
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		return nil, ctx.Err()
	case <-timeout:
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
		c.dropIfUnresponsive(conn)
		return nil, fmt.Errorf("%s: %w after %s", method, errNoResponse, limit)
	case res := <-p.ch:
		if errors.Is(res.err, ErrResponseTooLarge) {
			return nil, fmt.Errorf("%s: %w", method, res.err)
		}
		return res.result, res.err
	}
}

// callTimeout bounds a call whose context has no deadline. Long server work
// runs as jobs (the call itself returns a job id at once, and each poll is a
// separate short call), so this only has to cover slow synchronous methods.
// authCallTimeout bounds each login call (the server's rate limiter delays
// a rejection by at most 10s). writeTimeout bounds writing one request.
// Package variables so tests can shrink them.
var (
	callTimeout     = 10 * time.Minute
	authCallTimeout = 2 * time.Minute
	writeTimeout    = 30 * time.Second
)

// dropIfUnresponsive closes conn when it does not answer a ping, so the next
// call re-dials instead of waiting out callTimeout again on a dead
// connection. A connection that answers is left alone: one slow call must
// not fail the others in flight.
func (c *Client) dropIfUnresponsive(conn *wsConn) {
	ctx, cancel := context.WithTimeout(context.Background(), pingTimeout)
	defer cancel()
	if _, err := c.send(ctx, conn, "core.ping", nil); err != nil {
		_ = conn.Close()
	}
}

// errNoResponse marks a call that timed out waiting for its answer.
var errNoResponse = errors.New("no response from TrueNAS")

// pingTimeout bounds the liveness ping of dropIfUnresponsive.
var pingTimeout = 15 * time.Second

// Close shuts down the WebSocket connection and stops the keepalive.
func (c *Client) Close() error {
	c.stopOnce.Do(func() { close(c.keepaliveStop) })
	c.connMu.Lock()
	defer c.connMu.Unlock()
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}
