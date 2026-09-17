// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ntp_server

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestNTPServerSchema verifies that the resource schema has the expected
// attributes with correct types.
func TestNTPServerSchema(t *testing.T) {
	s := resourceSchema()

	// id must be Int64Attribute, Computed-only, with UseStateForUnknown.
	idAttr, ok := s.Attributes["id"]
	if !ok {
		t.Fatal("schema missing 'id' attribute")
	}
	idInt64, ok := idAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'id' attribute is %T, want schema.Int64Attribute", idAttr)
	}
	if !idInt64.IsComputed() {
		t.Error("'id' should be Computed")
	}
	if idInt64.IsRequired() || idInt64.IsOptional() {
		t.Error("'id' should be Computed-only (not Required/Optional)")
	}
	if len(idInt64.PlanModifiers) == 0 {
		t.Error("'id' should have a plan modifier (UseStateForUnknown)")
	}

	// address must be StringAttribute, Required-only.
	addrAttr, ok := s.Attributes["address"]
	if !ok {
		t.Fatal("schema missing 'address' attribute")
	}
	addrStr, ok := addrAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'address' attribute is %T, want schema.StringAttribute", addrAttr)
	}
	if !addrStr.IsRequired() {
		t.Error("'address' should be Required")
	}
	if addrStr.IsOptional() || addrStr.IsComputed() {
		t.Error("'address' should be Required-only")
	}

	// burst, iburst, prefer must be BoolAttribute, Optional+Computed, with plan modifiers.
	for _, name := range []string{"burst", "iburst", "prefer"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		boolAttr, ok := attr.(schema.BoolAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.BoolAttribute", name, attr)
		}
		if !boolAttr.IsOptional() {
			t.Errorf("%q should be Optional", name)
		}
		if !boolAttr.IsComputed() {
			t.Errorf("%q should be Computed", name)
		}
		if len(boolAttr.PlanModifiers) == 0 {
			t.Errorf("%q should have a plan modifier (UseStateForUnknown)", name)
		}
	}

	// minpoll, maxpoll must be Int64Attribute, Optional+Computed, with plan modifiers.
	for _, name := range []string{"minpoll", "maxpoll"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		intAttr, ok := attr.(schema.Int64Attribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.Int64Attribute", name, attr)
		}
		if !intAttr.IsOptional() {
			t.Errorf("%q should be Optional", name)
		}
		if !intAttr.IsComputed() {
			t.Errorf("%q should be Computed", name)
		}
		if len(intAttr.PlanModifiers) == 0 {
			t.Errorf("%q should have a plan modifier (UseStateForUnknown)", name)
		}
	}

	// force must be BoolAttribute, Optional-only (NOT Computed, write-only bypass).
	forceAttr, ok := s.Attributes["force"]
	if !ok {
		t.Fatal("schema missing 'force' attribute")
	}
	forceBool, ok := forceAttr.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'force' attribute is %T, want schema.BoolAttribute", forceAttr)
	}
	if !forceBool.IsOptional() {
		t.Error("'force' should be Optional")
	}
	if forceBool.IsComputed() {
		t.Error("'force' should NOT be Computed (write-only validation bypass)")
	}
	if forceBool.IsRequired() {
		t.Error("'force' should not be Required")
	}
}

// TestNTPServerApiPayload_AllSet verifies that apiPayload includes all
// optional fields, including force, when they are set.
func TestNTPServerApiPayload_AllSet(t *testing.T) {
	ctx := context.Background()

	m := NTPServerModel{
		Address: types.StringValue("pool.ntp.org"),
		Burst:   types.BoolValue(true),
		IBurst:  types.BoolValue(false),
		Prefer:  types.BoolValue(true),
		MinPoll: types.Int64Value(4),
		MaxPoll: types.Int64Value(10),
		Force:   types.BoolValue(true),
	}

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	expectedKeys := []string{"address", "burst", "iburst", "prefer", "minpoll", "maxpoll", "force"}
	if len(payload) != len(expectedKeys) {
		t.Errorf("payload has %d keys, want %d: %v", len(payload), len(expectedKeys), payload)
	}
	for _, k := range expectedKeys {
		if _, ok := payload[k]; !ok {
			t.Errorf("payload missing key %q", k)
		}
	}

	if payload["address"] != "pool.ntp.org" {
		t.Errorf("payload[address] = %v, want 'pool.ntp.org'", payload["address"])
	}
	if payload["burst"] != true {
		t.Errorf("payload[burst] = %v, want true", payload["burst"])
	}
	if payload["iburst"] != false {
		t.Errorf("payload[iburst] = %v, want false", payload["iburst"])
	}
	if payload["prefer"] != true {
		t.Errorf("payload[prefer] = %v, want true", payload["prefer"])
	}
	if payload["minpoll"] != int64(4) {
		t.Errorf("payload[minpoll] = %v, want 4", payload["minpoll"])
	}
	if payload["maxpoll"] != int64(10) {
		t.Errorf("payload[maxpoll] = %v, want 10", payload["maxpoll"])
	}
	if payload["force"] != true {
		t.Errorf("payload[force] = %v, want true", payload["force"])
	}
}

