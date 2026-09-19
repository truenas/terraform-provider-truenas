// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package kerberos_realm

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// KerberosRealmModel is the Terraform state/plan model for
// truenas_kerberos_realm.
type KerberosRealmModel struct {
	ID            types.Int64  `tfsdk:"id"`
	Realm         types.String `tfsdk:"realm"`
	PrimaryKDC    types.String `tfsdk:"primary_kdc"`
	KDC           types.List   `tfsdk:"kdc"`            // List[String]
	AdminServer   types.List   `tfsdk:"admin_server"`   // List[String]
	KpasswdServer types.List   `tfsdk:"kpasswd_server"` // List[String]
}

// KerberosRealmDataSourceModel is the read-only model for the
// truenas_kerberos_realm datasource, looked up by "realm".
type KerberosRealmDataSourceModel struct {
	ID            types.Int64  `tfsdk:"id"`
	Realm         types.String `tfsdk:"realm"`
	PrimaryKDC    types.String `tfsdk:"primary_kdc"`
	KDC           types.List   `tfsdk:"kdc"`
	AdminServer   types.List   `tfsdk:"admin_server"`
	KpasswdServer types.List   `tfsdk:"kpasswd_server"`
}

// kerberosRealmAPI mirrors the JSON object returned by
// kerberos.realm.create, kerberos.realm.update, kerberos.realm.get_instance,
// and kerberos.realm.query. Probed against a live TrueNAS 25.10 box
// (`core.get_methods` for kerberos.realm.create/update/get_instance/query,
// plus a throwaway kerberos.realm.create/delete round trip): kdc,
// admin_server, and kpasswd_server default to [] (never null) on read;
// primary_kdc is nullable (anyOf string|null, default null).
type kerberosRealmAPI struct {
	ID            int64    `json:"id"`
	Realm         string   `json:"realm"`
	PrimaryKDC    *string  `json:"primary_kdc"`
	KDC           []string `json:"kdc"`
	AdminServer   []string `json:"admin_server"`
	KpasswdServer []string `json:"kpasswd_server"`
}

// nullableStringValue converts a nullable API string pointer to a
// types.String, preserving true null rather than substituting "".
func nullableStringValue(v *string) types.String {
	if v == nil {
		return types.StringNull()
	}
	return types.StringValue(*v)
}

// stringListValue builds a types.List of String from a Go slice,
// normalizing a nil slice to an empty (non-null) list.
func stringListValue(ctx context.Context, ss []string) (types.List, diag.Diagnostics) {
	if ss == nil {
		ss = []string{}
	}
	return types.ListValueFrom(ctx, types.StringType, ss)
}

// responseToModel maps a kerberosRealmAPI response onto a
// KerberosRealmModel.
func responseToModel(ctx context.Context, api *kerberosRealmAPI, m *KerberosRealmModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Realm = types.StringValue(api.Realm)
	m.PrimaryKDC = nullableStringValue(api.PrimaryKDC)

	kdc, d := stringListValue(ctx, api.KDC)
	diags.Append(d...)
	m.KDC = kdc

	adminServer, d := stringListValue(ctx, api.AdminServer)
	diags.Append(d...)
	m.AdminServer = adminServer

	kpasswdServer, d := stringListValue(ctx, api.KpasswdServer)
	diags.Append(d...)
	m.KpasswdServer = kpasswdServer

	return diags
}

// responseToDataSourceModel maps a kerberosRealmAPI response onto a
// KerberosRealmDataSourceModel.
func responseToDataSourceModel(ctx context.Context, api *kerberosRealmAPI, m *KerberosRealmDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Realm = types.StringValue(api.Realm)
	m.PrimaryKDC = nullableStringValue(api.PrimaryKDC)

	kdc, d := stringListValue(ctx, api.KDC)
	diags.Append(d...)
	m.KDC = kdc

	adminServer, d := stringListValue(ctx, api.AdminServer)
	diags.Append(d...)
	m.AdminServer = adminServer

	kpasswdServer, d := stringListValue(ctx, api.KpasswdServer)
	diags.Append(d...)
	m.KpasswdServer = kpasswdServer

	return diags
}

// apiPayload builds the map expected by kerberos.realm.create/
// kerberos.realm.update. "realm" is always included (Required). primary_kdc,
// kdc, admin_server, and kpasswd_server are Optional+Computed: each is
// included only when known and non-null, so an unset optional is omitted
// entirely and the TrueNAS-side current value/default is left unchanged
// rather than overwritten with an explicit zero value. A user-supplied
// empty list (kdc = []) is known and non-null, so it IS sent — letting the
// user explicitly clear a previously-set list back to DNS-lookup defaults.
func (m *KerberosRealmModel) apiPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{
		"realm": m.Realm.ValueString(),
	}

	if !m.PrimaryKDC.IsNull() && !m.PrimaryKDC.IsUnknown() {
		p["primary_kdc"] = m.PrimaryKDC.ValueString()
	}
	if !m.KDC.IsNull() && !m.KDC.IsUnknown() {
		var v []string
		diags.Append(m.KDC.ElementsAs(ctx, &v, false)...)
		if v == nil {
			v = []string{}
		}
		p["kdc"] = v
	}
	if !m.AdminServer.IsNull() && !m.AdminServer.IsUnknown() {
		var v []string
		diags.Append(m.AdminServer.ElementsAs(ctx, &v, false)...)
		if v == nil {
			v = []string{}
		}
		p["admin_server"] = v
	}
	if !m.KpasswdServer.IsNull() && !m.KpasswdServer.IsUnknown() {
		var v []string
		diags.Append(m.KpasswdServer.ElementsAs(ctx, &v, false)...)
		if v == nil {
			v = []string{}
		}
		p["kpasswd_server"] = v
	}

	return p, diags
}
