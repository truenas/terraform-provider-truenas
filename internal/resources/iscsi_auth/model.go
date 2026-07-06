package iscsi_auth

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ISCSIAuthModel is the Terraform state model for truenas_iscsi_auth.
type ISCSIAuthModel struct {
	ID            types.Int64  `tfsdk:"id"`
	Tag           types.Int64  `tfsdk:"tag"`  // group ID referenced by target groups' "auth"
	User          types.String `tfsdk:"user"`
	Secret        types.String `tfsdk:"secret"`        // Sensitive
	PeerUser      types.String `tfsdk:"peeruser"`      // mutual CHAP
	PeerSecret    types.String `tfsdk:"peersecret"`    // Sensitive
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
// Secret preservation: whether the API echoes secret/peersecret back in
// query/get_instance responses is unverified, and TrueNAS may mask secrets
// with an empty string. To avoid silently wiping a secret the user set, this
// function only overwrites Secret/PeerSecret with the API's value when the
// API returns a non-empty string. If the API returns an empty string and the
// model already holds a non-empty value, the model value is kept.
func responseToModel(api *iscsiAuthAPI, m *ISCSIAuthModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Tag = types.Int64Value(api.Tag)
	m.User = types.StringValue(api.User)
	m.PeerUser = types.StringValue(api.PeerUser)
	m.DiscoveryAuth = types.StringValue(api.DiscoveryAuth)

	if api.Secret != "" {
		m.Secret = types.StringValue(api.Secret)
	} else if m.Secret.IsNull() || m.Secret.IsUnknown() {
		m.Secret = types.StringValue("")
	}
	// else: API returned empty, model already holds a non-empty value: keep it.

	if api.PeerSecret != "" {
		m.PeerSecret = types.StringValue(api.PeerSecret)
	} else if m.PeerSecret.IsNull() || m.PeerSecret.IsUnknown() {
		m.PeerSecret = types.StringValue("")
	}
	// else: API returned empty, model already holds a non-empty value: keep it.

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
