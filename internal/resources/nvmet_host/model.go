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
}

// nvmetHostAPI is the JSON wire format for a TrueNAS NVMe-oF host
// (initiator) object (nvmet.host.*). dhchap_key/dhchap_ctrl_key are
// intentionally NOT decoded here: they are write-only secrets and the API's
// get_instance/query responses are never used to populate them (mirrors
// internal/resources/mail's Pass and internal/resources/ups_config's
// pattern, and the fixed internal/resources/iscsi_auth).
type nvmetHostAPI struct {
	ID            int64   `json:"id"`
	HostNQN       string  `json:"hostnqn"`
	Description   string  `json:"description"`
	DHChapDHGroup *string `json:"dhchap_dhgroup"`
	DHChapHash    string  `json:"dhchap_hash"`
}

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

	if api.DHChapDHGroup != nil {
		m.DHChapDHGroup = types.StringValue(*api.DHChapDHGroup)
	} else {
		m.DHChapDHGroup = types.StringValue("")
	}

	return diags
}

// guardedFields builds the set of optional payload fields shared by create
// and update: description, dhchap_key, dhchap_ctrl_key, dhchap_dhgroup,
// dhchap_hash. description/dhchap_hash are included when known.
// dhchap_dhgroup is included only when known and non-empty. dhchap_key and
// dhchap_ctrl_key are included only when known and non-empty (write-only
// secrets: an unset/empty value must never overwrite an existing secret on
// the server).
func (m *NVMetHostModel) guardedFields() map[string]any {
	p := map[string]any{}

	if !m.Description.IsNull() && !m.Description.IsUnknown() {
		p["description"] = m.Description.ValueString()
	}
	if !m.DHChapDHGroup.IsNull() && !m.DHChapDHGroup.IsUnknown() && m.DHChapDHGroup.ValueString() != "" {
		p["dhchap_dhgroup"] = m.DHChapDHGroup.ValueString()
	}
	if !m.DHChapHash.IsNull() && !m.DHChapHash.IsUnknown() {
		p["dhchap_hash"] = m.DHChapHash.ValueString()
	}
	if !m.DHChapKey.IsNull() && !m.DHChapKey.IsUnknown() {
		if v := m.DHChapKey.ValueString(); v != "" {
			p["dhchap_key"] = v
		}
	}
	if !m.DHChapCtrlKey.IsNull() && !m.DHChapCtrlKey.IsUnknown() {
		if v := m.DHChapCtrlKey.ValueString(); v != "" {
			p["dhchap_ctrl_key"] = v
		}
	}

	return p
}

// createPayload builds the map[string]any payload for nvmet.host.create.
// "hostnqn" is always included (Required); the remaining fields are guarded
// (see guardedFields).
func (m *NVMetHostModel) createPayload(_ context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	payload := m.guardedFields()
	payload["hostnqn"] = m.HostNQN.ValueString()

	return payload, diags
}

// updatePayload builds the map[string]any payload for nvmet.host.update.
// hostnqn is updatable per the API and is Required (always known), so it is
// always included, same as createPayload.
func (m *NVMetHostModel) updatePayload(_ context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	payload := m.guardedFields()
	payload["hostnqn"] = m.HostNQN.ValueString()

	return payload, diags
}
