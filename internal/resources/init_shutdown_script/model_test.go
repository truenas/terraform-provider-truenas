// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package init_shutdown_script

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// baseUnsetModel returns a model with every optional field null/unknown
// except the two Required fields, for tests that only care about a subset.
func baseUnsetModel(typ, when string) *InitShutdownScriptModel {
	return &InitShutdownScriptModel{
		Type:    types.StringValue(typ),
		When:    types.StringValue(when),
		Command: types.StringNull(),
		Script:  types.StringNull(),
		Enabled: types.BoolNull(),
		Timeout: types.Int64Null(),
		Comment: types.StringNull(),
	}
}

// TestApiPayload_COMMANDType verifies the payload built for a COMMAND-type
// task: "command" is included, matching the probed create shape.
func TestApiPayload_COMMANDType(t *testing.T) {
	m := baseUnsetModel("COMMAND", "POSTINIT")
	m.Command = types.StringValue("/usr/bin/true")
	m.Enabled = types.BoolValue(false)

	p, diags := m.apiPayload()
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostics errors: %v", diags)
	}

	if p["type"] != "COMMAND" {
		t.Errorf("payload[type] = %v, want COMMAND", p["type"])
	}
	if p["when"] != "POSTINIT" {
		t.Errorf("payload[when] = %v, want POSTINIT", p["when"])
	}
	if p["command"] != "/usr/bin/true" {
		t.Errorf("payload[command] = %v, want /usr/bin/true", p["command"])
	}
	if p["enabled"] != false {
		t.Errorf("payload[enabled] = %v, want false", p["enabled"])
	}
	if _, ok := p["script"]; ok {
		t.Errorf("expected script to be omitted (null), got %v", p["script"])
	}
}

// TestApiPayload_SCRIPTType verifies the payload built for a SCRIPT-type
// task: "script" is included, "command" omitted.
func TestApiPayload_SCRIPTType(t *testing.T) {
	m := baseUnsetModel("SCRIPT", "SHUTDOWN")
	m.Script = types.StringValue("/usr/bin/true")
	m.Timeout = types.Int64Value(20)

	p, diags := m.apiPayload()
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostics errors: %v", diags)
	}

	if p["script"] != "/usr/bin/true" {
		t.Errorf("payload[script] = %v, want /usr/bin/true", p["script"])
	}
	if p["timeout"] != int64(20) {
		t.Errorf("payload[timeout] = %v, want 20", p["timeout"])
	}
	if _, ok := p["command"]; ok {
		t.Errorf("expected command to be omitted (null), got %v", p["command"])
	}
}

// TestApiPayload_UnsetOptionalsOmitted verifies that every Optional field is
// omitted from the payload when null/unknown, leaving only "type" and
// "when" — so TrueNAS-side defaults take effect. type=COMMAND without a
// command would normally fail validateTypeFields, so this test uses SCRIPT
// with script explicitly set to isolate "every OTHER optional is omitted".
func TestApiPayload_UnsetOptionalsOmitted(t *testing.T) {
	m := baseUnsetModel("SCRIPT", "PREINIT")
	m.Script = types.StringValue("/usr/bin/true")

	p, diags := m.apiPayload()
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostics errors: %v", diags)
	}

	if len(p) != 3 {
		t.Fatalf("payload has %d keys (%v), want 3 (type, when, script)", len(p), p)
	}
}

// TestApiPayload_COMMANDMissingCommand verifies the conditional-required
// preflight rejects type=COMMAND without a non-empty "command", matching
// the server-side EINVAL probed live ("init_shutdown_script_create.command:
// This field is required").
func TestApiPayload_COMMANDMissingCommand(t *testing.T) {
	m := baseUnsetModel("COMMAND", "PREINIT")

	_, diags := m.apiPayload()
	if !diags.HasError() {
		t.Fatal("expected an error for type=COMMAND with no command set")
	}
}

