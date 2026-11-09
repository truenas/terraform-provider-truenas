// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_device

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
)

// VMDeviceModel is the Terraform state model for truenas_vm_device.
type VMDeviceModel struct {
	ID         types.Int64  `tfsdk:"id"`
	VM         types.Int64  `tfsdk:"vm"`         // parent VM id, RequiresReplace
	Attributes types.String `tfsdk:"attributes"` // JSON doc incl. "dtype" key
	Order      types.Int64  `tfsdk:"order"`      // boot order; Optional+Computed
	// AttributesSecretsWO is a write-only JSON object merged over attributes when
	// sending to the API (e.g. a DISPLAY device's password), never stored in
	// state; AttributesSecretsWOVersion triggers re-send. (#36/#37 class)
	AttributesSecretsWO        types.String `tfsdk:"attributes_secrets_wo"`
	AttributesSecretsWOVersion types.Int64  `tfsdk:"attributes_secrets_wo_version"`
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

// secretAttributeKeys are the device attributes TrueNAS treats as secrets
// (Secret[str] in the API): the DISPLAY device's password is the only one in
// every device type on 25.10, 26.0 and 28.0. They belong in the write-only
// attributes_secrets_wo, never in attributes: the configuration is rejected
// at plan time when attributes holds one (validateNoSecretAttributes), and
// every read drops them, so attributes in state, plans and generated
// configuration never carries a secret. (B-159)
var secretAttributeKeys = []string{"password"}

// dropSecretAttributes removes the secret keys from a device attributes map
// read from the API.
func dropSecretAttributes(attrs map[string]any) {
	for _, k := range secretAttributeKeys {
		delete(attrs, k)
	}
}

// redactedValue is what vm.device.query/get_instance return in place of a
// secret attribute (the DISPLAY password, Secret[str] in the API) for a
// session without write access, e.g. a READONLY_ADMIN API key. (#57 class)
const redactedValue = "********"

// scrubRedacted replaces each redacted value in apiAttrs with the value known
// for that key (prior state, or the configuration), or drops the key when
// none is known, so the mask is never compared with or stored as real data.
// known may be nil. (BUGS B-033)
func scrubRedacted(apiAttrs, known map[string]any) {
	for k, v := range apiAttrs {
		if v != redactedValue {
			continue
		}
		if kv, ok := known[k]; ok {
			apiAttrs[k] = kv
		} else {
			delete(apiAttrs, k)
		}
	}
}

// attributesDrifted reports whether any key present in stateAttrs has a
// different value in apiAttrs. API-added default keys are ignored (not
// drift), at every nesting level, since the API may normalize/expand
// attributes with defaults. A NIC "mac" compares as a MAC address: the server
// stores it in lower case (BUGS B-082, B-083).
func attributesDrifted(stateAttrs, apiAttrs map[string]any) bool {
	return !normalize.EqualTreeFunc(stateAttrs, apiAttrs, true, attributeLeafEqual)
}

// attributeLeafEqual compares the attribute values the server normalizes.
var attributeLeafEqual = normalize.KeyEqual(map[string]func(a, b string) bool{"mac": normalize.EqualMAC})

// responseToModel maps deviceAPI onto VMDeviceModel. ID, VM, and Order are
// always taken from the API; Attributes is left untouched by this function
// so callers can implement write-what-you-said / drift-aware semantics.
func responseToModel(api *deviceAPI, m *VMDeviceModel) {
	m.ID = types.Int64Value(api.ID)
	m.VM = types.Int64Value(api.VM)
	m.Order = types.Int64Value(api.Order)
}

// apiAttributesJSON marshals api.Attributes to a canonical JSON string.
// Secret keys are dropped (see secretAttributeKeys).
func apiAttributesJSON(api *deviceAPI) (string, diag.Diagnostics) {
	var diags diag.Diagnostics
	dropSecretAttributes(api.Attributes)
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
	dropNullDisplayPorts(attrs)
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
	// A redacted value is not data: report it as null (BUGS B-033). Secret
	// keys are reported as null whatever the server returned, so the data
	// source never puts a secret into state. (B-159)
	for k, v := range api.Attributes {
		if v == redactedValue {
			api.Attributes[k] = nil
		}
	}
	for _, k := range secretAttributeKeys {
		if _, ok := api.Attributes[k]; ok {
			api.Attributes[k] = nil
		}
	}
	b, err := json.Marshal(api.Attributes)
	if err != nil {
		diags.AddError("Failed to marshal device attributes", err.Error())
		return diags
	}
	m.Attributes = types.StringValue(string(b))
	return diags
}

// portWizard is the slice of the client autoAssignDisplayPorts needs.
type portWizard interface {
	CallRead(ctx context.Context, method string, params ...any) (json.RawMessage, error)
}

// autoAssignDisplayPorts fills a DISPLAY device's port and web_port from
// vm.port_wizard when the configuration leaves both unset or null. The API
// documents null as "auto-assign", but vm.device.create validates the device
// (port must differ from web_port) before it fills the nulls, so two nulls
// fail with "Spice server port must not be same as web port" (usage log #4,
// middleware stable/27). The UI avoids it by calling vm.port_wizard itself;
// so does this. A single explicit port is left alone for the server to pair.
func autoAssignDisplayPorts(ctx context.Context, c portWizard, attrs map[string]any) error {
	if dtype, _ := attrs["dtype"].(string); dtype != "DISPLAY" {
		return nil
	}
	if attrs["port"] != nil || attrs["web_port"] != nil {
		return nil
	}
	raw, err := c.CallRead(ctx, "vm.port_wizard")
	if err != nil {
		return fmt.Errorf("vm.port_wizard: %w", err)
	}
	var ports struct {
		Port int64 `json:"port"`
		Web  int64 `json:"web"`
	}
	if err := json.Unmarshal(raw, &ports); err != nil {
		return fmt.Errorf("parse vm.port_wizard: %w", err)
	}
	attrs["port"] = ports.Port
	attrs["web_port"] = ports.Web
	return nil
}

// dropNullDisplayPorts removes a DISPLAY device's null port / web_port from an
// update: null means "keep the auto-assigned port", and vm.device.update
// merges the attributes it gets into the stored ones, while sending the nulls
// fails the same way create did (usage log #4).
func dropNullDisplayPorts(attrs map[string]any) {
	if dtype, _ := attrs["dtype"].(string); dtype != "DISPLAY" {
		return
	}
	for _, k := range []string{"port", "web_port"} {
		if v, present := attrs[k]; present && v == nil {
			delete(attrs, k)
		}
	}
}

// withoutNullKeys returns a copy of the user's attributes without the keys
// whose value is null. A null in the configuration means "the server
// decides" (a DISPLAY device's auto-assigned port / web_port), so the value
// the server chose is not drift.
func withoutNullKeys(attrs map[string]any) map[string]any {
	out := make(map[string]any, len(attrs))
	for k, v := range attrs {
		if v != nil {
			out[k] = v
		}
	}
	return out
}