// TestNTPServerApiPayload_OnlyAddress verifies that apiPayload contains only
// "address" when all optional fields (including force) are unset (null).
func TestNTPServerApiPayload_OnlyAddress(t *testing.T) {
	ctx := context.Background()

	m := NTPServerModel{
		Address: types.StringValue("pool.ntp.org"),
		Burst:   types.BoolNull(),
		IBurst:  types.BoolNull(),
		Prefer:  types.BoolNull(),
		MinPoll: types.Int64Null(),
		MaxPoll: types.Int64Null(),
		Force:   types.BoolNull(),
	}

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}

	if len(payload) != 1 {
		t.Fatalf("payload has %d keys, want 1 (address only): %v", len(payload), payload)
	}
	if payload["address"] != "pool.ntp.org" {
		t.Errorf("payload[address] = %v, want 'pool.ntp.org'", payload["address"])
	}

	// id must NOT be in the payload.
	if _, ok := payload["id"]; ok {
		t.Error("payload should not contain key 'id'")
	}
	if _, ok := payload["force"]; ok {
		t.Error("payload should not contain key 'force' when unset")
	}
}

// TestNTPServerApiPayload_ForceOmittedWhenUnset explicitly verifies force is
// omitted from the payload when unset, and included only when explicitly set.
func TestNTPServerApiPayload_ForceOmittedWhenUnset(t *testing.T) {
	ctx := context.Background()

	unset := NTPServerModel{
		Address: types.StringValue("pool.ntp.org"),
		Force:   types.BoolNull(),
	}
	payload, diags := unset.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}
	if _, ok := payload["force"]; ok {
		t.Error("force should be omitted from payload when unset")
	}

	set := NTPServerModel{
		Address: types.StringValue("pool.ntp.org"),
		Force:   types.BoolValue(false),
	}
	payload, diags = set.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostic errors: %v", diags)
	}
	if v, ok := payload["force"]; !ok {
		t.Error("force should be included in payload when explicitly set to false")
	} else if v != false {
		t.Errorf("payload[force] = %v, want false", v)
	}
}

// TestNTPServerResponseToModel verifies that responseToModel populates all
// fields from the ntpServerAPI struct correctly, and never sets Force (which
// has no corresponding field in the API response).
func TestNTPServerResponseToModel(t *testing.T) {
	ctx := context.Background()

	api := &ntpServerAPI{
		ID:      3,
		Address: "pool.ntp.org",
		Burst:   true,
		IBurst:  false,
		Prefer:  true,
		MinPoll: 4,
		MaxPoll: 10,
	}

	// Seed Force with a non-null value to prove responseToModel leaves it untouched.
	m := NTPServerModel{Force: types.BoolValue(true)}
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned diagnostic errors: %v", diags)
	}

	if m.ID.ValueInt64() != 3 {
		t.Errorf("ID = %v, want 3", m.ID.ValueInt64())
	}
	if m.Address.ValueString() != "pool.ntp.org" {
		t.Errorf("Address = %v, want 'pool.ntp.org'", m.Address.ValueString())
	}
	if !m.Burst.ValueBool() {
		t.Error("Burst = false, want true")
	}
	if m.IBurst.ValueBool() {
		t.Error("IBurst = true, want false")
	}
	if !m.Prefer.ValueBool() {
		t.Error("Prefer = false, want true")
	}
	if m.MinPoll.ValueInt64() != 4 {
		t.Errorf("MinPoll = %v, want 4", m.MinPoll.ValueInt64())
	}
	if m.MaxPoll.ValueInt64() != 10 {
		t.Errorf("MaxPoll = %v, want 10", m.MaxPoll.ValueInt64())
	}

	// Force must be untouched by responseToModel (still the seeded value).
	if !m.Force.Equal(types.BoolValue(true)) {
		t.Errorf("Force = %v, responseToModel must never modify Force", m.Force)
	}
}
