// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_namespace

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages an NVMe-oF namespace (the block device behind a subsystem) on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric NVMe-oF namespace ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"subsys_id": schema.Int64Attribute{
				Required:    true,
				Description: "ID of the NVMe-oF subsystem this namespace belongs to.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
			},
			"device_path": schema.StringAttribute{
				Required:    true,
				Description: "Path of the backing device, e.g. \"zvol/tank/vms/vm1\".",
			},
			"device_type": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Backing device type: ZVOL or FILE.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Whether the namespace is enabled.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"filesize": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Size in bytes of the backing file. Only applicable for FILE device_type.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"nsid": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Namespace ID within the subsystem. Auto-assigned by TrueNAS when unset. Immutable after creation.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
					int64planmodifier.RequiresReplace(),
				},
			},
			// Computed-only (server-generated)
			"device_nguid": schema.StringAttribute{
				Computed:    true,
				Description: "NVMe Globally Unique Identifier for the namespace's backing device.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"device_uuid": schema.StringAttribute{
				Computed:    true,
				Description: "UUID for the namespace's backing device.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"locked": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the namespace's backing device is locked (e.g. an encrypted dataset). Not stabilized with UseStateForUnknown since the server can change this independently of Terraform.",
			},
		},
	}
}
