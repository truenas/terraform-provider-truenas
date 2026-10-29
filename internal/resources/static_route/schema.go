// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package static_route

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/truenas/terraform-provider-truenas/internal/normalize"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a static route on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric static route ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"destination": schema.StringAttribute{
				Required:   true,
				CustomType: normalize.CIDRType{},
				Description: "Destination network in CIDR notation, e.g. \"10.20.0.0/16\". A bare address means " +
					"/32 (/128 for IPv6) and a dotted netmask is accepted; TrueNAS stores a canonical form " +
					"(IPv6 exploded), which compares equal to the configured spelling.",
				PlanModifiers: []planmodifier.String{normalize.KeepStateIfEquivalent(normalize.CIDRType{})},
			},
			"gateway": schema.StringAttribute{
				Required:      true,
				CustomType:    normalize.IPAddressType{},
				Description:   "Gateway IP address for this route (IPv4 or IPv6, any spelling).",
				PlanModifiers: []planmodifier.String{normalize.KeepStateIfEquivalent(normalize.IPAddressType{})},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Optional description for this static route.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}
