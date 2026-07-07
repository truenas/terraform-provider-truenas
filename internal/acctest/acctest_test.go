package acctest

import (
	"strings"
	"testing"
)

func TestRandName(t *testing.T) {
	name := RandName("foo")
	if !strings.HasPrefix(name, "foo-") {
		t.Fatalf("RandName(%q) = %q, want prefix %q", "foo", name, "foo-")
	}
	wantLen := len("foo") + 1 + 8
	if len(name) != wantLen {
		t.Fatalf("RandName(%q) = %q, len = %d, want %d", "foo", name, len(name), wantLen)
	}

	other := RandName("foo")
	if name == other {
		t.Fatalf("RandName(%q) returned the same value twice: %q", "foo", name)
	}
}

func TestEndpoint(t *testing.T) {
	if got := Endpoint(); got != TestTrueNASEndpoint {
		t.Fatalf("Endpoint() = %q, want default %q", got, TestTrueNASEndpoint)
	}

	t.Setenv("TRUENAS_ENDPOINT", "wss://example.invalid/websocket")
	if got := Endpoint(); got != "wss://example.invalid/websocket" {
		t.Fatalf("Endpoint() = %q, want env override %q", got, "wss://example.invalid/websocket")
	}
}

func TestTestPool(t *testing.T) {
	if got := TestPool(); got != "tank" {
		t.Fatalf("TestPool() = %q, want default %q", got, "tank")
	}

	t.Setenv("TRUENAS_TEST_POOL", "otherpool")
	if got := TestPool(); got != "otherpool" {
		t.Fatalf("TestPool() = %q, want env override %q", got, "otherpool")
	}
}
