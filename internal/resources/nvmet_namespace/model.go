// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_namespace

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// NVMetNamespaceModel is the Terraform state model for
// truenas_nvmet_namespace.
type NVMetNamespaceModel struct {
	ID         types.Int64  `tfsdk:"id"`
	SubsysID   types.Int64  `tfsdk:"subsys_id"`   // Required + RequiresReplace
	DevicePath types.String `tfsdk:"device_path"` // Required, e.g. "zvol/tank/vms/vm1"
	DeviceType types.String `tfsdk:"device_type"` // Optional+Computed+USFU; ZVOL or FILE
	Enabled    types.Bool   `tfsdk:"enabled"`     // Optional+Computed+USFU
	Filesize   types.Int64  `tfsdk:"filesize"`    // Optional+Computed+USFU; nullable (FILE type)
	NSID       types.Int64  `tfsdk:"nsid"`        // Optional+Computed+USFU; nullable -> auto-assign
	// Computed
	DeviceNGUID types.String `tfsdk:"device_nguid"` // Computed+USFU
	DeviceUUID  types.String `tfsdk:"device_uuid"`  // Computed+USFU
	Locked      types.Bool   `tfsdk:"locked"`       // Computed, NO USFU (server-mutable)
}

// NVMetNamespaceDataSourceModel is the read-only model for the
// truenas_nvmet_namespace datasource. It mirrors NVMetNamespaceModel exactly;
// the datasource keeps its own model + schema pair so schema/model drift is
// caught independently by TestNVMetNamespaceDataSourceModel_MatchesSchema.
type NVMetNamespaceDataSourceModel struct {
	ID          types.Int64  `tfsdk:"id"`
	SubsysID    types.Int64  `tfsdk:"subsys_id"`
	DevicePath  types.String `tfsdk:"device_path"`
	DeviceType  types.String `tfsdk:"device_type"`
	Enabled     types.Bool   `tfsdk:"enabled"`
	Filesize    types.Int64  `tfsdk:"filesize"`
	NSID        types.Int64  `tfsdk:"nsid"`
	DeviceNGUID types.String `tfsdk:"device_nguid"`
	DeviceUUID  types.String `tfsdk:"device_uuid"`
	Locked      types.Bool   `tfsdk:"locked"`
}

// nvmetNamespaceAPI is the JSON wire format for a TrueNAS NVMe-oF namespace
// object (nvmet.namespace.*).
//
// Subsys is decoded as json.RawMessage because create/update payloads use
// the key "subsys_id" (a bare integer), while query/get_instance responses
// embed a full "subsys" object ({"id": N, "name": "...", ...}).
// decodeSubsysID handles both shapes.
type nvmetNamespaceAPI struct {
	ID          int64           `json:"id"`
	NSID        int64           `json:"nsid"`
	Subsys      json.RawMessage `json:"subsys"`
	DeviceType  string          `json:"device_type"`
	DevicePath  string          `json:"device_path"`
	Filesize    *int64          `json:"filesize"`
	Enabled     bool            `json:"enabled"`
	DeviceNGUID string          `json:"device_nguid"`
	DeviceUUID  string          `json:"device_uuid"`
	Locked      bool            `json:"locked"`
}

// decodeSubsysID decodes the "subsys" field of a nvmetNamespaceAPI response,
// which may be an embedded object ({"id": N, ...}) or a bare integer ID
// depending on the calling method / API version.
func decodeSubsysID(raw json.RawMessage) (int64, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return 0, fmt.Errorf("subsys is null in API response")
	}

	var obj struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return obj.ID, nil
	}

	var id int64
	if err := json.Unmarshal(raw, &id); err == nil {
		return id, nil
	}

	return 0, fmt.Errorf("cannot decode subsys field %q as object with id or bare integer", string(raw))
}

