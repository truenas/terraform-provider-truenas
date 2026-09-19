// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package docker_config

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }

// pointer-string test helper reused across cases.

// --- updatePayload: three-way coverage ---------------------------------

// TestUpdatePayload_AllFieldsSet_Unified25 verifies every non-nvidia field
// is included with the unified (TrueNAS 26.0+) registry_mirrors shape when
// registryMirrorsUnified=true.
func TestUpdatePayload_AllFieldsSet_Unified(t *testing.T) {
	ctx := context.Background()
	pools, _ := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: addressPoolAttrTypes}, []AddressPoolModel{
		{Base: types.StringValue("172.17.0.0/12"), Size: types.Int64Value(24)},
	})
	mirrors, _ := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: registryMirrorAttrTypes}, []RegistryMirrorModel{
		{URL: types.StringValue("https://mirror.example.com"), Insecure: types.BoolValue(false)},
		{URL: types.StringValue("http://insecure.example.com"), Insecure: types.BoolValue(true)},
	})

	m := &DockerConfigModel{
		EnableImageUpdates: types.BoolValue(false),
		Pool:               types.StringValue("tank"),
		CIDRv6:             types.StringValue("fdd0::/64"),
		AddressPools:       pools,
		RegistryMirrors:    mirrors,
	}

	p, diags := m.updatePayload(ctx, true)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if p["enable_image_updates"] != false {
		t.Errorf("enable_image_updates = %v, want false", p["enable_image_updates"])
	}
	if p["pool"] != "tank" {
		t.Errorf("pool = %v, want tank", p["pool"])
	}
	if p["cidr_v6"] != "fdd0::/64" {
		t.Errorf("cidr_v6 = %v, want fdd0::/64", p["cidr_v6"])
	}
	gotPools, ok := p["address_pools"].([]map[string]any)
	if !ok || len(gotPools) != 1 || gotPools[0]["base"] != "172.17.0.0/12" || gotPools[0]["size"] != int64(24) {
		t.Errorf("address_pools = %#v, want one entry base=172.17.0.0/12 size=24", p["address_pools"])
	}
	gotMirrors, ok := p["registry_mirrors"].([]map[string]any)
	if !ok || len(gotMirrors) != 2 {
		t.Fatalf("registry_mirrors = %#v, want 2 unified entries", p["registry_mirrors"])
	}
	if gotMirrors[0]["url"] != "https://mirror.example.com" || gotMirrors[0]["insecure"] != false {
		t.Errorf("registry_mirrors[0] = %#v", gotMirrors[0])
	}
	if gotMirrors[1]["url"] != "http://insecure.example.com" || gotMirrors[1]["insecure"] != true {
		t.Errorf("registry_mirrors[1] = %#v", gotMirrors[1])
	}
	if _, present := p["secure_registry_mirrors"]; present {
		t.Error("secure_registry_mirrors should not be present in unified payload")
	}
	if _, present := p["insecure_registry_mirrors"]; present {
		t.Error("insecure_registry_mirrors should not be present in unified payload")
	}
}

// TestUpdatePayload_AllFieldsSet_Split verifies registry_mirrors is split
// into secure_registry_mirrors/insecure_registry_mirrors when
// registryMirrorsUnified=false (TrueNAS 25.10 wire shape).
func TestUpdatePayload_AllFieldsSet_Split(t *testing.T) {
	ctx := context.Background()
	mirrors, _ := types.ListValueFrom(ctx, types.ObjectType{AttrTypes: registryMirrorAttrTypes}, []RegistryMirrorModel{
		{URL: types.StringValue("https://mirror.example.com"), Insecure: types.BoolValue(false)},
		{URL: types.StringValue("http://insecure.example.com"), Insecure: types.BoolValue(true)},
		{URL: types.StringValue("https://mirror2.example.com"), Insecure: types.BoolValue(false)},
	})

	m := &DockerConfigModel{
		EnableImageUpdates: types.BoolValue(true),
		Pool:               types.StringNull(),
		CIDRv6:             types.StringValue("fdd0::/64"),
		AddressPools:       types.ListNull(types.ObjectType{AttrTypes: addressPoolAttrTypes}),
		RegistryMirrors:    mirrors,
	}

	p, diags := m.updatePayload(ctx, false)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if p["pool"] != nil {
		t.Errorf("pool = %v, want explicit nil (null pool sent through)", p["pool"])
	}
	if _, present := p["registry_mirrors"]; present {
		t.Error("registry_mirrors (unified key) should not be present in split payload")
	}
	secure, ok := p["secure_registry_mirrors"].([]string)
	if !ok || len(secure) != 2 || secure[0] != "https://mirror.example.com" || secure[1] != "https://mirror2.example.com" {
		t.Errorf("secure_registry_mirrors = %#v, want [https://mirror.example.com https://mirror2.example.com]", p["secure_registry_mirrors"])
	}
	insecure, ok := p["insecure_registry_mirrors"].([]string)
	if !ok || len(insecure) != 1 || insecure[0] != "http://insecure.example.com" {
		t.Errorf("insecure_registry_mirrors = %#v, want [http://insecure.example.com]", p["insecure_registry_mirrors"])
	}
	if _, present := p["address_pools"]; present {
		t.Error("address_pools should be omitted when null")
	}
}

