// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_initiator

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ISCSIInitiatorModel is the Terraform state model for truenas_iscsi_initiator.
type ISCSIInitiatorModel struct {
	ID         types.Int64  `tfsdk:"id"`
	Comment    types.String `tfsdk:"comment"`
	Initiators types.List   `tfsdk:"initiators"` // List[String] of IQN strings
}

// initiatorAPI is the JSON wire format for a TrueNAS iSCSI initiator object.
type initiatorAPI struct {
	ID         int64    `json:"id"`
	Comment    string   `json:"comment"`
	Initiators []string `json:"initiators"`
}

// responseToModel maps an API response onto a Terraform model.
func responseToModel(ctx context.Context, api *initiatorAPI, m *ISCSIInitiatorModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.Int64Value(api.ID)
	m.Comment = types.StringValue(api.Comment)
	inits := api.Initiators
	if inits == nil {
		inits = []string{}
	}
	list, d := types.ListValueFrom(ctx, types.StringType, inits)
	diags.Append(d...)
	m.Initiators = list
	return diags
}

// apiPayload builds the map[string]any payload for iscsi.initiator.create /
// iscsi.initiator.update.
//
// comment and initiators are Optional+Computed. An unset attribute is
// unknown on create (UseStateForUnknown carries the prior value on update);
// an unknown value is omitted rather than sent as ""/[]. Sending [] for an
// unset initiators list would turn an allowlist into "allow all initiators".
func (m *ISCSIInitiatorModel) apiPayload(ctx context.Context) (map[string]any, diag.Diagnostics) {
	var diags diag.Diagnostics
	payload := map[string]any{}
	if !m.Comment.IsUnknown() {
		payload["comment"] = m.Comment.ValueString()
	}
	if !m.Initiators.IsUnknown() {
		var initiators []string
		if !m.Initiators.IsNull() {
			diags.Append(m.Initiators.ElementsAs(ctx, &initiators, false)...)
		}
		if initiators == nil {
			initiators = []string{}
		}
		payload["initiators"] = initiators
	}
	return payload, diags
}
