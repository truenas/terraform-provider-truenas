// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestPreserveCompression guards the v1.5.12/#38-class compression fix: ZFS
// folds the level-1 fast tier to plain zstd-fast, so compression="ZSTD-FAST-1"
// must still round-trip (be kept) when the API reports "zstd-fast". Real drift
// (a different algorithm) must overwrite state with the lower-cased API value.
func TestPreserveCompression(t *testing.T) {
	cases := []struct {
		name    string
		current types.String
		apiVal  string
		want    string
	}{
		{"fast-1 alias kept", types.StringValue("ZSTD-FAST-1"), "zstd-fast", "ZSTD-FAST-1"},
		{"fast-1 alias lowercase kept", types.StringValue("zstd-fast-1"), "zstd-fast", "zstd-fast-1"},
		{"gzip-6 alias kept (F7: 28.0 stores gzip)", types.StringValue("GZIP-6"), "gzip", "GZIP-6"},
		{"gzip-9 is not an alias", types.StringValue("GZIP-9"), "gzip", "gzip"},
		{"equal-fold casing kept", types.StringValue("ZSTD-5"), "zstd-5", "ZSTD-5"},
		{"real drift overwrites", types.StringValue("lz4"), "zstd", "zstd"},
		{"null current -> lowercased api", types.StringNull(), "lz4", "lz4"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := preserveCompression(tc.current, tc.apiVal).ValueString()
			if got != tc.want {
				t.Errorf("preserveCompression(%v, %q) = %q, want %q", tc.current, tc.apiVal, got, tc.want)
			}
		})
	}
}

// TestResponseToModel_AcltypeInherit guards the v1.5.11 acltype fix: a non-LOCAL
// (inherited/default) acltype must read back as the INHERIT sentinel, never the
// resolved pool default (e.g. "posix"), so acltype="inherit" round-trips instead
// of failing with an inconsistent result. A LOCAL acltype reads back verbatim.
func TestResponseToModel_AcltypeInherit(t *testing.T) {
	r := &DatasetResource{}
	cases := []struct {
		name           string
		source, parsed string
		config         types.String // the plan/state value carried into read
		wantInherit    bool
		wantExact      string
	}{
		{"local nfsv4 verbatim", "LOCAL", "nfsv4", types.StringValue("nfsv4"), false, "nfsv4"},
		{"inherited resolves to INHERIT (config inherit)", "INHERITED", "posix", types.StringValue("inherit"), true, ""},
		{"default resolves to INHERIT (config unset)", "DEFAULT", "posix", types.StringNull(), true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api := &apiResponse{Name: "tank/x", Type: "FILESYSTEM"}
			api.AClType.Source = tc.source
			api.AClType.Parsed = tc.parsed
			m := &DatasetModel{AClType: tc.config}
			if d := r.responseToModel(api, m); d.HasError() {
				t.Fatalf("responseToModel: %v", d)
			}
			got := m.AClType.ValueString()
			if tc.wantInherit {
				if strings.EqualFold(got, "posix") || !strings.EqualFold(got, "inherit") {
					t.Errorf("acltype source=%s parsed=%s => %q, want INHERIT (never the resolved %q)", tc.source, tc.parsed, got, tc.parsed)
				}
			} else if got != tc.wantExact {
				t.Errorf("acltype source=%s => %q, want %q", tc.source, got, tc.wantExact)
			}
		})
	}
}
