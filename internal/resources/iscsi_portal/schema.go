// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package iscsi_portal

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages an iSCSI portal on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric iSCSI portal ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"comment": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Optional comment describing the portal.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"listen": schema.ListNestedAttribute{
				Required: true,
				Description: "List of IP addresses the portal listens on. Order does not matter: TrueNAS keeps its " +
					"stored order, and the provider reports the list in config order when the IPs match.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"ip": schema.StringAttribute{
							Required:    true,
							Description: "IP address to listen on.",
						},
						"port": schema.Int64Attribute{
							Computed: true,
							Description: "TCP port reported by TrueNAS. Read-only: no supported TrueNAS release accepts a " +
								"per-listen port (25.10 and later); set listen_port on truenas_iscsi_global instead.",
							PlanModifiers: []planmodifier.Int64{
								portFromState{},
							},
						},
					},
				},
			},
			"tag": schema.Int64Attribute{
				Computed:    true,
				Description: "Portal group tag assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}
