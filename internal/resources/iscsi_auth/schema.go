// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_auth

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages an iSCSI CHAP authentication (auth) credential group on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric iSCSI auth entry ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"tag": schema.Int64Attribute{
				Required:    true,
				Description: "Group ID for this CHAP credential. Referenced by an iSCSI target group's \"auth\" setting.",
			},
			"user": schema.StringAttribute{
				Required:    true,
				Description: "CHAP username presented by the initiator.",
			},
			"secret": schema.StringAttribute{
				Required:  true,
				Sensitive: true,
				WriteOnly: true,
				Description: "CHAP secret (password) for the initiator. Must be 12-16 characters. " +
					"Write-only: never stored in Terraform state. Requires Terraform >= 1.11.",
			},
			"peeruser": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Peer username for mutual CHAP (target authenticates to initiator).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"peersecret": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				WriteOnly: true,
				Description: "Peer secret for mutual CHAP. Must be 12-16 characters. Omit or leave empty to disable mutual CHAP. " +
					"Write-only: never stored in Terraform state. Requires Terraform >= 1.11.",
			},
			"discovery_auth": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Discovery authentication method. One of: NONE, CHAP, CHAP_MUTUAL.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}
