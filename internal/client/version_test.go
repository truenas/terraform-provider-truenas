// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package client

import "testing"

func TestVersionAtLeast(t *testing.T) {
	cases := []struct {
		version      string
		major, minor int
		want         bool
	}{
		{"26.0.0", 26, 0, true},
		{"26.0", 26, 0, true},
		{"25.10.3.1", 26, 0, false},
		{"25.10.3.1", 25, 10, true},
		{"25.10.3.1", 25, 4, true},
		{"25.04.2", 25, 10, false},
		{"27.0.0", 26, 0, true},
		{"", 26, 0, false},
		{"garbage", 26, 0, false},
		{"26.0-BETA.1", 26, 0, true},
	}
	for _, tc := range cases {
		if got := versionAtLeast(tc.version, tc.major, tc.minor); got != tc.want {
			t.Errorf("versionAtLeast(%q, %d, %d) = %v, want %v", tc.version, tc.major, tc.minor, got, tc.want)
		}
	}
}
