// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package boot_environment

import "github.com/hashicorp/terraform-plugin-framework/types"

// BootEnvironmentModel is the Terraform state model for truenas_boot_environment.
type BootEnvironmentModel struct {
	ID        types.String `tfsdk:"id"`     // BE name
	Name      types.String `tfsdk:"name"`   // target name (= id)
	Source    types.String `tfsdk:"source"` // BE cloned from; RequiresReplace; client-side only
	Activated types.Bool   `tfsdk:"activated"`
	Keep      types.Bool   `tfsdk:"keep"`
	// Computed only
	Dataset   types.String `tfsdk:"dataset"`
	Active    types.Bool   `tfsdk:"active"` // currently booted
	UsedBytes types.Int64  `tfsdk:"used_bytes"`
}

// BootEnvironmentDataSourceModel is used by the boot_environment data source.
// It omits Source because the API never returns the clone origin.
type BootEnvironmentDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	Activated types.Bool   `tfsdk:"activated"`
	Keep      types.Bool   `tfsdk:"keep"`
	Dataset   types.String `tfsdk:"dataset"`
	Active    types.Bool   `tfsdk:"active"`
	UsedBytes types.Int64  `tfsdk:"used_bytes"`
}

// bootEnvAPI mirrors the JSON object returned by boot.environment.query.
// Note: "source" (the clone origin) is never present in API responses; it is
// a client-side-only concept tracked purely in Terraform state.
type bootEnvAPI struct {
	ID        string `json:"id"`
	Dataset   string `json:"dataset"`
	Active    bool   `json:"active"`
	Activated bool   `json:"activated"`
	Keep      bool   `json:"keep"`
	UsedBytes int64  `json:"used_bytes"`
}

// responseToModel copies API response fields into m. It never touches
// m.Source: that field is a client-side-only concept (the BE this one was
// cloned from) and is never present in API responses, so any existing value
// in state/plan must be preserved by the caller.
func responseToModel(api *bootEnvAPI, m *BootEnvironmentModel) {
	m.ID = types.StringValue(api.ID)
	m.Name = types.StringValue(api.ID)
	m.Dataset = types.StringValue(api.Dataset)
	m.Active = types.BoolValue(api.Active)
	m.Activated = types.BoolValue(api.Activated)
	m.Keep = types.BoolValue(api.Keep)
	m.UsedBytes = types.Int64Value(api.UsedBytes)
	// m.Source is intentionally left unchanged (client-side only).
}

// responseToDataSourceModel copies API response fields into a data source model.
func responseToDataSourceModel(api *bootEnvAPI) BootEnvironmentDataSourceModel {
	return BootEnvironmentDataSourceModel{
		ID:        types.StringValue(api.ID),
		Name:      types.StringValue(api.ID),
		Activated: types.BoolValue(api.Activated),
		Keep:      types.BoolValue(api.Keep),
		Dataset:   types.StringValue(api.Dataset),
		Active:    types.BoolValue(api.Active),
		UsedBytes: types.Int64Value(api.UsedBytes),
	}
}

// activationChange describes what (if anything) must be done to reconcile
// the "activated" attribute between state and plan.
type activationChange int

const (
	// activationNoChange means no activate call is needed.
	activationNoChange activationChange = iota
	// activationActivate means activate({"id": name}) must be called
	// (state false/unknown -> plan true).
	activationActivate
	// activationUnsupportedDeactivate means the plan asks to flip an
	// activated BE back to false, which the API does not support.
	activationUnsupportedDeactivate
)

// planActivationChange decides what activation change (if any) is implied by
// moving from stateActivated to planActivated. It is a pure function so the
// decision logic can be unit tested without a live API.
func planActivationChange(stateActivated, planActivated bool) activationChange {
	if planActivated == stateActivated {
		return activationNoChange
	}
	if planActivated {
		return activationActivate
	}
	return activationUnsupportedDeactivate
}
