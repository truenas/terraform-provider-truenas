// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package zvol

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

func TestZvolApiPayload_ZFSProps(t *testing.T) {
	m := &ZvolModel{
		Name:           types.StringValue("tank/v"),
		VolSize:        types.Int64Value(1073741824),
		Checksum:       types.StringValue("sha256"), // uppercased
		ReadOnly:       types.StringValue("ON"),
		Snapdev:        types.StringValue("VISIBLE"),
		Copies:         types.Int64Value(2),
		RefReservation: types.Int64Value(1073741824),
	}
	p := m.apiPayload()
	if p["checksum"] != "SHA256" {
		t.Errorf("checksum = %v, want SHA256", p["checksum"])
	}
	if p["readonly"] != "ON" || p["snapdev"] != "VISIBLE" {
		t.Errorf("readonly/snapdev = %v/%v", p["readonly"], p["snapdev"])
	}
	if p["copies"] != int64(2) {
		t.Errorf("copies = %v", p["copies"])
	}
	if p["refreservation"] != int64(1073741824) {
		t.Errorf("refreservation = %v", p["refreservation"])
	}
	if _, ok := p["reservation"]; ok {
		t.Errorf("unset reservation should be absent")
	}
}

func TestZvolResponseToModel_SourceAware(t *testing.T) {
	var api zvolAPI
	api.Name = "tank/v"
	api.ChecksumP = sourced("SHA256", true, `"sha256"`, "LOCAL")
	api.ReadOnlyP = sourced("OFF", true, `false`, "INHERITED") // inherited -> null
	api.CopiesP = sourced("1", true, `1`, "DEFAULT")           // default -> null

	var m ZvolModel
	responseToModel(&api, &m)

	if m.Checksum.ValueString() != "SHA256" {
		t.Errorf("checksum LOCAL should be SHA256, got %v", m.Checksum)
	}
	if m.ReadOnly.ValueString() != "INHERIT" {
		t.Errorf("readonly INHERITED should read back INHERIT (source-aware enum), got %v", m.ReadOnly)
	}
	if !m.Copies.IsNull() {
		t.Errorf("copies DEFAULT should be null, got %v", m.Copies)
	}
}
