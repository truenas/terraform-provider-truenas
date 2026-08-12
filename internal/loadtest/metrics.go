// Copyright iXsystems, Inc. 2026
// SPDX-License-Identifier: MPL-2.0

// Package loadtest provides the shared safety gate, metrics, reporting, and
// cleanup used by the provider's load tests. See
// docs/superpowers/specs/2026-08-12-load-testing-design.md.
package loadtest

import (
	"sync"
	"time"
)

// Metrics is a concurrency-safe collector for a single load run.
type Metrics struct {
	mu           sync.Mutex
	attempts     int
	rateLimits   int
	retries      int
	totalBackoff time.Duration
	failures     map[string]int
}

// Snapshot is an immutable copy of a Metrics state for reporting.
type Snapshot struct {
	Attempts      int
	RateLimitHits int
	Retries       int
	TotalBackoff  time.Duration
	Failures      map[string]int
}

func NewMetrics() *Metrics { return &Metrics{failures: map[string]int{}} }

func (m *Metrics) RecordAttempt()   { m.mu.Lock(); m.attempts++; m.mu.Unlock() }
func (m *Metrics) RecordRateLimit() { m.mu.Lock(); m.rateLimits++; m.mu.Unlock() }

func (m *Metrics) RecordRetry(backoff time.Duration) {
	m.mu.Lock()
	m.retries++
	m.totalBackoff += backoff
	m.mu.Unlock()
}

func (m *Metrics) RecordFailure(class string) {
	m.mu.Lock()
	m.failures[class]++
	m.mu.Unlock()
}

func (m *Metrics) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	f := make(map[string]int, len(m.failures))
	for k, v := range m.failures {
		f[k] = v
	}
	return Snapshot{
		Attempts:      m.attempts,
		RateLimitHits: m.rateLimits,
		Retries:       m.retries,
		TotalBackoff:  m.totalBackoff,
		Failures:      f,
	}
}
