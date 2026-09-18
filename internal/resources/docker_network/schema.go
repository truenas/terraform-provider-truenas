// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package docker_network

import (
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// datasourceSchema returns the schema shared by the truenas_docker_network
// data source. docker_network is a DATASOURCE-ONLY namespace: TrueNAS
// exposes docker.network.query/get_instance but no create/update/delete —
// Docker networks are managed by Docker itself (and by installed
// applications), not by TrueNAS's own config surface — so there is nothing
// for a truenas_docker_network resource to manage; only a datasource
// exists (see provider.go's registration comment for how this is wired
// into the provider).
func datasourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Looks up a Docker network on TrueNAS by name (docker.network.query). Read-only: " +
			"Docker networks are created/destroyed by Docker itself (and by installed applications), not by " +
			"this provider — there is no corresponding truenas_docker_network resource.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Full Docker network identifier.",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Network name to look up (e.g. \"bridge\", \"host\", or an application's own network such as \"ix-plex_default\").",
			},
			"driver": schema.StringAttribute{
				Computed:    true,
				Description: "Network driver type (e.g. bridge, host, null).",
			},
			"scope": schema.StringAttribute{
				Computed:    true,
				Description: "Network scope (e.g. local, global, swarm).",
			},
			"short_id": schema.StringAttribute{
				Computed:    true,
				Description: "Shortened Docker network identifier.",
			},
			"created": schema.StringAttribute{
				Computed:    true,
				Description: "Timestamp when the network was created.",
			},
			"ipam": schema.SingleNestedAttribute{
				Computed:    true,
				Description: "IP Address Management configuration for the network.",
				Attributes: map[string]schema.Attribute{
					"driver": schema.StringAttribute{Computed: true, Description: "IPAM driver (e.g. \"default\")."},
					"config": schema.ListNestedAttribute{
						Computed:    true,
						Description: "Subnet/gateway pools allocated to this network.",
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"subnet":   schema.StringAttribute{Computed: true, Description: "Subnet in CIDR notation."},
								"gateway":  schema.StringAttribute{Computed: true, Description: "Gateway address for the subnet."},
								"ip_range": schema.StringAttribute{Computed: true, Description: "Allocatable IP range within the subnet, if narrower than the full subnet."},
							},
						},
					},
				},
			},
			"labels": schema.MapAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Metadata labels attached to the network (e.g. the owning application's Docker Compose project labels).",
			},
		},
	}
}
