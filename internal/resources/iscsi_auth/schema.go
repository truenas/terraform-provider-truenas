// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_auth

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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
					"Write-only: never stored in Terraform state. Requires Terraform >= 1.11. Generated configuration (`terraform plan -generate-config-out`) has this as null, because Terraform never writes sensitive or write-only values there; fill it in before applying.",
			},
			"peeruser": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString(""),
				Description: "Peer username for mutual CHAP (target authenticates to initiator). Leave unset (or " +
					"\"\") for one-way CHAP: removing it from config turns mutual CHAP off and clears the peer secret.",
			},
			"peersecret": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				WriteOnly: true,
				Description: "Peer secret for mutual CHAP. Must be 12-16 characters. Required while peeruser is set; " +
					"cleared on the server when peeruser is unset. " +
					"Write-only: never stored in Terraform state. Requires Terraform >= 1.11.",
			},
			"secret_wo_version": schema.Int64Attribute{
				Optional: true,
				Description: "Rotation trigger for the write-only secret. A write-only value is absent from state, " +
					"so Terraform cannot detect that secret changed. Bump this integer whenever secret changes " +
					"to make Terraform re-send it. secret is sent on create and, after that, only when this integer changes; other updates leave the stored value alone. Optional.",
			},
			"peersecret_wo_version": schema.Int64Attribute{
				Optional: true,
				Description: "Rotation trigger for the write-only peersecret. A write-only value is absent from state, " +
					"so Terraform cannot detect that peersecret changed. Bump this integer whenever peersecret changes " +
					"to make Terraform re-send it. peersecret is sent on create and, after that, only when this integer changes or when mutual CHAP is turned on (peeruser set where it was empty); other updates leave the stored value alone. Optional.",
			},
			"discovery_auth": schema.StringAttribute{
				Validators:  []validator.String{stringvalidator.OneOf("CHAP", "CHAP_MUTUAL", "NONE")},
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
