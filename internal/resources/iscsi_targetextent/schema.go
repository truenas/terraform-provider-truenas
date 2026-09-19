// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_targetextent

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages an iSCSI target/extent association (LUN mapping) on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric iSCSI target/extent association ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"target": schema.Int64Attribute{
				Required:    true,
				Description: "ID of the iSCSI target to associate.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"extent": schema.Int64Attribute{
				Required:    true,
				Description: "ID of the iSCSI extent to associate.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"lunid": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "LUN ID for this association. Omit to let TrueNAS auto-assign the next available LUN ID.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}
