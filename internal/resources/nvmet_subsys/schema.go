// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_subsys

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages an NVMe-oF subsystem on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric NVMe-oF subsystem ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the NVMe-oF subsystem.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"subnqn": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "NVMe Qualified Name (NQN) for the subsystem. Derived by TrueNAS when unset.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"allow_any_host": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Allow any host to connect to this subsystem without an explicit host mapping.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"ana": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable Asymmetric Namespace Access (ANA) for this subsystem.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"ieee_oui": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "IEEE Organizationally Unique Identifier used when generating namespace identifiers.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"pi_enable": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable end-to-end data protection (PI) for this subsystem.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"qid_max": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Maximum number of queues supported by this subsystem.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			// Computed-only (server-generated)
			"serial": schema.StringAttribute{
				Computed:    true,
				Description: "Serial number assigned by TrueNAS.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}
