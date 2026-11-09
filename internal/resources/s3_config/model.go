// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

// Package s3_config manages the TrueNAS S3 service configuration (s3.config /
// s3.update), a singleton available from TrueNAS 27.0.
package s3_config

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// s3ConfigResourceID is the fixed Terraform ID for this singleton resource.
const s3ConfigResourceID = "s3_config"

// ListenerModel is one S3 listener.
type ListenerModel struct {
	Address types.String `tfsdk:"address"`
	Port    types.Int64  `tfsdk:"port"`
	TLS     types.Bool   `tfsdk:"tls"`
}

// GrantModel is one global grant.
type GrantModel struct {
	PrincipalType types.String `tfsdk:"principal_type"`
	XID           types.Int64  `tfsdk:"xid"`
	Access        types.String `tfsdk:"access"`
}

var listenerAttrTypes = map[string]attr.Type{
	"address": types.StringType,
	"port":    types.Int64Type,
	"tls":     types.BoolType,
}

var grantAttrTypes = map[string]attr.Type{
	"principal_type": types.StringType,
	"xid":            types.Int64Type,
	"access":         types.StringType,
}

// S3ConfigModel is the Terraform state model for truenas_s3_config (and its
// data source, which has the same attributes).
type S3ConfigModel struct {
	ID                   types.String `tfsdk:"id"`
	Listeners            types.List   `tfsdk:"listeners"` // List[ListenerModel]
	Servers              types.Int64  `tfsdk:"servers"`
	Certificate          types.Int64  `tfsdk:"certificate"` // nullable
	Region               types.String `tfsdk:"region"`
	LogLevel             types.String `tfsdk:"log_level"`
	DefaultAudit         types.List   `tfsdk:"default_audit"` // List[String]; ["ALL"] <-> the API's "ALL"
	DefaultAuditOverflow types.String `tfsdk:"default_audit_overflow"`
	GlobalGrants         types.List   `tfsdk:"global_grants"` // List[GrantModel]
	ManagedRootDataset   types.String `tfsdk:"managed_root_dataset"`
}

// s3ConfigAPI is the JSON shape of s3.config.
type s3ConfigAPI struct {
	ID        int64 `json:"id"`
	Listeners []struct {
		Address string `json:"address"`
		Port    int64  `json:"port"`
		TLS     bool   `json:"tls"`
	} `json:"listeners"`
	Servers              int64           `json:"servers"`
	Certificate          *int64          `json:"certificate"`
	Region               string          `json:"region"`
	LogLevel             string          `json:"log_level"`
	DefaultAudit         json.RawMessage `json:"default_audit"` // array of strings or the string "ALL"
	DefaultAuditOverflow string          `json:"default_audit_overflow"`
	GlobalGrants         []struct {
		PrincipalType string `json:"principal_type"`
		XID           *int64 `json:"xid"`
		Access        string `json:"access"`
		Name          string `json:"name"`
	} `json:"global_grants"`
	ManagedRootDataset *string `json:"managed_root_dataset"`
}

// AuditAll is the default_audit list element that stands for the API's "ALL".
const AuditAll = "ALL"

// decodeDefaultAudit maps the API's default_audit (array or "ALL") to the
// model list.
func decodeDefaultAudit(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return []string{}, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []string{s}, nil
	}
	var list []string
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("default_audit %s: %w", string(raw), err)
	}
	if list == nil {
		list = []string{}
	}
	return list, nil
}

// encodeDefaultAudit maps the model list to the API value: ["ALL"] becomes
// the string "ALL", anything else is sent as the list.
func encodeDefaultAudit(list []string) any {
	if len(list) == 1 && strings.EqualFold(list[0], AuditAll) {
		return AuditAll
	}
	if list == nil {
		return []string{}
	}
	return list
}

func responseToModel(ctx context.Context, api *s3ConfigAPI, m *S3ConfigModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(s3ConfigResourceID)

	listeners := make([]ListenerModel, 0, len(api.Listeners))
	for _, l := range api.Listeners {
		listeners = append(listeners, ListenerModel{
			Address: types.StringValue(l.Address),
			Port:    types.Int64Value(l.Port),
			TLS:     types.BoolValue(l.TLS),
		})
	}
	lv, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: listenerAttrTypes}, listeners)
	diags.Append(d...)
	m.Listeners = lv

	m.Servers = types.Int64Value(api.Servers)
	if api.Certificate != nil {
		m.Certificate = types.Int64Value(*api.Certificate)
	} else {
		m.Certificate = types.Int64Null()
	}
	m.Region = types.StringValue(api.Region)
	m.LogLevel = types.StringValue(api.LogLevel)

	audit, err := decodeDefaultAudit(api.DefaultAudit)
	if err != nil {
		diags.AddError("Parse s3.config default_audit", err.Error())
		return diags
	}
	av, d := types.ListValueFrom(ctx, types.StringType, audit)
	diags.Append(d...)
	m.DefaultAudit = av
	m.DefaultAuditOverflow = types.StringValue(api.DefaultAuditOverflow)

	grants := make([]GrantModel, 0, len(api.GlobalGrants))
	for _, g := range api.GlobalGrants {
		gm := GrantModel{PrincipalType: types.StringValue(g.PrincipalType), Access: types.StringValue(g.Access), XID: types.Int64Null()}
		if g.XID != nil {
			gm.XID = types.Int64Value(*g.XID)
		}
		grants = append(grants, gm)
	}
	gv, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: grantAttrTypes}, grants)
	diags.Append(d...)
	m.GlobalGrants = gv

	if api.ManagedRootDataset != nil {
		m.ManagedRootDataset = types.StringValue(*api.ManagedRootDataset)
	} else {
		m.ManagedRootDataset = types.StringNull()
	}
	return diags
}

