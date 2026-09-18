// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package kerberos_config

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages the TrueNAS Kerberos configuration (kerberos.config) — the [appdefaults]/" +
			"[libdefaults] free-form additions to krb5.conf. This is a singleton resource — there is exactly one " +
			"Kerberos configuration per TrueNAS system, so it is never created or deleted on TrueNAS; Terraform " +
			"create/update calls kerberos.update, and Terraform delete only removes the resource from state (the " +
			"configuration is left in place).",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"kerberos_config\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"appdefaults_aux": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Advanced field for manually setting additional parameters inside the appdefaults " +
					"section of krb5.conf. Generally not required — the necessary krb5.conf settings are " +
					"automatically detected and set for the environment. See the MIT krb5.conf manpage.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"libdefaults_aux": schema.StringAttribute{
				Optional: true,
				Computed: true,
				Description: "Advanced field for manually setting additional parameters inside the libdefaults " +
					"section of krb5.conf. Generally not required — the necessary krb5.conf settings are " +
					"automatically detected and set for the environment. See the MIT krb5.conf manpage.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
		},
	}
}
