// Copyright iXsystems, Inc. 2026
// SPDX-License-Identifier: MPL-2.0

package nfs

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages an NFS share on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"path": schema.StringAttribute{
				Required:    true,
				Description: "Absolute path to the directory being shared.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"comment": schema.StringAttribute{Optional: true, Computed: true},
			"enabled": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(true),
			},
			"ro": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Export as read-only.",
			},
			"networks": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				Description: "List of allowed networks in CIDR notation.",
				ElementType: types.StringType,
			},
			"hosts": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				Description: "List of allowed hosts (IP or hostname).",
				ElementType: types.StringType,
			},
			"maproot_user": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Map root user to this user.",
			},
			"maproot_group": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Map root group to this group.",
			},
		},
	}
}
