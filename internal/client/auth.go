// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

// AuthAPIKey authenticates using a TrueNAS API key.
// The API key format is "<id>-<secret>", e.g. "1-abcdef1234".
func AuthAPIKey(ctx context.Context, c *Client, apiKey string) error {
	raw, err := c.Call(ctx, "auth.login_with_api_key", apiKey)
	if err != nil {
		return fmt.Errorf("auth.login_with_api_key: %w", err)
	}
	var ok bool
	if err := json.Unmarshal(raw, &ok); err != nil {
		return fmt.Errorf("parse auth response: %w", err)
	}
	if !ok {
		return fmt.Errorf("authentication rejected: invalid API key")
	}
	return nil
}

// AuthPassword authenticates using a username and password.
func AuthPassword(ctx context.Context, c *Client, username, password string) error {
	raw, err := c.Call(ctx, "auth.login", username, password)
	if err != nil {
		return fmt.Errorf("auth.login: %w", err)
	}
	var ok bool
	if err := json.Unmarshal(raw, &ok); err != nil {
		return fmt.Errorf("parse auth response: %w", err)
	}
	if !ok {
		return fmt.Errorf("authentication rejected: invalid username or password")
	}
	return nil
}

// authRetryBackoff is the base sequence of sleep durations between retry
// attempts made by WithRetry after a rate-limited auth failure. Terraform
// opens a fresh provider connection (and re-authenticates) for every
// plan/apply/destroy step, and a fan-out of parallel Terraform runs (or
// rapid acceptance test runs) can exhaust TrueNAS's ~20-logins/minute auth
// rate limit; these delays give the rate limit window time to clear. Each
// delay is jittered (see jitteredBackoff) so parallel callers that all trip
// the limit at once do not retry in lockstep and re-collide.
var authRetryBackoff = []time.Duration{
	5 * time.Second, 10 * time.Second, 20 * time.Second,
	30 * time.Second, 45 * time.Second, 60 * time.Second,
}

// jitteredBackoff returns d plus a random fraction of d in [0, d/2), spreading
// out otherwise-synchronized retries. It uses math/rand — this is scheduling
// jitter, not a security value.
func jitteredBackoff(d time.Duration) time.Duration {
	if d <= 0 {
		return d
	}
	return d + time.Duration(rand.Int63n(int64(d/2)))
}

// IsRateLimited reports whether err represents a TrueNAS auth rate-limit
// rejection, e.g. "auth.login_with_api_key: truenas API error (code 16):
// [EBUSY] Rate Limit Exceeded". It checks the wrapped *APIError's code
// (16) first, then falls back to a case-insensitive substring match on the
// error text so it still catches a rate-limit rejection that reaches this
// code some other way (e.g. wrapped without preserving *APIError).
func IsRateLimited(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Code == 16 {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "rate limit")
}

// WithRetry calls authFn and, if it fails with a rate-limit error (see
// IsRateLimited), retries up to len(authRetryBackoff) more times with
// increasing backoff between attempts. Any non-rate-limit error, or
// context cancellation/deadline while waiting between attempts, returns
// immediately.
func WithRetry(ctx context.Context, authFn func(ctx context.Context) error) error {
	var err error
	for attempt := 0; ; attempt++ {
		err = authFn(ctx)
		if err == nil || !IsRateLimited(err) || attempt >= len(authRetryBackoff) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(jitteredBackoff(authRetryBackoff[attempt])):
		}
	}
}
