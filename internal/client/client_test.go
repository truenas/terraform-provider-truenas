// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package client_test

import (
	"fmt"
	"testing"

	"github.com/truenas/terraform-provider-truenas/internal/client"
)

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
