// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

// Prototype of the "class 1" fixed-point consistency test.
//
// The invariant every resource promises Terraform: the value the provider
// planned == what it returns after apply == what it reads on the next refresh.
// This test exercises that invariant at the API level, using the resource's OWN
// mapping functions (responseToModel + updateAPIPayload) against a live box,
// with NO field change:
//
//	S0      = get_instance(id)
//	model   = responseToModel(S0)
//	payload = model.updateAPIPayload(model)   // the no-op update the provider would send
//	update(id, payload)
//	S1      = get_instance(id)
//	assert S0 == S1 (modulo volatile usage stats)
//
// If applying the provider's own no-op round-trip changes anything on the
// server, that is the class-1 bug: #61 (quota source flips to LOCAL), #62
// (comments written as LOCAL ""), the normalization class (#47/#52), etc.
//
// The LOGIC here is identical for every resource — only the three names
// (create/get_instance/update methods) and the two map functions differ, so
// this generalizes behind a small per-resource adapter. This file proves it on
// truenas_dataset, where #61/#62 lived.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// datasetUsageVolatile are the ZFS usage/accounting keys that legitimately move
// between two reads (pool activity); they are excluded from the fixed-point
// comparison. Everything else must be stable across a no-op provider update.
var datasetUsageVolatile = map[string]bool{
	"available": true, "used": true, "referenced": true, "written": true,
	"usedbychildren": true, "usedbydataset": true, "usedbysnapshots": true,
	"usedbyrefreservation": true, "logicalused": true, "logicalreferenced": true,
}

func fpClient(t *testing.T) *client.Client {
	t.Helper()
	ep := os.Getenv("TRUENAS_ENDPOINT")
	if os.Getenv("TF_ACC") == "" || ep == "" {
		t.Skip("set TF_ACC=1 and TRUENAS_ENDPOINT/TRUENAS_API_KEY to run the fixed-point consistency test")
	}
	tlsCfg, _ := client.BuildTLSConfig(true, "")
	c := client.New(ep, tlsCfg)
	apiKey := os.Getenv("TRUENAS_API_KEY")
	username := os.Getenv("TRUENAS_USERNAME")
	auth := func(ctx context.Context) error {
		return client.WithRetry(ctx, func(ctx context.Context) error { return client.AuthAPIKeyAuto(ctx, c, username, apiKey) })
	}
	if err := c.Connect(context.Background(), auth); err != nil {
		t.Fatalf("connect: %v", err)
	}
	return c
}

// TestDatasetFixedPoint_noopUpdateIsStable is the class-1 prototype.
func TestDatasetFixedPoint_noopUpdateIsStable(t *testing.T) {
	c := fpClient(t)
	ctx := context.Background()
	pool := os.Getenv("TRUENAS_TEST_POOL")
	if pool == "" {
		pool = "tank"
	}
	name := fmt.Sprintf("%s/tf-fixedpoint-%d", pool, os.Getpid())

	// Create a plain dataset — comments and quota UNSET, which is exactly the
	// #61/#62 trigger (absent values read back as known sentinels).
	if _, err := c.Call(ctx, "pool.dataset.create", map[string]any{"name": name}); err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() {
		_, _ = c.CallJob(ctx, "pool.dataset.delete", name, map[string]any{"recursive": true})
	})

	readInstance := func(stage string) (apiResponse, map[string]any) {
		raw, err := c.CallRead(ctx, "pool.dataset.get_instance", name)
		if err != nil {
			t.Fatalf("%s get_instance: %v", stage, err)
		}
		var api apiResponse
		if err := json.Unmarshal(raw, &api); err != nil {
			t.Fatalf("%s unmarshal apiResponse: %v", stage, err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("%s unmarshal map: %v", stage, err)
		}
		return api, m
	}

	// S0 + the model the provider would hold for it.
	api0, s0 := readInstance("before")
	r := &DatasetResource{client: c}
	var model DatasetModel
	if diags := r.responseToModel(&api0, &model); diags.HasError() {
		t.Fatalf("responseToModel: %v", diags)
	}

	// The provider's own NO-OP update payload (prior == current model).
	payload := model.updateAPIPayload(&model)
	t.Logf("no-op update payload: %v", payload)

	if _, err := c.Call(ctx, "pool.dataset.update", name, payload); err != nil {
		t.Fatalf("no-op update: %v", err)
	}

	// S1 — must equal S0 everywhere but the volatile usage stats.
	_, s1 := readInstance("after")

	if diffs := fixedPointDiff(s0, s1); len(diffs) > 0 {
		sort.Strings(diffs)
		t.Errorf("FIXED-POINT VIOLATION: the provider's no-op update changed %d server field(s):\n  %v\n"+
			"This is the class-1 bug family (#61 quota source flip, #62 comments write, #47/#52 normalization).",
			len(diffs), diffs)
	}
}

// fixedPointDiff flattens both instances to leaf dotted paths and returns every
// leaf that differs, excluding the top-level volatile usage keys.
func fixedPointDiff(a, b map[string]any) []string {
	fa, fb := map[string]any{}, map[string]any{}
	flatten("", a, fa)
	flatten("", b, fb)
	var diffs []string
	seen := map[string]bool{}
	for k := range fa {
		seen[k] = true
	}
	for k := range fb {
		seen[k] = true
	}
	for k := range seen {
		top := k
		if i := indexDot(k); i >= 0 {
			top = k[:i]
		}
		if datasetUsageVolatile[top] {
			continue
		}
		if fmt.Sprintf("%v", fa[k]) != fmt.Sprintf("%v", fb[k]) {
			diffs = append(diffs, fmt.Sprintf("%s: %v -> %v", k, fa[k], fb[k]))
		}
	}
	return diffs
}

func indexDot(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			return i
		}
	}
	return -1
}

func flatten(prefix string, v any, out map[string]any) {
	switch t := v.(type) {
	case map[string]any:
		for k, vv := range t {
			p := k
			if prefix != "" {
				p = prefix + "." + k
			}
			flatten(p, vv, out)
		}
	default:
		out[prefix] = v
	}
}
