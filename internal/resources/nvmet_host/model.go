// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_host

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// NVMetHostModel is the Terraform state model for truenas_nvmet_host.
type NVMetHostModel struct {
	ID            types.Int64  `tfsdk:"id"`
	HostNQN       types.String `tfsdk:"hostnqn"`         // Required
	Description   types.String `tfsdk:"description"`     // Optional+Computed+USFU
	DHChapKey     types.String `tfsdk:"dhchap_key"`      // Optional+Sensitive; write-only, never read back
	DHChapCtrlKey types.String `tfsdk:"dhchap_ctrl_key"` // Optional+Sensitive; write-only, never read back
	DHChapDHGroup types.String `tfsdk:"dhchap_dhgroup"`  // Optional+Computed+USFU; nullable in API
	DHChapHash    types.String `tfsdk:"dhchap_hash"`     // Optional+Computed+USFU; SHA-256|SHA-384|SHA-512

	DHChapKeyWOVersion     types.Int64 `tfsdk:"dhchap_key_wo_version"`
	DHChapCtrlKeyWOVersion types.Int64 `tfsdk:"dhchap_ctrl_key_wo_version"`
	DHChapKeySet           types.Bool  `tfsdk:"dhchap_key_set"`      // Computed: server has a key
	DHChapCtrlKeySet       types.Bool  `tfsdk:"dhchap_ctrl_key_set"` // Computed: server has a ctrl key
}

// NVMetHostDataSourceModel is the read-only model for the truenas_nvmet_host
// datasource. It intentionally has no "dhchap_key" or "dhchap_ctrl_key"
// fields: DH-CHAP secrets must never be exposed through a datasource read.
type NVMetHostDataSourceModel struct {
	ID            types.Int64  `tfsdk:"id"`
	HostNQN       types.String `tfsdk:"hostnqn"`
	Description   types.String `tfsdk:"description"`
	DHChapDHGroup types.String `tfsdk:"dhchap_dhgroup"`
	DHChapHash    types.String `tfsdk:"dhchap_hash"`
	DHChapKeySet  types.Bool   `tfsdk:"dhchap_key_set"`
	DHChapCtrlSet types.Bool   `tfsdk:"dhchap_ctrl_key_set"`
}

// nvmetHostAPI is the JSON wire format for a TrueNAS NVMe-oF host
// (initiator) object (nvmet.host.*). dhchap_key/dhchap_ctrl_key are decoded
// only to tell whether a key is set (dhchap_key_set/dhchap_ctrl_key_set);
// the values are write-only secrets and are never copied into a model
// (mirrors internal/resources/mail's Pass and internal/resources/iscsi_auth).
type nvmetHostAPI struct {
	ID            int64   `json:"id"`
	HostNQN       string  `json:"hostnqn"`
	Description   string  `json:"description"`
	DHChapKey     *string `json:"dhchap_key"`
	DHChapCtrlKey *string `json:"dhchap_ctrl_key"`
	DHChapDHGroup *string `json:"dhchap_dhgroup"`
	DHChapHash    string  `json:"dhchap_hash"`
}

// keySet reports whether an API key field holds a key (26.0 returns null
// for none, 28.0 "").
func keySet(k *string) bool { return k != nil && *k != "" }

// responseToModel maps an API response onto a Terraform model.
//
// DHChapKey/DHChapCtrlKey are write-only: they are never assigned here, and
// whatever value the caller already has in m.DHChapKey/m.DHChapCtrlKey (from
// plan or prior state) is left untouched. Both are Optional and NOT
// Computed, so rewriting them to "" here whenever the user omits them (or
// the API doesn't echo them back) would cause Terraform Core to report
// "Provider produced inconsistent result after apply" because the planned
// value was null but the applied value became a known empty string. Mirrors
// the convention in internal/resources/iscsi_auth (Secret/PeerSecret).
func responseToModel(_ context.Context, api *nvmetHostAPI, m *NVMetHostModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.HostNQN = types.StringValue(api.HostNQN)
	m.Description = types.StringValue(api.Description)
	m.DHChapHash = types.StringValue(api.DHChapHash)
	m.DHChapKeySet = types.BoolValue(keySet(api.DHChapKey))
	m.DHChapCtrlKeySet = types.BoolValue(keySet(api.DHChapCtrlKey))

	if api.DHChapDHGroup != nil {
		m.DHChapDHGroup = types.StringValue(*api.DHChapDHGroup)
	} else {
		m.DHChapDHGroup = types.StringValue("")
	}

	// NOTE: DHChapKey and DHChapCtrlKey are NOT set here (write-only, never
	// read back from the API).

	return diags
}

