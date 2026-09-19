// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package loadtest

import "testing"

func TestSweepMatch(t *testing.T) {
	// A row whose field has the prefix returns its id and true.
	id, ok := sweepMatch(map[string]any{"id": float64(42), "username": "tf-load-usr-1"}, "username", "tf-load-")
	if !ok || id != 42 {
		t.Fatalf("got (%d,%v), want (42,true)", id, ok)
	}
	// Wrong prefix: no match.
	if _, ok := sweepMatch(map[string]any{"id": float64(7), "username": "root"}, "username", "tf-load-"); ok {
		t.Error("root should not match tf-load- prefix")
	}
	// Missing/blank field: no match, no panic.
	if _, ok := sweepMatch(map[string]any{"id": float64(7)}, "username", "tf-load-"); ok {
		t.Error("missing field should not match")
	}
	// Missing id: no match.
	if _, ok := sweepMatch(map[string]any{"username": "tf-load-x"}, "username", "tf-load-"); ok {
		t.Error("missing id should not match")
	}
}
