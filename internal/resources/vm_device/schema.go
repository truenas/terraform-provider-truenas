// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_device

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a device attached to a TrueNAS VM (disk, NIC, CD-ROM, display, PCI passthrough, raw file, or USB).",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric VM device ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"vm": schema.Int64Attribute{
				Required: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.RequiresReplace(),
				},
				Description: "ID of the VM this device belongs to.",
			},
			"attributes": schema.StringAttribute{
				Required: true,
				// Marked Sensitive because this opaque JSON blob can carry secrets for some
				// device types (e.g. the DISPLAY device's VNC password). It is over-broad for
				// device types with no secret fields, but there is no per-dtype schema to
				// scope the flag to, so the whole attribute is masked as the pragmatic fix.
				Sensitive:   true,
				Description: "JSON document of device attributes. Must include \"dtype\": DISK, NIC, CDROM, DISPLAY, PCI, RAW, or USB.",
			},
			"order": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
				Description: "Boot/attach order.",
			},
		},
	}
}
