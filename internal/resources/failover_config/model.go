// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package failover_config

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// failoverConfigResourceID is the fixed Terraform ID for this singleton
// resource: there is exactly one failover configuration per TrueNAS system
// (failover.config always returns a single record, even on a box that
// isn't licensed for Enterprise HA — probed live), and it is never created
// or deleted on TrueNAS itself. The API's own numeric "id" (probed live on
// both TrueNAS 25.10.4 HA and 26.0: always 1) is an internal implementation
// detail and is intentionally not surfaced in the model, mirroring the
// tn_connect_config/webshare_config singleton pattern.
const failoverConfigResourceID = "failover_config"

// failoverConfigAPI mirrors the JSON object returned by failover.config and
// failover.update. Probed live:
//
//   - TrueNAS 25.10.4 Enterprise HA (wss://10.220.16.188, licensed=true,
//     status=MASTER, node=B): {"id": 1, "disabled": false, "master": true,
//     "timeout": 0}.
//   - TrueNAS 26.0 (wss://192.168.1.68, licensed=false, status=SINGLE,
//     node=MANUAL): {"id": 1, "disabled": false, "master": false,
//     "timeout": 0} — failover.config reads cleanly even when the box
//     isn't HA-licensed.
//
// Confirmed via failover.update's own accepts schema (core.get_methods,
// probed on the 26.0 box — the method is present there but was absent
// from core.get_methods' listing on the 25.10.4 HA box despite being
// directly callable there too, a release-specific introspection quirk,
// not a capability difference): all three fields are optional booleans/
// integer on the "data" payload object, and all three are always present
// (never null/absent) on the response on both releases — unlike
// tn_connect_config, there is no release-specific field shape to handle
// here.
// KNOWN QUIRK, probed live during the Plan 20 Task 1 real controlled
// failover exercise (see .superpowers/sdd/task-1-report.md for the full
// transcript): after failover.become_passive was called on the (then-)
// master node B, the newly-active node A's failover.config kept reporting
// "master": false even once the event had fully stabilized and
// failover.disabled.reasons had cleared back to [] — while failover.status
// on that same connection simultaneously read "MASTER". So "master" is NOT
// a reliable live "am I active" indicator immediately following a
// failover; failover.status/failover.node are. This is why the
// truenas_failover_config datasource surfaces "status"/"node" as separate,
// independently-fetched fields rather than deriving them from "master".
type failoverConfigAPI struct {
	ID       int64 `json:"id"`
	Disabled bool  `json:"disabled"`
	Master   bool  `json:"master"`
	Timeout  int64 `json:"timeout"`
}

// FailoverConfigModel is the Terraform state model for
// truenas_failover_config. All three fields are Optional+Computed
// (user-writable) — see updatePayload's doc comment for the safety
// contract governing when each is actually sent to failover.update, and
// schema.go's Description strings for the safety notes on "disabled" and
// "master" specifically.
type FailoverConfigModel struct {
	ID       types.String `tfsdk:"id"`
	Disabled types.Bool   `tfsdk:"disabled"`
	Master   types.Bool   `tfsdk:"master"`
	Timeout  types.Int64  `tfsdk:"timeout"`
}

// FailoverConfigDataSourceModel is the read-only model for the
// truenas_failover_config datasource. Beyond the resource's three
// writable fields it additionally exposes "status" (failover.status),
// "node" (failover.node), and "disabled_reasons" (failover.disabled.reasons)
// — live HA operational state that has no meaningful "desired value" a
// resource could manage, so per the design doc it is datasource-only.
type FailoverConfigDataSourceModel struct {
	ID              types.String `tfsdk:"id"`
	Disabled        types.Bool   `tfsdk:"disabled"`
	Master          types.Bool   `tfsdk:"master"`
	Timeout         types.Int64  `tfsdk:"timeout"`
	Status          types.String `tfsdk:"status"`
	Node            types.String `tfsdk:"node"`
	DisabledReasons types.List   `tfsdk:"disabled_reasons"`
}

