// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package ipmi_lan

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// settlePollInterval and settlePollAttempts bound lookupChannelSettled's
// retry budget (~24s worst case): probed live against the disposable
// Enterprise HA test box, a static (dhcp=false) ipmi.lan.update call that
// succeeds and even round-trips a matching value from the immediately-next
// ipmi.lan.query can still be followed by a read a moment later reporting
// "ip_address"/"subnet_mask" as transient "0.0.0.0" — real BMC LAN
// controller settle-time behavior after any config change (observed after
// a "vlan" change; not a client bug) — before it stabilizes back to the
// values just sent. A single retry after a short sleep was sufficient in
// that observed case; this budget gives real hardware several chances
// rather than surfacing a spurious "Provider produced inconsistent result
// after apply" from a transient read.
const (
	settlePollInterval = 3 * time.Second
	settlePollAttempts = 8
)

var _ resource.Resource = &IPMILanResource{}
var _ resource.ResourceWithImportState = &IPMILanResource{}

// IPMILanResource implements the truenas_ipmi_lan resource: the LAN
// configuration of one pre-existing physical BMC/IPMI channel. See
// schema.go's resourceSchema Description for the full safety contract.
type IPMILanResource struct{ client *client.Client }

// NewResource returns a new IPMILanResource.
func NewResource() resource.Resource { return &IPMILanResource{} }

func (r *IPMILanResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ipmi_lan"
}

func (r *IPMILanResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *IPMILanResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data",
			fmt.Sprintf("expected *client.Client, got %T", req.ProviderData),
		)
		return
	}
	r.client = c
}

// lookupChannel finds a channel's current config via ipmi.lan.query
// (ipmi.lan has no get_instance method — see model.go's doc comments for
// the full probed query/update accepts schemas). Mirrors the boot_
// environment package's lookupByName pattern: a filtered query plus a
// synthesized client.IsNotFound-compatible error when the channel is
// absent, since a query returning zero results is not itself an API error.
func (r *IPMILanResource) lookupChannel(ctx context.Context, channel int64) (*ipmiLanAPI, error) {
	raw, err := r.client.CallRead(ctx, "ipmi.lan.query", ipmiLanQueryArgs(channel))
	if err != nil {
		return nil, err
	}
	var results []ipmiLanAPI
	if err := json.Unmarshal(raw, &results); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, &client.APIError{Code: 2, Message: fmt.Sprintf("IPMI LAN channel %d not found", channel)}
	}
	return &results[0], nil
}

// lookupChannelSettled calls lookupChannel after ipmi.lan.update, retrying
// for up to settlePollAttempts*settlePollInterval when the payload just
// sent configured a static address (wantIP/wantNetmask non-empty) and the
// freshly-read value doesn't match yet — see settlePollInterval's doc
// comment for the live-probed BMC settle-time behavior this works around.
// When wantIP/wantNetmask are empty (dhcp = true, where there is no
// deterministic target to converge to — the BMC assigns whatever its DHCP
// server hands out) it returns the very first read unconditionally.
func (r *IPMILanResource) lookupChannelSettled(ctx context.Context, channel int64, wantIP, wantNetmask string) (*ipmiLanAPI, error) {
	var api *ipmiLanAPI
	var err error
	for attempt := 1; attempt <= settlePollAttempts; attempt++ {
		api, err = r.lookupChannel(ctx, channel)
		if err != nil {
			return nil, err
		}
		if wantIP == "" && wantNetmask == "" {
			return api, nil
		}
		if api.IPAddress == wantIP && api.SubnetMask == wantNetmask {
			return api, nil
		}
		if attempt < settlePollAttempts {
			time.Sleep(settlePollInterval)
		}
	}
	// Best effort: return the last read even if it never fully converged,
	// rather than fail the apply outright — the next Read/refresh will
	// pick up the box's true settled state either way.
	return api, nil
}

