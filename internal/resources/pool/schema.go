// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package pool

import (
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a ZFS pool on TrueNAS.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"topology": schema.SingleNestedAttribute{
				Required: true,
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.RequiresReplace(),
				},
				Attributes: map[string]schema.Attribute{
					"data": schema.ListNestedAttribute{
						Required: true,
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"type":  schema.StringAttribute{Required: true, Description: vdevTypeDesc},
								"disks": schema.ListAttribute{Required: true, ElementType: types.StringType, Description: disksDesc},
							},
						},
					},
					"log": schema.ListNestedAttribute{
						Optional: true,
						Computed: true,
						PlanModifiers: []planmodifier.List{
							listplanmodifier.UseStateForUnknown(),
						},
						NestedObject: schema.NestedAttributeObject{
							Attributes: map[string]schema.Attribute{
								"type":  schema.StringAttribute{Required: true, Description: vdevTypeDesc},
								"disks": schema.ListAttribute{Required: true, ElementType: types.StringType, Description: disksDesc},
							},
						},
					},
					"cache": schema.ListAttribute{
						Optional:    true,
						Computed:    true,
						ElementType: types.StringType,
						Description: disksDesc,
						PlanModifiers: []planmodifier.List{
							listplanmodifier.UseStateForUnknown(),
						},
					},
					"spare": schema.ListAttribute{
						Optional:    true,
						Computed:    true,
						ElementType: types.StringType,
						Description: disksDesc,
						PlanModifiers: []planmodifier.List{
							listplanmodifier.UseStateForUnknown(),
						},
					},
				},
			},
			"autotrim": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			// These pure-Computed attributes carry UseStateForUnknown so a plan
			// that makes no real change keeps their known state values instead
			// of proposing them as "(known after apply)". Without it, any
			// config edit that reconciles to a no-op (e.g. renaming a disk from
			// sdX to its serial, or "DISK" vs "STRIPE") would still show a
			// spurious in-place update of every computed field (issue #9). On a
			// genuine replace they are recomputed regardless.
			"guid":      schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"status":    schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"healthy":   schema.BoolAttribute{Computed: true, PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()}},
			"path":      schema.StringAttribute{Computed: true, PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"size":      schema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"free":      schema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
			"allocated": schema.Int64Attribute{Computed: true, PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}},
		},
	}
}

// vdevTypeDesc and disksDesc document the topology vdev fields, including the
// stable-identifier behavior added for issue #9.
const (
	vdevTypeDesc = `Vdev layout: "STRIPE" (single disk or JBOD), "MIRROR", or ` +
		`"RAIDZ1"/"RAIDZ2"/"RAIDZ3". "DISK" is accepted as an alias for ` +
		`"STRIPE" (it is the spelling TrueNAS's own query and UI use for a ` +
		`single-disk vdev).`
	disksDesc = `Disks in this vdev. Each may be given as the disk serial ` +
		`(recommended, e.g. "SMC0515D90925BQ85185"), the TrueNAS disk ` +
		`identifier, a /dev/disk/by-id path, or the kernel device name ` +
		`(sdX). The provider stores the stable serial in state and resolves ` +
		`any form to the same physical disk, so a kernel-name renumber ` +
		`across reboots does not plan a pool replacement.`
)

func datasourceSchema() dschema.Schema {
	return dschema.Schema{
		Description: "Fetches a TrueNAS ZFS pool by name.",
		Attributes: map[string]dschema.Attribute{
			"id":   dschema.Int64Attribute{Computed: true},
			"name": dschema.StringAttribute{Required: true, Description: "Pool name to look up."},
			"topology": dschema.SingleNestedAttribute{
				Computed: true,
				Attributes: map[string]dschema.Attribute{
					"data": dschema.ListNestedAttribute{
						Computed: true,
						NestedObject: dschema.NestedAttributeObject{
							Attributes: map[string]dschema.Attribute{
								"type":  dschema.StringAttribute{Computed: true},
								"disks": dschema.ListAttribute{Computed: true, ElementType: types.StringType},
							},
						},
					},
					"log": dschema.ListNestedAttribute{
						Computed: true,
						NestedObject: dschema.NestedAttributeObject{
							Attributes: map[string]dschema.Attribute{
								"type":  dschema.StringAttribute{Computed: true},
								"disks": dschema.ListAttribute{Computed: true, ElementType: types.StringType},
							},
						},
					},
					"cache": dschema.ListAttribute{Computed: true, ElementType: types.StringType},
					"spare": dschema.ListAttribute{Computed: true, ElementType: types.StringType},
				},
			},
			"autotrim":  dschema.BoolAttribute{Computed: true},
			"guid":      dschema.StringAttribute{Computed: true},
			"status":    dschema.StringAttribute{Computed: true},
			"healthy":   dschema.BoolAttribute{Computed: true},
			"path":      dschema.StringAttribute{Computed: true},
			"size":      dschema.Int64Attribute{Computed: true},
			"free":      dschema.Int64Attribute{Computed: true},
			"allocated": dschema.Int64Attribute{Computed: true},
		},
	}
}
