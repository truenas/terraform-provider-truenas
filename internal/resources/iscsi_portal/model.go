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

	// Convert listen.
	var listenModels []ListenModel
	for _, l := range api.Listen {
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

	// TrueNAS 26.0 removed per-listen port from iscsi.portal.create/update: the
	// global iSCSI service listen_port governs all portals now. Only "ip" is
	// sent per listen entry; "port" is never included in the payload, even
	// when set in config (see the "port" schema attribute description). The
	// response still reports port per listen, and responseToModel keeps
	// mapping it back so it remains visible as a computed value.
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