// TestApiPayload_COMMANDEmptyCommand verifies an explicitly empty-string
// command is treated the same as unset (rejected), since an empty command
// is indistinguishable from "not configured" and the server itself would
// reject it.
func TestApiPayload_COMMANDEmptyCommand(t *testing.T) {
	m := baseUnsetModel("COMMAND", "PREINIT")
	m.Command = types.StringValue("")

	_, diags := m.apiPayload()
	if !diags.HasError() {
		t.Fatal("expected an error for type=COMMAND with an empty-string command")
	}
}

// TestApiPayload_SCRIPTMissingScript verifies the symmetric
// conditional-required preflight for type=SCRIPT without a non-empty
// "script".
func TestApiPayload_SCRIPTMissingScript(t *testing.T) {
	m := baseUnsetModel("SCRIPT", "SHUTDOWN")

	_, diags := m.apiPayload()
	if !diags.HasError() {
		t.Fatal("expected an error for type=SCRIPT with no script set")
	}
}

// TestResponseToModel_COMMANDShape verifies responseToModel against the
// exact shape observed from a live initshutdownscript.create call in
// COMMAND mode: "script" comes back as an empty string, not null.
func TestResponseToModel_COMMANDShape(t *testing.T) {
	api := &initShutdownScriptAPI{
		ID:      1,
		Type:    "COMMAND",
		Command: "/usr/bin/true",
		Script:  "",
		When:    "POSTINIT",
		Enabled: false,
		Timeout: 10,
		Comment: "",
	}

	m := &InitShutdownScriptModel{}
	responseToModel(api, m)

	if m.ID.ValueInt64() != 1 {
		t.Errorf("ID = %v, want 1", m.ID)
	}
	if m.Type.ValueString() != "COMMAND" {
		t.Errorf("Type = %q, want COMMAND", m.Type.ValueString())
	}
	if m.Command.ValueString() != "/usr/bin/true" {
		t.Errorf("Command = %q, want /usr/bin/true", m.Command.ValueString())
	}
	if m.Script.IsNull() {
		t.Error("Script should be an empty string, not null")
	}
	if m.Script.ValueString() != "" {
		t.Errorf("Script = %q, want empty string", m.Script.ValueString())
	}
	if m.Timeout.ValueInt64() != 10 {
		t.Errorf("Timeout = %v, want 10", m.Timeout)
	}
}

// TestResponseToModel_SCRIPTShape mirrors TestResponseToModel_COMMANDShape
// for the SCRIPT type.
func TestResponseToModel_SCRIPTShape(t *testing.T) {
	api := &initShutdownScriptAPI{
		ID:      2,
		Type:    "SCRIPT",
		Command: "",
		Script:  "/usr/bin/true",
		When:    "SHUTDOWN",
		Enabled: false,
		Timeout: 20,
		Comment: "",
	}

	m := &InitShutdownScriptModel{}
	responseToModel(api, m)

	if m.Command.IsNull() {
		t.Error("Command should be an empty string, not null")
	}
	if m.Command.ValueString() != "" {
		t.Errorf("Command = %q, want empty string", m.Command.ValueString())
	}
	if m.Script.ValueString() != "/usr/bin/true" {
		t.Errorf("Script = %q, want /usr/bin/true", m.Script.ValueString())
	}
}

// TestResponseToDataSourceModel_ProbedShape mirrors
// TestResponseToModel_COMMANDShape for the datasource model.
func TestResponseToDataSourceModel_ProbedShape(t *testing.T) {
	api := &initShutdownScriptAPI{
		ID:      1,
		Type:    "COMMAND",
		Command: "/usr/bin/true",
		Script:  "",
		When:    "POSTINIT",
		Enabled: false,
		Timeout: 10,
		Comment: "tf-probe comment",
	}

	m := &InitShutdownScriptDataSourceModel{}
	responseToDataSourceModel(api, m)

	if m.Comment.ValueString() != "tf-probe comment" {
		t.Errorf("Comment = %q, want \"tf-probe comment\"", m.Comment.ValueString())
	}
	if m.When.ValueString() != "POSTINIT" {
		t.Errorf("When = %q, want POSTINIT", m.When.ValueString())
	}
}
