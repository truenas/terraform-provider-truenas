package dataset

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/truenas/terraform-provider-truenas/internal/client"
)

var _ resource.Resource = &DatasetResource{}
var _ resource.ResourceWithImportState = &DatasetResource{}

type DatasetResource struct {
	client *client.Client
}

func NewResource() resource.Resource { return &DatasetResource{} }

func (r *DatasetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dataset"
}

func (r *DatasetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *DatasetResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	c, ok := req.ProviderData.(*client.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data type",
			fmt.Sprintf("expected *client.Client, got %T", req.ProviderData))
		return
	}
	r.client = c
}

func (r *DatasetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DatasetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.Call(ctx, "pool.dataset.create", plan.apiPayload())
	if err != nil {
		resp.Diagnostics.AddError("Create dataset failed", err.Error())
		return
	}

	// Read back via get_instance for canonical state (create response omits some properties).
	raw, err := r.client.Call(ctx, "pool.dataset.get_instance", plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read after create failed", err.Error())
		return
	}

	var apiResp apiResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse read response", err.Error())
		return
	}

	resp.Diagnostics.Append(r.responseToModel(&apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DatasetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DatasetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	raw, err := r.client.Call(ctx, "pool.dataset.get_instance", state.Name.ValueString())
	if err != nil {
		if client.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Read dataset failed", err.Error())
		return
	}

	var apiResp apiResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse read response", err.Error())
		return
	}

	resp.Diagnostics.Append(r.responseToModel(&apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DatasetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DatasetModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := plan.apiPayload()
	delete(payload, "name") // name is not updatable via update endpoint

	_, err := r.client.Call(ctx, "pool.dataset.update", plan.Name.ValueString(), payload)
	if err != nil {
		resp.Diagnostics.AddError("Update dataset failed", err.Error())
		return
	}

	// Re-read to get computed fields
	raw, err := r.client.Call(ctx, "pool.dataset.get_instance", plan.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Read after update failed", err.Error())
		return
	}

	var apiResp apiResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse update response", err.Error())
		return
	}

	resp.Diagnostics.Append(r.responseToModel(&apiResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *DatasetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DatasetModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.CallJob(ctx, "pool.dataset.delete", state.Name.ValueString(),
		map[string]any{"recursive": true})
	if err != nil && !client.IsNotFound(err) {
		resp.Diagnostics.AddError("Delete dataset failed", err.Error())
	}
}

func (r *DatasetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	// Import by dataset name (e.g. "terraform import truenas_dataset.example tank/mydata")
	var state DatasetModel
	state.Name = types.StringValue(req.ID)
	state.ID = types.StringValue(req.ID)

	raw, err := r.client.Call(ctx, "pool.dataset.get_instance", req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Import dataset failed", err.Error())
		return
	}

	var apiResp apiResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		resp.Diagnostics.AddError("Parse import response", err.Error())
		return
	}

	resp.Diagnostics.Append(r.responseToModel(&apiResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *DatasetResource) responseToModel(api *apiResponse, m *DatasetModel) diag.Diagnostics {
	var diags diag.Diagnostics
	m.ID = types.StringValue(api.Name)
	m.Name = types.StringValue(api.Name)
	m.Type = types.StringValue(strings.ToLower(api.Type))
	m.MountPoint = types.StringValue(api.MountPoint)
	m.Encrypted = types.BoolValue(api.Encrypted)
	m.Pool = types.StringValue(api.Pool)
	m.Compression = types.StringValue(strings.ToLower(api.Properties.Compression.Value))
	m.AClType = types.StringValue(strings.ToLower(api.Properties.AClType.Value))
	m.ShareType = types.StringValue(strings.ToLower(api.Properties.ShareType.Value))
	m.Comments = types.StringValue(api.Properties.Comments.Value)

	q, err := parseInt64OrZero(api.Properties.Quota.Value)
	if err != nil {
		diags.AddWarning("Unexpected quota value", err.Error())
	}
	m.Quota = types.Int64Value(q)

	rq, err := parseInt64OrZero(api.Properties.RefQuota.Value)
	if err != nil {
		diags.AddWarning("Unexpected refquota value", err.Error())
	}
	m.RefQuota = types.Int64Value(rq)

	res, err := parseInt64OrZero(api.Properties.Reservation.Value)
	if err != nil {
		diags.AddWarning("Unexpected reservation value", err.Error())
	}
	m.Reservation = types.Int64Value(res)

	m.VolSize = types.Int64Value(api.Properties.VolSize.Parsed)
	return diags
}

func parseInt64OrZero(s string) (int64, error) {
	if s == "none" || s == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("unexpected quota value %q: %w", s, err)
	}
	return v, nil
}
