// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_global

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages the TrueNAS global iSCSI service configuration. This is a singleton " +
			"resource — there is exactly one iSCSI global configuration per TrueNAS system, so it is never " +
			"created or deleted on TrueNAS; Terraform create/update calls iscsi.global.update, and Terraform " +
			"delete only removes the resource from state (the configuration is left in place, since it serves " +
			"live storage).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"iscsi_global\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"basename": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Base name (iSCSI Qualified Name prefix) used when generating target names.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"listen_port": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "TCP port the iSCSI service listens on.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"alua": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether Asymmetric Logical Unit Access (ALUA) is enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"iser": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether iSER (iSCSI Extensions for RDMA) is enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"isns_servers": schema.ListAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				Description:   "List of iSNS server hostnames or IP addresses.",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"pool_avail_threshold": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Percentage of pool free space remaining that triggers a warning alert. A value of 0 clears the threshold (disabled).",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}
