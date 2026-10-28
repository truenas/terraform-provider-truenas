// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// coverageRatchetEntry matches one entry line in coverage_ratchet_test.go:
//
//	"truenas_<res>.<attr>": true, // <reason>
//
// capturing the key and whatever trails the value (the reason comment, if any).
var coverageRatchetEntry = regexp.MustCompile(`^\s*"(truenas_[a-z0-9_]+\.[a-z0-9_]+)"\s*:\s*(?:true|false)\s*,(.*)$`)

// TestCoverageRatchet_Justified fails if any coverageRatchet entry is BARE — a
// writable attribute parked as accepted debt with no stated reason. This is the
// enforcement that keeps the "223 unexercised attributes hiding behind a green
// check" problem from recurring: every entry must either be exercised (and
// removed) or justified in-line (external dependency / unsafe to toggle on a
// shared box / write-only / etc.). A new bare entry turns this test red.
//
// It is a plain unit test (no TF_ACC): it reads the ratchet source directly.
func TestCoverageRatchet_Justified(t *testing.T) {
	root := repoRoot(t)
	path := filepath.Join(root, "internal", "acctest", "coverage_ratchet_test.go")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	var bare []string
	for _, line := range strings.Split(string(src), "\n") {
		m := coverageRatchetEntry.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		key, trailer := m[1], m[2]
		// A reason is a // comment with real text after the entry.
		reason := ""
		if i := strings.Index(trailer, "//"); i >= 0 {
			reason = strings.TrimSpace(trailer[i+2:])
		}
		if reason == "" {
			bare = append(bare, key)
		}
	}

	if len(bare) > 0 {
		t.Errorf("%d coverageRatchet entries are BARE (no reason). Either exercise the attribute "+
			"in an acceptance config and delete the entry, or add an inline // reason "+
			"(external dependency / unsafe on a shared box / write-only). Bare entries:\n  %s",
			len(bare), strings.Join(bare, "\n  "))
	}
}
