// Copyright TrueNAS 2026
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
	samples      map[string]string
}

// Snapshot is an immutable copy of a Metrics state for reporting.
type Snapshot struct {
	Attempts      int
	RateLimitHits int
	Retries       int
	TotalBackoff  time.Duration
	Failures      map[string]int
	// FailureSamples holds one representative error string per failure class,
	// so a report shows WHAT failed, not just how many.
	FailureSamples map[string]string
}

func NewMetrics() *Metrics {
	return &Metrics{failures: map[string]int{}, samples: map[string]string{}}
}

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

// RecordFailureSample records both a failure in class and, if none is stored
// yet for that class, keeps msg as the representative error string.
func (m *Metrics) RecordFailureSample(class, msg string) {
	m.mu.Lock()
	m.failures[class]++
	if _, ok := m.samples[class]; !ok {
		m.samples[class] = msg
	}
	m.mu.Unlock()
}

func (m *Metrics) Snapshot() Snapshot {
	m.mu.Lock()
	defer m.mu.Unlock()
	f := make(map[string]int, len(m.failures))
	for k, v := range m.failures {
		f[k] = v
	}
	s := make(map[string]string, len(m.samples))
	for k, v := range m.samples {
		s[k] = v
	}
	return Snapshot{
		Attempts:       m.attempts,
		RateLimitHits:  m.rateLimits,
		Retries:        m.retries,
		TotalBackoff:   m.totalBackoff,
		Failures:       f,
		FailureSamples: s,
	}
}