// nonNilStrings returns s unchanged, or an empty (non-nil) slice, so
// types.ListValueFrom always produces a known empty list rather than a
// null one when a present-but-empty JSON array decodes to a nil Go slice.
// Mirrors the tn_connect_config/webshare_config helper of the same name.
// failover.disabled.reasons was observed live returning "[]" (a present,
// empty array, never a null key) on both probed releases, so unlike
// tn_connect_config's release-specific fields, this list is never null in
// the datasource model — only ever known, possibly empty.
func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// responseToModel maps a failoverConfigAPI response onto a
// FailoverConfigModel.
func responseToModel(api *failoverConfigAPI, m *FailoverConfigModel) {
	m.ID = types.StringValue(failoverConfigResourceID)
	m.Disabled = types.BoolValue(api.Disabled)
	m.Master = types.BoolValue(api.Master)
	m.Timeout = types.Int64Value(api.Timeout)
}

// responseToDataSourceModel maps a failoverConfigAPI response plus the
// separately-fetched status/node/disabled_reasons values onto a
// FailoverConfigDataSourceModel.
func responseToDataSourceModel(
	ctx context.Context,
	api *failoverConfigAPI,
	status, node string,
	disabledReasons []string,
	m *FailoverConfigDataSourceModel,
) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(failoverConfigResourceID)
	m.Disabled = types.BoolValue(api.Disabled)
	m.Master = types.BoolValue(api.Master)
	m.Timeout = types.Int64Value(api.Timeout)
	m.Status = types.StringValue(status)
	m.Node = types.StringValue(node)

	reasons, d := types.ListValueFrom(ctx, types.StringType, nonNilStrings(disabledReasons))
	diags.Append(d...)
	m.DisabledReasons = reasons

	return diags
}

// updatePayload builds the map[string]any payload for failover.update.
//
// SAFETY-CRITICAL, config-driven design (twofactor_auth/webshare_config/
// tn_connect_config precedent): a key is included ONLY when the user
// explicitly configured it in HCL (i.e. the model field is non-null and
// non-unknown) — never as a side effect of the resource's own defaults.
// This matters more here than in most singleton resources this provider
// manages: "disabled" governs whether HA is administratively disabled at
// all, and "master" directly asserts which node in the chassis is the
// active one — probed live (see the 25.10.4 HA box's failover.update
// accepts schema and model.go's doc comment), sending a "master" value
// that DIFFERS from the box's current state is a real failover trigger,
// not a cosmetic toggle. Silently resending either field on every Update
// (e.g. by sourcing from req.Plan instead of req.Config, which would echo
// the prior state's value back via each field's UseStateForUnknown plan
// modifier even when the user never set it in HCL) would risk
// unintentionally re-asserting — or worse, on some future release,
// changing — live HA state on every single Terraform run. "timeout" alone
// is the field this provider's own committed acceptance tests ever
// exercise (Tier 2 set-and-restore, HACheck+DisruptiveCheck gated); this
// provider's own tests never set "disabled" or "master" to a value that
// differs from the box's live state — see acceptance_test.go's
// TestAccFailoverConfig_setAndRestore for that safety contract's
// enforcement.
//
// CALLERS MUST invoke this on a model populated from req.Config
// (req.Config.Get), never from req.Plan — see the paragraph above.
func (m *FailoverConfigModel) updatePayload() map[string]any {
	p := map[string]any{}
	if !m.Disabled.IsNull() && !m.Disabled.IsUnknown() {
		p["disabled"] = m.Disabled.ValueBool()
	}
	if !m.Master.IsNull() && !m.Master.IsUnknown() {
		p["master"] = m.Master.ValueBool()
	}
	if !m.Timeout.IsNull() && !m.Timeout.IsUnknown() {
		p["timeout"] = m.Timeout.ValueInt64()
	}
	return p
}

// deleteWarningDiagnostics builds the warning diagnostic emitted by
// Delete. Delete makes NO client calls: failover.config governs whether
// this HA system is administratively disabled and which node is master,
// so removing this resource from Terraform state must NEVER change either
// — there is no sane "delete" semantics for live HA state. Mirrors the
// tn_connect_config/webshare_config precedent exactly.
func deleteWarningDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"Failover configuration left in place",
		"Failover configuration left in place; removed from Terraform state only",
	)
	return diags
}
