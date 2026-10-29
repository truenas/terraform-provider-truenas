// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb_config

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// liveSMBConfig is smb.config as returned by TrueNAS 28.0 (.70).
const liveSMBConfig = `{"id": 1, "netbiosname": "TFTESTNAS", "netbiosalias": [], "workgroup": "WORKGROUP", "description": "TrueNAS Server", "minimum_protocol": "SMB2", "unixcharset": "UTF-8", "localmaster": false, "syslog": false, "aapl_extensions": true, "search_protocols": [], "admin_group": null, "guest": "nobody", "filemask": "0664", "dirmask": "0775", "ntlmv1_auth": false, "multichannel": false, "encryption": "DEFAULT", "bindip": [], "server_sid": "S-1-5-21-2442899652-1355660130-1490787297", "smb_options": "", "debug": false, "stateful_failover": false}`

func TestIsServiceRestartFailure(t *testing.T) {
	if !isServiceRestartFailure(&client.APIError{Code: 1, ErrName: "ESERVICESTARTFAILURE", Message: "The cifs service failed to start"}) {
		t.Error("errname ESERVICESTARTFAILURE not recognized")
	}
	if !isServiceRestartFailure(errors.New("truenas API error (code 19): [ESERVICESTARTFAILURE] The cifs service failed to start")) {
		t.Error("message form not recognized")
	}
	if isServiceRestartFailure(&client.APIError{Code: 22, ErrName: "EINVAL", Message: "[EINVAL] smb_update.unixcharset: bad"}) {
		t.Error("a validation error must not count as a restart failure")
	}
}

// TestPayloadSaved_RoundTrip guards B-153: the payload the provider builds
// from a config equal to the box's current values must compare as saved
// against smb.config, so a post-save restart failure becomes a warning.
func TestPayloadSaved_RoundTrip(t *testing.T) {
	ctx := context.Background()
	var api smbConfigAPI
	if err := json.Unmarshal([]byte(liveSMBConfig), &api); err != nil {
		t.Fatal(err)
	}
	var m SMBConfigModel
	if d := responseToModel(ctx, &api, &m); d.HasError() {
		t.Fatal(d)
	}
	payload, d := m.updatePayload(ctx)
	if d.HasError() {
		t.Fatal(d)
	}
	if len(payload) == 0 {
		t.Fatal("empty payload")
	}
	// Fields whose wire form differs from smb.config's are not compared by
	// payloadSaved callers; report them so a change here is visible.
	for k := range payload {
		single := map[string]any{k: payload[k]}
		if !payloadSaved(single, json.RawMessage(liveSMBConfig)) {
			t.Errorf("field %q = %#v does not compare as saved against smb.config", k, payload[k])
		}
	}
	changed := map[string]any{"description": "something else"}
	if payloadSaved(changed, json.RawMessage(liveSMBConfig)) {
		t.Error("a value that differs from smb.config must not compare as saved")
	}
}
