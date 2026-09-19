// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_auth

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ISCSIAuthModel is the Terraform state model for truenas_iscsi_auth.
type ISCSIAuthModel struct {
	ID            types.Int64  `tfsdk:"id"`
	Tag           types.Int64  `tfsdk:"tag"` // group ID referenced by target groups' "auth"
	User          types.String `tfsdk:"user"`
	Secret        types.String `tfsdk:"secret"`         // Sensitive
	PeerUser      types.String `tfsdk:"peeruser"`       // mutual CHAP
	PeerSecret    types.String `tfsdk:"peersecret"`     // Sensitive
	DiscoveryAuth types.String `tfsdk:"discovery_auth"` // NONE, CHAP, CHAP_MUTUAL
}

// ISCSIAuthDataSourceModel is the read-only model for the truenas_iscsi_auth
// datasource. It intentionally has no "secret" or "peersecret" fields: CHAP
// credentials must never be exposed through a datasource read.
type ISCSIAuthDataSourceModel struct {
	ID            types.Int64  `tfsdk:"id"`
	Tag           types.Int64  `tfsdk:"tag"`
	User          types.String `tfsdk:"user"`
	PeerUser      types.String `tfsdk:"peeruser"`
	DiscoveryAuth types.String `tfsdk:"discovery_auth"`
}

// iscsiAuthAPI is the JSON wire format for a TrueNAS iSCSI CHAP auth entry.
type iscsiAuthAPI struct {
	ID            int64  `json:"id"`
	Tag           int64  `json:"tag"`
	User          string `json:"user"`
	Secret        string `json:"secret"`
	PeerUser      string `json:"peeruser"`
	PeerSecret    string `json:"peersecret"`
	DiscoveryAuth string `json:"discovery_auth"`
}

// responseToModel maps an API response onto a Terraform model.
//
// Secret/PeerSecret are write-only: they are never assigned here, and
// whatever value the caller already has in m.Secret/m.PeerSecret (from plan
// or prior state) is left untouched. peersecret is Optional and NOT
// Computed, so if it is rewritten to "" here whenever the user omits it (or
// the API doesn't echo it back), Terraform Core reports "Provider produced
// inconsistent result after apply" because the planned value was null but
// the applied value became a known empty string. Mirrors the convention in
// internal/resources/mail (Pass) and internal/resources/user (Password).
func responseToModel(api *iscsiAuthAPI, m *ISCSIAuthModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Tag = types.Int64Value(api.Tag)
	m.User = types.StringValue(api.User)
	m.PeerUser = types.StringValue(api.PeerUser)
	m.DiscoveryAuth = types.StringValue(api.DiscoveryAuth)

	// NOTE: Secret and PeerSecret are NOT set here (write-only, never read
	// back from the API).

	return diags
}

// responseToDataSourceModel maps an API response onto an
// ISCSIAuthDataSourceModel. Secret/PeerSecret are never populated.
func responseToDataSourceModel(api *iscsiAuthAPI, m *ISCSIAuthDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Tag = types.Int64Value(api.Tag)
	m.User = types.StringValue(api.User)
	m.PeerUser = types.StringValue(api.PeerUser)
	m.DiscoveryAuth = types.StringValue(api.DiscoveryAuth)

	return diags
}

// apiPayload builds the map[string]any payload for iscsi.auth.create /
// iscsi.auth.update. tag/user/secret are always included (Required).
// peeruser/discovery_auth are Optional+Computed, so they are only included
// when known. peersecret is Optional (not Computed) and is only included
// when known and non-empty, so an unset peersecret never overwrites an
// existing one on the server.
func (m *ISCSIAuthModel) apiPayload() map[string]any {
	p := map[string]any{
		"tag":    m.Tag.ValueInt64(),
		"user":   m.User.ValueString(),
		"secret": m.Secret.ValueString(),
	}

	if !m.PeerUser.IsNull() && !m.PeerUser.IsUnknown() {
		p["peeruser"] = m.PeerUser.ValueString()
	}
	if !m.DiscoveryAuth.IsNull() && !m.DiscoveryAuth.IsUnknown() {
		p["discovery_auth"] = m.DiscoveryAuth.ValueString()
	}
	if !m.PeerSecret.IsNull() && !m.PeerSecret.IsUnknown() {
		if v := m.PeerSecret.ValueString(); v != "" {
			p["peersecret"] = v
		}
	}

	return p
}
