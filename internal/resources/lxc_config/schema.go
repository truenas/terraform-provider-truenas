// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package lxc_config

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages the TrueNAS LXC service configuration (lxc.config): the storage pool " +
			"backing LXC-based instances, the network bridge interface, and the IPv4/IPv6 network CIDR blocks " +
			"used for instance networking. This is a singleton resource — there is exactly one LXC configuration " +
			"per TrueNAS system, so it is never created or deleted on TrueNAS; Terraform create/update calls " +
			"lxc.update (probed job:false), and Terraform delete only removes the resource from state (the " +
			"configuration is left in place)." +
			"\n\n" +
			"Requires TrueNAS 26.0 or later: the lxc namespace does not exist on earlier releases " +
			"(probed live — TrueNAS 25.10 returns \"Method does not exist\" for lxc.config). Using this resource " +
			"against an older server fails with a clean error during Create/Read/Update rather than a raw " +
			"API error." +
			"\n\n" +
			"This is distinct from the deprecated incus system-container family (`container`, " +
			"`container.device`, `container.image`), which this provider intentionally does not cover — LXC " +
			"itself remains a fully supported TrueNAS 26.0+ surface.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"lxc_config\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"preferred_pool": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "ZFS storage pool backing LXC-based instances and image datasets, or null if " +
					"not configured.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"bridge": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Network bridge interface used for LXC instance networking, or null if not " +
					"configured (managed/created automatically). See the lxc.bridge_choices API method on the " +
					"target box for the set of valid values; this schema does not validate them at plan time.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"v4_network": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "IPv4 network CIDR for the LXC instance bridge network (e.g. \"172.200.0.0/24\").",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"v6_network": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "IPv6 network CIDR for the LXC instance bridge network " +
					"(e.g. \"fd42:4c58:43ae::/64\").",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}
