// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package zvol

import "testing"

// Hunt F7 (see dataset/compression_alias_test.go): GZIP-6 is ZFS's default
// gzip level and reads back as gzip on 28.0; send it as GZIP everywhere.
func TestCompressionAPI_FoldsAliases(t *testing.T) {
	cases := map[string]string{
		"GZIP-6":      "GZIP",
		"GZIP-9":      "GZIP-9",
		"ZSTD-FAST-1": "ZSTD-FAST",
		"zstd":        "ZSTD",
	}
	for in, want := range cases {
		if got := compressionAPI(in); got != want {
			t.Errorf("compressionAPI(%q) = %q, want %q", in, got, want)
		}
	}
}
