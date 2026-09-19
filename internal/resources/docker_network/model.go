// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package docker_network

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// ipamConfigAttrTypes describes the attribute types of one entry in the
// nested "ipam.config" list.
var ipamConfigAttrTypes = map[string]attr.Type{
	"subnet":   types.StringType,
	"gateway":  types.StringType,
	"ip_range": types.StringType,
}

// IPAMConfigModel maps to one entry of the nested "ipam.config" list: one
// subnet/gateway/ip_range triple for the network (probed live shape — the
// method schema itself declares "ipam" generically as an opaque object,
// but every live response observed on TrueNAS 26.0 used this concrete shape,
// see task-1-report.md).
type IPAMConfigModel struct {
	Subnet  types.String `tfsdk:"subnet"`
	Gateway types.String `tfsdk:"gateway"`
	IPRange types.String `tfsdk:"ip_range"`
}

// ipamAttrTypes describes the attribute types of the nested "ipam" object.
// "options" (a docker-driver-specific, free-form key-value map — always
// null in every probed sample) is intentionally omitted; see model
// doc comments on ipamObjectValue for the full rationale.
var ipamAttrTypes = map[string]attr.Type{
	"driver": types.StringType,
	"config": types.ListType{ElemType: types.ObjectType{AttrTypes: ipamConfigAttrTypes}},
}

// IPAMModel maps to the nested "ipam" attribute: IP Address Management
// configuration for the network.
type IPAMModel struct {
	Driver types.String `tfsdk:"driver"`
	Config types.List   `tfsdk:"config"`
}

// DockerNetworkDataSourceModel is the read-only model for the
// truenas_docker_network datasource.
type DockerNetworkDataSourceModel struct {
	ID      types.String `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	Driver  types.String `tfsdk:"driver"`
	Scope   types.String `tfsdk:"scope"`
	ShortID types.String `tfsdk:"short_id"`
	Created types.String `tfsdk:"created"`
	IPAM    types.Object `tfsdk:"ipam"`
	Labels  types.Map    `tfsdk:"labels"`
}

// ipamConfigAPI mirrors one entry of docker.network.query's nested
// "ipam.config" array.
type ipamConfigAPI struct {
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway"`
	IPRange string `json:"ip_range"`
}

// ipamAPI mirrors the nested "ipam" object returned by
// docker.network.query/get_instance. "options" is decoded but
// intentionally not surfaced (see ipamAttrTypes doc comment).
type ipamAPI struct {
	Driver  string          `json:"driver"`
	Config  []ipamConfigAPI `json:"config"`
	Options *map[string]any `json:"options"`
}

// dockerNetworkAPI mirrors one item of the JSON array returned by
// docker.network.query, and the single object returned by
// docker.network.get_instance. Probed live against TrueNAS 26.0 (the only
// release with Docker configured — see task-1-report.md); the method
// schema itself is version-identical on 25.10, so no gating is needed even
// though 25.10 had no live networks to sample.
type dockerNetworkAPI struct {
	ID      *string           `json:"id"`
	Name    *string           `json:"name"`
	Driver  *string           `json:"driver"`
	Scope   *string           `json:"scope"`
	ShortID *string           `json:"short_id"`
	Created *string           `json:"created"`
	IPAM    *ipamAPI          `json:"ipam"`
	Labels  map[string]string `json:"labels"`
}

// ipamObjectValue builds a types.Object for the nested "ipam" attribute
// from an API response, or ObjectNull when the API returned ipam: null
// (probed live: the "none" network's ipam has driver/options set but
// config: null — an empty config list, not a null ipam object, in that
// case).
func ipamObjectValue(ctx context.Context, api *ipamAPI) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics
	if api == nil {
		return types.ObjectNull(ipamAttrTypes), diags
	}

	entries := make([]IPAMConfigModel, 0, len(api.Config))
	for _, c := range api.Config {
		entries = append(entries, IPAMConfigModel{
			Subnet:  types.StringValue(c.Subnet),
			Gateway: types.StringValue(c.Gateway),
			IPRange: types.StringValue(c.IPRange),
		})
	}
	configList, d := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: ipamConfigAttrTypes}, entries)
	diags.Append(d...)

	obj, d := types.ObjectValueFrom(ctx, ipamAttrTypes, IPAMModel{
		Driver: types.StringValue(api.Driver),
		Config: configList,
	})
	diags.Append(d...)
	return obj, diags
}

// labelsMapValue converts a possibly-nil map[string]string from the API
// into a non-null types.Map, matching the nil-guard convention used
// elsewhere for API-returned collections (e.g. audit_config's
// stringListOrEmpty).
func labelsMapValue(ctx context.Context, labels map[string]string) (types.Map, diag.Diagnostics) {
	if labels == nil {
		labels = map[string]string{}
	}
	return types.MapValueFrom(ctx, types.StringType, labels)
}

// responseToDataSourceModel maps a dockerNetworkAPI response onto a
// DockerNetworkDataSourceModel.
func responseToDataSourceModel(ctx context.Context, api *dockerNetworkAPI, m *DockerNetworkDataSourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringPointerValue(api.ID)
	m.Name = types.StringPointerValue(api.Name)
	m.Driver = types.StringPointerValue(api.Driver)
	m.Scope = types.StringPointerValue(api.Scope)
	m.ShortID = types.StringPointerValue(api.ShortID)
	m.Created = types.StringPointerValue(api.Created)

	ipam, d := ipamObjectValue(ctx, api.IPAM)
	diags.Append(d...)
	m.IPAM = ipam

	labels, d := labelsMapValue(ctx, api.Labels)
	diags.Append(d...)
	m.Labels = labels

	return diags
}
