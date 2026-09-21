// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package loadtest

import "testing"

func TestLoadGateDecision(t *testing.T) {
	cases := []struct {
		name, load, allowed, endpoint string
		wantSkip                      bool
		wantFatal                     bool
	}{
		{"unset skips", "", "", "wss://x/api/current", true, false},
		{"set but no allowed fatals", "1", "", "wss://x/api/current", false, true},
		{"mismatch fatals", "1", "wss://y/api/current", "wss://x/api/current", false, true},
		{"match passes", "1", "wss://x/api/current", "wss://x/api/current", false, false},
	}
	for _, c := range cases {
		skip, fatal := loadGateDecision(c.load, c.allowed, c.endpoint)
		if skip != c.wantSkip || (fatal != "") != c.wantFatal {
			t.Errorf("%s: skip=%v fatal=%q, want skip=%v fatal=%v",
				c.name, skip, fatal, c.wantSkip, c.wantFatal)
		}
	}
}
