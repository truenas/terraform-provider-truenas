// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// readRetryDelays is the sequence of backoff delays between CallRead retry
// attempts. It's a package variable (not a const) so tests can shrink it to
// avoid slow, real-time-bound test runs.
var readRetryDelays = []time.Duration{2 * time.Second, 5 * time.Second, 10 * time.Second}

// isTransient reports whether err is safe to retry for an idempotent read:
//
//   - a transport-level failure. Most of these are plain (non-APIError)
//     errors — "not connected" and write failures, returned directly by
//     Call before a request is even sent. But a connection that dies while
//     a call is in flight is reported differently: readLoop's failPending
//     delivers it through the same channel as a server response, wrapped as
//     &APIError{Message: "connection closed: ..."} with Code left at its
//     zero value (client.go's failPending is the only place that
//     constructs an APIError without an explicit Code). Since real TrueNAS
//     wire errors always carry a nonzero `error` code, Code == 0 reliably
//     identifies this synthetic transport-failure wrapping rather than a
//     genuine API response.
//   - a TrueNAS rate-limit rejection (*APIError with Code == 16, or whose
//     Message contains "Rate Limit", case-insensitively).
//   - a concurrent-call-cap rejection (*APIError with Code == -32000, or
//     whose Message contains "concurrent calls"). The client's own
//     concurrency limiter (see Client.sem) keeps this from occurring for our
//     calls, but retrying it defends against a transient overflow (e.g.
//     another client sharing the box, or a burst during reconnect).
//
// Any other *APIError (EINVAL, ENOENT/code 2, ...) is a real API response,
// not a transient condition, and must NOT be retried — retrying those would
// change IsNotFound semantics and mask genuine errors as flakiness.
func isTransient(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		msg := strings.ToLower(apiErr.Message)
		return apiErr.Code == 0 || apiErr.Code == 16 || apiErr.Code == -32000 ||
			strings.Contains(msg, "rate limit") || strings.Contains(msg, "concurrent calls")
	}
	return true
}

// CallRead invokes an idempotent read method (get_instance / query /
// <singleton>.config) with automatic retries on transient failures.
//
// A successful create followed by a failed read-back would orphan the
// created object in Terraform state: the framework leaves state null, so
// Terraform believes creation failed, and the next apply re-creates the
// object — duplicate or conflict. The realistic read-back failure causes
// are transient (auth/API rate limiting, momentary transport drops), so
// retrying idempotent reads closes that window.
//
// Retries happen after readRetryDelays (2s, 5s, 10s by default), are
// context-aware (an expired/cancelled ctx aborts the wait immediately), and
// fire ONLY on transient failures per isTransient — never on other API
// errors, so IsNotFound semantics are unchanged for callers.
//
// A transport failure leaves the underlying connection dead (readLoop nils
// c.conn out when its ReadJSON fails), so between attempts CallRead calls
// reconnect, which re-dials only when there is no live connection.
func (c *Client) CallRead(ctx context.Context, method string, params ...any) (json.RawMessage, error) {
	var lastErr error
	for attempt := 0; ; attempt++ {
		raw, err := c.Call(ctx, method, params...)
		if err == nil {
			return raw, nil
		}
		lastErr = err

		if !isTransient(err) || attempt >= len(readRetryDelays) {
			return nil, lastErr
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(readRetryDelays[attempt]):
		}

		if rErr := c.reconnect(ctx); rErr != nil {
			return nil, fmt.Errorf("%s: reconnect after transient failure (%v): %w", method, lastErr, rErr)
		}
	}
}
