// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package planmod

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// fakePrivate is an in-memory stand-in for the framework's private state.
type fakePrivate map[string][]byte

func (f fakePrivate) GetKey(_ context.Context, k string) ([]byte, diag.Diagnostics) { return f[k], nil }
func (f fakePrivate) SetKey(_ context.Context, k string, v []byte) diag.Diagnostics {
	if v == nil {
		delete(f, k)
	} else {
		f[k] = v
	}
	return nil
}

func TestResetWhenUnset_decide(t *testing.T) {
	ctx := context.Background()
	m := resetWhenUnset{types.BoolValue(false)}
	p := path.Root("ro")
	key := "planmod.managed.ro"
	pd := fakePrivate{}

	// 1. configured: flag recorded, plan left to the framework
	v, d := m.decide(ctx, p, pd, pd, types.BoolValue(true), types.BoolValue(false))
	if d.HasError() || v != nil || len(pd[key]) == 0 {
		t.Fatalf("configured: v=%v flag=%q diags=%v", v, pd[key], d)
	}
	// 2. removed after being configured: reset to default once, flag cleared
	v, _ = m.decide(ctx, p, pd, pd, types.BoolNull(), types.BoolValue(true))
	if v == nil || v.(types.Bool).ValueBool() != false || len(pd[key]) != 0 {
		t.Fatalf("removed: v=%v flag=%q", v, pd[key])
	}
	// 3. never configured (import / upgrade), state known: keep state
	v, _ = m.decide(ctx, p, fakePrivate{}, fakePrivate{}, types.BoolNull(), types.BoolValue(true))
	if v == nil || !v.(types.Bool).ValueBool() {
		t.Fatalf("never configured: want state kept, got %v", v)
	}
	// 4. create with it unset: leave unknown so the server default applies
	v, _ = m.decide(ctx, p, fakePrivate{}, fakePrivate{}, types.BoolNull(), types.BoolNull())
	if v != nil {
		t.Fatalf("create unset: want nil (unknown), got %v", v)
	}
	// 5. nil private state (typed nil pointer or untyped) must not panic
	var nilPD fakePrivate
	v, _ = m.decide(ctx, p, nilPD, nilPD, types.BoolNull(), types.BoolValue(true))
	if v == nil || !v.(types.Bool).ValueBool() {
		t.Fatalf("nil private: want state kept, got %v", v)
	}
}

func TestResetWhenUnset_listAndString(t *testing.T) {
	ctx := context.Background()
	pd := fakePrivate{}
	nets := types.ListValueMust(types.StringType, []attr.Value{types.StringValue("192.0.2.0/24")})
	empty := types.ListValueMust(types.StringType, nil)
	l := resetWhenUnset{empty}
	// imported share: config silent, keep the server's networks
	v, _ := l.decide(ctx, path.Root("networks"), pd, pd, types.ListNull(types.StringType), nets)
	if v == nil || !v.(types.List).Equal(nets) {
		t.Fatalf("list keep: %v", v)
	}
	// configured, then removed: empty list
	l.decide(ctx, path.Root("networks"), pd, pd, nets, nets)
	v, _ = l.decide(ctx, path.Root("networks"), pd, pd, types.ListNull(types.StringType), nets)
	if v == nil || !v.(types.List).Equal(empty) {
		t.Fatalf("list reset: %v", v)
	}
	s := resetWhenUnset{types.StringValue("")}
	v, _ = s.decide(ctx, path.Root("comment"), fakePrivate{}, fakePrivate{}, types.StringNull(), types.StringValue("set in UI"))
	if v == nil || v.(types.String).ValueString() != "set in UI" {
		t.Fatalf("string keep: %v", v)
	}
}
