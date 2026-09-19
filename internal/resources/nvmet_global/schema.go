// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_global

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages the TrueNAS global NVMe-oF target configuration. This is a singleton " +
			"resource — there is exactly one NVMe-oF global configuration per TrueNAS system, so it is never " +
			"created or deleted on TrueNAS; Terraform create/update calls nvmet.global.update, and Terraform " +
			"delete only removes the resource from state (the configuration is left in place, since it serves " +
			"live storage).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"nvmet_global\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"basenqn": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Base NVMe Qualified Name (NQN) prefix used when generating subsystem NQNs.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"ana": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether Asymmetric Namespace Access (ANA) is enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"kernel": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether the kernel-mode NVMe-oF target implementation is used.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"rdma": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether NVMe-oF RDMA transport is enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"xport_referral": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether transport referral is enabled.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
		},
	}
}
