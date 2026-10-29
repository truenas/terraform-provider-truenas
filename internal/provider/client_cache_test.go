// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"errors"
	"testing"

	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// Configure reuses the connected client for an identical configuration
// (in-process acceptance tests call it for every Terraform command, and each
// call used to leave a connection open), never reuses one across different
// configurations, and does not keep a failed connect.
func TestSharedClient(t *testing.T) {
	k1 := clientCacheKey("wss://a/api/current", "1-key", "root", "", "", true, "dev")
	k2 := clientCacheKey("wss://a/api/current", "2-key", "root", "", "", true, "dev")
	if k1 == k2 {
		t.Fatal("different API keys share a cache key")
	}
	t.Cleanup(func() {
		clientsMu.Lock()
		delete(clients, k1)
		delete(clients, k2)
		clientsMu.Unlock()
	})

	connects := 0
	connect := func() (*client.Client, error) {
		connects++
		return client.New("wss://a/api/current", nil), nil
	}
	if _, err := sharedClient(k2, func() (*client.Client, error) { connects++; return nil, errors.New("refused") }); err == nil {
		t.Fatal("failed connect returned no error")
	}
	c1, _ := sharedClient(k1, connect)
	c1again, _ := sharedClient(k1, connect)
	c2, _ := sharedClient(k2, connect)
	if c1 != c1again || c1 == c2 {
		t.Fatalf("reuse wrong: same-config reused=%v, other-config shared=%v", c1 == c1again, c1 == c2)
	}
	if connects != 3 {
		t.Fatalf("connects = %d, want 3 (failed k2, k1 once, k2 again)", connects)
	}
}
