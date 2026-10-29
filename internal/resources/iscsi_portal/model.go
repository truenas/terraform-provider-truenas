// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_portal

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ListenModel maps to the nested listen list items.
type ListenModel struct {
	IP   types.String `tfsdk:"ip"`
	Port types.Int64  `tfsdk:"port"`
}

// ISCSIPortalModel is the Terraform state model for truenas_iscsi_portal.
type ISCSIPortalModel struct {
	ID      types.Int64  `tfsdk:"id"`
	Comment types.String `tfsdk:"comment"`
	Listen  types.List   `tfsdk:"listen"` // List[ListenModel], Required
	Tag     types.Int64  `tfsdk:"tag"`    // Computed, server-assigned
}

// listenAPI is the JSON wire format for a listen entry.
type listenAPI struct {
	IP   string `json:"ip"`
	Port int64  `json:"port"`
}

// portalAPI is the JSON wire format for a TrueNAS iSCSI portal object.
type portalAPI struct {
	ID      int64       `json:"id"`
	Comment string      `json:"comment"`
	Listen  []listenAPI `json:"listen"`
	Tag     int64       `json:"tag"`
}

// listenAttrTypes is the attribute type map for a ListenModel object.
var listenAttrTypes = map[string]attr.Type{
	"ip":   types.StringType,
	"port": types.Int64Type,
}

// responseToModel maps an API response onto a Terraform model.
func responseToModel(ctx context.Context, api *portalAPI, m *ISCSIPortalModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Comment = types.StringValue(api.Comment)
	m.Tag = types.Int64Value(api.Tag)

	// Convert listen, in the order the model already has (plan on
	// create/update, prior state on read) when it holds the same IPs:
	// TrueNAS keeps its stored order on update, so a reordered config would
	// otherwise fail with "inconsistent result after apply".
	var listenModels []ListenModel
	for _, l := range orderListen(api.Listen, modelListenIPs(ctx, m.Listen)) {
		listenModels = append(listenModels, ListenModel{
			IP:   types.StringValue(l.IP),
			Port: types.Int64Value(l.Port),
		})
	}
	if listenModels == nil {
		listenModels = []ListenModel{}
	}
	listenList, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: listenAttrTypes}, listenModels)
	diags.Append(d...)
	m.Listen = listenList

	return diags
}

// apiPayload builds the map[string]any payload for iscsi.portal.create / update.
func (m *ISCSIPortalModel) apiPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Listen.
	var listenItems []ListenModel
	if !m.Listen.IsNull() && !m.Listen.IsUnknown() {
		diags.Append(m.Listen.ElementsAs(ctx, &listenItems, false)...)
	}

	// iscsi.portal.create/update take only "ip" per listen entry on every
	// supported release (25.10+): the global iSCSI listen_port governs all
	// portals. The response still reports port per listen, and
	// responseToModel maps it back as a Computed-only value.
	var listenPayload []map[string]any
	for _, l := range listenItems {
		listenPayload = append(listenPayload, map[string]any{
			"ip": l.IP.ValueString(),
		})
	}
	if listenPayload == nil {
		listenPayload = []map[string]any{}
	}

	payload := map[string]any{
		"listen": listenPayload,
	}

	if !m.Comment.IsNull() && !m.Comment.IsUnknown() {
		payload["comment"] = m.Comment.ValueString()
	}

	return payload, diags
}

// modelListenIPs returns the IPs of a listen list in order, or nil when the
// list (or any IP in it) is not known.
func modelListenIPs(ctx context.Context, l types.List) []string {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	var items []ListenModel
	if d := l.ElementsAs(ctx, &items, false); d.HasError() {
		return nil
	}
	ips := make([]string, 0, len(items))
	for _, it := range items {
		if it.IP.IsNull() || it.IP.IsUnknown() {
			return nil
		}
		ips = append(ips, it.IP.ValueString())
	}
	return ips
}

// orderListen returns api reordered to follow want when both hold the same
// IPs (as a multiset), and api unchanged otherwise.
func orderListen(api []listenAPI, want []string) []listenAPI {
	if len(want) != len(api) || len(api) == 0 {
		return api
	}
	used := make([]bool, len(api))
	out := make([]listenAPI, 0, len(api))
	for _, ip := range want {
		found := false
		for i, l := range api {
			if !used[i] && l.IP == ip {
				used[i] = true
				out = append(out, l)
				found = true
				break
			}
		}
		if !found {
			return api
		}
	}
	return out
}
