// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package kerberos_realm

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages a Kerberos realm on TrueNAS (kerberos.realm). Realms are normally populated " +
			"automatically during an Active Directory domain join, but can also be managed directly — e.g. for a " +
			"realm outside of any AD domain this system joins.",
		Attributes: map[string]schema.Attribute{
			"id": schema.Int64Attribute{
				Computed:    true,
				Description: "Numeric identifier of the Kerberos realm.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"realm": schema.StringAttribute{
				Required: true,
				Description: "Kerberos realm name. External to TrueNAS and case-sensitive; convention is " +
					"upper-case (e.g. \"EXAMPLE.COM\"). Updatable in place.",
			},
			"primary_kdc": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "The master Kerberos domain controller for this realm, used as a fallback when " +
					"TrueNAS cannot get credentials because of an invalid password (helpful in hub-and-spoke " +
					"topologies). Null (the default) if unset.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"kdc": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of Kerberos domain controllers for this realm. If empty (the default), the " +
					"Kerberos libraries use DNS to look up KDCs.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"admin_server": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of Kerberos admin servers for this realm. If empty (the default), the " +
					"Kerberos libraries use DNS to look them up.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"kpasswd_server": schema.ListAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				Description: "List of Kerberos kpasswd servers for this realm. If empty (the default), DNS is " +
					"used to look them up if needed.",
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}
