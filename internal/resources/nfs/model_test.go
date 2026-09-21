// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package nfs

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func strList(vals ...string) types.List {
	elems := make([]attr.Value, 0, len(vals))
	for _, v := range vals {
		elems = append(elems, types.StringValue(v))
	}
	return types.ListValueMust(types.StringType, elems)
}

// TestNFSApiPayload_MapallSecurityExpose verifies the new attributes are sent
// when set.
func TestNFSApiPayload_MapallSecurityExpose(t *testing.T) {
	m := &NFSShareModel{
		Path:      types.StringValue("/mnt/tank/x"),
		MapAll:    types.StringValue("root"),
		MapAllGr:  types.StringValue("root"),
		Security:  strList("SYS", "KRB5"),
		ExposeSns: types.BoolValue(true),
		// leave the rest null so they are omitted
		Comment:  types.StringNull(),
		Enabled:  types.BoolNull(),
		ReadOnly: types.BoolNull(),
		MapRoot:  types.StringNull(),
		MapGroup: types.StringNull(),
		Networks: types.ListNull(types.StringType),
		Hosts:    types.ListNull(types.StringType),
	}

	p := m.apiPayload()

	if p["mapall_user"] != "root" {
		t.Errorf("mapall_user = %v, want root", p["mapall_user"])
	}
	if p["mapall_group"] != "root" {
		t.Errorf("mapall_group = %v, want root", p["mapall_group"])
	}
	if p["expose_snapshots"] != true {
		t.Errorf("expose_snapshots = %v, want true", p["expose_snapshots"])
	}
	sec, ok := p["security"].([]string)
	if !ok {
		t.Fatalf("security is %T, want []string", p["security"])
	}
	if len(sec) != 2 || sec[0] != "SYS" || sec[1] != "KRB5" {
		t.Errorf("security = %v, want [SYS KRB5]", sec)
	}
}

// TestNFSApiPayload_OmitsUnset verifies unset new attributes are not sent.
func TestNFSApiPayload_OmitsUnset(t *testing.T) {
	m := &NFSShareModel{
		Path:      types.StringValue("/mnt/tank/x"),
		MapAll:    types.StringNull(),
		MapAllGr:  types.StringNull(),
		Security:  types.ListNull(types.StringType),
		ExposeSns: types.BoolNull(),
		Comment:   types.StringNull(),
		Enabled:   types.BoolNull(),
		ReadOnly:  types.BoolNull(),
		MapRoot:   types.StringNull(),
		MapGroup:  types.StringNull(),
		Networks:  types.ListNull(types.StringType),
		Hosts:     types.ListNull(types.StringType),
	}
	p := m.apiPayload()
	for _, k := range []string{"mapall_user", "mapall_group", "security", "expose_snapshots"} {
		if _, ok := p[k]; ok {
			t.Errorf("payload should omit unset %q, got %v", k, p[k])
		}
	}
}

// TestNFSResponseToModel_NewFields verifies the new attributes round-trip from
// an API response.
func TestNFSResponseToModel_NewFields(t *testing.T) {
	r := &NFSShareResource{}
	var m NFSShareModel
	api := &apiResponse{
		ID:       1,
		Path:     "/mnt/tank/x",
		Security: []string{"SYS", "KRB5I"},
		ExposeSn: true,
		MapAll:   "nobody",
		MapAllGr: "nogroup",
		Networks: []string{},
		Hosts:    []string{},
	}
	if d := r.responseToModel(context.Background(), api, &m); d.HasError() {
		t.Fatalf("responseToModel: %v", d)
	}
	if m.MapAll.ValueString() != "nobody" || m.MapAllGr.ValueString() != "nogroup" {
		t.Errorf("mapall = %q/%q", m.MapAll.ValueString(), m.MapAllGr.ValueString())
	}
	if !m.ExposeSns.ValueBool() {
		t.Error("expose_snapshots should be true")
	}
	var sec []string
	if d := m.Security.ElementsAs(context.Background(), &sec, false); d.HasError() {
		t.Fatalf("security ElementsAs: %v", d)
	}
	if len(sec) != 2 || sec[0] != "SYS" || sec[1] != "KRB5I" {
		t.Errorf("security = %v, want [SYS KRB5I]", sec)
	}
}
