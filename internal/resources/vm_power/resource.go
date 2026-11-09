// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

// Package vm_power implements truenas_vm_power: the power state of an existing
// VM as its own resource, so it can be ordered after the VM's
// truenas_vm_device resources (usage log #1: `running = true` on truenas_vm
// boots a new VM before its disks and NICs exist, and the VM cannot depend on
// its own devices). It is the resource form of the truenas_vm_start,
// truenas_vm_stop and truenas_vm_restart actions, for OpenTofu, which has no
// actions.
package vm_power

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
	"github.com/truenas/terraform-provider-truenas/internal/listing"
)

var _ resource.Resource = &VMPowerResource{}
var _ resource.ResourceWithImportState = &VMPowerResource{}
var _ resource.ResourceWithIdentity = &VMPowerResource{}

// VMPowerResource implements the truenas_vm_power resource.
type VMPowerResource struct{ client *client.Client }

// NewResource returns a new VMPowerResource.
func NewResource() resource.Resource { return &VMPowerResource{} }

// VMPowerModel is the Terraform model for truenas_vm_power.
type VMPowerModel struct {
	ID              types.String `tfsdk:"id"`
	VMID            types.Int64  `tfsdk:"vm_id"`
	Running         types.Bool   `tfsdk:"running"`
	Overcommit      types.Bool   `tfsdk:"overcommit"`
	RestartTriggers types.Map    `tfsdk:"restart_triggers"`
	Status          types.String `tfsdk:"status"`
}

func (r *VMPowerResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vm_power"
}

func (r *VMPowerResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func resourceSchema() schema.Schema {
	return schema.Schema{
		Description: "Manages the power state of an existing VM (vm.start / vm.stop / vm.restart) as its own resource, " +
			"so it can be applied after the VM's truenas_vm_device resources: a new VM then boots with its disks and " +
			"NICs attached. Leave `running` unset on the truenas_vm. Works in OpenTofu, which has no actions. " +
			"Destroying this resource leaves the VM in its current state; the truenas_vm's own destroy stops it.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "The VM id as a string.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"vm_id": schema.Int64Attribute{
				Required:      true,
				Description:   "ID of the VM (truenas_vm.<name>.id). Changing it moves the resource to another VM (replacement, no API call for the old VM).",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"running": schema.BoolAttribute{
				Required:    true,
				Description: "Desired power state: true starts the VM (vm.start), false stops it (vm.stop, graceful with force after the VM's shutdown timeout). Read back from the VM's status, so a VM stopped or started outside Terraform plans its way back.",
			},
			"overcommit": schema.BoolAttribute{
				Optional:    true,
				Description: "Allow vm.start even if it would overcommit host memory. Default false.",
			},
			"restart_triggers": schema.MapAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Arbitrary strings; when any value changes while running = true, the VM is restarted (vm.restart) so a change the guest only picks up at boot takes effect, for example `{ disk = truenas_vm_device.disk.attributes, memory = tostring(truenas_vm.x.memory) }`. Only compared, never sent to the API.",
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "VM status as reported by vm.status after the last apply or refresh: RUNNING, STOPPED, SUSPENDED.",
			},
		},
	}
}

func (r *VMPowerResource) IdentitySchema(_ context.Context, _ resource.IdentitySchemaRequest, resp *resource.IdentitySchemaResponse) {
	resp.IdentitySchema = listing.IntIDIdentitySchema()
}