// updatePayload builds the s3.update argument from every known attribute,
// then drops the keys whose value prior state would send (nil prior: send
// all). Resending an unchanged value is not free here: default_audit is an
// Enterprise-licensed feature and s3.update rejects it on an unlicensed
// system even when the value is the one it already stores, so an unrelated
// change must not touch it. A null certificate / managed_root_dataset is
// sent as null (clears it); an unknown one is left out.
func (m *S3ConfigModel) updatePayload(ctx context.Context, prior *S3ConfigModel) (map[string]any, diag.Diagnostics) {
	p, diags := m.fullPayload(ctx)
	if diags.HasError() || prior == nil {
		return p, diags
	}
	pp, d := prior.fullPayload(ctx)
	diags.Append(d...)
	for k, v := range p {
		if pv, ok := pp[k]; ok && reflect.DeepEqual(v, pv) {
			delete(p, k)
		}
	}
	return p, diags
}

func (m *S3ConfigModel) fullPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	p := map[string]any{}
	if !m.Listeners.IsNull() && !m.Listeners.IsUnknown() {
		var items []ListenerModel
		diags.Append(m.Listeners.ElementsAs(ctx, &items, false)...)
		out := make([]map[string]any, 0, len(items))
		for _, l := range items {
			e := map[string]any{"address": l.Address.ValueString()}
			if !l.Port.IsNull() && !l.Port.IsUnknown() {
				e["port"] = l.Port.ValueInt64()
			}
			if !l.TLS.IsNull() && !l.TLS.IsUnknown() {
				e["tls"] = l.TLS.ValueBool()
			}
			out = append(out, e)
		}
		p["listeners"] = out
	}
	if !m.Servers.IsNull() && !m.Servers.IsUnknown() {
		p["servers"] = m.Servers.ValueInt64()
	}
	// certificate is Optional only: null in the configuration means the UI
	// certificate and is sent as null.
	if !m.Certificate.IsUnknown() {
		if m.Certificate.IsNull() {
			p["certificate"] = nil
		} else {
			p["certificate"] = m.Certificate.ValueInt64()
		}
	}
	if !m.Region.IsNull() && !m.Region.IsUnknown() {
		p["region"] = m.Region.ValueString()
	}
	if !m.LogLevel.IsNull() && !m.LogLevel.IsUnknown() {
		p["log_level"] = m.LogLevel.ValueString()
	}
	if !m.DefaultAudit.IsNull() && !m.DefaultAudit.IsUnknown() {
		var list []string
		diags.Append(m.DefaultAudit.ElementsAs(ctx, &list, false)...)
		p["default_audit"] = encodeDefaultAudit(list)
	}
	if !m.DefaultAuditOverflow.IsNull() && !m.DefaultAuditOverflow.IsUnknown() {
		p["default_audit_overflow"] = m.DefaultAuditOverflow.ValueString()
	}
	if !m.GlobalGrants.IsNull() && !m.GlobalGrants.IsUnknown() {
		var items []GrantModel
		diags.Append(m.GlobalGrants.ElementsAs(ctx, &items, false)...)
		out := make([]map[string]any, 0, len(items))
		for _, g := range items {
			e := map[string]any{"principal_type": g.PrincipalType.ValueString(), "access": g.Access.ValueString()}
			if !g.XID.IsNull() && !g.XID.IsUnknown() {
				e["xid"] = g.XID.ValueInt64()
			}
			out = append(out, e)
		}
		p["global_grants"] = out
	}
	if !m.ManagedRootDataset.IsUnknown() {
		if m.ManagedRootDataset.IsNull() || m.ManagedRootDataset.ValueString() == "" {
			p["managed_root_dataset"] = nil
		} else {
			p["managed_root_dataset"] = m.ManagedRootDataset.ValueString()
		}
	}
	return p, diags
}

// IsUnavailable reports the JSON-RPC "Method does not exist" (-32601) error:
// the connected release has no S3 API (added in TrueNAS 27.0).
func IsUnavailable(err error) bool {
	var apiErr *client.APIError
	if errors.As(err, &apiErr) && apiErr.Code == -32601 {
		return true
	}
	return err != nil && strings.Contains(err.Error(), "Method does not exist")
}

// UnavailableMessage is the detail shown when the S3 API is missing.
const UnavailableMessage = "S3 API not available on this TrueNAS release (needs 27.0 or later): the s3.* methods do not exist on the connected system."

// wrapUnavailable turns a missing-method error into the clear message.
func wrapUnavailable(err error) string {
	if IsUnavailable(err) {
		return UnavailableMessage
	}
	return err.Error()
}

// deleteWarningDiagnostics is Delete's only output: the S3 configuration is
// left in place on TrueNAS.
func deleteWarningDiagnostics() diag.Diagnostics {
	var diags diag.Diagnostics
	diags.AddWarning("S3 configuration left in place", "S3 configuration left in place; removed from Terraform state only")
	return diags
}