// responseToModel maps an API response onto a Terraform model. A nil
// Filesize maps to null rather than 0, so that an update payload built from
// this model omits it (see guardedFields) instead of sending an explicit 0
// that overwrites a server-side null.
func responseToModel(_ context.Context, api *nvmetNamespaceAPI, m *NVMetNamespaceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	subsysID, err := decodeSubsysID(api.Subsys)
	if err != nil {
		diags.AddError("Invalid subsys in API response", err.Error())
		return diags
	}

	m.ID = types.Int64Value(api.ID)
	m.SubsysID = types.Int64Value(subsysID)
	m.DevicePath = types.StringValue(api.DevicePath)
	m.DeviceType = types.StringValue(api.DeviceType)
	m.Enabled = types.BoolValue(api.Enabled)
	m.NSID = types.Int64Value(api.NSID)
	m.DeviceNGUID = types.StringValue(api.DeviceNGUID)
	m.DeviceUUID = types.StringValue(api.DeviceUUID)
	m.Locked = types.BoolValue(api.Locked)

	if api.Filesize != nil {
		m.Filesize = types.Int64Value(*api.Filesize)
	} else {
		m.Filesize = types.Int64Null()
	}

	return diags
}

// responseToDataSourceModel maps an API response onto an
// NVMetNamespaceDataSourceModel using the same nil-pointer-to-null rule as
// responseToModel.
func responseToDataSourceModel(_ context.Context, api *nvmetNamespaceAPI, m *NVMetNamespaceDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	subsysID, err := decodeSubsysID(api.Subsys)
	if err != nil {
		diags.AddError("Invalid subsys in API response", err.Error())
		return diags
	}

	m.ID = types.Int64Value(api.ID)
	m.SubsysID = types.Int64Value(subsysID)
	m.DevicePath = types.StringValue(api.DevicePath)
	m.DeviceType = types.StringValue(api.DeviceType)
	m.Enabled = types.BoolValue(api.Enabled)
	m.NSID = types.Int64Value(api.NSID)
	m.DeviceNGUID = types.StringValue(api.DeviceNGUID)
	m.DeviceUUID = types.StringValue(api.DeviceUUID)
	m.Locked = types.BoolValue(api.Locked)

	if api.Filesize != nil {
		m.Filesize = types.Int64Value(*api.Filesize)
	} else {
		m.Filesize = types.Int64Null()
	}

	return diags
}

// guardedFields builds the set of optional payload fields shared by create
// and update: device_type, enabled, filesize. Each is included only when its
// model value is known (not null, not unknown).
func (m *NVMetNamespaceModel) guardedFields() map[string]any {
	p := map[string]any{}

	if !m.DeviceType.IsNull() && !m.DeviceType.IsUnknown() {
		p["device_type"] = m.DeviceType.ValueString()
	}
	if !m.Enabled.IsNull() && !m.Enabled.IsUnknown() {
		p["enabled"] = m.Enabled.ValueBool()
	}
	if !m.Filesize.IsNull() && !m.Filesize.IsUnknown() {
		p["filesize"] = m.Filesize.ValueInt64()
	}

	return p
}

// createPayload builds the map[string]any payload for
// nvmet.namespace.create. "subsys_id" and "device_path" are always
// included. "device_type"/"enabled"/"filesize" are guarded (see
// guardedFields). "nsid" is guarded separately and omitted entirely when
// unset, so the server auto-assigns the next available namespace ID.
func (m *NVMetNamespaceModel) createPayload(_ context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	payload := m.guardedFields()
	payload["subsys_id"] = m.SubsysID.ValueInt64()
	payload["device_path"] = m.DevicePath.ValueString()

	if !m.NSID.IsNull() && !m.NSID.IsUnknown() {
		payload["nsid"] = m.NSID.ValueInt64()
	}

	return payload, diags
}

// updatePayload builds the map[string]any payload for
// nvmet.namespace.update. It never includes "subsys_id" (RequiresReplace)
// or "nsid" (immutable after create). "device_path"/"device_type"/
// "enabled"/"filesize" are guarded (see guardedFields).
func (m *NVMetNamespaceModel) updatePayload(_ context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	payload := m.guardedFields()
	if !m.DevicePath.IsNull() && !m.DevicePath.IsUnknown() {
		payload["device_path"] = m.DevicePath.ValueString()
	}

	return payload, diags
}