// TestUpdatePayload_UnsetOptionalsOmitted verifies every field is omitted
// when null/unknown, so the current TrueNAS-side value is left unchanged.
func TestUpdatePayload_UnsetOptionalsOmitted(t *testing.T) {
	ctx := context.Background()
	m := &DockerConfigModel{
		EnableImageUpdates: types.BoolUnknown(),
		Pool:               types.StringUnknown(),
		CIDRv6:             types.StringUnknown(),
		AddressPools:       types.ListUnknown(types.ObjectType{AttrTypes: addressPoolAttrTypes}),
		RegistryMirrors:    types.ListUnknown(types.ObjectType{AttrTypes: registryMirrorAttrTypes}),
	}

	p, diags := m.updatePayload(ctx, true)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if len(p) != 0 {
		t.Errorf("payload has %d keys (%v), want 0 (all omitted)", len(p), p)
	}
}

// --- responseToModel: address_pools + registry_mirrors nesting ---------

// TestResponseToModel_AddressPoolsNesting verifies address_pools decodes
// into the nested {base,size} model shape.
func TestResponseToModel_AddressPoolsNesting(t *testing.T) {
	ctx := context.Background()
	api := &dockerConfigAPI{
		ID:                 1,
		EnableImageUpdates: true,
		Dataset:            strPtr("tank/ix-apps"),
		Pool:               strPtr("tank"),
		Nvidia:             boolPtr(false),
		AddressPools: []addressPoolAPI{
			{Base: "172.17.0.0/12", Size: 24},
			{Base: "fdd0::/48", Size: 64},
		},
		CIDRv6:                  "fdd0::/64",
		SecureRegistryMirrors:   []string{},
		InsecureRegistryMirrors: []string{},
	}

	m := &DockerConfigModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueString() != dockerConfigResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), dockerConfigResourceID)
	}
	if m.Pool.ValueString() != "tank" {
		t.Errorf("Pool = %q, want tank", m.Pool.ValueString())
	}
	if m.Dataset.ValueString() != "tank/ix-apps" {
		t.Errorf("Dataset = %q, want tank/ix-apps", m.Dataset.ValueString())
	}
	if m.Nvidia.IsNull() || m.Nvidia.ValueBool() {
		t.Errorf("Nvidia = %v, want known false", m.Nvidia)
	}

	var pools []AddressPoolModel
	diags = m.AddressPools.ElementsAs(ctx, &pools, false)
	if diags.HasError() {
		t.Fatalf("reading back address_pools: %v", diags)
	}
	if len(pools) != 2 {
		t.Fatalf("address_pools has %d entries, want 2", len(pools))
	}
	if pools[0].Base.ValueString() != "172.17.0.0/12" || pools[0].Size.ValueInt64() != 24 {
		t.Errorf("address_pools[0] = %+v", pools[0])
	}
	if pools[1].Base.ValueString() != "fdd0::/48" || pools[1].Size.ValueInt64() != 64 {
		t.Errorf("address_pools[1] = %+v", pools[1])
	}
}

// TestResponseToModel_NullablePoolAndDataset verifies pool/dataset decode
// to null when the API returns null (docker unconfigured), matching the
// mail/ups precedent for detecting "unconfigured" in acceptance PreCheck.
func TestResponseToModel_NullablePoolAndDataset(t *testing.T) {
	ctx := context.Background()
	api := &dockerConfigAPI{
		ID:                      1,
		Pool:                    nil,
		Dataset:                 nil,
		Nvidia:                  nil,
		SecureRegistryMirrors:   []string{},
		InsecureRegistryMirrors: []string{},
	}

	m := &DockerConfigModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !m.Pool.IsNull() {
		t.Errorf("Pool = %v, want null", m.Pool)
	}
	if !m.Dataset.IsNull() {
		t.Errorf("Dataset = %v, want null", m.Dataset)
	}
	if !m.Nvidia.IsNull() {
		t.Errorf("Nvidia = %v, want null when the API response omits the key (defensive case — in practice both probed releases always include it)", m.Nvidia)
	}
}

