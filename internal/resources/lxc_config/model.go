package lxc_config

import (
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// lxcConfigResourceID is the fixed Terraform ID for this singleton resource:
// there is exactly one LXC configuration per TrueNAS system (lxc.config
// always returns a single record), and it is never created or deleted on
// TrueNAS itself. The API's own numeric "id" (probed live on SCALE 26.0:
// always 1) is an internal implementation detail and is intentionally not
// surfaced in the model, mirroring the docker_config/catalog_config
// singleton pattern.
const lxcConfigResourceID = "lxc_config"

// lxcConfigVersionFloorMajor/Minor is the minimum TrueNAS SCALE release that
// exposes the lxc.config/lxc.update namespace at all. Probed live: SCALE
// 25.10 returns a JSON-RPC "Method does not exist" error for lxc.config;
// SCALE 26.0 supports it in full (lxc.config, lxc.update, lxc.bridge_choices,
// all job:false). See resource.go/datasource.go for where this gates every
// Create/Read/Update/datasource-Read before any lxc.* call reaches the wire.
const (
	lxcConfigVersionFloorMajor = 26
	lxcConfigVersionFloorMinor = 0
)

// versionGateDiagnostics reports whether the given TrueNAS release string
// (as returned by system.version_short via client.ServerVersion, e.g.
// "25.10.3.1" or "26.0.0") is at or above the SCALE 26.0 floor
// lxc.config/lxc.update require, returning a single clean error diagnostic
// when it is not (instead of letting a raw "Method does not exist" JSON-RPC
// error reach the practitioner). It is a pure function of the already-probed
// version string — no live client access — built on the same dotted
// major.minor comparison client.VersionAtLeast itself uses (via the exported
// client.VersionAtLeastString), so it is independently unit-testable without
// a TrueNAS connection.
func versionGateDiagnostics(version string) diag.Diagnostics {
	var diags diag.Diagnostics
	if !client.VersionAtLeastString(version, lxcConfigVersionFloorMajor, lxcConfigVersionFloorMinor) {
		diags.AddError(
			"TrueNAS SCALE version too old",
			"truenas_lxc_config requires TrueNAS SCALE 26.0 or later",
		)
	}
	return diags
}

// LXCConfigModel is the Terraform state model for truenas_lxc_config.
//
// PreferredPool and Bridge are nullable strings following the three-way
// convention established by docker_config's "pool" field: unknown/omitted
// leaves the current TrueNAS-side value unchanged, an explicit null clears
// it, and a known value sets it. V4Network and V6Network are plain
// Optional+Computed strings: probed live, lxc.config always returns a real
// (non-null) CIDR string for both, so there is no null case to model for
// them.
type LXCConfigModel struct {
	ID            types.String `tfsdk:"id"`
	PreferredPool types.String `tfsdk:"preferred_pool"` // nullable, three-way
	Bridge        types.String `tfsdk:"bridge"`         // nullable, three-way
	V4Network     types.String `tfsdk:"v4_network"`
	V6Network     types.String `tfsdk:"v6_network"`
}

// LXCConfigDataSourceModel is the read-only model for the
// truenas_lxc_config datasource.
type LXCConfigDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	PreferredPool types.String `tfsdk:"preferred_pool"`
	Bridge        types.String `tfsdk:"bridge"`
	V4Network     types.String `tfsdk:"v4_network"`
	V6Network     types.String `tfsdk:"v6_network"`
}

// lxcConfigAPI mirrors the JSON object returned by lxc.config and
// lxc.update. Probed live against SCALE 26.0 (the only release that exposes
// this namespace — see versionGateDiagnostics):
//
//	{"id": 1, "preferred_pool": null, "bridge": null,
//	 "v4_network": "172.200.0.0/24", "v6_network": "fd42:4c58:43ae::/64"}
type lxcConfigAPI struct {
	ID            int64   `json:"id"`
	PreferredPool *string `json:"preferred_pool"`
	Bridge        *string `json:"bridge"`
	V4Network     string  `json:"v4_network"`
	V6Network     string  `json:"v6_network"`
}

// responseToModel maps an lxcConfigAPI response onto an LXCConfigModel.
func responseToModel(api *lxcConfigAPI, m *LXCConfigModel) {
	m.ID = types.StringValue(lxcConfigResourceID)
	m.PreferredPool = types.StringPointerValue(api.PreferredPool)
	m.Bridge = types.StringPointerValue(api.Bridge)
	m.V4Network = types.StringValue(api.V4Network)
	m.V6Network = types.StringValue(api.V6Network)
}

// responseToDataSourceModel maps an lxcConfigAPI response onto an
// LXCConfigDataSourceModel.
func responseToDataSourceModel(api *lxcConfigAPI, m *LXCConfigDataSourceModel) {
	m.ID = types.StringValue(lxcConfigResourceID)
	m.PreferredPool = types.StringPointerValue(api.PreferredPool)
	m.Bridge = types.StringPointerValue(api.Bridge)
	m.V4Network = types.StringValue(api.V4Network)
	m.V6Network = types.StringValue(api.V6Network)
}

// updatePayload builds the lxc.update argument.
//
// preferred_pool and bridge use the three-way guard: omitted entirely when
// null/unknown (leaving the current TrueNAS-side value unchanged), sent as
// JSON nil when the model holds an explicit null (clearing the value), and
// sent as their string value otherwise. v4_network and v6_network use a
// plain guard (omitted when null/unknown, sent as-is otherwise) since
// lxc.config never returns null for them.
func (m *LXCConfigModel) updatePayload() map[string]any {
	p := map[string]any{}

	if !m.PreferredPool.IsUnknown() {
		if m.PreferredPool.IsNull() {
			p["preferred_pool"] = nil
		} else {
			p["preferred_pool"] = m.PreferredPool.ValueString()
		}
	}
	if !m.Bridge.IsUnknown() {
		if m.Bridge.IsNull() {
			p["bridge"] = nil
		} else {
			p["bridge"] = m.Bridge.ValueString()
		}
	}
	if !m.V4Network.IsNull() && !m.V4Network.IsUnknown() {
		p["v4_network"] = m.V4Network.ValueString()
	}
	if !m.V6Network.IsNull() && !m.V6Network.IsUnknown() {
		p["v6_network"] = m.V6Network.ValueString()
	}

	return p
}

// deleteWarningDiagnostics builds the warning diagnostic emitted by Delete.
// Delete makes NO client calls: the LXC pool/bridge/network configuration
// underpins any running LXC-backed instances, so removing this resource
// from Terraform state must never reset or reconfigure the box's LXC setup.
// Splitting this into its own function keeps Delete's "no client calls"
// contract independently unit-testable.
func deleteWarningDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"LXC configuration left in place",
		"LXC configuration left in place; removed from Terraform state only",
	)
	return diags
}
