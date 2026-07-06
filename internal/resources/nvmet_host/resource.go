package nvmet_host

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &NVMetHostResource{}
var _ resource.ResourceWithImportState = &NVMetHostResource{}

// NVMetHostResource implements the truenas_nvmet_host resource.
type NVMetHostResource struct{ client *client.Client }

// NewResource returns a new NVMetHostResource.
func NewResource() resource.Resource { return &NVMetHostResource{} }

func (r *NVMetHostResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_nvmet_host"
}

func (r *NVMetHostResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *NVMetHostResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *NVMetHostResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan NVMetHostModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload, diags := plan.createPayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "nvmet.host.create", payload)
	if err != nil {
		resp.Diagnostics.AddError("Create NVMe-oF host failed", err.Error())
		return
	}

	// Parse create response to get the ID, then read back via get_instance.
	var created nvmetHostAPI
	if err := json.Unmarshal(raw, &created); err != nil {
		resp.Diagnostics.AddError("Parse create response", err.Error())
		return
	}

	raw, err = r.client.Call(ctx, "nvmet.host.get_instance", created.ID)
	if err != nil {
		resp.Diagnostics.AddError("Read-back after create failed", err.Error())
		return
	}

	var apiResp nvmetHostAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse get_instance response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *NVMetHostResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state NVMetHostModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "nvmet.host.get_instance", state.ID.ValueInt64())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read NVMe-oF host failed", err.Error())
		return
	}

	var apiResp nvmetHostAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *NVMetHostResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan NVMetHostModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Carry the existing state ID into the plan (plan.ID is Unknown until apply).
	var state NVMetHostModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID

	payload, diags := plan.updatePayload(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "nvmet.host.update", plan.ID.ValueInt64(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Update NVMe-oF host failed", err.Error())
		return
	}

	raw, err := r.client.Call(ctx, "nvmet.host.get_instance", plan.ID.ValueInt64())
	if err != nil {
		resp.Diagnostics.AddError("Read-back after update failed", err.Error())
		return
	}

	var apiResp nvmetHostAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse get_instance response", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *NVMetHostResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state NVMetHostModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "nvmet.host.delete", state.ID.ValueInt64())
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete NVMe-oF host failed", err.Error())
	}
}

func (r *NVMetHostResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	id, err := strconv.ParseInt(req.ID, 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Import ID must be an integer", req.ID)
		return
	}

	raw, err := r.client.Call(ctx, "nvmet.host.get_instance", id)
	if err != nil {
		resp.Diagnostics.AddError("Import NVMe-oF host failed", err.Error())
		return
	}

	var apiResp nvmetHostAPI
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	var state NVMetHostModel
	resp.Diagnostics.Append(responseToModel(ctx, &apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