func (r *VMPowerResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

// vmStatus is the subset of vm.status the resource reads.
type vmStatus struct {
	State string `json:"state"`
}

// status returns the VM's run state; a missing VM is reported with an error
// client.IsNotFound recognises.
func (r *VMPowerResource) status(ctx context.Context, id int64) (string, error) {
	raw, err := r.client.CallRead(ctx, "vm.status", id)
	if err != nil {
		return "", err
	}
	var st vmStatus
	if err := json.Unmarshal(raw, &st); err != nil {
		return "", fmt.Errorf("parse vm.status: %w", err)
	}
	return st.State, nil
}

// converge starts or stops the VM to match want, and returns the state read
// back afterwards. Starting an already running VM and stopping a stopped one
// are no-ops, so a concurrent change outside Terraform does not fail the apply.
func (r *VMPowerResource) converge(ctx context.Context, id int64, want, overcommit bool) (string, diag.Diagnostics) {
	var diags diag.Diagnostics
	state, err := r.status(ctx, id)
	if err != nil {
		diags.AddError("Read VM status failed", err.Error())
		return "", diags
	}
	running := state == "RUNNING"
	switch {
	case want && !running:
		if _, err := r.client.Call(ctx, "vm.start", id, map[string]any{"overcommit": overcommit}); err != nil {
			diags.AddError("Start VM failed", err.Error())
			return "", diags
		}
	case !want && running:
		if _, err := r.client.CallJob(ctx, "vm.stop", id, map[string]any{"force": false, "force_after_timeout": true}); err != nil {
			diags.AddError("Stop VM failed", err.Error())
			return "", diags
		}
	}
	state, err = r.status(ctx, id)
	if err != nil {
		diags.AddError("Read VM status after power change failed", err.Error())
		return "", diags
	}
	if (state == "RUNNING") != want {
		diags.AddError("VM power state did not change",
			fmt.Sprintf("VM %d reports %s after the power change; wanted running = %t.", id, state, want))
	}
	return state, diags
}

func (r *VMPowerResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan VMPowerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := plan.VMID.ValueInt64()
	state, diags := r.converge(ctx, id, plan.Running.ValueBool(), plan.Overcommit.ValueBool())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = types.StringValue(strconv.FormatInt(id, 10))
	plan.Status = types.StringValue(state)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, id)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *VMPowerResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state VMPowerModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.VMID.ValueInt64()
	st, err := r.status(ctx, id)
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read VM status failed", err.Error())
		return
	}
	state.Status = types.StringValue(st)
	state.Running = types.BoolValue(st == "RUNNING")
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, id)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *VMPowerResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state VMPowerModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.VMID.ValueInt64()
	want := plan.Running.ValueBool()
	// A restart is due when the triggers changed and the VM is running and
	// stays running: a VM started or stopped by this update needs none.
	restart := want && state.Running.ValueBool() && !plan.RestartTriggers.Equal(state.RestartTriggers)
	st, diags := r.converge(ctx, id, want, plan.Overcommit.ValueBool())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if restart && st == "RUNNING" {
		if _, err := r.client.CallJob(ctx, "vm.restart", id); err != nil {
			resp.Diagnostics.AddError("Restart VM failed", err.Error())
			return
		}
		var err error
		if st, err = r.status(ctx, id); err != nil {
			resp.Diagnostics.AddError("Read VM status after restart failed", err.Error())
			return
		}
	}
	plan.ID = state.ID
	plan.Status = types.StringValue(st)
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, id)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete only forgets the resource: the VM's power state is left as it is.
// Stopping a VM because its power resource was removed from the configuration
// would be a surprise, and the truenas_vm's own destroy stops it anyway.
func (r *VMPowerResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {}

func (r *VMPowerResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, idDiags := listing.ImportInt64(ctx, req)
	resp.Diagnostics.Append(idDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	st, err := r.status(ctx, id)
	if err != nil {
		resp.Diagnostics.AddError("Import VM power state failed", err.Error())
		return
	}
	state := VMPowerModel{
		ID:              types.StringValue(strconv.FormatInt(id, 10)),
		VMID:            types.Int64Value(id),
		Running:         types.BoolValue(st == "RUNNING"),
		Status:          types.StringValue(st),
		RestartTriggers: types.MapNull(types.StringType),
	}
	resp.Diagnostics.Append(listing.SetIdentity(ctx, resp.Identity, id)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
