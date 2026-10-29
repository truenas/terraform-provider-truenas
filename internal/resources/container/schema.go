// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/planmod"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages an LXC container on TrueNAS (container.*: create/update/start/stop/delete). " +
			"Requires TrueNAS 26.0 or later: the container namespace does not exist on earlier releases " +
			"(probed live — TrueNAS 25.10 exposes 0 container.* methods). Using this resource against an older " +
			"server fails with a clean error during Create/Read/Update rather than a raw API error." +
			"\n\n" +
			"This is the modern, actively-developed LXC container surface — distinct from the deprecated incus " +
			"system-container family this provider does not otherwise track. See truenas_container_device for " +
			"per-container device attachment (filesystem/NIC/USB passthrough), and truenas_lxc_config for the " +
			"service-wide pool/bridge/network configuration this resource's containers run under.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric container ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"uuid": schema.StringAttribute{
				Computed:    true,
				Description: "Container UUID (used internally for libvirt).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Container name. Mutable: container.update accepts and applies a rename (probed live).",
			},
			"pool": schema.StringAttribute{
				Required: true,
				Description: "ZFS pool the container's root dataset is created on. Immutable: container.update " +
					"rejects \"pool\" with a clean EINVAL (probed live), so changing it replaces the container.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"image": schema.SingleNestedAttribute{
				Required: true,
				Description: "LXC registry image to create the container from. Used only at create: " +
					"container.update does not accept \"image\" on 26.0 or 28.0 (probed live), so changing the " +
					"name or the version replaces the container, destroying its root filesystem. The upstream " +
					"registry prunes old builds (observed live: a version the registry still listed 404'd on " +
					"download once pruned), so look up a current version with the truenas_container_image " +
					"datasource — but if you reference its latest_version, add " +
					"lifecycle { ignore_changes = [image] } to the container, or every new registry build " +
					"(roughly daily) replaces it. Remove the ignore_changes when you do want to rebuild the " +
					"container from a newer image.",
				// The API never returns "image": it reads back null after import,
				// and the configured value is then recorded in place (BUGS B-005).
				PlanModifiers: []planmodifier.Object{
					planmod.ReplaceFromKnownObject(),
				},
				Attributes: map[string]schema.Attribute{
					"name": schema.StringAttribute{
						Required:    true,
						Description: "Image name, e.g. \"alpine:3.22:amd64:default\".",
					},
					"version": schema.StringAttribute{
						Required:    true,
						Description: "Image version, e.g. \"20260722_17:16\". See truenas_container_image.",
					},
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Container description. Defaults to an empty string.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"autostart": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Automatically start the container on boot. Defaults to true server-side.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"cpuset": schema.StringAttribute{
				Optional: true,
				Description: "Physical CPU numbers the container's domain process and virtual CPUs may be " +
					"pinned to (e.g. \"0-3\"). Unset means no pinning: removing it from the configuration clears it.",
			},
			"time": schema.StringAttribute{
				Validators:  []validator.String{stringvalidator.OneOf("LOCAL", "UTC")},
				Optional:    true,
				Computed:    true,
				Description: "Container clock: LOCAL or UTC. Defaults to LOCAL.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"shutdown_timeout": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Seconds to wait for the container to shut down gracefully before killing it " +
					"(5-300). Defaults to 90.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"init": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "\"init\" process command line. Defaults to \"/sbin/init\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"initdir": schema.StringAttribute{
				Optional: true,
				Description: "\"init\" process working directory. Unset means the image default: removing it from the " +
					"configuration clears it.",
			},
			"initenv": schema.MapAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "\"init\" process environment variables. Defaults to an empty map.",
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.UseStateForUnknown(),
				},
			},
			"inituser": schema.StringAttribute{
				Optional: true,
				Description: "\"init\" process username. Unset means the image default: removing it from the " +
					"configuration clears it.",
			},
			"initgroup": schema.StringAttribute{
				Optional: true,
				Description: "\"init\" process group. Unset means the image default: removing it from the " +
					"configuration clears it.",
			},
			"idmap": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Idmap configuration as a JSON-encoded string, passed through opaquely: either " +
					"jsonencode({type = \"DEFAULT\"}) (the server default — offsets container UIDs starting " +
					"from root at host UID 2147000001) or jsonencode({type = \"ISOLATED\", slice = N}) (same " +
					"offset scheme plus a unique slice number per container, or slice = null to let the server " +
					"pick one). Immutable: container.update does not accept \"idmap\" (probed live), so changing " +
					"it replaces the container.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					idmapRequiresReplace(),
				},
			},
			"capabilities_policy": schema.StringAttribute{
				Validators: []validator.String{stringvalidator.OneOf("ALLOW", "DEFAULT", "DENY")},
				Optional:   true,
				Computed:   true,
				Description: "Default Linux capabilities policy: DEFAULT (drop sys_module, sys_time, mknod, " +
					"audit_control, mac_admin), ALLOW (keep all), or DENY (drop all). Defaults to DEFAULT.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"capabilities_state": schema.MapAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.BoolType,
				Description: "Per-capability overrides on top of capabilities_policy. Defaults to an empty map.",
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.UseStateForUnknown(),
				},
			},
			"running": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the container is currently running. Set to true to start, false to stop.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			// Computed only — server generated, changes independently of Terraform.
			"dataset": schema.StringAttribute{
				Computed:    true,
				Description: "ZFS dataset backing the container's root filesystem.",
				PlanModifiers: []planmodifier.String{
					keepUnlessChanged{input: "name"},
				},
			},
			"default_network": schema.StringAttribute{
				Computed: true,
				Description: "Default network bridge used when no NIC devices are explicitly attached, or " +
					"null when explicit NIC devices are configured.",
				PlanModifiers: []planmodifier.String{
					keepUnlessChanged{},
				},
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Current container status: RUNNING, STOPPED.",
				PlanModifiers: []planmodifier.String{
					keepUnlessChanged{input: "running"},
				},
			},
		},
	}
}
