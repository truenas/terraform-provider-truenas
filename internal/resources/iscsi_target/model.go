package iscsi_target

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TargetGroupModel maps to the nested groups list items.
type TargetGroupModel struct {
	Portal     types.Int64  `tfsdk:"portal"`
	Initiator  types.Int64  `tfsdk:"initiator"` // nullable → 0 = unset
	Auth       types.Int64  `tfsdk:"auth"`       // nullable → 0 = unset
	AuthMethod types.String `tfsdk:"authmethod"` // NONE, CHAP, CHAP_MUTUAL
}

// ISCSITargetModel is the Terraform state model for truenas_iscsi_target.
type ISCSITargetModel struct {
	ID           types.Int64  `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Alias        types.String `tfsdk:"alias"`         // Optional, nullable
	Mode         types.String `tfsdk:"mode"`           // ISCSI, FC, BOTH
	Groups       types.List   `tfsdk:"groups"`         // List[TargetGroupModel]
	AuthNetworks types.List   `tfsdk:"auth_networks"`  // List[String]
	RelTgtID     types.Int64  `tfsdk:"rel_tgt_id"`    // Computed only
}

// targetGroupAPI is the JSON wire format for a group entry.
type targetGroupAPI struct {
	Portal     int64  `json:"portal"`
	Initiator  *int64 `json:"initiator"`
	Auth       *int64 `json:"auth"`
	AuthMethod string `json:"authmethod"`
}

// targetAPI is the JSON wire format for a TrueNAS iSCSI target object.
type targetAPI struct {
	ID           int64            `json:"id"`
	Name         string           `json:"name"`
	Alias        *string          `json:"alias"`
	Mode         string           `json:"mode"`
	Groups       []targetGroupAPI `json:"groups"`
	AuthNetworks []string         `json:"auth_networks"`
	RelTgtID     int64            `json:"rel_tgt_id"`
}

// groupsAttrTypes is the attribute type map for a TargetGroupModel object.
var groupsAttrTypes = map[string]attr.Type{
	"portal":     types.Int64Type,
	"initiator":  types.Int64Type,
	"auth":       types.Int64Type,
	"authmethod": types.StringType,
}

// responseToModel maps an API response onto a Terraform model.
func responseToModel(ctx context.Context, api *targetAPI, m *ISCSITargetModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.Int64Value(api.ID)
	m.Name = types.StringValue(api.Name)
	m.Mode = types.StringValue(api.Mode)
	m.RelTgtID = types.Int64Value(api.RelTgtID)

	if api.Alias != nil {
		m.Alias = types.StringValue(*api.Alias)
	} else {
		m.Alias = types.StringValue("")
	}

	// Convert groups.
	var groupModels []TargetGroupModel
	for _, g := range api.Groups {
		var initiator, auth int64
		if g.Initiator != nil {
			initiator = *g.Initiator
		}
		if g.Auth != nil {
			auth = *g.Auth
		}
		groupModels = append(groupModels, TargetGroupModel{
			Portal:     types.Int64Value(g.Portal),
			Initiator:  types.Int64Value(initiator),
			Auth:       types.Int64Value(auth),
			AuthMethod: types.StringValue(g.AuthMethod),
		})
	}
	if groupModels == nil {
		groupModels = []TargetGroupModel{}
	}
	groupList, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: groupsAttrTypes}, groupModels)
	diags.Append(d...)
	m.Groups = groupList

	// Convert auth_networks.
	authNetworks := api.AuthNetworks
	if authNetworks == nil {
		authNetworks = []string{}
	}
	anList, d := types.ListValueFrom(ctx, types.StringType, authNetworks)
	diags.Append(d...)
	m.AuthNetworks = anList

	return diags
}

// apiPayload builds the map[string]any payload for iscsi.target.create / update.
func (m *ISCSITargetModel) apiPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics

	// Groups.
	var groupItems []TargetGroupModel
	if !m.Groups.IsNull() && !m.Groups.IsUnknown() {
		diags.Append(m.Groups.ElementsAs(ctx, &groupItems, false)...)
	}

	var groupsPayload []map[string]any
	for _, g := range groupItems {
		item := map[string]any{
			"portal":     g.Portal.ValueInt64(),
			"authmethod": g.AuthMethod.ValueString(),
		}
		if g.Initiator.ValueInt64() != 0 {
			item["initiator"] = g.Initiator.ValueInt64()
		} else {
			item["initiator"] = nil
		}
		if g.Auth.ValueInt64() != 0 {
			item["auth"] = g.Auth.ValueInt64()
		} else {
			item["auth"] = nil
		}
		groupsPayload = append(groupsPayload, item)
	}
	if groupsPayload == nil {
		groupsPayload = []map[string]any{}
	}

	// Auth networks.
	var authNetworks []string
	if !m.AuthNetworks.IsNull() && !m.AuthNetworks.IsUnknown() {
		diags.Append(m.AuthNetworks.ElementsAs(ctx, &authNetworks, false)...)
	}
	if authNetworks == nil {
		authNetworks = []string{}
	}

	payload := map[string]any{
		"name":          m.Name.ValueString(),
		"alias":         m.Alias.ValueString(),
		"mode":          m.Mode.ValueString(),
		"groups":        groupsPayload,
		"auth_networks": authNetworks,
	}

	return payload, diags
}
