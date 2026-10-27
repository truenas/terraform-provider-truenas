// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

import (
	"strings"
	"testing"
)

// TestServerTruthRatchets_NoStale makes "ratchets only shrink" enforceable. It
// needs no TrueNAS box. It fails when a ratchet still lists something that has
// since been wired (or that never belonged there), so the entry must be deleted.
func TestServerTruthRatchets_NoStale(t *testing.T) {
	isReal := map[string]bool{}
	for _, typ := range allProviderResourceTypes(t) {
		isReal[typ] = true
	}

	wired := map[string]serverTruthResource{}
	for _, r := range serverTruthResources() {
		wired[r.typ] = r
	}

	singletons := map[string]bool{}
	for _, r := range singletonResources() {
		singletons[r.typ] = true
		if _, ok := wired[r.typ]; ok {
			t.Errorf("singletonResources: %q is also in serverTruthResources(); a singleton is not created", r.typ)
		}
		if _, ok := unsetDefaultsRatchet[r.typ]; ok {
			t.Errorf("unsetDefaultsRatchet: %q is a wired singleton; delete its ratchet entry", r.typ)
		}
		if _, ok := unrelatedUpdateRatchet[r.typ]; ok {
			t.Errorf("unrelatedUpdateRatchet: %q is a wired singleton; delete its ratchet entry", r.typ)
		}
	}

	for typ := range unsetDefaultsRatchet {
		if !isReal[typ] {
			t.Errorf("unsetDefaultsRatchet: %q is not a provider resource type (dangling entry); delete it", typ)
		}
		if _, ok := wired[typ]; ok {
			t.Errorf("unsetDefaultsRatchet: %q is wired in serverTruthResources(); delete its ratchet entry", typ)
		}
	}

	for typ := range unrelatedUpdateRatchet {
		if !isReal[typ] {
			t.Errorf("unrelatedUpdateRatchet: %q is not a provider resource type (dangling entry); delete it", typ)
		}
		if r, ok := wired[typ]; ok && r.perturb != nil {
			t.Errorf("unrelatedUpdateRatchet: %q is wired for Guard 1 (has a perturb); delete its ratchet entry", typ)
		}
		if lifecycleRatchet[typ].noUpdate {
			t.Errorf("unrelatedUpdateRatchet: %q has no in-place update (lifecycleRatchet noUpdate); Guard 1 skips it by design, so it must not be ratcheted", typ)
		}
	}

	for key := range immutableDefaultTrapRatchet {
		typ, _, ok := strings.Cut(key, ".")
		if !ok {
			t.Errorf("immutableDefaultTrapRatchet: key %q is not of the form <type>.<attr>", key)
			continue
		}
		if !isReal[typ] {
			t.Errorf("immutableDefaultTrapRatchet: %q names %q, which is not a provider resource type (dangling entry); delete it", key, typ)
		}
	}
}
