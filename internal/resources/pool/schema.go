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
		Description: "Manages a ZFS pool on TrueNAS SCALE.",
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
								"type":  schema.StringAttribute{Required: true},
								"disks": schema.ListAttribute{Required: true, ElementType: types.StringType},
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
								"type":  schema.StringAttribute{Required: true},
								"disks": schema.ListAttribute{Required: true, ElementType: types.StringType},
							},
						},
					},
					"cache": schema.ListAttribute{
						Optional:    true,
						Computed:    true,
						ElementType: types.StringType,
						PlanModifiers: []planmodifier.List{
							listplanmodifier.UseStateForUnknown(),
						},
					},
					"spare": schema.ListAttribute{
						Optional:    true,
						Computed:    true,
						ElementType: types.StringType,
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
			"guid":      schema.StringAttribute{Computed: true},
			"status":    schema.StringAttribute{Computed: true},
			"healthy":   schema.BoolAttribute{Computed: true},
			"path":      schema.StringAttribute{Computed: true},
			"size":      schema.Int64Attribute{Computed: true},
			"free":      schema.Int64Attribute{Computed: true},
			"allocated": schema.Int64Attribute{Computed: true},
		},
	}
}

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