// transientZeroRead reports whether a freshly-read channel is showing the
// BMC LAN controller's all-zero settle-time transient rather than a real
// steady state: ip_address_source is NOT "dhcp" (a DHCP channel legitimately
// has whatever its server hands out, including transiently nothing) yet
// ip_address or subnet_mask reads the all-zero "0.0.0.0".
//
// Live trace (.68 channel 1, immediately after an ipmi.lan.update that
// changed "vlan"): the controller FLAPS for ~10-15s, cycling through BOTH
// {"static","0.0.0.0"} AND {"unspecified","0.0.0.0"} reads before settling
// back to its real {"static","192.168.1.103"} — see settlePollInterval's doc
// comment. So the transient is not confined to a "static" source; it must be
// recognized under "unspecified" too.
//
// This predicate alone cannot tell that transient apart from a channel that
// is GENUINELY unconfigured — channel 8 on .68 rests permanently at
// {"unspecified","0.0.0.0"} (see model.go's isDHCP doc comment). That
// ambiguity is resolved by the caller, not here: lookupChannelReadSettled
// only waits it out when it has a reason to expect a real address (its
// waitOutZero argument), so a genuinely-unconfigured channel is never made to
// poll on every ordinary refresh.
func transientZeroRead(api *ipmiLanAPI) bool {
	if isDHCP(api.IPAddressSource) {
		return false
	}
	return api.IPAddress == "0.0.0.0" || api.SubnetMask == "0.0.0.0"
}

// lookupChannelReadSettled reads a channel for Read/refresh/import, optionally
// retrying past the all-zero settle transient (transientZeroRead) on the same
// budget as lookupChannelSettled.
//
// waitOutZero MUST be set only when the caller has a reason to expect a real
// (non-zero) address to converge to, so a genuinely-unconfigured channel that
// legitimately rests at "0.0.0.0" (e.g. .68's channel 8) is never made to
// burn the full retry budget on an ordinary refresh:
//   - Read passes true only when the PRIOR Terraform state was a configured
//     static channel (a real ip in state that just read back all-zero can
//     only be the transient).
//   - ImportState passes true unconditionally: it has no prior state, so it
//     waits out a possible flap once; a truly-unconfigured channel simply
//     costs one full budget at import time (a rare, manual operation).
//
// It converges on ANY settled non-zero read, not a specific target, so an
// out-of-band change to a different static address is still surfaced as drift
// (that read is non-zero, so it returns immediately) rather than masked.
func (r *IPMILanResource) lookupChannelReadSettled(ctx context.Context, channel int64, waitOutZero bool) (*ipmiLanAPI, error) {
	var api *ipmiLanAPI
	var err error
	for attempt := 1; attempt <= settlePollAttempts; attempt++ {
		api, err = r.lookupChannel(ctx, channel)
		if err != nil {
			return nil, err
		}
		if !waitOutZero || !transientZeroRead(api) {
			return api, nil
		}
		if attempt < settlePollAttempts {
			time.Sleep(settlePollInterval)
		}
	}
	// Best effort: return the last read even if it never converged, rather
	// than fail — the next refresh picks up the box's true settled state.
	return api, nil
}

// priorStateIsStatic reports whether a channel's prior Terraform state
// described a configured static address, so a subsequent all-zero read can
// only be the settle transient (transientZeroRead) rather than the channel's
// real state. See lookupChannelReadSettled's waitOutZero contract.
func priorStateIsStatic(state *IPMILanModel) bool {
	if state.DHCP.ValueBool() {
		return false
	}
	ip := state.IPAddress.ValueString()
	return ip != "" && ip != "0.0.0.0"
}

