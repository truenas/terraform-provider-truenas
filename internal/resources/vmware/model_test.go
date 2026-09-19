// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vmware

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// TestApiPayload_AllFieldsAlwaysIncluded verifies apiPayload includes all
// five keys every time: every field is Required in the schema (no
// Optional/Computed fields at all, unlike cloud_backup/app_registry), so
// there is no "omit unless set" branch to test.
func TestApiPayload_AllFieldsAlwaysIncluded(t *testing.T) {
	m := &VMwareModel{
		Datastore:  types.StringValue("ds1"),
		Filesystem: types.StringValue("tank/vmware"),
		Hostname:   types.StringValue("vcenter.example.com"),
		Username:   types.StringValue("tfuser"),
	}
	p := m.apiPayload(types.StringValue("s3cr3t"))

	wantKeys := []string{"datastore", "filesystem", "hostname", "username", "password"}
	if len(p) != len(wantKeys) {
		t.Fatalf("apiPayload() = %v, want exactly %d keys (%v)", p, len(wantKeys), wantKeys)
	}
	for _, k := range wantKeys {
		if _, ok := p[k]; !ok {
			t.Errorf("payload missing key %q", k)
		}
	}
	if p["datastore"] != "ds1" {
		t.Errorf("payload[datastore] = %v, want ds1", p["datastore"])
	}
	if p["filesystem"] != "tank/vmware" {
		t.Errorf("payload[filesystem] = %v, want tank/vmware", p["filesystem"])
	}
	if p["hostname"] != "vcenter.example.com" {
		t.Errorf("payload[hostname] = %v, want vcenter.example.com", p["hostname"])
	}
	if p["username"] != "tfuser" {
		t.Errorf("payload[username] = %v, want tfuser", p["username"])
	}
	if p["password"] != "s3cr3t" {
		t.Errorf("payload[password] = %v, want s3cr3t", p["password"])
	}
}

// TestApiPayload_UsesConfigPasswordNotModelPassword verifies apiPayload
// always uses the explicitly-passed cfgPassword argument, not m.Password —
// the write-only safety contract (m.Password may be null/unknown from a
// plan where the framework has nulled the WriteOnly attribute).
func TestApiPayload_UsesConfigPasswordNotModelPassword(t *testing.T) {
	m := &VMwareModel{
		Datastore:  types.StringValue("ds1"),
		Filesystem: types.StringValue("tank"),
		Hostname:   types.StringValue("host"),
		Username:   types.StringValue("user"),
		Password:   types.StringNull(), // as the framework leaves it in req.Plan
	}
	p := m.apiPayload(types.StringValue("real-config-password"))
	if p["password"] != "real-config-password" {
		t.Errorf("payload[password] = %v, want real-config-password (from cfgPassword arg, not m.Password)", p["password"])
	}
}

// TestResponseToModel_FullShape verifies responseToModel against the field
// shape probed live via core.get_methods (identical on both TrueNAS 25.10.4
// HA and 26.0) and does NOT touch Password.
func TestResponseToModel_FullShape(t *testing.T) {
	ctx := context.Background()
	errText := "connection refused"
	datetime := "2026-07-23T12:00:00+00:00"
	api := &vmwareAPI{
		ID:         5,
		Datastore:  "ds1",
		Filesystem: "tank/vmware",
		Hostname:   "vcenter.example.com",
		Username:   "tfuser",
		Password:   "should-never-be-read",
		State: stateAPI{
			State:    "ERROR",
			Error:    &errText,
			Datetime: &datetime,
		},
	}

	m := &VMwareModel{Password: types.StringValue("preserve-me")}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueInt64() != 5 {
		t.Errorf("ID = %v, want 5", m.ID)
	}
	if m.Datastore.ValueString() != "ds1" {
		t.Errorf("Datastore = %q, want ds1", m.Datastore.ValueString())
	}
	if m.Hostname.ValueString() != "vcenter.example.com" {
		t.Errorf("Hostname = %q, want vcenter.example.com", m.Hostname.ValueString())
	}
	if m.Password.ValueString() != "preserve-me" {
		t.Errorf("Password = %q, want it left untouched (write-only, never read back)", m.Password.ValueString())
	}

	var state StateModel
	diags = m.State.As(ctx, &state, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("unexpected error reading state object: %v", diags)
	}
	if state.State.ValueString() != "ERROR" {
		t.Errorf("state.state = %q, want ERROR", state.State.ValueString())
	}
	if state.Error.ValueString() != errText {
		t.Errorf("state.error = %q, want %q", state.Error.ValueString(), errText)
	}
	if state.Datetime.ValueString() != datetime {
		t.Errorf("state.datetime = %q, want %q", state.Datetime.ValueString(), datetime)
	}
}

// TestResponseToModel_StateNullableFieldsNull verifies state.error/
// state.datetime decode to true Terraform null (not empty string) when the
// API omits them, matching the probed schema's individually-optional
// sub-fields.
func TestResponseToModel_StateNullableFieldsNull(t *testing.T) {
	ctx := context.Background()
	api := &vmwareAPI{
		ID:         1,
		Datastore:  "ds1",
		Filesystem: "tank",
		Hostname:   "host",
		Username:   "user",
		State: stateAPI{
			State:    "PENDING",
			Error:    nil,
			Datetime: nil,
		},
	}

	m := &VMwareModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	var state StateModel
	diags = m.State.As(ctx, &state, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("unexpected error reading state object: %v", diags)
	}
	if !state.Error.IsNull() {
		t.Errorf("state.error = %v, want null", state.Error)
	}
	if !state.Datetime.IsNull() {
		t.Errorf("state.datetime = %v, want null", state.Datetime)
	}
}

// TestResponseToDataSourceModel_FullShape mirrors
// TestResponseToModel_FullShape for the datasource model.
func TestResponseToDataSourceModel_FullShape(t *testing.T) {
	ctx := context.Background()
	api := &vmwareAPI{
		ID:         7,
		Datastore:  "ds2",
		Filesystem: "tank/vmware2",
		Hostname:   "esxi.example.com",
		Username:   "tfuser2",
		Password:   "should-never-appear",
		State:      stateAPI{State: "SUCCESS"},
	}

	m := &VMwareDataSourceModel{}
	diags := responseToDataSourceModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.ID.ValueInt64() != 7 {
		t.Errorf("ID = %v, want 7", m.ID)
	}
	if m.Hostname.ValueString() != "esxi.example.com" {
		t.Errorf("Hostname = %q, want esxi.example.com", m.Hostname.ValueString())
	}
}
