// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container_image

import "testing"

// --- versionGateDiagnostics ------------------------------------------------

func TestVersionGateDiagnostics_BelowFloor(t *testing.T) {
	for _, version := range []string{"25.10.3.1", "25.10", "24.10.2", "1.0", ""} {
		diags := versionGateDiagnostics(version)
		if !diags.HasError() {
			t.Fatalf("version %q: expected an error diagnostic, got none", version)
		}
		if diags[0].Detail() != "truenas_container_image requires TrueNAS 26.0 or later" {
			t.Errorf("version %q: detail = %q, want the documented message", version, diags[0].Detail())
		}
	}
}

func TestVersionGateDiagnostics_AtOrAboveFloor(t *testing.T) {
	for _, version := range []string{"26.0.0", "26.0.0-BETA.2", "26.0", "26.1.0", "27.0.0"} {
		diags := versionGateDiagnostics(version)
		if diags.HasError() {
			t.Errorf("version %q: unexpected error diagnostic: %v", version, diags)
		}
	}
}

// --- findRegistryImage --------------------------------------------------------

func TestFindRegistryImage(t *testing.T) {
	results := []registryImageAPI{
		{Name: "alpine:3.21:amd64:default", Versions: []registryVersionAPI{{Version: "v1"}}},
		{Name: "alpine:3.22:amd64:default", Versions: []registryVersionAPI{
			{Version: "20260718_13:00"}, {Version: "20260719_13:00"}, {Version: "20260722_17:16"},
		}},
	}

	t.Run("found", func(t *testing.T) {
		entry, ok := findRegistryImage(results, "alpine:3.22:amd64:default")
		if !ok {
			t.Fatal("expected to find alpine:3.22:amd64:default")
		}
		if len(entry.Versions) != 3 {
			t.Errorf("got %d versions, want 3", len(entry.Versions))
		}
	})
	t.Run("not found", func(t *testing.T) {
		_, ok := findRegistryImage(results, "does-not-exist")
		if ok {
			t.Error("expected not found")
		}
	})
	t.Run("empty registry", func(t *testing.T) {
		_, ok := findRegistryImage(nil, "alpine:3.22:amd64:default")
		if ok {
			t.Error("expected not found in an empty registry")
		}
	})
}

// --- versionStrings / latestVersion --------------------------------------------

func TestVersionStrings_PreservesOrder(t *testing.T) {
	in := []registryVersionAPI{{Version: "a"}, {Version: "b"}, {Version: "c"}}
	got := versionStrings(in)
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestVersionStrings_Empty(t *testing.T) {
	got := versionStrings(nil)
	if len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}

func TestLatestVersion(t *testing.T) {
	t.Run("last entry, registry order (probed live: oldest to newest)", func(t *testing.T) {
		got, ok := latestVersion([]string{"20260718_13:00", "20260719_13:00", "20260722_17:16"})
		if !ok {
			t.Fatal("expected ok=true")
		}
		if got != "20260722_17:16" {
			t.Errorf("latestVersion = %q, want the last (newest) entry", got)
		}
	})
	t.Run("empty", func(t *testing.T) {
		_, ok := latestVersion(nil)
		if ok {
			t.Error("expected ok=false for an empty version list")
		}
	})
	t.Run("single entry", func(t *testing.T) {
		got, ok := latestVersion([]string{"only"})
		if !ok || got != "only" {
			t.Errorf("latestVersion = %q, %v, want \"only\", true", got, ok)
		}
	})
}
