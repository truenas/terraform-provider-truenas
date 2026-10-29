// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package system_general

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestUICertificateDecode_redacted guards B-030: a READONLY_ADMIN session
// gets ui_certificate = "********". Decoding must not fail; the resource keeps
// the prior value and the data source reports null.
func TestUICertificateDecode_redacted(t *testing.T) {
	var api systemGeneralAPI
	raw := `{"id":1,"ui_certificate":"********","ui_certificate_name":"truenas_default","ui_address":["0.0.0.0"]}`
	if err := json.Unmarshal([]byte(raw), &api); err != nil {
		t.Fatalf("decode redacted ui_certificate: %v", err)
	}
	if !api.UICertificate.Redacted || api.UICertificate.ID != nil {
		t.Fatalf("decoded %+v, want Redacted with no id", api.UICertificate)
	}

	m := SystemGeneralModel{UICertificate: types.Int64Value(1)}
	responseToModel(context.Background(), &api, &m)
	if m.UICertificate.ValueInt64() != 1 {
		t.Errorf("resource ui_certificate = %v, want the prior value 1 kept", m.UICertificate)
	}
	if m.UICertificateName.ValueString() != "truenas_default" {
		t.Errorf("ui_certificate_name = %v", m.UICertificateName)
	}

	var ds SystemGeneralDataSourceModel
	responseToDataSourceModel(context.Background(), &api, &ds)
	if !ds.UICertificate.IsNull() {
		t.Errorf("data source ui_certificate = %v, want null", ds.UICertificate)
	}

	var bad systemGeneralAPI
	if err := json.Unmarshal([]byte(`{"ui_certificate":"x"}`), &bad); err == nil {
		t.Error("an unexpected string must still fail to decode")
	}
}

// nullConfig returns a config model with every writable attribute null.
func nullConfig() *SystemGeneralModel {
	return &SystemGeneralModel{
		DSAuth:           types.BoolNull(),
		Kbdmap:           types.StringNull(),
		Timezone:         types.StringNull(),
		UIAddress:        types.ListNull(types.StringType),
		UIAllowlist:      types.ListNull(types.StringType),
		UICertificate:    types.Int64Null(),
		UIConsolemsg:     types.BoolNull(),
		UIHTTPSPort:      types.Int64Null(),
		UIHTTPSProtocols: types.ListNull(types.StringType),
		UIHTTPSRedirect:  types.BoolNull(),
		UIPort:           types.Int64Null(),
		UIV6Address:      types.ListNull(types.StringType),
		UIXFrameOptions:  types.StringNull(),
		UsageCollection:  types.BoolNull(),
	}
}

// TestUpdatePayload_UnsetUsageCollectionNotSent guards B-143: a config that
// does not set usage_collection must not send it (sending it records an
// explicit opt-in on the server).
func TestUpdatePayload_UnsetUsageCollectionNotSent(t *testing.T) {
	cfg := nullConfig()
	cfg.Timezone = types.StringValue("America/New_York")
	p, d := cfg.updatePayload(context.Background())
	if d.HasError() {
		t.Fatal(d)
	}
	if _, ok := p["usage_collection"]; ok {
		t.Errorf("usage_collection sent although unset: %v", p)
	}
	if len(p) != 1 || p["timezone"] != "America/New_York" {
		t.Errorf("payload = %v, want only timezone", p)
	}
}

// TestWebServerSettingsChanged guards B-027: a change to a web server
// setting needs a restart; re-sending the current values, or changing only
// non-web-server settings, does not.
func TestWebServerSettingsChanged(t *testing.T) {
	one := int64(1)
	current := &systemGeneralAPI{
		UIAddress:        []string{"0.0.0.0"},
		UIV6Address:      []string{"::"},
		UIAllowlist:      nil,
		UICertificate:    uiCertificate{ID: &one},
		UIHTTPSPort:      443,
		UIHTTPSProtocols: []string{"TLSv1.2", "TLSv1.3"},
		UIPort:           80,
		UIXFrameOptions:  "SAMEORIGIN",
	}
	same := map[string]any{
		"ui_address": []string{"0.0.0.0"}, "ui_v6address": []string{"::"}, "ui_allowlist": []string{},
		"ui_certificate": int64(1), "ui_httpsport": int64(443), "ui_port": int64(80),
		"ui_httpsprotocols": []string{"TLSv1.2", "TLSv1.3"}, "ui_httpsredirect": false,
		"ui_x_frame_options": "SAMEORIGIN", "timezone": "UTC", "ui_consolemsg": true,
	}
	if webServerSettingsChanged(current, same) {
		t.Error("re-sending the current web server settings must not restart the web server")
	}
	for key, v := range map[string]any{
		"ui_httpsprotocols":  []string{"TLSv1.3"},
		"ui_x_frame_options": "DENY",
		"ui_certificate":     int64(2),
		"ui_allowlist":       []string{"192.0.2.0/24"},
		"ui_httpsredirect":   true,
		"ui_port":            int64(8080),
	} {
		if !webServerSettingsChanged(current, map[string]any{key: v}) {
			t.Errorf("%s change not detected", key)
		}
	}
}

// TestUICertificateDecode_redactedObject25: 25.10 returns the certificate as
// an object, which a READONLY_ADMIN session gets as "********" with no
// top-level name; the resource keeps the prior name, the data source null.
func TestUICertificateDecode_redactedObject25(t *testing.T) {
	var api systemGeneralAPI
	if err := json.Unmarshal([]byte(`{"id":1,"ui_certificate":"********"}`), &api); err != nil {
		t.Fatal(err)
	}
	m := SystemGeneralModel{UICertificate: types.Int64Value(1), UICertificateName: types.StringValue("truenas_default")}
	responseToModel(context.Background(), &api, &m)
	if m.UICertificateName.ValueString() != "truenas_default" {
		t.Errorf("ui_certificate_name = %v, want the prior value", m.UICertificateName)
	}
	var ds SystemGeneralDataSourceModel
	responseToDataSourceModel(context.Background(), &api, &ds)
	if !ds.UICertificateName.IsNull() {
		t.Errorf("data source ui_certificate_name = %v, want null", ds.UICertificateName)
	}
}
