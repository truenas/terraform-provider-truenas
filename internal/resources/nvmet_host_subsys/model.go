// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_host_subsys

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// HostSubsysModel is the Terraform state model for
// truenas_nvmet_host_subsys.
type HostSubsysModel struct {
	ID       types.Int64 `tfsdk:"id"`
	HostID   types.Int64 `tfsdk:"host_id"`   // Required + RequiresReplace
	SubsysID types.Int64 `tfsdk:"subsys_id"` // Required + RequiresReplace
}

// HostSubsysDataSourceModel is the Terraform state model for the
// truenas_nvmet_host_subsys data source.
type HostSubsysDataSourceModel struct {
	ID       types.Int64 `tfsdk:"id"`
	HostID   types.Int64 `tfsdk:"host_id"`
	SubsysID types.Int64 `tfsdk:"subsys_id"`
}

// hostSubsysAPI is the JSON wire format for a TrueNAS NVMe-oF host/subsystem
// association object (nvmet.host_subsys.*).
//
// Host and Subsys are decoded as json.RawMessage because create/update
// payloads use the keys "host_id"/"subsys_id" (bare integers), while
// query/get_instance responses embed full "host"/"subsys" objects
// ({"id": N, ...}). decodeEmbeddedID handles both shapes.
type hostSubsysAPI struct {
	ID     int64           `json:"id"`
	Host   json.RawMessage `json:"host"`
	Subsys json.RawMessage `json:"subsys"`
}

// decodeEmbeddedID decodes a field that may be an embedded object
// ({"id": N, ...}), a bare integer ID, or null, depending on the calling
// method / API version. fieldName is used only for error messages.
func decodeEmbeddedID(raw json.RawMessage, fieldName string) (int64, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, fmt.Errorf("%s is null in API response", fieldName)
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil {
		idRaw, ok := obj["id"]
		if !ok || len(idRaw) == 0 || string(idRaw) == "null" {
			return 0, fmt.Errorf("%s object has no non-null \"id\" field in API response: %q", fieldName, string(raw))
		}
		var id int64
		if err := json.Unmarshal(idRaw, &id); err != nil {
			return 0, fmt.Errorf("cannot decode %s.id field %q as integer: %w", fieldName, string(idRaw), err)
		}
		return id, nil
	}

	var id int64
	if err := json.Unmarshal(raw, &id); err == nil {
		return id, nil
	}

	return 0, fmt.Errorf("cannot decode %s field %q as object with id or bare integer", fieldName, string(raw))
}

// responseToModel maps an API response onto a Terraform resource model.
func responseToModel(_ context.Context, api *hostSubsysAPI, m *HostSubsysModel) diag.Diagnostics {
	var diags diag.Diagnostics

	hostID, err := decodeEmbeddedID(api.Host, "host")
	if err != nil {
		diags.AddError("Invalid host in API response", err.Error())
		return diags
	}
	subsysID, err := decodeEmbeddedID(api.Subsys, "subsys")
	if err != nil {
		diags.AddError("Invalid subsys in API response", err.Error())
		return diags
	}

	m.ID = types.Int64Value(api.ID)
	m.HostID = types.Int64Value(hostID)
	m.SubsysID = types.Int64Value(subsysID)
	return diags
}

// responseToDataSourceModel maps an API response onto a Terraform data
// source model.
func responseToDataSourceModel(api *hostSubsysAPI, m *HostSubsysDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	hostID, err := decodeEmbeddedID(api.Host, "host")
	if err != nil {
		diags.AddError("Invalid host in API response", err.Error())
		return diags
	}
	subsysID, err := decodeEmbeddedID(api.Subsys, "subsys")
	if err != nil {
		diags.AddError("Invalid subsys in API response", err.Error())
		return diags
	}

	m.ID = types.Int64Value(api.ID)
	m.HostID = types.Int64Value(hostID)
	m.SubsysID = types.Int64Value(subsysID)
	return diags
}

// apiPayload builds the map[string]any payload for nvmet.host_subsys.create.
// host_id and subsys_id are always included; both are Required in the
// schema so they are always known.
func (m *HostSubsysModel) apiPayload(_ context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{
		"host_id":   m.HostID.ValueInt64(),
		"subsys_id": m.SubsysID.ValueInt64(),
	}
	return p, diags
}
