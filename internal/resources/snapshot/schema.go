// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snapshot

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a ZFS snapshot on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Snapshot identifier in the form \"dataset@name\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"dataset": schema.StringAttribute{
				Required:    true,
				Description: "Dataset or zvol to snapshot (e.g. tank/mydata).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Snapshot name (without @), e.g. snap1.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"recursive": schema.BoolAttribute{
				Optional:    true,
				Description: "Take recursive snapshot of child datasets (write-only; not stored in state).",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"pool": schema.StringAttribute{
				Computed:    true,
				Description: "Name of the pool that contains the snapshot.",
			},
			"createtxg": schema.StringAttribute{
				Computed:    true,
				Description: "ZFS transaction group at which the snapshot was created.",
			},
		},
	}
}
