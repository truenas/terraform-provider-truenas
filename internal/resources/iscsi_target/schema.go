// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_target

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages an iSCSI target on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric iSCSI target ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "iSCSI target name (IQN suffix).",
			},
			"alias": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Optional human-readable alias for the target.",
			},
			"mode": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Protocol mode: ISCSI (default), FC, or BOTH.",
			},
			"groups": schema.ListNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: "List of portal/initiator/auth group associations.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"portal": schema.Int64Attribute{
							Required:    true,
							Description: "Portal group ID.",
						},
						"initiator": schema.Int64Attribute{
							Optional:    true,
							Computed:    true,
							Description: "Initiator group ID (0 = none).",
						},
						"auth": schema.Int64Attribute{
							Optional:    true,
							Computed:    true,
							Description: "Auth group ID (0 = none).",
						},
						"authmethod": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Description: "Authentication method: NONE, CHAP, CHAP_MUTUAL.",
						},
					},
				},
			},
			"auth_networks": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of authorized network CIDRs.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"iscsi_parameters": schema.SingleNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Optional iSCSI-specific parameters for this target.",
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{
					"queued_commands": schema.Int64Attribute{
						Optional:    true,
						Computed:    true,
						Description: "Maximum queued commands per iSCSI session: 32 or 128.",
						Validators: []validator.Int64{
							int64validator.OneOf(32, 128),
						},
					},
				},
			},
			"rel_tgt_id": schema.Int64Attribute{
				Computed:    true,
				Description: "Relative target ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}
