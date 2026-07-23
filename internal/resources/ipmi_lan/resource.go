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

	api, err := r.lookupChannel(ctx, state.Channel.ValueInt64())
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

	api, err := r.lookupChannel(ctx, channel)
	if err != nil {
		resp.Diagnostics.AddError("Import IPMI LAN channel failed", err.Error())
		return
	}

	var state IPMILanModel
	responseToModel(api, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
