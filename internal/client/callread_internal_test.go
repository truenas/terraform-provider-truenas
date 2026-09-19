// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"fmt"
	"testing"
)

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
		{"concurrent-call cap APIError code -32000", &APIError{Code: -32000, Message: "Maximum number of concurrent calls (20) has exceeded"}, true},
		{"concurrent-call cap by message only", &APIError{Code: 22, Message: "Maximum number of concurrent calls (20) has exceeded"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTransient(tt.err); got != tt.want {
				t.Errorf("isTransient(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}
