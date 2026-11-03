// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package pool

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// fakeCaller answers the three queries poolContents makes.
type fakeCaller struct {
	datasets string // pool.dataset.query result
	snaps    string // pool.snapshot.query result ("" = method error)
	listdir  string // filesystem.listdir result
	calls    []string
}

func (f *fakeCaller) Call(_ context.Context, method string, _ ...any) (json.RawMessage, error) {
	f.calls = append(f.calls, method)
	switch method {
	case "pool.dataset.query":
		return json.RawMessage(f.datasets), nil
	case "pool.snapshot.query":
		if f.snaps == "" {
			return nil, errMethod
		}
		return json.RawMessage(f.snaps), nil
	case "filesystem.listdir":
		return json.RawMessage(f.listdir), nil
	}
	return nil, errMethod
}

func (f *fakeCaller) CallJob(ctx context.Context, method string, params ...any) (json.RawMessage, error) {
	return f.Call(ctx, method, params...)
}

type methodErr struct{}

func (methodErr) Error() string { return "Method does not exist" }

var errMethod = methodErr{}

func TestPoolContents_EmptyPoolIsEmpty(t *testing.T) {
	c := &fakeCaller{
		datasets: `[{"name":"tank","type":"FILESYSTEM"}]`,
		snaps:    `[]`,
		listdir:  `[]`,
	}
	got, err := poolContents(context.Background(), c, "tank")
	if err != nil || len(got) != 0 {
		t.Fatalf("empty pool: got %v, %v", got, err)
	}
}

func TestPoolContents_ListsEverythingADestroyWouldTake(t *testing.T) {
	c := &fakeCaller{
		datasets: `[{"name":"tank","type":"FILESYSTEM"},{"name":"tank/data","type":"FILESYSTEM"},{"name":"tank/vm0","type":"VOLUME"}]`,
		snaps:    `[{"name":"tank@daily"}]`,
		listdir:  `[{"name":"notes.txt","type":"FILE"}]`,
	}
	got, err := poolContents(context.Background(), c, "tank")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(got, "|")
	for _, want := range []string{"tank/data (FILESYSTEM)", "tank/vm0 (VOLUME)", "tank@daily (snapshot)", "notes.txt (file)"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in %v", want, got)
		}
	}
	if strings.Contains(joined, "tank (FILESYSTEM)") {
		t.Errorf("the root dataset is the pool itself and must not be listed: %v", got)
	}
}

func TestPoolContents_SnapshotNamespaceMissingIsNotABlocker(t *testing.T) {
	c := &fakeCaller{
		datasets: `[{"name":"tank","type":"FILESYSTEM"}]`,
		snaps:    "", // 28.0 moved pool.snapshot.*; the guard must not fail closed on that alone
		listdir:  `[]`,
	}
	got, err := poolContents(context.Background(), c, "tank")
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

func TestRefusalMessage_CapsTheList(t *testing.T) {
	var many []string
	for i := 0; i < maxListedPoolContents+5; i++ {
		many = append(many, "tank/d"+string(rune('a'+i))+" (FILESYSTEM)")
	}
	msg := refusalMessage("tank", many)
	if !strings.Contains(msg, "and 5 more") || !strings.Contains(msg, "force_destroy = true") {
		t.Errorf("unexpected message: %s", msg)
	}
}
