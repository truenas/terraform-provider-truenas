// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_device

import (
	"encoding/json"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// VMDeviceModel is the Terraform state model for truenas_vm_device.
type VMDeviceModel struct {
	ID         types.Int64  `tfsdk:"id"`
	VM         types.Int64  `tfsdk:"vm"`         // parent VM id, RequiresReplace
	Attributes types.String `tfsdk:"attributes"` // JSON doc incl. "dtype" key
	Order      types.Int64  `tfsdk:"order"`      // boot order; Optional+Computed
}

// VMDeviceDataSourceModel is the read-only lookup model for the
// truenas_vm_device datasource.
type VMDeviceDataSourceModel struct {
	ID         types.Int64  `tfsdk:"id"`
	VM         types.Int64  `tfsdk:"vm"`
	Attributes types.String `tfsdk:"attributes"`
	Order      types.Int64  `tfsdk:"order"`
}

// deviceAPI is the JSON wire format for a TrueNAS VM device object.
type deviceAPI struct {
	ID         int64          `json:"id"`
	VM         int64          `json:"vm"`
	Attributes map[string]any `json:"attributes"`
	Order      int64          `json:"order"`
}

// attributesMap parses the attributes JSON string and validates that it
// contains a "dtype" key.
func (m *VMDeviceModel) attributesMap() (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	var attrs map[string]any
	if err := json.Unmarshal([]byte(m.Attributes.ValueString()), &attrs); err != nil {
		diags.AddError("Invalid attributes JSON", err.Error())
		return nil, diags
	}
	if _, ok := attrs["dtype"]; !ok {
		diags.AddError("Invalid attributes", "attributes JSON must contain a \"dtype\" key (DISK, NIC, CDROM, DISPLAY, PCI, RAW, USB)")
		return nil, diags
	}
	return attrs, diags
}

// attributesDrifted reports whether any key present in stateAttrs has a
// different value in apiAttrs. API-added default keys are ignored (not
// drift) since the API may normalize/expand attributes with defaults.
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

// responseToModel maps deviceAPI onto VMDeviceModel. ID, VM, and Order are
// always taken from the API; Attributes is left untouched by this function
// so callers can implement write-what-you-said / drift-aware semantics.
func responseToModel(api *deviceAPI, m *VMDeviceModel) {
	m.ID = types.Int64Value(api.ID)
	m.VM = types.Int64Value(api.VM)
	m.Order = types.Int64Value(api.Order)
}

// apiAttributesJSON marshals api.Attributes to a canonical JSON string.
func apiAttributesJSON(api *deviceAPI) (string, diag.Diagnostics) {
	var diags diag.Diagnostics
	b, err := json.Marshal(api.Attributes)
	if err != nil {
		diags.AddError("Failed to marshal device attributes", err.Error())
		return "", diags
	}
	return string(b), diags
}

// createPayload builds the vm.device.create argument.
func (m *VMDeviceModel) createPayload() (map[string]any, diag.Diagnostics) {
	attrs, diags := m.attributesMap()
	if diags.HasError() {
		return nil, diags
	}
	p := map[string]any{
		"vm":         m.VM.ValueInt64(),
		"attributes": attrs,
	}
	if !m.Order.IsNull() && !m.Order.IsUnknown() {
		p["order"] = m.Order.ValueInt64()
	}
	return p, diags
}

// updatePayload builds the second arg of vm.device.update. It intentionally
// omits "vm" — a device cannot be moved between VMs (vm is RequiresReplace).
func (m *VMDeviceModel) updatePayload() (map[string]any, diag.Diagnostics) {
	attrs, diags := m.attributesMap()
	if diags.HasError() {
		return nil, diags
	}
	p := map[string]any{
		"attributes": attrs,
	}
	if !m.Order.IsNull() && !m.Order.IsUnknown() {
		p["order"] = m.Order.ValueInt64()
	}
	return p, diags
}

// responseToDataSourceModel maps deviceAPI into VMDeviceDataSourceModel.
func responseToDataSourceModel(api *deviceAPI, m *VMDeviceDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.Int64Value(api.ID)
	m.VM = types.Int64Value(api.VM)
	m.Order = types.Int64Value(api.Order)
	b, err := json.Marshal(api.Attributes)
	if err != nil {
		diags.AddError("Failed to marshal device attributes", err.Error())
		return diags
	}
	m.Attributes = types.StringValue(string(b))
	return diags
}
