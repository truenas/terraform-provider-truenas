// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package snapshot

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/truenas/terraform-provider-truenas/internal/planmod"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a ZFS snapshot on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Snapshot identifier in the form \"dataset@name\".",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"dataset": schema.StringAttribute{
				Required:    true,
				Description: "Dataset or zvol to snapshot (e.g. tank/mydata).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Snapshot name (without @), e.g. snap1.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"recursive": schema.BoolAttribute{
				Optional: true,
				Description: "Take recursive snapshot of child datasets. Create-time only: the API never returns it, " +
					"so it reads back null after import and setting it then records the value in place; any later " +
					"change replaces the snapshot.",
				PlanModifiers: []planmodifier.Bool{
					planmod.ReplaceFromKnownBool(),
				},
			},
			"allow_recreate": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				Description: "A snapshot is a point in time: replacing it (a change to `dataset`, `name` or " +
					"`recursive`) destroys that copy and takes a new one of the data as it is now. Default false: " +
					"such a change is refused at plan time. Set true to allow the destroy and recreate. Changing " +
					"it updates state only.",
			},
			"pool": schema.StringAttribute{
				Computed:    true,
				Description: "Name of the pool that contains the snapshot.",
				// Fixed for the snapshot's life; without this the in-place
				// record of "recursive" after import shows it as unknown.
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"createtxg": schema.StringAttribute{
				Computed:    true,
				Description: "ZFS transaction group at which the snapshot was created.",
				// Fixed for the snapshot's life; without this the in-place
				// record of "recursive" after import shows it as unknown.
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}