// responseToDataSourceModel maps an API response onto an
// NVMetHostDataSourceModel. dhchap_key/dhchap_ctrl_key are never populated
// (the datasource model has no such fields at all).
func responseToDataSourceModel(_ context.Context, api *nvmetHostAPI, m *NVMetHostDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.HostNQN = types.StringValue(api.HostNQN)
	m.Description = types.StringValue(api.Description)
	m.DHChapHash = types.StringValue(api.DHChapHash)
	m.DHChapKeySet = types.BoolValue(keySet(api.DHChapKey))
	m.DHChapCtrlSet = types.BoolValue(keySet(api.DHChapCtrlKey))

	if api.DHChapDHGroup != nil {
		m.DHChapDHGroup = types.StringValue(*api.DHChapDHGroup)
	} else {
		m.DHChapDHGroup = types.StringValue("")
	}

	return diags
}

// guardedFields builds the set of optional payload fields shared by create
// and update: description, dhchap_dhgroup, dhchap_hash.
// description/dhchap_hash are included when known. dhchap_dhgroup is
// included when known; "" clears it (sent as null).
func (m *NVMetHostModel) guardedFields() map[string]any {
	p := map[string]any{}

	if !m.Description.IsNull() && !m.Description.IsUnknown() {
		p["description"] = m.Description.ValueString()
	}
	if !m.DHChapDHGroup.IsNull() && !m.DHChapDHGroup.IsUnknown() {
		if v := m.DHChapDHGroup.ValueString(); v != "" {
			p["dhchap_dhgroup"] = v
		} else {
			p["dhchap_dhgroup"] = nil
		}
	}
	if !m.DHChapHash.IsNull() && !m.DHChapHash.IsUnknown() {
		p["dhchap_hash"] = m.DHChapHash.ValueString()
	}

	return p
}

// configKey returns the write-only key from config, or "" when unset.
func configKey(v types.String) string {
	if v.IsNull() || v.IsUnknown() {
		return ""
	}
	return v.ValueString()
}

// createPayload builds the map[string]any payload for nvmet.host.create.
// "hostnqn" is always included (Required); the keys are included only when
// set in config, so the server default (no key) applies otherwise.
func (m *NVMetHostModel) createPayload(_ context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	payload := m.guardedFields()
	payload["hostnqn"] = m.HostNQN.ValueString()
	if k := configKey(m.DHChapKey); k != "" {
		payload["dhchap_key"] = k
	}
	if k := configKey(m.DHChapCtrlKey); k != "" {
		payload["dhchap_ctrl_key"] = k
	}

	return payload, diags
}

// updatePayload builds the map[string]any payload for nvmet.host.update.
// hostnqn is updatable per the API and is Required (always known), so it is
// always included, same as createPayload. The keys always go out: the
// config value, or null when config has none, so removing a key from config
// turns DH-CHAP off instead of leaving the old key active. Update then drops
// a configured key that was not rotated (omitUnrotatedKeys). (An update only
// runs when the plan changes; dhchap_key_set and the *_wo_version triggers
// are what make adding, removing or rotating a key a plan change.)
func (m *NVMetHostModel) updatePayload(_ context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	payload := m.guardedFields()
	payload["hostnqn"] = m.HostNQN.ValueString()
	for name, v := range map[string]types.String{"dhchap_key": m.DHChapKey, "dhchap_ctrl_key": m.DHChapCtrlKey} {
		if k := configKey(v); k != "" {
			payload[name] = k
		} else {
			payload[name] = nil
		}
	}

	return payload, diags
}

// omitUnrotatedKeys removes a DH-CHAP key from an update payload when the
// update has no reason to send it: the key is configured, the server already
// has a key (prior dhchap_*_set), and its *_wo_version did not change. Each
// re-send would replace the key with the configured value, so a key changed
// in config without a version bump is not applied, matching the other
// write-only secrets. (B-141) A configured key the server does not have yet
// still goes out (adding a key), and a key removed from config still goes
// out as null (turning DH-CHAP off).
func omitUnrotatedKeys(payload map[string]any, plan, prior NVMetHostModel) {
	keys := []struct {
		name        string
		cfg         types.String
		ver, priorV types.Int64
		priorSet    types.Bool
	}{
		{"dhchap_key", plan.DHChapKey, plan.DHChapKeyWOVersion, prior.DHChapKeyWOVersion, prior.DHChapKeySet},
		{"dhchap_ctrl_key", plan.DHChapCtrlKey, plan.DHChapCtrlKeyWOVersion, prior.DHChapCtrlKeyWOVersion, prior.DHChapCtrlKeySet},
	}
	for _, k := range keys {
		if configKey(k.cfg) != "" && k.priorSet.ValueBool() && k.ver.Equal(k.priorV) {
			delete(payload, k.name)
		}
	}
}
