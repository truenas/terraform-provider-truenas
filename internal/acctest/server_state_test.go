// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest

import (
	"strings"
	"testing"
)

func sampleInstance() map[string]any {
	return map[string]any{
		"name": "tank/ds",
		"quota": map[string]any{
			"source": "LOCAL",
			"parsed": float64(1024),
		},
		"updated": "t1",
	}
}

func TestServerPath(t *testing.T) {
	inst := sampleInstance()
	v, ok := ServerPath(inst, "quota.source")
	if !ok || v != "LOCAL" {
		t.Fatalf("quota.source = %v, %v; want LOCAL, true", v, ok)
	}
	if _, ok := ServerPath(inst, "quota.nope"); ok {
		t.Fatal("quota.nope reported found")
	}
	if _, ok := ServerPath(inst, "name.deeper"); ok {
		t.Fatal("name.deeper reported found through a non-map")
	}
}

// failRecorder runs fn against a recording TB and reports whether it failed.
func failed(fn func(tb testing.TB)) bool {
	r := &recorder{}
	func() {
		defer func() { _ = recover() }()
		fn(r)
	}()
	return r.failed
}

func TestServerStateAssertUnchangedExcept(t *testing.T) {
	before := sampleInstance()

	after := sampleInstance()
	after["quota"].(map[string]any)["parsed"] = float64(2048)
	AssertUnchangedExcept(t, before, after, []string{"quota.parsed"}, nil)

	after = sampleInstance()
	after["name"] = "tank/other"
	if !failed(func(tb testing.TB) { assertUnchangedExcept(tb, before, after, []string{"quota.parsed"}, nil) }) {
		t.Fatal("unlisted difference did not fail")
	}

	after = sampleInstance()
	after["updated"] = "t2"
	AssertUnchangedExcept(t, before, after, nil, []string{"updated"})

	// A prefix entry exempts leaves beneath it, but not sibling names that
	// merely share the string prefix.
	after = sampleInstance()
	after["quota"].(map[string]any)["parsed"] = float64(2048)
	AssertUnchangedExcept(t, before, after, []string{"quota"}, nil)
	AssertUnchangedExcept(t, before, after, nil, []string{"quota"})
	after = sampleInstance()
	after["quota"].(map[string]any)["source"] = "INHERITED"
	if !failed(func(tb testing.TB) { assertUnchangedExcept(tb, before, after, []string{"quota.parsed"}, nil) }) {
		t.Fatal("sibling leaf under a different entry did not fail")
	}
	after = sampleInstance()
	after["name"] = "tank/other"
	if !failed(func(tb testing.TB) { assertUnchangedExcept(tb, before, after, []string{"nam"}, nil) }) {
		t.Fatal("non-dotted string prefix wrongly exempted")
	}

	// A path present on only one side is a difference.
	after = sampleInstance()
	delete(after, "updated")
	if !failed(func(tb testing.TB) { assertUnchangedExcept(tb, before, after, nil, nil) }) {
		t.Fatal("removed path did not fail")
	}
}

func TestServerStateAssertServerSource(t *testing.T) {
	inst := sampleInstance()
	AssertServerSource(t, inst, "quota.source", "LOCAL")
	AssertServerValue(t, inst, "quota.parsed", float64(1024))
	if !failed(func(tb testing.TB) { assertServerSource(tb, inst, "quota.source", "INHERITED") }) {
		t.Fatal("source mismatch did not fail")
	}
	if !failed(func(tb testing.TB) { assertServerValue(tb, inst, "quota.missing", "x") }) {
		t.Fatal("missing path did not fail")
	}
}

func TestServerStateFlatten(t *testing.T) {
	got := flattenLeaves(sampleInstance())
	if _, ok := got["quota.source"]; !ok {
		t.Fatalf("flatten missing quota.source: %v", got)
	}
	if strings.Contains(strings.Join(keys(got), ","), "quota,") {
		t.Fatalf("flatten kept a non-leaf: %v", got)
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// recorder captures failures instead of failing the real test.
type recorder struct {
	testing.TB
	failed bool
}

func (r *recorder) Helper()               {}
func (r *recorder) Errorf(string, ...any) { r.failed = true }
func (r *recorder) Fatalf(string, ...any) { r.failed = true; panic("fatal") }
