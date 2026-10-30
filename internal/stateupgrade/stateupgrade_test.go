// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package stateupgrade

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

func TestDropAttrsJSON(t *testing.T) {
	in := []byte(`{"id":"1","address":"a.example","force":true,"maxpoll":9}`)
	out, err := DropAttrsJSON(in, "force", "missing")
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["force"]; ok {
		t.Fatalf("force not dropped: %s", out)
	}
	if m["address"] != "a.example" || m["maxpoll"] != float64(9) || m["id"] != "1" {
		t.Fatalf("other attributes changed: %s", out)
	}
}

func TestDropAttrs_upgrader(t *testing.T) {
	up := DropAttrs("recursive", "traverse")
	req := resource.UpgradeStateRequest{RawState: &tfprotov6.RawState{JSON: []byte(`{"path":"/mnt/x","recursive":false,"traverse":true,"mode":"0755"}`)}}
	var resp resource.UpgradeStateResponse
	up.StateUpgrader(context.Background(), req, &resp)
	if resp.Diagnostics.HasError() {
		t.Fatal(resp.Diagnostics)
	}
	if resp.DynamicValue == nil {
		t.Fatal("no DynamicValue returned")
	}
	var m map[string]any
	if err := json.Unmarshal(resp.DynamicValue.JSON, &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"recursive", "traverse"} {
		if _, ok := m[k]; ok {
			t.Errorf("%s still present: %s", k, resp.DynamicValue.JSON)
		}
	}
	if m["path"] != "/mnt/x" || m["mode"] != "0755" {
		t.Errorf("unrelated attributes changed: %s", resp.DynamicValue.JSON)
	}
}

func TestDropAttrs_nilRawState(t *testing.T) {
	var resp resource.UpgradeStateResponse
	DropAttrs("x").StateUpgrader(context.Background(), resource.UpgradeStateRequest{}, &resp)
	if resp.Diagnostics.HasError() || resp.DynamicValue != nil {
		t.Fatalf("nil raw state must be a no-op, got %+v", resp)
	}
}
