// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package kerberos_config

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// kerberosConfigResourceID is the fixed Terraform ID for this singleton
// resource: there is exactly one Kerberos configuration per TrueNAS system
// (kerberos.config always returns a single record), and it is never created
// or deleted on TrueNAS itself. The API's own numeric "id" (probed: always
// 1) is an internal implementation detail and is intentionally not surfaced
// in the model, mirroring the resilver_config/snmp_config singleton pattern.
const kerberosConfigResourceID = "kerberos_config"

// KerberosConfigModel is the Terraform state model for
// truenas_kerberos_config.
type KerberosConfigModel struct {
	ID             types.String `tfsdk:"id"` // fixed: "kerberos_config"
	AppdefaultsAux types.String `tfsdk:"appdefaults_aux"`
	LibdefaultsAux types.String `tfsdk:"libdefaults_aux"`
}

// KerberosConfigDataSourceModel is the read-only model for the
// truenas_kerberos_config datasource.
type KerberosConfigDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	AppdefaultsAux types.String `tfsdk:"appdefaults_aux"`
	LibdefaultsAux types.String `tfsdk:"libdefaults_aux"`
}

// kerberosConfigAPI mirrors the JSON object returned by kerberos.config and
// kerberos.update. Probed against a live TrueNAS 25.10 box
// (`core.get_methods` for kerberos.config/kerberos.update): the response is
// exactly {"id": <int>, "appdefaults_aux": <string>, "libdefaults_aux":
// <string>} — both aux fields are plain (non-nullable) strings, defaulting
// to "" on a stock box.
type kerberosConfigAPI struct {
	ID             int64  `json:"id"`
	AppdefaultsAux string `json:"appdefaults_aux"`
	LibdefaultsAux string `json:"libdefaults_aux"`
}

// responseToModel maps a kerberosConfigAPI response onto a
// KerberosConfigModel.
func responseToModel(_ context.Context, api *kerberosConfigAPI, m *KerberosConfigModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(kerberosConfigResourceID)
	m.AppdefaultsAux = types.StringValue(api.AppdefaultsAux)
	m.LibdefaultsAux = types.StringValue(api.LibdefaultsAux)

	return diags
}

// responseToDataSourceModel maps a kerberosConfigAPI response onto a
// KerberosConfigDataSourceModel.
func responseToDataSourceModel(_ context.Context, api *kerberosConfigAPI, m *KerberosConfigDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(kerberosConfigResourceID)
	m.AppdefaultsAux = types.StringValue(api.AppdefaultsAux)
	m.LibdefaultsAux = types.StringValue(api.LibdefaultsAux)

	return diags
}

// updatePayload builds the kerberos.update argument. Both fields are
// guarded: each is only included when known (Optional+Computed), so an
// unset optional is omitted entirely and the TrueNAS-side current value is
// left unchanged rather than overwritten with an explicit zero value.
func (m *KerberosConfigModel) updatePayload(_ context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{}

	if !m.AppdefaultsAux.IsNull() && !m.AppdefaultsAux.IsUnknown() {
		p["appdefaults_aux"] = m.AppdefaultsAux.ValueString()
	}
	if !m.LibdefaultsAux.IsNull() && !m.LibdefaultsAux.IsUnknown() {
		p["libdefaults_aux"] = m.LibdefaultsAux.ValueString()
	}

	return p, diags
}

// deleteWarningDiagnostics builds the warning diagnostic emitted by Delete.
// Delete makes NO client calls: the Kerberos configuration is system-wide
// directory-service configuration, so removing this resource from Terraform
// state must never reset the box's krb5.conf aux settings. It only emits a
// warning diagnostic; Terraform itself drops the resource from state once
// Delete returns. Splitting this into its own function keeps Delete's "no
// client calls" contract independently unit-testable.
func deleteWarningDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning(
		"Kerberos configuration left in place",
		"Kerberos configuration left in place; removed from Terraform state only",
	)
	return diags
}
