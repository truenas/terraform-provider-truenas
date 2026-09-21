// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package network_config

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages the TrueNAS global network configuration (hostname, domain, DNS servers, " +
			"default gateways, service announcement). This is a singleton resource — there is exactly one " +
			"global network configuration per TrueNAS system, so it is never created or deleted on TrueNAS; " +
			"Terraform create/update calls network.configuration.update, and Terraform delete only removes the " +
			"resource from state (the configuration is left in place). WARNING: hostname affects how the system " +
			"identifies itself on the network, and ipv4gateway/ipv6gateway affect outbound network connectivity; " +
			"changing them incorrectly can disrupt access to the system.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"network_config\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"hostname": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "System hostname. WARNING: this affects how the system identifies itself on the " +
					"network (e.g. in directory services, certificates, and other services keyed on hostname).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"domain": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Primary DNS domain name.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"domains": schema.ListAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				Description:   "Additional DNS search domains.",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"hosts": schema.ListAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.StringType,
				Description:   "Additional /etc/hosts entries.",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"httpproxy": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "HTTP proxy URL used by the system for outbound HTTP(S) requests.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"ipv4gateway": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Default IPv4 gateway. An empty string clears the assigned gateway (null on the " +
					"wire). WARNING: changing this can disrupt outbound network connectivity to the system.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"ipv6gateway": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Default IPv6 gateway. An empty string clears the assigned gateway (null on the " +
					"wire). WARNING: changing this can disrupt outbound network connectivity to the system.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"nameserver1": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Primary DNS nameserver. An empty string clears the assigned nameserver (null on " +
					"the wire).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"nameserver2": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Secondary DNS nameserver. An empty string clears the assigned nameserver (null on " +
					"the wire).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"nameserver3": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Tertiary DNS nameserver. An empty string clears the assigned nameserver (null on " +
					"the wire).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"service_announcement": schema.SingleNestedAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Network service announcement/discovery protocols enabled on the system.",
				PlanModifiers: []planmodifier.Object{objectplanmodifier.UseStateForUnknown()},
				Attributes: map[string]schema.Attribute{
					"mdns":    schema.BoolAttribute{Required: true, Description: "Enable mDNS (multicast DNS) service announcement."},
					"netbios": schema.BoolAttribute{Required: true, Description: "Enable NetBIOS name service announcement."},
					"wsd":     schema.BoolAttribute{Required: true, Description: "Enable WS-Discovery service announcement."},
				},
			},
		},
	}
}
