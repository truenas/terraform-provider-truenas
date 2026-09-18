// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package rsync_task

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// buildSchedule constructs a known (non-null, non-unknown) schedule object
// value for use in tests.
func buildSchedule(t *testing.T, minute, hour, dom, month, dow string) types.Object {
	t.Helper()
	obj, diags := types.ObjectValueFrom(context.Background(), scheduleAttrTypes, ScheduleModel{
		Minute: types.StringValue(minute),
		Hour:   types.StringValue(hour),
		Dom:    types.StringValue(dom),
		Month:  types.StringValue(month),
		Dow:    types.StringValue(dow),
	})
	if diags.HasError() {
		t.Fatalf("building schedule object: %v", diags)
	}
	return obj
}

// baseUnsetModel returns a model with every field null/unknown except the
// two Required fields, for tests that only care about a subset.
func baseUnsetModel(path, user string) *RsyncTaskModel {
	return &RsyncTaskModel{
		Path:           types.StringValue(path),
		User:           types.StringValue(user),
		Mode:           types.StringNull(),
		RemoteHost:     types.StringNull(),
		RemotePort:     types.Int64Null(),
		RemoteModule:   types.StringNull(),
		SSHCredentials: types.Int64Null(),
		RemotePath:     types.StringNull(),
		Direction:      types.StringNull(),
		Desc:           types.StringNull(),
		Schedule:       types.ObjectNull(scheduleAttrTypes),
		Recursive:      types.BoolNull(),
		Times:          types.BoolNull(),
		Compress:       types.BoolNull(),
		Archive:        types.BoolNull(),
		Delete:         types.BoolNull(),
		Quiet:          types.BoolNull(),
		PreservePerm:   types.BoolNull(),
		PreserveAttr:   types.BoolNull(),
		DelayUpdates:   types.BoolNull(),
		Extra:          types.ListNull(types.StringType),
		Enabled:        types.BoolNull(),
		ValidateRPath:  types.BoolNull(),
		SSHKeyscan:     types.BoolNull(),
	}
}

// TestApiPayload_MODULEMode verifies the payload built for a MODULE-mode
// task: remotehost/remotemodule are included, ssh_credentials/remoteport
// are omitted (both null), matching the probed create shape.
func TestApiPayload_MODULEMode(t *testing.T) {
	ctx := context.Background()
	m := baseUnsetModel("/mnt/tank/tf-acc", "root")
	m.Mode = types.StringValue("MODULE")
	m.RemoteHost = types.StringValue("192.0.2.10")
	m.RemoteModule = types.StringValue("tfacc")
	m.Enabled = types.BoolValue(false)
	m.Desc = types.StringValue("tf-acc rsync module")
	m.Schedule = buildSchedule(t, "00", "*", "*", "*", "*")

	p, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostics errors: %v", diags)
	}

	if p["path"] != "/mnt/tank/tf-acc" {
		t.Errorf("payload[path] = %v, want /mnt/tank/tf-acc", p["path"])
	}
	if p["user"] != "root" {
		t.Errorf("payload[user] = %v, want root", p["user"])
	}
	if p["mode"] != "MODULE" {
		t.Errorf("payload[mode] = %v, want MODULE", p["mode"])
	}
	if p["remotehost"] != "192.0.2.10" {
		t.Errorf("payload[remotehost] = %v, want 192.0.2.10", p["remotehost"])
	}
	if p["remotemodule"] != "tfacc" {
		t.Errorf("payload[remotemodule] = %v, want tfacc", p["remotemodule"])
	}
	if p["enabled"] != false {
		t.Errorf("payload[enabled] = %v, want false", p["enabled"])
	}
	for _, key := range []string{"remoteport", "ssh_credentials"} {
		if _, ok := p[key]; ok {
			t.Errorf("expected %q to be omitted (null), got %v", key, p[key])
		}
	}
}

