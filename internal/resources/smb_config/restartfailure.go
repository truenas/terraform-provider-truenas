// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package smb_config

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// isServiceRestartFailure reports whether err is the ESERVICESTARTFAILURE
// smb.update raises when the cifs restart it always performs after saving
// the configuration reports failure. On TrueNAS 28.0 that happens when the
// discovery service reload the restart waits on times out, even though smbd
// itself started (B-153).
func isServiceRestartFailure(err error) bool {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) && apiErr.ErrName == "ESERVICESTARTFAILURE" {
		return true
	}
	return err != nil && strings.Contains(err.Error(), "ESERVICESTARTFAILURE")
}

// payloadSaved reports whether every field in payload reads back from
// smb.config (raw) with the same value, i.e. the update was stored even
// though the call returned an error. Values are compared as decoded JSON.
func payloadSaved(payload map[string]any, raw json.RawMessage) bool {
	var current map[string]any
	if err := json.Unmarshal(raw, &current); err != nil {
		return false
	}
	for key, sent := range payload {
		got, ok := current[key]
		if !ok {
			return false
		}
		b, err := json.Marshal(sent)
		if err != nil {
			return false
		}
		var want any
		if err := json.Unmarshal(b, &want); err != nil {
			return false
		}
		if !reflect.DeepEqual(want, got) {
			return false
		}
	}
	return true
}

// update calls smb.update. When it fails only because the cifs restart that
// follows the save reported failure, and the configuration read back shows
// every sent field stored, the apply continues with a warning instead of an
// error: the settings are in place, and failing would leave Terraform state
// out of step with the box until the next apply.
func (r *SMBConfigResource) update(ctx context.Context, payload map[string]any, diags *diag.Diagnostics) error {
	_, err := r.client.Call(ctx, "smb.update", payload)
	if err == nil || !isServiceRestartFailure(err) {
		return err
	}
	raw, readErr := r.client.CallRead(ctx, "smb.config")
	if readErr != nil || !payloadSaved(payload, raw) {
		return err
	}
	diags.AddWarning(
		"SMB configuration saved, but the SMB service restart reported a failure",
		"TrueNAS stored the new SMB settings, then reported that restarting the SMB service failed: "+
			err.Error()+". Check the SMB service state on the system.",
	)
	return nil
}