// TestResponseToModel_RegistryMirrors_UnifiedShape verifies the TrueNAS
// 26.0+ unified registry_mirrors array decodes directly.
func TestResponseToModel_RegistryMirrors_UnifiedShape(t *testing.T) {
	ctx := context.Background()
	api := &dockerConfigAPI{
		ID: 1,
		RegistryMirrors: []registryMirrorAPI{
			{URL: "https://mirror.example.com", Insecure: false},
			{URL: "http://insecure.example.com", Insecure: true},
		},
	}

	m := &DockerConfigModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	var mirrors []RegistryMirrorModel
	diags = m.RegistryMirrors.ElementsAs(ctx, &mirrors, false)
	if diags.HasError() {
		t.Fatalf("reading back registry_mirrors: %v", diags)
	}
	if len(mirrors) != 2 {
		t.Fatalf("registry_mirrors has %d entries, want 2", len(mirrors))
	}
	if mirrors[0].URL.ValueString() != "https://mirror.example.com" || mirrors[0].Insecure.ValueBool() {
		t.Errorf("registry_mirrors[0] = %+v", mirrors[0])
	}
	if mirrors[1].URL.ValueString() != "http://insecure.example.com" || !mirrors[1].Insecure.ValueBool() {
		t.Errorf("registry_mirrors[1] = %+v", mirrors[1])
	}
}

// TestResponseToModel_RegistryMirrors_SplitShape verifies the TrueNAS 25.10
// split secure/insecure string arrays are translated into the unified
// {url,insecure} shape, secure entries first.
func TestResponseToModel_RegistryMirrors_SplitShape(t *testing.T) {
	ctx := context.Background()
	api := &dockerConfigAPI{
		ID:                      1,
		SecureRegistryMirrors:   []string{"https://a.example.com", "https://b.example.com"},
		InsecureRegistryMirrors: []string{"http://c.example.com"},
	}

	m := &DockerConfigModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	var mirrors []RegistryMirrorModel
	diags = m.RegistryMirrors.ElementsAs(ctx, &mirrors, false)
	if diags.HasError() {
		t.Fatalf("reading back registry_mirrors: %v", diags)
	}
	if len(mirrors) != 3 {
		t.Fatalf("registry_mirrors has %d entries, want 3", len(mirrors))
	}
	want := []struct {
		url      string
		insecure bool
	}{
		{"https://a.example.com", false},
		{"https://b.example.com", false},
		{"http://c.example.com", true},
	}
	for i, w := range want {
		if mirrors[i].URL.ValueString() != w.url || mirrors[i].Insecure.ValueBool() != w.insecure {
			t.Errorf("registry_mirrors[%d] = %+v, want {%s %v}", i, mirrors[i], w.url, w.insecure)
		}
	}
}

// TestHasUnifiedRegistryMirrors_KeyPresenceDetection verifies shape
// detection: an empty-but-present "registry_mirrors" JSON array (a
// non-nil, zero-length Go slice) is still detected as the unified shape,
// distinct from the key being entirely absent (nil slice).
func TestHasUnifiedRegistryMirrors_KeyPresenceDetection(t *testing.T) {
	present := &dockerConfigAPI{RegistryMirrors: []registryMirrorAPI{}}
	if !present.hasUnifiedRegistryMirrors() {
		t.Error("hasUnifiedRegistryMirrors() = false for a present-but-empty array, want true")
	}

	absent := &dockerConfigAPI{}
	if absent.hasUnifiedRegistryMirrors() {
		t.Error("hasUnifiedRegistryMirrors() = true for an absent key (nil slice), want false")
	}
}

// TestResponseToDataSourceModel mirrors TestResponseToModel_AddressPoolsNesting
// for the datasource model.
func TestResponseToDataSourceModel(t *testing.T) {
	ctx := context.Background()
	api := &dockerConfigAPI{
		ID:                 1,
		EnableImageUpdates: true,
		Pool:               strPtr("tank"),
		Dataset:            strPtr("tank/ix-apps"),
		AddressPools:       []addressPoolAPI{{Base: "172.16.0.0/12", Size: 24}},
		CIDRv6:             "fdd0::/64",
		RegistryMirrors:    []registryMirrorAPI{},
	}

	m := &DockerConfigDataSourceModel{}
	diags := responseToDataSourceModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.ID.ValueString() != dockerConfigResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), dockerConfigResourceID)
	}
	if m.Pool.ValueString() != "tank" {
		t.Errorf("Pool = %q, want tank", m.Pool.ValueString())
	}
	var pools []AddressPoolModel
	diags = m.AddressPools.ElementsAs(ctx, &pools, false)
	if diags.HasError() {
		t.Fatalf("reading back address_pools: %v", diags)
	}
	if len(pools) != 1 || pools[0].Base.ValueString() != "172.16.0.0/12" {
		t.Errorf("address_pools = %+v", pools)
	}
}

// TestDeleteWarningDiagnostics verifies Delete's diagnostic builder returns
// exactly one warning and never touches a client.
func TestDeleteWarningDiagnostics(t *testing.T) {
	diags := deleteWarningDiagnostics()
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if len(diags) != 1 {
		t.Fatalf("expected exactly 1 diagnostic, got %d: %v", len(diags), diags)
	}
	if diags[0].Detail() != "Docker configuration left in place; removed from Terraform state only" {
		t.Errorf("detail = %q, want the documented warning text", diags[0].Detail())
	}
}
