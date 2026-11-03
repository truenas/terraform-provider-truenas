// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package pool

import (
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	dschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: poolResourceDesc,
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed: true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			// name and topology are effectively immutable, but they are
			// deliberately NOT marked RequiresReplace: replacing a pool means
			// destroying it and all its data. PoolResource.ModifyPlan refuses a
			// post-create change to either with an actionable error instead of
			// ever planning a destroy/recreate. See its doc comment.
			"name": schema.StringAttribute{
				Required: true,
			},
			"topology": schema.SingleNestedAttribute{
				Required: true,
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
			// Root-dataset properties (coverage audit): set on the pool's root
			// dataset at creation, read back from it (pool.query returns null).
			"deduplication": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Deduplication on the pool's root dataset (inherited by children): ON, VERIFY, or OFF. Reads back null when inherited/default.",
				Validators:  []validator.String{stringvalidator.OneOf("ON", "VERIFY", "OFF")},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"checksum": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Checksum algorithm on the pool's root dataset: ON, OFF, FLETCHER2, FLETCHER4, SHA256, SHA512, SKEIN, EDONR, or BLAKE3. Reads back null when inherited/default.",
				Validators:  []validator.String{stringvalidator.OneOf("ON", "OFF", "FLETCHER2", "FLETCHER4", "SHA256", "SHA512", "SKEIN", "EDONR", "BLAKE3")},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			// These pure-Computed attributes carry UseStateForUnknown so a plan
			// that makes no real change keeps their known state values instead
			// of proposing them as "(known after apply)". Without it, any
			// config edit that reconciles to a no-op (e.g. renaming a disk from
			// sdX to its serial, or "DISK" vs "STRIPE") would still show a
			// spurious in-place update of every computed field (issue #9). On a
			// genuine replace they are recomputed regardless.
			"force_destroy": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				Description: "When true, `terraform destroy` (or removing this resource from the configuration) " +
					"destroys the pool and everything on it (`pool.export` with destroy). Default false: the " +
					"destroy is refused while the pool holds any dataset, zvol, snapshot or file, and the error " +
					"lists what it found. Changing it updates state only; set it and apply before the destroy " +
					"that needs it. Import sets it to false.",
			},
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

// poolResourceDesc, vdevTypeDesc and disksDesc document the pool resource and
// its topology fields, including the data-safety contract and the
// stable-identifier behavior.
const (
	poolResourceDesc = "Manages a ZFS pool on TrueNAS.\n\n" +
		"**Data safety — this resource never plans a pool destroy/recreate.** " +
		"`name` and `topology` are treated as immutable after creation: a change " +
		"to either is refused at plan time rather than replacing (and thereby " +
		"erasing) the pool. To make a real topology change — grow the pool, " +
		"replace a disk, or add/remove a cache/log/spare — do it in the TrueNAS " +
		"UI or with `zpool`, then run `terraform apply -refresh-only` to " +
		"reconcile state. A deliberate teardown is `terraform destroy`. Adding " +
		"`lifecycle { prevent_destroy = true }` is recommended as " +
		"defense-in-depth.\n\n" +
		"**Disk failures are not configuration changes.** If a member faults, a " +
		"hot spare activates, or a disk is physically removed, the pool still " +
		"reconciles to your configuration and plans no change — provided disks " +
		"are pinned by their stable serial (recommended; see `disks`)."
	vdevTypeDesc = `Vdev layout: "STRIPE" (single disk or JBOD), "MIRROR", or ` +
		`"RAIDZ1"/"RAIDZ2"/"RAIDZ3". "DISK" is accepted as an alias for ` +
		`"STRIPE" (it is the spelling TrueNAS's own query and UI use for a ` +
		`single-disk vdev).`
	disksDesc = `Disks in this vdev. Each may be given as the disk serial ` +
		`(recommended, e.g. "SMC0515D90925BQ85185"), the TrueNAS disk ` +
		`identifier, a /dev/disk/by-id path, or the kernel device name (sdX). ` +
		`The provider resolves any of these to the same physical disk at plan ` +
		`time, so pinning disks by serial keeps a configuration stable across ` +
		`device renumbering and through a failed, removed, or spare-covered ` +
		`disk. State reflects the current kernel device name, not the ` +
		`configured form.`
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
			"autotrim":      dschema.BoolAttribute{Computed: true},
			"deduplication": dschema.StringAttribute{Computed: true},
			"checksum":      dschema.StringAttribute{Computed: true},
			"guid":          dschema.StringAttribute{Computed: true},
			"status":        dschema.StringAttribute{Computed: true},
			"healthy":       dschema.BoolAttribute{Computed: true},
			"path":          dschema.StringAttribute{Computed: true},
			"size":          dschema.Int64Attribute{Computed: true},
			"free":          dschema.Int64Attribute{Computed: true},
			"allocated":     dschema.Int64Attribute{Computed: true},
		},
	}
}
