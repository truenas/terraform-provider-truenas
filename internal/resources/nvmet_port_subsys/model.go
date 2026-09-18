// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_port_subsys

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// PortSubsysModel is the Terraform state model for
// truenas_nvmet_port_subsys.
type PortSubsysModel struct {
	ID       types.Int64 `tfsdk:"id"`
	PortID   types.Int64 `tfsdk:"port_id"`   // Required + RequiresReplace
	SubsysID types.Int64 `tfsdk:"subsys_id"` // Required + RequiresReplace
}

// PortSubsysDataSourceModel is the Terraform state model for the
// truenas_nvmet_port_subsys data source.
type PortSubsysDataSourceModel struct {
	ID       types.Int64 `tfsdk:"id"`
	PortID   types.Int64 `tfsdk:"port_id"`
	SubsysID types.Int64 `tfsdk:"subsys_id"`
}

// portSubsysAPI is the JSON wire format for a TrueNAS NVMe-oF port/subsystem
// association object (nvmet.port_subsys.*).
//
// Port and Subsys are decoded as json.RawMessage because create/update
// payloads use the keys "port_id"/"subsys_id" (bare integers), while
// query/get_instance responses embed full "port"/"subsys" objects
// ({"id": N, ...}). decodeEmbeddedID handles both shapes.
type portSubsysAPI struct {
	ID     int64           `json:"id"`
	Port   json.RawMessage `json:"port"`
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
func responseToModel(_ context.Context, api *portSubsysAPI, m *PortSubsysModel) diag.Diagnostics {
	var diags diag.Diagnostics

	portID, err := decodeEmbeddedID(api.Port, "port")
	if err != nil {
		diags.AddError("Invalid port in API response", err.Error())
		return diags
	}
	subsysID, err := decodeEmbeddedID(api.Subsys, "subsys")
	if err != nil {
		diags.AddError("Invalid subsys in API response", err.Error())
		return diags
	}

	m.ID = types.Int64Value(api.ID)
	m.PortID = types.Int64Value(portID)
	m.SubsysID = types.Int64Value(subsysID)
	return diags
}

// responseToDataSourceModel maps an API response onto a Terraform data
// source model.
func responseToDataSourceModel(api *portSubsysAPI, m *PortSubsysDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	portID, err := decodeEmbeddedID(api.Port, "port")
	if err != nil {
		diags.AddError("Invalid port in API response", err.Error())
		return diags
	}
	subsysID, err := decodeEmbeddedID(api.Subsys, "subsys")
	if err != nil {
		diags.AddError("Invalid subsys in API response", err.Error())
		return diags
	}

	m.ID = types.Int64Value(api.ID)
	m.PortID = types.Int64Value(portID)
	m.SubsysID = types.Int64Value(subsysID)
	return diags
}

// apiPayload builds the map[string]any payload for nvmet.port_subsys.create.
// port_id and subsys_id are always included; both are Required in the
// schema so they are always known.
func (m *PortSubsysModel) apiPayload(_ context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{
		"port_id":   m.PortID.ValueInt64(),
		"subsys_id": m.SubsysID.ValueInt64(),
	}
	return p, diags
}
