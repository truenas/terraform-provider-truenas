// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package zvol

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func baseZvol() ZvolModel {
	return ZvolModel{
		Name:           types.StringValue("fast/v"),
		VolSize:        types.Int64Value(1 << 30),
		Comments:       types.StringValue(""),
		Sync:           types.StringValue("STANDARD"),
		RefReservation: types.Int64Value(1<<30 + 4096),
		Reservation:    types.Int64Null(),
	}
}

// An unrelated update sends only the changed key: not the empty comments
// (B-039) and not the unchanged refreservation (B-038).
func TestZvolUpdatePayloadOnlyChanged(t *testing.T) {
	prior := baseZvol()
	plan := baseZvol()
	plan.Sync = types.StringValue("ALWAYS")
	p := plan.updateAPIPayload(&prior)
	if len(p) != 1 || p["sync"] != "ALWAYS" {
		t.Errorf("payload = %v, want only sync=ALWAYS", p)
	}
	if p := prior.updateAPIPayload(&prior); len(p) != 0 {
		t.Errorf("unchanged plan payload = %v, want empty", p)
	}
}

// A grow with refreservation unset (unknown in the plan) sends volsize only;
// with refreservation configured it is sent alongside even though unchanged.
func TestZvolUpdatePayloadGrow(t *testing.T) {
	prior := baseZvol()
	plan := baseZvol()
	plan.VolSize = types.Int64Value(2 << 30)
	plan.RefReservation = types.Int64Unknown()
	if p := plan.updateAPIPayload(&prior); len(p) != 1 || p["volsize"] != int64(2<<30) {
		t.Errorf("grow payload = %v, want only volsize", p)
	}
	// A 25.10/26.0 thick zvol (refreservation == volsize) stays thick.
	prior.RefReservation = prior.VolSize
	if p := plan.updateAPIPayload(&prior); p["refreservation"] != int64(2<<30) {
		t.Errorf("thick grow payload = %v, want refreservation = new volsize", p)
	}
	prior.RefReservation = types.Int64Value(1 << 30)
	plan.RefReservation = prior.RefReservation
	p := plan.updateAPIPayload(&prior)
	if p["refreservation"] != int64(1<<30) || p["volsize"] != int64(2<<30) || len(p) != 2 {
		t.Errorf("grow with configured refreservation payload = %v", p)
	}
}

// encryption set without inherit_encryption sends inherit_encryption = false
// at create (B-042); with neither set nothing is added.
func TestZvolCreatePayloadInheritEncryption(t *testing.T) {
	m := baseZvol()
	m.Encryption = types.BoolValue(false)
	if p := m.createAPIPayload(); p["inherit_encryption"] != false {
		t.Errorf("encryption=false payload inherit_encryption = %v, want false", p["inherit_encryption"])
	}
	m.Encryption = types.BoolUnknown()
	if p := m.createAPIPayload(); p["inherit_encryption"] != nil {
		t.Errorf("unset encryption payload carries inherit_encryption = %v", p["inherit_encryption"])
	}
	m.Encryption = types.BoolNull()
	m.InheritEncryption = types.BoolValue(true)
	if p := m.createAPIPayload(); p["inherit_encryption"] != true {
		t.Errorf("configured inherit_encryption lost: %v", p)
	}
}

func decodeZvol(t *testing.T, js string) *zvolAPI {
	t.Helper()
	var a zvolAPI
	if err := json.Unmarshal([]byte(js), &a); err != nil {
		t.Fatal(err)
	}
	return &a
}

// A LOCAL 0 reservation comes back as parsed = null, rawvalue "0" (B-090);
// sparse derives from it (B-109); "INHERIT" comments from 26.0 read as ""
// (B-147).
func TestZvolReadBack(t *testing.T) {
	a := decodeZvol(t, `{"name":"fast/v","volsize":{"parsed":67108864},
		"refreservation":{"parsed":null,"rawvalue":"0","value":"0","source":"LOCAL"},
		"reservation":{"parsed":null,"rawvalue":"0","value":"0","source":"LOCAL"},
		"user_properties":{"comments":{"value":"INHERIT","source":"LOCAL"}}}`)
	var m ZvolModel
	responseToModel(a, &m)
	if m.RefReservation.IsNull() || m.RefReservation.ValueInt64() != 0 {
		t.Errorf("refreservation = %v, want 0", m.RefReservation)
	}
	if m.Reservation.IsNull() || m.Reservation.ValueInt64() != 0 {
		t.Errorf("reservation = %v, want 0", m.Reservation)
	}
	if m.Comments.ValueString() != "" {
		t.Errorf("comments = %q, want empty", m.Comments.ValueString())
	}
	if !sparseFromAPI(a) {
		t.Error("refreservation 0: sparse = false, want true")
	}
	thick := decodeZvol(t, `{"name":"fast/v","refreservation":{"parsed":68157440,"rawvalue":"68157440","source":"LOCAL"}}`)
	if sparseFromAPI(thick) {
		t.Error("refreservation > 0: sparse = true, want false")
	}
	none := decodeZvol(t, `{"name":"fast/v","refreservation":{"parsed":null,"rawvalue":"none","source":"DEFAULT"}}`)
	if !sparseFromAPI(none) {
		t.Error("refreservation none: sparse = false, want true")
	}
	inherited := decodeZvol(t, `{"name":"fast/v","reservation":{"parsed":null,"rawvalue":"0","source":"DEFAULT"}}`)
	var mi ZvolModel
	responseToModel(inherited, &mi)
	if !mi.Reservation.IsNull() {
		t.Errorf("DEFAULT reservation = %v, want null", mi.Reservation)
	}
}

// Spaces in a zvol name are "+" in its /dev/zvol node. (B-160)
func TestZvolDevicePath(t *testing.T) {
	if got := zvolDevicePath("tank/my vol/a b"); got != "/dev/zvol/tank/my+vol/a+b" {
		t.Errorf("zvolDevicePath = %q", got)
	}
}
