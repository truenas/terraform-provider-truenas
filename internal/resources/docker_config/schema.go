// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package docker_config

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages the TrueNAS Docker service configuration (docker.config): the storage " +
			"pool/dataset backing Docker, automatic image update checks, container networking address pools, " +
			"the IPv6 CIDR block, and registry mirrors. This is a singleton resource — there is exactly one " +
			"Docker configuration per TrueNAS system, so it is never created or deleted on TrueNAS; Terraform " +
			"create/update calls docker.update (a job), and Terraform delete only removes the resource from " +
			"state (the configuration is left in place, and Docker is never stopped or unconfigured). " +
			"\n\n" +
			"`migrate_applications` is intentionally NOT exposed: docker.update accepts it as an apply-time " +
			"action flag controlling whether existing applications are migrated when \"pool\" changes, not " +
			"persisted configuration — there is nothing for docker.config to read back, so it has no place in " +
			"this resource's schema. Changing \"pool\" via this resource migrates Docker without moving " +
			"applications; use the TrueNAS UI/API directly if application migration is required." +
			"\n\n" +
			"`nvidia` (NVIDIA GPU support) is readable on every probed release, but writable only on " +
			"TrueNAS 25.10 and earlier: TrueNAS 26.0+ dropped it from docker.update's accepted fields (probed live) " +
			"— docker.config still reports its current value there, this resource can still display it, but " +
			"setting it explicitly in configuration against a 26.0+ target is an apply-time error.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"docker_config\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"enable_image_updates": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether TrueNAS periodically checks for and downloads updates to Docker images used by installed applications.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"pool": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "ZFS storage pool backing Docker, or null if Docker has not been configured. " +
					"Changing this pool migrates the Docker dataset; see \"migrate_applications\" above for " +
					"why application migration itself is not controlled by this resource.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"dataset": schema.StringAttribute{
				Computed:    true,
				Description: "Read-only: the ZFS dataset TrueNAS created under \"pool\" for Docker data storage, or null if Docker is unconfigured. Not settable via docker.update.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"nvidia": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Whether NVIDIA GPU support is enabled for containers. Readable on every probed " +
					"release; writable only on TrueNAS 25.10 and earlier — TrueNAS 26.0+ dropped it from " +
					"docker.update's accepted fields, so explicitly setting it there is an apply-time error.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"address_pools": schema.ListNestedAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Network address pools Docker allocates per-container-network subnets from.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"base": schema.StringAttribute{
							Required:    true,
							Description: "Base network address with prefix for the pool (e.g. \"172.17.0.0/12\").",
						},
						"size": schema.Int64Attribute{
							Required:    true,
							Description: "Subnet size for networks allocated from this pool (e.g. 24).",
						},
					},
				},
			},
			"cidr_v6": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "IPv6 CIDR block for Docker container networking (e.g. \"fdd0::/64\").",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"registry_mirrors": schema.ListNestedAttribute{
				Optional: true,
				Computed: true,
				Description: "Registry mirror URLs Docker pulls images through. Unified across both probed " +
					"releases: on TrueNAS 25.10 (which has no per-entry insecure flag on the wire — two " +
					"separate secure/insecure arrays instead) each entry's \"insecure\" value is translated to " +
					"and from that split shape transparently.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"url": schema.StringAttribute{
							Required:    true,
							Description: "Registry mirror URL.",
						},
						"insecure": schema.BoolAttribute{
							Required:    true,
							Description: "Whether this registry mirror uses an insecure (HTTP) connection.",
						},
					},
				},
			},
		},
	}
}
