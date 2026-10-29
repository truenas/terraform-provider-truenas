// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package audit_config_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// restoreConfigKeys reads audit.config now and, when the test ends, writes
// every listed key back with its original value through audit.update. A
// set-and-restore test must restore every field its configuration writes,
// not only the one it varies: the earlier tests here restored a single field
// and left the rest of their fixed test values on every box they ran on.
func restoreConfigKeys(t *testing.T, keys ...string) {
	t.Helper()
	raw, err := acctest.RestoreCall(context.Background(), "audit.config")
	if err != nil {
		t.Fatalf("read audit.config: %v", err)
	}
	var orig map[string]any
	if err := json.Unmarshal(raw, &orig); err != nil {
		t.Fatalf("parse audit.config: %v", err)
	}
	t.Cleanup(func() {
		p := map[string]any{}
		for _, k := range keys {
			if v, ok := orig[k]; ok {
				p[k] = v
			}
		}
		if _, err := acctest.RestoreCall(context.Background(), "audit.update", p); err != nil {
			t.Errorf("restore audit.config %v: %v", p, err)
		}
	})
}
