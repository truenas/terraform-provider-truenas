// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package webshare

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// --- versionGateDiagnostics ------------------------------------------------

func TestVersionGateDiagnostics_BelowFloor(t *testing.T) {
	for _, version := range []string{"25.10.3.1", "25.10", "24.10.2", "1.0", ""} {
		diags := versionGateDiagnostics(version)
		if !diags.HasError() {
			t.Fatalf("version %q: expected an error diagnostic, got none", version)
		}
		if len(diags) != 1 {
			t.Fatalf("version %q: expected exactly 1 diagnostic, got %d: %v", version, len(diags), diags)
		}
		if diags[0].Detail() != "truenas_webshare requires TrueNAS 26.0 or later" {
			t.Errorf("version %q: detail = %q, want the documented message", version, diags[0].Detail())
		}
	}
}

func TestVersionGateDiagnostics_AtOrAboveFloor(t *testing.T) {
	for _, version := range []string{"26.0.0", "26.0.0-BETA.2", "26.0", "26.1.0", "27.0.0"} {
		diags := versionGateDiagnostics(version)
		if diags.HasError() {
			t.Errorf("version %q: unexpected error diagnostic: %v", version, diags)
		}
	}
}

// --- responseToModel / responseToDataSourceModel ------------------------------

func fullAPI() *webshareAPI {
	dataset := "tank/tf-probe-webshare-ds"
	relPath := ""
	locked := false
	return &webshareAPI{
		ID:           2,
		Name:         "tf-probe-webshare",
		Path:         "/mnt/tank/tf-probe-webshare-ds",
		Dataset:      &dataset,
		RelativePath: &relPath,
		Enabled:      true,
		IsHomeBase:   false,
		Locked:       &locked,
	}
}

func TestResponseToModel(t *testing.T) {
	api := fullAPI()
	m := &WebshareModel{}
	responseToModel(api, m)

	if m.ID.ValueInt64() != 2 {
		t.Errorf("ID = %d, want 2", m.ID.ValueInt64())
	}
	if m.Name.ValueString() != "tf-probe-webshare" {
		t.Errorf("Name = %q", m.Name.ValueString())
	}
	if m.Path.ValueString() != "/mnt/tank/tf-probe-webshare-ds" {
		t.Errorf("Path = %q", m.Path.ValueString())
	}
	if !m.Enabled.ValueBool() {
		t.Error("Enabled should be true")
	}
	if m.IsHomeBase.ValueBool() {
		t.Error("IsHomeBase should be false")
	}
	if m.Dataset.ValueString() != "tank/tf-probe-webshare-ds" {
		t.Errorf("Dataset = %q", m.Dataset.ValueString())
	}
	if m.RelativePath.ValueString() != "" {
		t.Errorf("RelativePath = %q, want empty string", m.RelativePath.ValueString())
	}
	if m.RelativePath.IsNull() {
		t.Error("RelativePath should be a known empty string, not null (locked was a non-nil pointer to \"\")")
	}
	if m.Locked.ValueBool() {
		t.Error("Locked should be false")
	}
}

func TestResponseToModel_NilPointersBecomeNull(t *testing.T) {
	// Probed live: dataset/relative_path read back nil (JSON null)
	// immediately after create/get_instance — see model.go's doc comment.
	// locked can also be null when lock status wasn't requested/available.
	api := fullAPI()
	api.Dataset = nil
	api.RelativePath = nil
	api.Locked = nil
	m := &WebshareModel{}
	responseToModel(api, m)

	if !m.Dataset.IsNull() {
		t.Errorf("Dataset = %#v, want null", m.Dataset)
	}
	if !m.RelativePath.IsNull() {
		t.Errorf("RelativePath = %#v, want null", m.RelativePath)
	}
	if !m.Locked.IsNull() {
		t.Errorf("Locked = %#v, want null", m.Locked)
	}
}

func TestResponseToDataSourceModel(t *testing.T) {
	api := fullAPI()
	m := &WebshareDataSourceModel{}
	responseToDataSourceModel(api, m)
	if m.Name.ValueString() != "tf-probe-webshare" {
		t.Errorf("Name = %q", m.Name.ValueString())
	}
	if m.Dataset.ValueString() != "tank/tf-probe-webshare-ds" {
		t.Errorf("Dataset = %q", m.Dataset.ValueString())
	}
}

// --- createPayload / updatePayload --------------------------------------------

func TestCreatePayload(t *testing.T) {
	m := &WebshareModel{
		Name:       types.StringValue("tf-acc-webshare"),
		Path:       types.StringValue("/mnt/tank/tf-acc-ds"),
		Enabled:    types.BoolValue(true),
		IsHomeBase: types.BoolValue(false),
	}
	p := m.createPayload()

	if p["name"] != "tf-acc-webshare" {
		t.Errorf(`p["name"] = %#v`, p["name"])
	}
	if p["path"] != "/mnt/tank/tf-acc-ds" {
		t.Errorf(`p["path"] = %#v`, p["path"])
	}
	if p["enabled"] != true {
		t.Errorf(`p["enabled"] = %#v`, p["enabled"])
	}
	if p["is_home_base"] != false {
		t.Errorf(`p["is_home_base"] = %#v`, p["is_home_base"])
	}
	if _, present := p["comment"]; present {
		t.Error(`p["comment"] should never be present: sharing.webshare has no comment field (probed live)`)
	}
}

func TestUpdatePayload(t *testing.T) {
	m := &WebshareModel{
		Name:       types.StringValue("renamed"),
		Path:       types.StringValue("/mnt/tank/tf-acc-ds"),
		Enabled:    types.BoolValue(false),
		IsHomeBase: types.BoolValue(false),
	}
	p := m.updatePayload()

	if p["name"] != "renamed" {
		t.Errorf(`p["name"] = %#v, want "renamed"`, p["name"])
	}
	if p["enabled"] != false {
		t.Errorf(`p["enabled"] = %#v, want false`, p["enabled"])
	}
}