// TestApiPayload_SSHMode verifies the payload built for an SSH-mode task:
// remotehost/remoteport/ssh_credentials are all included, remotemodule is
// omitted (irrelevant to SSH mode and left null).
func TestApiPayload_SSHMode(t *testing.T) {
	ctx := context.Background()
	m := baseUnsetModel("/mnt/tank/tf-acc", "root")
	m.Mode = types.StringValue("SSH")
	m.RemoteHost = types.StringValue("203.0.113.5")
	m.RemotePort = types.Int64Value(2222)
	m.SSHCredentials = types.Int64Value(7)
	m.RemotePath = types.StringValue("/backup")
	m.Direction = types.StringValue("PUSH")

	p, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostics errors: %v", diags)
	}

	if p["mode"] != "SSH" {
		t.Errorf("payload[mode] = %v, want SSH", p["mode"])
	}
	if p["remotehost"] != "203.0.113.5" {
		t.Errorf("payload[remotehost] = %v, want 203.0.113.5", p["remotehost"])
	}
	if p["remoteport"] != int64(2222) {
		t.Errorf("payload[remoteport] = %v, want 2222", p["remoteport"])
	}
	if p["ssh_credentials"] != int64(7) {
		t.Errorf("payload[ssh_credentials] = %v, want 7", p["ssh_credentials"])
	}
	if p["remotepath"] != "/backup" {
		t.Errorf("payload[remotepath] = %v, want /backup", p["remotepath"])
	}
	if p["direction"] != "PUSH" {
		t.Errorf("payload[direction] = %v, want PUSH", p["direction"])
	}
	if _, ok := p["remotemodule"]; ok {
		t.Errorf("expected remotemodule to be omitted (null), got %v", p["remotemodule"])
	}
}

// TestApiPayload_UnsetOptionalsOmitted verifies that every Optional field is
// omitted from the payload when null/unknown, leaving only the two Required
// fields "path" and "user" — so TrueNAS-side defaults take effect.
func TestApiPayload_UnsetOptionalsOmitted(t *testing.T) {
	ctx := context.Background()
	m := baseUnsetModel("/mnt/tank/tf-acc", "root")

	p, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostics errors: %v", diags)
	}

	if len(p) != 2 {
		t.Fatalf("payload has %d keys (%v), want 2 (path, user)", len(p), p)
	}
	if p["path"] != "/mnt/tank/tf-acc" || p["user"] != "root" {
		t.Errorf("payload = %v, want only path/user set", p)
	}
}

// TestApiPayload_ValidateRPathAndSSHKeyscan verifies both write-only flags
// are included when known, alongside the rest of the payload.
func TestApiPayload_ValidateRPathAndSSHKeyscan(t *testing.T) {
	ctx := context.Background()
	m := baseUnsetModel("/mnt/tank/tf-acc", "root")
	m.ValidateRPath = types.BoolValue(false)
	m.SSHKeyscan = types.BoolValue(true)

	p, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostics errors: %v", diags)
	}
	if p["validate_rpath"] != false {
		t.Errorf("payload[validate_rpath] = %v, want false", p["validate_rpath"])
	}
	if p["ssh_keyscan"] != true {
		t.Errorf("payload[ssh_keyscan] = %v, want true", p["ssh_keyscan"])
	}
}

