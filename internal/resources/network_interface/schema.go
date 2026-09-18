// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package network_interface

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
		Description: "Manages a TrueNAS network interface (BRIDGE, LINK_AGGREGATION, or VLAN). " +
			"Changes are staged and then committed with an auto-rollback safety window; " +
			"physical interfaces can only be imported, never created or deleted.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Interface name (same as name).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Interface name, e.g. br0, bond0, vlan100.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				Required:    true,
				Description: "Interface type: BRIDGE, LINK_AGGREGATION, VLAN. PHYSICAL is import-only.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Optional interface description.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"mtu": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "MTU size (0 = unset, uses the default).",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"ipv4_dhcp": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable DHCP for IPv4.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"ipv6_auto": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable SLAAC autoconfiguration for IPv6.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"aliases": schema.ListNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Static IP address aliases assigned to this interface.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"address": schema.StringAttribute{
							Required:    true,
							Description: "IP address.",
						},
						"netmask": schema.Int64Attribute{
							Required:    true,
							Description: "Netmask/prefix length.",
						},
						"type": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Description: "Address family: INET, INET6.",
						},
					},
				},
			},
			"bridge_members": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Member interfaces for a BRIDGE.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"stp": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable spanning tree protocol (BRIDGE only).",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"lag_protocol": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Link aggregation protocol (LINK_AGGREGATION only, \"\" = unset).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"lag_ports": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "Member ports for a LINK_AGGREGATION interface.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"vlan_parent_interface": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Parent interface for a VLAN.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"vlan_tag": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "VLAN tag ID (0 = unset).",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}
