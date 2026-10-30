// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import "testing"

// Hunt F7: compression = "GZIP-6" created the dataset and then failed with
// "inconsistent result after apply" on 28.0 (ZFS stores gzip level 6, its
// default, as plain gzip) and was rejected outright by 25.10/26.0. The alias
// is folded both on the way out and when comparing what came back.
func TestCompressionAPI_FoldsAliases(t *testing.T) {
	cases := map[string]string{
		"GZIP-6":      "GZIP",
		"gzip-6":      "GZIP",
		"GZIP-9":      "GZIP-9",
		"ZSTD-FAST-1": "ZSTD-FAST",
		"zstd-3":      "ZSTD-3",
		"lz4":         "LZ4",
		"Inherit":     "INHERIT",
	}
	for in, want := range cases {
		if got := compressionAPI(in); got != want {
			t.Errorf("compressionAPI(%q) = %q, want %q", in, got, want)
		}
	}
}
