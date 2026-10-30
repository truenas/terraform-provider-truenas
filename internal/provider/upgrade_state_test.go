// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package provider_test

// Guards the write-only/StateUpgrader class (hunt finding F5, 2026-10-30):
// Terraform rejects prior state that carries a value for a write-only
// attribute, so every attribute that was stored by an earlier release and is
// write-only now must be removed by a StateUpgrader, and the resource's schema
// version must be bumped so the upgrader runs.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	fwschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/truenas/terraform-provider-truenas/internal/provider"
)

// storedThenWriteOnly lists attributes that provider <= 1.5.24 kept in state
// and that are write-only from 1.6.0 on. Each MUST be dropped by the v0
// upgrader of its resource. Extend this list whenever an existing attribute
// is made write-only (and add the upgrader).
var storedThenWriteOnly = map[string][]string{
	"truenas_filesystem_acl":         {"recursive", "traverse"},
	"truenas_filesystem_permissions": {"recursive", "traverse"},
	"truenas_ipmi_lan":               {"apply_remote"},
	"truenas_ntp_server":             {"force"},
	"truenas_rsync_task":             {"validate_rpath", "ssh_keyscan"},
	"truenas_tunable":                {"update_initramfs"},
}

func TestWriteOnlyAttributes_haveStateUpgrader(t *testing.T) {
	ctx := context.Background()
	p := provider.New("test")()
	pp := p.(interface {
		Resources(context.Context) []func() resource.Resource
	})
	seen := map[string]bool{}
	for _, ctor := range pp.Resources(ctx) {
		res := ctor()
		var md resource.MetadataResponse
		res.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "truenas"}, &md)
		var sr resource.SchemaResponse
		res.Schema(ctx, resource.SchemaRequest{}, &sr)
		want, tracked := storedThenWriteOnly[md.TypeName]
		if !tracked {
			continue
		}
		seen[md.TypeName] = true
		if sr.Schema.Version < 1 {
			t.Errorf("%s: schema Version is %d; must be >= 1 so the v0 upgrader runs", md.TypeName, sr.Schema.Version)
		}
		ups, ok := res.(resource.ResourceWithUpgradeState)
		if !ok {
			t.Errorf("%s: no UpgradeState; stored write-only attributes %v would block every 1.5.x user", md.TypeName, want)
			continue
		}
		up, ok := ups.UpgradeState(ctx)[0]
		if !ok {
			t.Errorf("%s: no upgrader registered for schema version 0", md.TypeName)
			continue
		}
		for _, a := range want {
			if attr, ok := sr.Schema.Attributes[a]; !ok || !isWriteOnly(attr) {
				t.Errorf("%s.%s: expected a write-only attribute in the current schema", md.TypeName, a)
			}
		}
		// feed a v0-shaped state with every tracked attribute set
		prior := map[string]any{"id": "1"}
		for _, a := range want {
			prior[a] = "stale"
		}
		raw, _ := json.Marshal(prior)
		var resp resource.UpgradeStateResponse
		up.StateUpgrader(ctx, resource.UpgradeStateRequest{RawState: &tfprotov6.RawState{JSON: raw}}, &resp)
		if resp.Diagnostics.HasError() {
			t.Errorf("%s: upgrader failed: %v", md.TypeName, resp.Diagnostics)
			continue
		}
		if resp.DynamicValue == nil {
			t.Errorf("%s: upgrader returned no state", md.TypeName)
			continue
		}
		var out map[string]any
		_ = json.Unmarshal(resp.DynamicValue.JSON, &out)
		for _, a := range want {
			if _, still := out[a]; still {
				t.Errorf("%s: v0 upgrader did not drop write-only attribute %q", md.TypeName, a)
			}
		}
		if out["id"] != "1" {
			t.Errorf("%s: upgrader lost unrelated attribute id: %s", md.TypeName, resp.DynamicValue.JSON)
		}
	}
	for tn := range storedThenWriteOnly {
		if !seen[tn] {
			t.Errorf("%s listed in storedThenWriteOnly but not registered by the provider", tn)
		}
	}
}

func isWriteOnly(a fwschema.Attribute) bool {
	type wo interface{ IsWriteOnly() bool }
	if w, ok := a.(wo); ok {
		return w.IsWriteOnly()
	}
	return false
}
