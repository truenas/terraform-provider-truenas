// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package docker_network

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func strPtr(s string) *string { return &s }

// TestResponseToDataSourceModel_FullNetwork verifies decoding a real
// bridge network with ipam.config populated and labels set (probed live
// shape, "ix-plex_default" on TrueNAS 26.0 — see task-1-report.md).
func TestResponseToDataSourceModel_FullNetwork(t *testing.T) {
	ctx := context.Background()
	api := &dockerNetworkAPI{
		ID:      strPtr("1b12280f567ff78eeac1f4c05e49bad72f6d850c54c35d321f6baa6c7da1cf87"),
		Name:    strPtr("ix-plex_default"),
		Driver:  strPtr("bridge"),
		Scope:   strPtr("local"),
		ShortID: strPtr("1b12280f567f"),
		Created: strPtr("2026-02-18T08:09:55.719111532-08:00"),
		IPAM: &ipamAPI{
			Driver: "default",
			Config: []ipamConfigAPI{
				{Subnet: "172.16.1.0/24", Gateway: "172.16.1.1", IPRange: ""},
				{Subnet: "fdd0:0:0:1::/64", Gateway: "fdd0:0:0:1::1", IPRange: ""},
			},
		},
		Labels: map[string]string{
			"com.docker.compose.project": "ix-plex",
		},
	}

	m := &DockerNetworkDataSourceModel{}
	diags := responseToDataSourceModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueString() != *api.ID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), *api.ID)
	}
	if m.Name.ValueString() != "ix-plex_default" {
		t.Errorf("Name = %q, want ix-plex_default", m.Name.ValueString())
	}
	if m.Driver.ValueString() != "bridge" {
		t.Errorf("Driver = %q, want bridge", m.Driver.ValueString())
	}

	var ipam IPAMModel
	diags = m.IPAM.As(ctx, &ipam, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("reading back ipam: %v", diags)
	}
	if ipam.Driver.ValueString() != "default" {
		t.Errorf("ipam.driver = %q, want default", ipam.Driver.ValueString())
	}
	var configs []IPAMConfigModel
	diags = ipam.Config.ElementsAs(ctx, &configs, false)
	if diags.HasError() {
		t.Fatalf("reading back ipam.config: %v", diags)
	}
	if len(configs) != 2 {
		t.Fatalf("ipam.config has %d entries, want 2", len(configs))
	}
	if configs[0].Subnet.ValueString() != "172.16.1.0/24" || configs[0].Gateway.ValueString() != "172.16.1.1" {
		t.Errorf("ipam.config[0] = %+v", configs[0])
	}

	var labels map[string]string
	diags = m.Labels.ElementsAs(ctx, &labels, false)
	if diags.HasError() {
		t.Fatalf("reading back labels: %v", diags)
	}
	if labels["com.docker.compose.project"] != "ix-plex" {
		t.Errorf("labels = %v, want com.docker.compose.project=ix-plex", labels)
	}
}

// TestResponseToDataSourceModel_NullIPAMConfig verifies the "none" network
// shape: ipam is present (driver set) but config is null (probed live) —
// this must decode to a non-null ipam object containing an empty config
// list, not a null ipam object.
func TestResponseToDataSourceModel_NullIPAMConfig(t *testing.T) {
	ctx := context.Background()
	api := &dockerNetworkAPI{
		ID:      strPtr("357059bbaaf32421b446c4ee409f56348240349f1118a5b386fa4d13629a4a0a"),
		Name:    strPtr("none"),
		Driver:  strPtr("null"),
		Scope:   strPtr("local"),
		ShortID: strPtr("357059bbaaf3"),
		Created: strPtr("2025-04-08T11:05:07.275003446-04:00"),
		IPAM:    &ipamAPI{Driver: "default", Config: nil},
		Labels:  map[string]string{},
	}

	m := &DockerNetworkDataSourceModel{}
	diags := responseToDataSourceModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.IPAM.IsNull() {
		t.Fatal("ipam should not be null when the API returns a non-null ipam object")
	}

	var ipam IPAMModel
	diags = m.IPAM.As(ctx, &ipam, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		t.Fatalf("reading back ipam: %v", diags)
	}
	if ipam.Config.IsNull() {
		t.Error("ipam.config should be an empty list, not null, when the API returns config: null")
	}
	var configs []IPAMConfigModel
	diags = ipam.Config.ElementsAs(ctx, &configs, false)
	if diags.HasError() {
		t.Fatalf("reading back ipam.config: %v", diags)
	}
	if len(configs) != 0 {
		t.Errorf("ipam.config = %+v, want empty", configs)
	}
}

// TestResponseToDataSourceModel_NilIPAM verifies a wholly-null "ipam"
// field (per the probed nullable schema) decodes to a null ipam object.
func TestResponseToDataSourceModel_NilIPAM(t *testing.T) {
	ctx := context.Background()
	api := &dockerNetworkAPI{
		ID:     strPtr("deadbeef"),
		Name:   strPtr("weird"),
		IPAM:   nil,
		Labels: nil,
	}

	m := &DockerNetworkDataSourceModel{}
	diags := responseToDataSourceModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !m.IPAM.IsNull() {
		t.Error("ipam should be null when the API returns ipam: null")
	}
	if m.Labels.IsNull() {
		t.Error("labels should be an empty map, not null, when the API returns labels: null")
	}
	var labels map[string]string
	diags = m.Labels.ElementsAs(ctx, &labels, false)
	if diags.HasError() {
		t.Fatalf("reading back labels: %v", diags)
	}
	if len(labels) != 0 {
		t.Errorf("labels = %v, want empty", labels)
	}
}

// TestResponseToDataSourceModel_AllFieldsNull verifies every nullable
// top-level field (per the probed docker.network.query schema, where id/
// name/driver/scope/short_id/created are each anyOf string-or-null) decodes
// cleanly to a null string rather than panicking or producing "".
func TestResponseToDataSourceModel_AllFieldsNull(t *testing.T) {
	ctx := context.Background()
	api := &dockerNetworkAPI{}

	m := &DockerNetworkDataSourceModel{}
	diags := responseToDataSourceModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	for name, v := range map[string]types.String{
		"id": m.ID, "name": m.Name, "driver": m.Driver,
		"scope": m.Scope, "short_id": m.ShortID, "created": m.Created,
	} {
		if !v.IsNull() {
			t.Errorf("%s = %v, want null", name, v)
		}
	}
}