func (r *IPMILanResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan IPMILanModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// write-only: the framework nulls WriteOnly attributes (password) in
	// req.Plan, so the actual value is only available via req.Config.
	// updatePayload must also be built from config, not plan, for every
	// other field too (config-driven payload contract — see its doc
	// comment): Optional+Computed fields echo the prior state's value into
	// the plan via UseStateForUnknown even when the user never set them,
	// but this is a brand-new resource on Create, so req.Config and
	// req.Plan agree for those fields regardless; req.Config is still the
	// single source of truth applied consistently with Update below.
	var config IPMILanModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Password = config.Password

	payload, err := config.updatePayload()
	if err != nil {
		resp.Diagnostics.AddError("Invalid configuration", err.Error())
		return
	}

	channel := config.Channel.ValueInt64()
	if _, err := r.client.Call(ctx, "ipmi.lan.update", channel, payload); err != nil {
		resp.Diagnostics.AddError("Update IPMI LAN channel failed", err.Error())
		return
	}

	wantIP, _ := payload["ipaddress"].(string)
	wantNetmask, _ := payload["netmask"].(string)
	api, err := r.lookupChannelSettled(ctx, channel, wantIP, wantNetmask)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}

	responseToModel(api, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *IPMILanResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state IPMILanModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	api, err := r.lookupChannelReadSettled(ctx, state.Channel.ValueInt64(), priorStateIsStatic(&state))
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read IPMI LAN channel failed", err.Error())
		return
	}

	responseToModel(api, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *IPMILanResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan IPMILanModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state IPMILanModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// "channel" is RequiresReplace, so plan.Channel always equals
	// state.Channel here.

	// write-only + config-driven payload: see Create's comment above.
	var config IPMILanModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.Password = config.Password

	// See vlanCannotBeCleared's doc comment in model.go (and updatePayload's,
	// above it) for the full reasoning: Terraform cannot distinguish an
	// explicit `vlan = null` from "vlan" simply never being managed in
	// config, at any layer (config, plan, or otherwise), so this resource
	// never guesses — it refuses the apply with an actionable diagnostic
	// instead of either silently ignoring the user's clear request or
	// silently clearing a tag nobody asked to touch (both of which were
	// tried and rejected; the latter also produced a live-confirmed
	// "Provider produced inconsistent result after apply" crash, since
	// Terraform's own planned value for "vlan" stays the prior non-null
	// state in this exact situation).
	if vlanCannotBeCleared(state.Vlan, config.Vlan) {
		resp.Diagnostics.AddError(
			"Cannot clear \"vlan\" via Terraform",
			"This channel's prior state has a non-null \"vlan\", but this apply's configuration does not "+
				"set it. Terraform's Optional+Computed attribute semantics make an explicit `vlan = null` "+
				"indistinguishable from \"vlan\" simply never being managed in configuration at all, so this "+
				"provider can never safely infer that you want to clear it — silently guessing either way "+
				"was tried and rejected (guessing \"clear\" produces a \"Provider produced inconsistent "+
				"result after apply\" error, since Terraform's own plan keeps \"vlan\" pinned to its prior "+
				"value in both cases).\n\n"+
				"If you want this resource to keep managing \"vlan\" unchanged going forward (so this error "+
				"stops appearing on unrelated changes), set `vlan = <its current value>` explicitly in "+
				"configuration.\n\n"+
				"To actually clear the VLAN tag on the BMC, call ipmi.lan.update directly (TrueNAS UI, API, "+
				"or midclient) with an explicit \"vlan\": null, then run `terraform apply -refresh-only` (or "+
				"a plan/apply cycle) to reconcile Terraform state with the change. `terraform apply -replace` "+
				"does NOT achieve this on its own: Create() treats a null \"vlan\" the same safe way — "+
				"leaving whatever the hardware already has untouched — for the identical reason.",
		)
		return
	}

	payload, err := config.updatePayload()
	if err != nil {
		resp.Diagnostics.AddError("Invalid configuration", err.Error())
		return
	}

	channel := plan.Channel.ValueInt64()
	if _, err := r.client.Call(ctx, "ipmi.lan.update", channel, payload); err != nil {
		resp.Diagnostics.AddError("Update IPMI LAN channel failed", err.Error())
		return
	}

	wantIP, _ := payload["ipaddress"].(string)
	wantNetmask, _ := payload["netmask"].(string)
	api, err := r.lookupChannelSettled(ctx, channel, wantIP, wantNetmask)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	responseToModel(api, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete makes NO client calls: an IPMI LAN channel is a fixed, pre-
// existing piece of hardware (see schema.go's resourceSchema Description),
// so there is no sane "delete" semantics — resetting it to some default
// configuration on Terraform delete could leave the BMC unreachable, which
// this resource must never do. It only emits a warning diagnostic;
// Terraform itself drops the resource from state once Delete returns.
func (r *IPMILanResource) Delete(_ context.Context, _ resource.DeleteRequest, resp *resource.DeleteResponse) {
	resp.Diagnostics.AddWarning(
		"IPMI LAN configuration left in place",
		"IPMI LAN channel configuration left in place on the BMC; removed from Terraform state only.",
	)
}

func (r *IPMILanResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	channel, err := parseChannel(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Import ID must be an integer channel number", err.Error())
		return
	}

	api, err := r.lookupChannelReadSettled(ctx, channel, true)
	if err != nil {
		resp.Diagnostics.AddError("Import IPMI LAN channel failed", err.Error())
		return
	}

	var state IPMILanModel
	responseToModel(api, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