// TestDecodeSSHCredentialsID_Null verifies a JSON null decodes to (nil, nil).
func TestDecodeSSHCredentialsID_Null(t *testing.T) {
	id, err := decodeSSHCredentialsID([]byte("null"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != nil {
		t.Errorf("id = %v, want nil", *id)
	}
}

// TestDecodeSSHCredentialsID_EmbeddedObject verifies the shape actually
// observed on rsynctask.query/get_instance reads: an embedded
// KeychainCredentialEntry object.
func TestDecodeSSHCredentialsID_EmbeddedObject(t *testing.T) {
	raw := []byte(`{"id": 5, "name": "my-ssh-creds", "type": "SSH_CREDENTIALS", "attributes": {}}`)
	id, err := decodeSSHCredentialsID(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == nil || *id != 5 {
		t.Errorf("id = %v, want 5", id)
	}
}

// TestDecodeSSHCredentialsID_BareInteger verifies defensive support for a
// bare integer, in case a future API version returns one directly.
func TestDecodeSSHCredentialsID_BareInteger(t *testing.T) {
	id, err := decodeSSHCredentialsID([]byte("9"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id == nil || *id != 9 {
		t.Errorf("id = %v, want 9", id)
	}
}

// TestDecodeSSHCredentialsID_Invalid verifies an undecodable shape returns
// an error rather than silently defaulting to nil.
func TestDecodeSSHCredentialsID_Invalid(t *testing.T) {
	_, err := decodeSSHCredentialsID([]byte(`"not an object or integer"`))
	if err == nil {
		t.Fatal("expected an error for an undecodable ssh_credentials shape")
	}
}

// TestResponseToModel_MODULEQueryShape verifies responseToModel against the
// exact shape observed from a live rsynctask.create/query call in MODULE
// mode: remoteport and ssh_credentials both null, remotehost/remotemodule
// plain strings.
func TestResponseToModel_MODULEQueryShape(t *testing.T) {
	ctx := context.Background()
	host := "192.0.2.10"
	module := "tfacc"
	api := &rsyncTaskAPI{
		ID:             1,
		Path:           "/mnt/tank",
		User:           "root",
		Mode:           "MODULE",
		RemoteHost:     &host,
		RemotePort:     nil,
		RemoteModule:   &module,
		SSHCredentials: nil,
		RemotePath:     "",
		Direction:      "PUSH",
		Desc:           "tf-probe rsync module",
		Schedule:       scheduleAPI{Minute: "00", Hour: "*", Dom: "*", Month: "*", Dow: "*"},
		Recursive:      true,
		Times:          true,
		Compress:       true,
		Archive:        false,
		Delete:         false,
		Quiet:          false,
		PreservePerm:   false,
		PreserveAttr:   false,
		DelayUpdates:   true,
		Extra:          nil,
		Enabled:        false,
	}

	m := &RsyncTaskModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueInt64() != 1 {
		t.Errorf("ID = %v, want 1", m.ID)
	}
	if !m.RemotePort.IsNull() {
		t.Errorf("RemotePort = %v, want null", m.RemotePort)
	}
	if !m.SSHCredentials.IsNull() {
		t.Errorf("SSHCredentials = %v, want null", m.SSHCredentials)
	}
	if m.RemoteHost.ValueString() != "192.0.2.10" {
		t.Errorf("RemoteHost = %q, want 192.0.2.10", m.RemoteHost.ValueString())
	}
	if m.RemoteModule.ValueString() != "tfacc" {
		t.Errorf("RemoteModule = %q, want tfacc", m.RemoteModule.ValueString())
	}
	if m.Enabled.ValueBool() {
		t.Error("Enabled = true, want false")
	}
	if m.Extra.IsNull() {
		t.Error("Extra should be an empty list, not null, when API returns nil")
	}
	var extra []string
	diags = m.Extra.ElementsAs(ctx, &extra, false)
	if diags.HasError() {
		t.Fatalf("reading back extra: %v", diags)
	}
	if len(extra) != 0 {
		t.Errorf("Extra = %v, want empty", extra)
	}

	var sched ScheduleModel
	diags = m.Schedule.As(ctx, &sched, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("reading back schedule: %v", diags)
	}
	if sched.Minute.ValueString() != "00" {
		t.Errorf("schedule.minute = %q, want \"00\"", sched.Minute.ValueString())
	}
}

// TestResponseToModel_SSHCredentialsEmbeddedObject verifies responseToModel
// correctly unwraps the embedded KeychainCredentialEntry shape into a plain
// int64 ID.
func TestResponseToModel_SSHCredentialsEmbeddedObject(t *testing.T) {
	ctx := context.Background()
	host := "203.0.113.5"
	api := &rsyncTaskAPI{
		ID:             2,
		Path:           "/mnt/tank",
		User:           "root",
		Mode:           "SSH",
		RemoteHost:     &host,
		SSHCredentials: []byte(`{"id": 5, "name": "creds", "type": "SSH_CREDENTIALS", "attributes": {}}`),
		Direction:      "PUSH",
		Schedule:       scheduleAPI{Minute: "00", Hour: "*", Dom: "*", Month: "*", Dow: "*"},
	}

	m := &RsyncTaskModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.SSHCredentials.ValueInt64() != 5 {
		t.Errorf("SSHCredentials = %v, want 5", m.SSHCredentials)
	}
}

// TestResponseToDataSourceModel_MODULEQueryShape mirrors
// TestResponseToModel_MODULEQueryShape for the datasource model.
func TestResponseToDataSourceModel_MODULEQueryShape(t *testing.T) {
	ctx := context.Background()
	host := "192.0.2.10"
	module := "tfacc"
	api := &rsyncTaskAPI{
		ID:             1,
		Path:           "/mnt/tank",
		User:           "root",
		Mode:           "MODULE",
		RemoteHost:     &host,
		RemoteModule:   &module,
		SSHCredentials: nil,
		Direction:      "PUSH",
		Desc:           "tf-probe rsync module",
		Schedule:       scheduleAPI{Minute: "00", Hour: "*", Dom: "*", Month: "*", Dow: "*"},
		Enabled:        false,
	}

	m := &RsyncTaskDataSourceModel{}
	diags := responseToDataSourceModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.Desc.ValueString() != "tf-probe rsync module" {
		t.Errorf("Desc = %q, want \"tf-probe rsync module\"", m.Desc.ValueString())
	}
	if !m.RemotePort.IsNull() {
		t.Errorf("RemotePort = %v, want null", m.RemotePort)
	}
}
