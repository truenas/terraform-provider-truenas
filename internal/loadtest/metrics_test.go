// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package loadtest

import (
	"sync"
	"testing"
	"time"
)

func TestMetrics_ConcurrentRecording(t *testing.T) {
	m := NewMetrics()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.RecordAttempt()
			m.RecordRateLimit()
			m.RecordRetry(2 * time.Second)
			m.RecordFailure("calljob")
		}()
	}
	wg.Wait()
	s := m.Snapshot()
	if s.Attempts != 100 || s.RateLimitHits != 100 || s.Retries != 100 {
		t.Fatalf("counters = %+v, want 100 each", s)
	}
	if s.TotalBackoff != 200*time.Second {
		t.Fatalf("TotalBackoff = %v, want 200s", s.TotalBackoff)
	}
	if s.Failures["calljob"] != 100 {
		t.Fatalf("Failures[calljob] = %d, want 100", s.Failures["calljob"])
	}
	// Snapshot must be a copy: mutating the returned map must not affect m.
	s.Failures["calljob"] = 0
	if m.Snapshot().Failures["calljob"] != 100 {
		t.Fatal("Snapshot did not return a defensive copy of Failures")
	}
}
