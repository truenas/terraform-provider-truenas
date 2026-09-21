// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package zvol

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a ZFS volume (zvol/block device) on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Zvol name (used as Terraform ID).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Full zvol path, e.g. tank/myvol.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"volsize": schema.Int64Attribute{
				Required:    true,
				Description: "Volume size in bytes.",
			},
			"volblocksize": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Block size in bytes (512, 1024, 2048, 4096, 8192, 16384, 32768, 65536, 131072). Set at create time only.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
					int64planmodifier.RequiresReplace(),
				},
			},
			"compression": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Compression algorithm. Case-insensitive: lz4, zstd, off, etc.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"sync": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Sync setting: standard, always, or disabled.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"dedup": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Deduplication: off, on, or verify.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"sparse": schema.BoolAttribute{
				Optional:    true,
				Description: "Sparse provisioning (write-only; not returned by API).",
			},
			"comments": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Human-readable description.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"pool": schema.StringAttribute{
				Computed:    true,
				Description: "Name of the pool containing this zvol.",
			},
			"encrypted": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the zvol is encrypted.",
			},
		},
	}
}
