// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// GetInstance calls <ns>.get_instance for id (int64 or string, as the
// resource is keyed) and returns the server's stored
// state as a generic map. It fatals on any call or decode error.
func GetInstance(t *testing.T, ns string, id any) map[string]any {
	t.Helper()
	raw, err := Client().Call(context.Background(), ns+".get_instance", id)
	if err != nil {
		t.Fatalf("acctest: %s.get_instance(%v): %v", ns, id, err)
	}
	var inst map[string]any
	if err := json.Unmarshal(raw, &inst); err != nil {
		t.Fatalf("acctest: decode %s.get_instance(%v): %v", ns, id, err)
	}
	return inst
}

// ServerPath looks up a dotted path ("quota.source") in a decoded instance.
func ServerPath(inst map[string]any, path string) (any, bool) {
	var cur any = inst
	for _, part := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}

// AssertServerValue fails unless the value at path equals want.
func AssertServerValue(t *testing.T, inst map[string]any, path string, want any) {
	t.Helper()
	assertServerValue(t, inst, path, want)
}

func assertServerValue(t testing.TB, inst map[string]any, path string, want any) {
	t.Helper()
	got, ok := ServerPath(inst, path)
	if !ok {
		t.Errorf("server state has no %q", path)
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("server state %q = %#v, want %#v", path, got, want)
	}
}

// AssertServerSource fails unless the string at path (typically a
// "<prop>.source" field) equals want.
func AssertServerSource(t *testing.T, inst map[string]any, path, want string) {
	t.Helper()
	assertServerSource(t, inst, path, want)
}

func assertServerSource(t testing.TB, inst map[string]any, path, want string) {
	t.Helper()
	assertServerValue(t, inst, path, want)
}

// AssertUnchangedExcept fails for every leaf path whose value differs between
// before and after (or exists on only one side) and is not listed in changed
// or volatile.
func AssertUnchangedExcept(t *testing.T, before, after map[string]any, changed, volatile []string) {
	t.Helper()
	assertUnchangedExcept(t, before, after, changed, volatile)
}

func assertUnchangedExcept(t testing.TB, before, after map[string]any, changed, volatile []string) {
	t.Helper()
	// An entry exempts a leaf that equals it or lies beneath it (dotted prefix).
	exemptList := append(append([]string{}, changed...), volatile...)
	isExempt := func(leaf string) bool {
		for _, e := range exemptList {
			if leaf == e || strings.HasPrefix(leaf, e+".") {
				return true
			}
		}
		return false
	}
	b, a := flattenLeaves(before), flattenLeaves(after)
	seen := map[string]bool{}
	var paths []string
	for p := range b {
		seen[p] = true
		paths = append(paths, p)
	}
	for p := range a {
		if !seen[p] {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	for _, p := range paths {
		if isExempt(p) {
			continue
		}
		bv, bok := b[p]
		av, aok := a[p]
		if bok != aok || !reflect.DeepEqual(bv, av) {
			t.Errorf("server state %q changed unexpectedly: before=%s after=%s", p, fmtLeaf(bv, bok), fmtLeaf(av, aok))
		}
	}
}

func fmtLeaf(v any, ok bool) string {
	if !ok {
		return "<absent>"
	}
	return fmt.Sprintf("%#v", v)
}

// flattenLeaves maps each non-map leaf to its dotted path. Empty maps are
// kept as leaves so their presence is compared.
func flattenLeaves(m map[string]any) map[string]any {
	out := map[string]any{}
	var walk func(prefix string, v any)
	walk = func(prefix string, v any) {
		if mm, ok := v.(map[string]any); ok && len(mm) > 0 {
			for k, c := range mm {
				p := k
				if prefix != "" {
					p = prefix + "." + k
				}
				walk(p, c)
			}
			return
		}
		out[prefix] = v
	}
	walk("", m)
	delete(out, "")
	return out
}
