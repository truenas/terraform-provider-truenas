// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm_device

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestAttributesDrifted_MACCase: the server stores a NIC mac in lower case; an
// upper-case configured mac is the same address, not drift. (B-082, B-083)
func TestAttributesDrifted_MACCase(t *testing.T) {
	state := map[string]any{"dtype": "NIC", "mac": "00:A0:98:12:34:56"}
	api := map[string]any{"dtype": "NIC", "mac": "00:a0:98:12:34:56", "trust_guest_rx_filters": false}
	if attributesDrifted(state, api) {
		t.Error("upper-case mac reported as drift")
	}
	api["mac"] = "00:a0:98:12:34:57"
	if !attributesDrifted(state, api) {
		t.Error("different mac not reported as drift")
	}
}

// TestAttributesDrifted_NestedServerKey: keys the server adds inside a nested
// object are not drift either.
func TestAttributesDrifted_NestedServerKey(t *testing.T) {
	state := map[string]any{"dtype": "USB", "usb": map[string]any{"vendor_id": "0x046b"}}
	api := map[string]any{"dtype": "USB", "usb": map[string]any{"vendor_id": "0x046b", "product_id": "0xff10"}}
	if attributesDrifted(state, api) {
		t.Error("server-added nested key reported as drift")
	}
}

// TestKeepAttributesIfConfigMatchesState_notImported: without the imported
// flag (state holds what Terraform wrote), a configuration that drops a key is
// a real change and must not be hidden by keeping the state value. The
// imported case (B-082, B-083, B-084) is covered by the acceptance tests,
// since the framework's private state cannot be built outside it.
func TestKeepAttributesIfConfigMatchesState_notImported(t *testing.T) {
	state := types.StringValue(`{"dtype":"NIC","mac":"00:a0:98:12:34:56","nic_attach":"br0"}`)
	cfg := types.StringValue(`{"dtype":"NIC","nic_attach":"br0"}`)
	req := planmodifier.StringRequest{ConfigValue: cfg, PlanValue: cfg, StateValue: state}
	resp := &planmodifier.StringResponse{PlanValue: cfg}
	keepAttributesIfConfigMatchesState{}.PlanModifyString(context.Background(), req, resp)
	if !resp.PlanValue.Equal(cfg) {
		t.Errorf("plan = %s, want the configured value", resp.PlanValue)
	}
}
