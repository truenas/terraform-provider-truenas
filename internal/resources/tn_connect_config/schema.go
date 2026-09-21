// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package tn_connect_config

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages the TrueNAS Connect service configuration (tn_connect.config/" +
			"tn_connect.update): whether the system is enrolled with the TrueNAS Connect cloud service, plus its " +
			"read-only enrollment/status metadata. This is a singleton resource — there is exactly one TrueNAS " +
			"Connect configuration per TrueNAS system, so it is never created or deleted on TrueNAS; Terraform " +
			"create/update calls tn_connect.update, and Terraform delete only removes the resource from state " +
			"(the configuration is left in place)." +
			"\n\n" +
			"SAFETY (read before setting \"enabled\"): setting \"enabled\" to true starts TrueNAS Connect cloud " +
			"enrollment — a real, external side effect (this system registers with the TrueNAS Connect account " +
			"service and begins periodic heartbeat reporting), not a mere local configuration change. This " +
			"provider's own tests never set \"enabled\" to true and never exercise the enrollment flow (claim " +
			"token generation, registration URI) at all — those are out of scope for this resource. Leave " +
			"\"enabled\" unconfigured to manage this resource purely for its read-only status fields without " +
			"asserting any particular enrollment state." +
			"\n\n" +
			"\"enabled\" is the ONLY field this resource ever writes. Probed live: on TrueNAS 26.0, " +
			"tn_connect.update's own accepts schema exposes EXACTLY \"enabled\" — there is no other writable " +
			"field on that release at all. TrueNAS 25.10's tn_connect.update additionally accepts \"ips\", " +
			"\"interfaces\", and \"use_all_interfaces\", but this resource deliberately does not expose them as " +
			"writable, keeping one stable schema across releases that matches the narrower (and " +
			"production-targeted) TrueNAS 26.0 capability rather than branching resource behavior by release. " +
			"Every other attribute below is Computed-only, sourced from tn_connect.config." +
			"\n\n" +
			"Several attributes exist on only one of the two probed releases and read as null on the other " +
			"(distinct from a present-but-empty value): \"tier\" and \"last_heartbeat_failure_datetime\" exist " +
			"only on TrueNAS 26.0; \"ips\", \"interfaces\", \"interfaces_ips\", and \"use_all_interfaces\" exist " +
			"only on TrueNAS 25.10.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"tn_connect_config\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"enabled": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Whether the TrueNAS Connect cloud service is enabled. SAFETY: setting this to " +
					"true starts TrueNAS Connect cloud enrollment (see the resource description above) — a " +
					"real, only-reversible-by-disabling-again external side effect. This provider's own tests " +
					"never set this to true. Leave unconfigured (the default) to manage this resource purely " +
					"for its read-only status fields, or set explicitly to false to ensure TrueNAS Connect " +
					"stays disabled. Change deliberately.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"status": schema.StringAttribute{
				Computed:      true,
				Description:   "Current operational status of the TrueNAS Connect service (e.g. DISABLED, CONFIGURED).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"status_reason": schema.StringAttribute{
				Computed:      true,
				Description:   "Human-readable explanation of the current status.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"certificate": schema.Int64Attribute{
				Computed:      true,
				Description:   "ID of the SSL certificate used for TrueNAS Connect communications. Null when using the default.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"account_service_base_url": schema.StringAttribute{
				Computed:      true,
				Description:   "Base URL for the TrueNAS Connect account service API.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"leca_service_base_url": schema.StringAttribute{
				Computed:      true,
				Description:   "Base URL for the Let's Encrypt Certificate Authority service used by TrueNAS Connect.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"tnc_base_url": schema.StringAttribute{
				Computed:      true,
				Description:   "Base URL for the TrueNAS Connect service.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"heartbeat_url": schema.StringAttribute{
				Computed:      true,
				Description:   "URL endpoint for sending heartbeat signals to maintain connection status.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"registration_details": schema.StringAttribute{
				Computed: true,
				Description: "Registration information and credentials for TrueNAS Connect, as a JSON-encoded " +
					"object (\"{}\" when not enrolled).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"tier": schema.StringAttribute{
				Computed: true,
				Description: "TrueNAS Connect tier (FOUNDATION, PLUS, or BUSINESS). Only present on TrueNAS " +
					"26.0+; reads as null on TrueNAS 25.10 (probed live: the API response has no \"tier\" key " +
					"at all on that release).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"last_heartbeat_failure_datetime": schema.StringAttribute{
				Computed: true,
				Description: "Datetime the current heartbeat failure streak began, or null if heartbeat is " +
					"not currently failing. Only present on TrueNAS 26.0+; reads as null on TrueNAS 25.10 (probed " +
					"live: the API response has no \"last_heartbeat_failure_datetime\" key at all on that " +
					"release).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"ips": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "IP addresses TrueNAS Connect binds to and advertises. Only present on TrueNAS " +
					"25.10; reads as null on TrueNAS 26.0 (probed live: the API response has no \"ips\" key at " +
					"all on that release — TrueNAS 26.0 selects addresses automatically instead).",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"interfaces": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Network interface names TrueNAS Connect uses. Only present on TrueNAS 25.10; " +
					"reads as null on TrueNAS 26.0 (probed live: the API response has no \"interfaces\" key at " +
					"all on that release).",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"interfaces_ips": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "IP addresses associated with the selected interfaces. Only present on TrueNAS " +
					"25.10; reads as null on TrueNAS 26.0 (probed live: the API response has no " +
					"\"interfaces_ips\" key at all on that release).",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"use_all_interfaces": schema.BoolAttribute{
				Computed: true,
				Description: "Whether TrueNAS Connect automatically uses all available network interfaces. " +
					"Only present on TrueNAS 25.10; reads as null on TrueNAS 26.0 (probed live: the API response " +
					"has no \"use_all_interfaces\" key at all on that release).",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
		},
	}
}
