// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package failover_config

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages the TrueNAS Enterprise HA failover configuration (failover.config/" +
			"failover.update): whether failover is administratively disabled, which node in the chassis is " +
			"currently marked master, and the failover timeout. This is a singleton resource — there is exactly " +
			"one failover configuration per TrueNAS system, so it is never created or deleted on TrueNAS; " +
			"Terraform create/update calls failover.update, and Terraform delete only removes the resource from " +
			"state (the configuration is left in place). failover.config reads cleanly even on a box that isn't " +
			"licensed for Enterprise HA (probed live), so this resource has no version or license gate." +
			"\n\n" +
			"SAFETY (read before setting \"disabled\" or \"master\"): unlike \"timeout\", which is a cosmetic " +
			"wait-time setting, \"disabled\" and \"master\" are direct controls over live HA state. Setting " +
			"\"disabled\" to true administratively disables failover on this system — a real, only-reversible-by-" +
			"disabling-again change to HA readiness. Setting \"master\" to a value that differs from the node's " +
			"current live state is a genuine failover trigger, not a cosmetic toggle (probed live against a " +
			"disposable Enterprise HA pair — see the provider's task report for the full transcript of a " +
			"real, controlled failover exercised outside of any committed test). This provider's own committed " +
			"acceptance tests only ever exercise \"timeout\"; they never set \"disabled\" or \"master\" to a " +
			"value that differs from the box's live state. Leave \"disabled\" and \"master\" unconfigured to " +
			"manage this resource purely for its \"timeout\" field and read-only status without asserting any " +
			"particular HA state, and change them deliberately, understanding the side effect, when you do set " +
			"them.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Fixed identifier for this singleton resource: always \"failover_config\".",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"disabled": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "When true, HA failover is administratively disabled on this system. SAFETY: this " +
					"is a real change to HA readiness, not a cosmetic setting (see the resource description " +
					"above). This provider's own tests never set this field. Leave unconfigured (the default) " +
					"to manage this resource without asserting a particular HA-enabled state. Change " +
					"deliberately.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"master": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Description: "Marks this node in the chassis as the master (active) node; the standby node has " +
					"the opposite value. SAFETY: setting this to a value that differs from the node's current " +
					"live state is a genuine failover trigger (probed live — see the resource description " +
					"above), not a cosmetic toggle. This provider's own tests never set this field. Leave " +
					"unconfigured (the default) to manage this resource without asserting a particular " +
					"mastership state. Change deliberately, and only against a system where you intend the " +
					"resulting failover." +
					"\n\n" +
					"NOTE, probed live during a real controlled failover: this field can read false on the node " +
					"that failover.status simultaneously reports as MASTER, immediately after a failover " +
					"triggered by failover.become_passive on the peer — it did not visibly flip to true on the " +
					"newly-active node even once failover.disabled.reasons had fully cleared. Use the " +
					"truenas_failover_config datasource's \"status\"/\"node\" attributes (failover.status/" +
					"failover.node), not this field, to determine which node is currently active.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"timeout": schema.Int64Attribute{
				Optional: true,
				Computed: true,
				Description: "Time to wait, in seconds, before a failover occurs after a network event on an " +
					"interface marked critical for failover (while HA is enabled and working normally). Cosmetic " +
					"and safe to change — does not itself affect \"disabled\" or \"master\" (probed live: sending " +
					"only \"timeout\" leaves both untouched). Defaults to the box's current value if left " +
					"unconfigured.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
		},
	}
}
