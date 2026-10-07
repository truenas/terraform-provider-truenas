// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func sourced(value string, hasValue bool, parsed, source string) zfsSourced {
	z := zfsSourced{Source: source}
	if hasValue {
		v := value
		z.Value = &v
	}
	if parsed != "" {
		z.Parsed = json.RawMessage(parsed)
	}
	return z
}

func TestLocalString(t *testing.T) {
	if got := localString(sourced("OFF", true, `false`, "LOCAL")); got.ValueString() != "OFF" {
		t.Errorf("LOCAL: got %v, want OFF", got)
	}
	if got := localString(sourced("OFF", true, `false`, "DEFAULT")); !got.IsNull() {
		t.Errorf("DEFAULT should be null, got %v", got)
	}
	if got := localString(sourced("ON", true, `true`, "INHERITED")); !got.IsNull() {
		t.Errorf("INHERITED should be null, got %v", got)
	}
	if got := localString(sourced("", false, ``, "LOCAL")); !got.IsNull() {
		t.Errorf("LOCAL with nil value should be null, got %v", got)
	}
}

func TestLocalInt(t *testing.T) {
	if got := localInt(sourced("", false, `131072`, "LOCAL")); got.ValueInt64() != 131072 {
		t.Errorf("LOCAL number: got %v, want 131072", got)
	}
	// tolerate parsed as a numeric string (get_instance is inconsistent)
	if got := localInt(sourced("", false, `"0"`, "LOCAL")); got.IsNull() || got.ValueInt64() != 0 {
		t.Errorf("LOCAL string-0: got %v, want 0", got)
	}
	if got := localInt(sourced("", false, `16384`, "DEFAULT")); !got.IsNull() {
		t.Errorf("DEFAULT should be null, got %v", got)
	}
}

func TestApiPayload_ZFSProps(t *testing.T) {
	m := &DatasetModel{
		Name:                  types.StringValue("tank/x"),
		ATime:                 types.StringValue("off"), // lowercase -> uppercased
		Sync:                  types.StringValue("ALWAYS"),
		Dedup:                 types.StringValue("ON"),
		RecordSize:            types.StringValue("128K"),
		Copies:                types.Int64Value(2),
		SpecialSmallBlockSize: types.Int64Value(0), // 0 is meaningful, must be sent
		RefReservation:        types.Int64Value(1073741824),
	}
	p := m.apiPayload()
	if p["atime"] != "OFF" {
		t.Errorf("atime = %v, want OFF (uppercased)", p["atime"])
	}
	if p["sync"] != "ALWAYS" {
		t.Errorf("sync = %v", p["sync"])
	}
	if p["deduplication"] != "ON" {
		t.Errorf("dedup should map to wire key 'deduplication', got %+v", p)
	}
	if _, ok := p["dedup"]; ok {
		t.Errorf("must not send 'dedup' key")
	}
	if p["recordsize"] != "128K" {
		t.Errorf("recordsize = %v, want 128K verbatim", p["recordsize"])
	}
	if p["copies"] != int64(2) {
		t.Errorf("copies = %v", p["copies"])
	}
	if v, ok := p["special_small_block_size"]; !ok || v != int64(0) {
		t.Errorf("special_small_block_size must be sent as 0, got %v ok=%v", v, ok)
	}
	if p["refreservation"] != int64(1073741824) {
		t.Errorf("refreservation = %v", p["refreservation"])
	}
	// unset props absent
	for _, k := range []string{"aclmode", "exec", "readonly", "checksum", "snapdir"} {
		if _, ok := p[k]; ok {
			t.Errorf("unset %s should be absent", k)
		}
	}
}

func TestResponseToModel_SourceAware(t *testing.T) {
	var api apiResponse
	api.Name = "tank/x"
	api.ATimeP = sourced("OFF", true, `false`, "LOCAL")       // set locally
	api.ExecP = sourced("ON", true, `true`, "DEFAULT")        // inherited/default
	api.RecordSizeP = sourced("1M", true, `1048576`, "LOCAL") // local
	api.CopiesP = sourced("1", true, `1`, "DEFAULT")          // default
	api.SSBSP = sourced("0", true, `0`, "LOCAL")              // local zero

	var m DatasetModel
	r := &DatasetResource{}
	r.responseToModel(&api, &m)

	if m.ATime.ValueString() != "OFF" {
		t.Errorf("atime LOCAL should be OFF, got %v", m.ATime)
	}
	if m.Exec.ValueString() != "INHERIT" {
		t.Errorf("exec DEFAULT should read back INHERIT (source-aware enum), got %v", m.Exec)
	}
	if m.RecordSize.ValueString() != "1M" {
		t.Errorf("recordsize LOCAL should be 1M, got %v", m.RecordSize)
	}
	if !m.Copies.IsNull() {
		t.Errorf("copies DEFAULT should be null, got %v", m.Copies)
	}
	if m.SpecialSmallBlockSize.IsNull() || m.SpecialSmallBlockSize.ValueInt64() != 0 {
		t.Errorf("ssbs LOCAL 0 should be 0 (not null), got %v", m.SpecialSmallBlockSize)
	}
}
