// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nvmet_port

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages an NVMe-oF transport port on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric NVMe-oF port ID assigned by TrueNAS.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"addr_trtype": schema.StringAttribute{
				Required:    true,
				Description: "TCP or RDMA. Fibre Channel is not supported by this provider.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"addr_traddr": schema.StringAttribute{
				Required:    true,
				Description: "Transport address (IP address) the port listens on.",
			},
			"addr_trsvcid": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Transport service ID (TCP/RDMA port number). Defaults to 4420.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Cannot change an enabled port while the NVMe target service is running.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"inline_data_size": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Inline data size (bytes) for this port.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"max_queue_size": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Maximum queue size for this port.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"pi_enable": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Enable end-to-end data protection (PI) for this port.",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			// Computed-only (server-generated)
			"index": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric index assigned by TrueNAS for this port.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"addr_adrfam": schema.StringAttribute{
				Computed:    true,
				Description: "Address family (e.g. IPV4) derived from addr_traddr.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}
