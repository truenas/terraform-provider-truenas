// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"testing"
	"time"
)

// TestJitteredBackoff checks the jitter stays in [d, d+d/2) and that a
// non-positive base is returned unchanged.
func TestJitteredBackoff(t *testing.T) {
	base := 10 * time.Second
	for i := 0; i < 1000; i++ {
		got := jitteredBackoff(base)
		if got < base || got >= base+base/2 {
			t.Fatalf("jitteredBackoff(%v) = %v, want in [%v, %v)", base, got, base, base+base/2)
		}
	}
	if jitteredBackoff(0) != 0 {
		t.Errorf("jitteredBackoff(0) = non-zero, want 0")
	}
	if jitteredBackoff(-5) != -5 {
		t.Errorf("jitteredBackoff(-5) should return the input unchanged")
	}
}

// TestWithMaxConcurrentCalls checks the in-flight cap option and its default.
func TestWithMaxConcurrentCalls(t *testing.T) {
	if c := New("wss://x/api/current", nil); cap(c.sem) != defaultMaxConcurrentCalls {
		t.Errorf("default cap(sem) = %d, want %d", cap(c.sem), defaultMaxConcurrentCalls)
	}
	if c := New("wss://x/api/current", nil, WithMaxConcurrentCalls(5)); cap(c.sem) != 5 {
		t.Errorf("cap(sem) = %d, want 5", cap(c.sem))
	}
	// A value < 1 is ignored; the default stands.
	if c := New("wss://x/api/current", nil, WithMaxConcurrentCalls(0)); cap(c.sem) != defaultMaxConcurrentCalls {
		t.Errorf("cap(sem) with 0 = %d, want default %d", cap(c.sem), defaultMaxConcurrentCalls)
	}
}
