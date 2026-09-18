// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container_device

import (
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// containerDeviceVersionFloorMajor/Minor is the minimum TrueNAS
// release that exposes the container.device.* namespace at all. Probed
// live: TrueNAS 25.10 returns 0 container.device.* methods from
// core.get_methods (namespace genuinely absent, matching container.* itself
// — see the container package's own versionGateDiagnostics); TrueNAS 26.0
// supports the full namespace (container.device.create/update/delete/query/
// get_instance, all job:false, plus the usb_choices/nic_attach_choices/
// gpu_choices helpers, confirmed live this session — see resource.go's
// checkVersion for where this gates every entry point, including Delete,
// before any container.device.* call reaches the wire).
const (
	containerDeviceVersionFloorMajor = 26
	containerDeviceVersionFloorMinor = 0
)

// versionGateDiagnostics reports whether the given TrueNAS release string
// (as returned by system.version_short via client.ServerVersion) is at or
// above the TrueNAS 26.0 floor the container.device namespace requires,
// returning a single clean error diagnostic when it is not. Pure function
// of an already-probed version string (no live client access), matching
// the lxc_config/container/webshare precedent so it stays independently
// unit-testable.
func versionGateDiagnostics(version string) diag.Diagnostics {
	var diags diag.Diagnostics
	if !client.VersionAtLeastString(version, containerDeviceVersionFloorMajor, containerDeviceVersionFloorMinor) {
		diags.AddError(
			"TrueNAS version too old",
			"truenas_container_device requires TrueNAS 26.0 or later",
		)
	}
	return diags
}

// ContainerDeviceModel is the Terraform state model for
// truenas_container_device.
//
// "attributes" is carried as an opaque JSON-encoded string rather than
// modeled field-by-field, mirroring truenas_vm_device: container.device.
// create's own accepts schema (probed live via core.get_methods) is a
// "dtype"-discriminated union (FILESYSTEM/GPU/NIC/USB) whose per-type field
// set this resource does not need to validate or diff structurally. See
// schema.go's Description for the exact field names of each dtype this
// package's probe evidence covers.
type ContainerDeviceModel struct {
	ID         types.Int64  `tfsdk:"id"`
	Container  types.Int64  `tfsdk:"container"`  // parent container id, RequiresReplace
	Attributes types.String `tfsdk:"attributes"` // JSON doc incl. "dtype" key
}

// ContainerDeviceDataSourceModel is the read-only lookup model for the
// truenas_container_device datasource.
type ContainerDeviceDataSourceModel struct {
	ID         types.Int64  `tfsdk:"id"`
	Container  types.Int64  `tfsdk:"container"`
	Attributes types.String `tfsdk:"attributes"`
}

// containerDeviceAPI is the JSON wire format for a TrueNAS container device
// object, as returned by container.device.create/update/get_instance/query.
// Probed live against TrueNAS 26.0 (the only release with this namespace —
// see versionGateDiagnostics), e.g.:
//
//	{"id": 2, "attributes": {"dtype": "FILESYSTEM", "target": "/data",
//	 "source": "/mnt/tank/tf-probe-container-device-ds"}, "container": 11}
type containerDeviceAPI struct {
	ID         int64          `json:"id"`
	Attributes map[string]any `json:"attributes"`
	Container  int64          `json:"container"`
}

// attributesMap parses the attributes JSON string and validates that it
// contains a "dtype" key, mirroring truenas_vm_device's own
// attributesMap.
func (m *ContainerDeviceModel) attributesMap() (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	var attrs map[string]any
	if err := json.Unmarshal([]byte(m.Attributes.ValueString()), &attrs); err != nil {
		diags.AddError("Invalid attributes JSON", err.Error())
		return nil, diags
	}
	if _, ok := attrs["dtype"]; !ok {
		diags.AddError("Invalid attributes", "attributes JSON must contain a \"dtype\" key (FILESYSTEM, NIC, or USB — see the resource documentation)")
		return nil, diags
	}
	return attrs, diags
}

// attributesDrifted reports whether any key present in stateAttrs has a
// different value in apiAttrs. API-added default keys are ignored (not
// drift) since the API may normalize/expand attributes with defaults (e.g.
// NIC's auto-generated "mac", or FILESYSTEM's "target" default when the
// user never set one — see schema.go's Description for the latter's known
// quirk). Mirrors truenas_vm_device's own attributesDrifted exactly.
func attributesDrifted(stateAttrs, apiAttrs map[string]any) bool {
	for k, v := range stateAttrs {
		av, ok := apiAttrs[k]
		if !ok {
			return true
		}
		sb, _ := json.Marshal(v)
		ab, _ := json.Marshal(av)
		if string(sb) != string(ab) {
			return true
		}
	}
	return false
}

// responseToModel maps containerDeviceAPI onto ContainerDeviceModel. ID and
// Container are always taken from the API; Attributes is left untouched by
// this function so callers can implement write-what-you-said / drift-aware
// semantics (see attributesDrifted).
func responseToModel(api *containerDeviceAPI, m *ContainerDeviceModel) {
	m.ID = types.Int64Value(api.ID)
	m.Container = types.Int64Value(api.Container)
}

// apiAttributesJSON marshals api.Attributes to a canonical JSON string.
func apiAttributesJSON(api *containerDeviceAPI) (string, diag.Diagnostics) {
	var diags diag.Diagnostics
	b, err := json.Marshal(api.Attributes)
	if err != nil {
		diags.AddError("Failed to marshal device attributes", err.Error())
		return "", diags
	}
	return string(b), diags
}

// createPayload builds the container.device.create argument.
func (m *ContainerDeviceModel) createPayload() (map[string]any, diag.Diagnostics) {
	attrs, diags := m.attributesMap()
	if diags.HasError() {
		return nil, diags
	}
	return map[string]any{
		"container":  m.Container.ValueInt64(),
		"attributes": attrs,
	}, diags
}

// updatePayload builds the second arg of container.device.update. It
// intentionally omits "container": probed live, container.device.update's
// own accepts schema DOES list "container" as an (optional) accepted key —
// unlike vm.device.update, which omits "vm" entirely — but this provider
// forces device-to-container reassignment through replacement instead
// (RequiresReplace in schema.go), the same convention truenas_webshare
// applies to "path" even though sharing.webshare.update also technically
// accepts a changed path. Omitting the key here keeps Update from ever
// being able to move a device, regardless of what the plan happens to
// contain.
func (m *ContainerDeviceModel) updatePayload() (map[string]any, diag.Diagnostics) {
	attrs, diags := m.attributesMap()
	if diags.HasError() {
		return nil, diags
	}
	return map[string]any{
		"attributes": attrs,
	}, diags
}

// responseToDataSourceModel maps containerDeviceAPI into
// ContainerDeviceDataSourceModel.
func responseToDataSourceModel(api *containerDeviceAPI, m *ContainerDeviceDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.Int64Value(api.ID)
	m.Container = types.Int64Value(api.Container)
	b, err := json.Marshal(api.Attributes)
	if err != nil {
		diags.AddError("Failed to marshal device attributes", err.Error())
		return diags
	}
	m.Attributes = types.StringValue(string(b))
	return diags
}
