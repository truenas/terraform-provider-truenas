// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// B-094: defaults the server fills into each item of a list of objects are
// not drift; a changed item value or an extra item is.
func TestProjectConfigOntoUserShape_ListOfObjects(t *testing.T) {
	user := `{"storage":{"additional_storage":[{"type":"host_path","mount_path":"/a","host_path_config":{"path":"/mnt/x"}}]}}`
	live := map[string]any{"storage": map[string]any{"additional_storage": []any{
		map[string]any{"type": "host_path", "mount_path": "/a", "read_only": false,
			"host_path_config": map[string]any{"path": "/mnt/x", "acl_enable": false}},
	}}}
	if got := projectConfigOntoUserShape(user, live); got != user {
		t.Errorf("server-filled item defaults reported as drift:\n got %s\nwant %s", got, user)
	}

	live["storage"].(map[string]any)["additional_storage"].([]any)[0].(map[string]any)["mount_path"] = "/b"
	if got := projectConfigOntoUserShape(user, live); got == user {
		t.Error("a changed item value was not reported as drift")
	}

	two := []any{
		map[string]any{"type": "host_path", "mount_path": "/a", "host_path_config": map[string]any{"path": "/mnt/x"}},
		map[string]any{"type": "ix_volume", "mount_path": "/c"},
	}
	live["storage"].(map[string]any)["additional_storage"] = two
	if got := projectConfigOntoUserShape(user, live); got == user {
		t.Error("an extra list item was not reported as drift")
	}
}

// B-095: a heredoc or hand-formatted document that matches the live config is
// kept as written.
func TestProjectConfigOntoUserShape_KeepsUserFormatting(t *testing.T) {
	user := "{\n  \"network\": {\"web_port\": {\"port_number\": 30993}},\n  \"flame\": {\"password\": \"x\"}\n}\n"
	live := map[string]any{
		"flame":   map[string]any{"password": "x", "extra": 1.0},
		"network": map[string]any{"web_port": map[string]any{"port_number": 30993.0, "bind_mode": "published"}},
	}
	if got := projectConfigOntoUserShape(user, live); got != user {
		t.Errorf("formatting reported as drift:\n got %q\nwant %q", got, user)
	}
	live["flame"].(map[string]any)["password"] = "y"
	if got := projectConfigOntoUserShape(user, live); got == user {
		t.Error("a changed value was not reported as drift")
	}
}

// B-096: "latest" stays in state; a resolved version is not an upgrade target
// when the configured version is "latest".
func TestResponseToModel_VersionLatestKept(t *testing.T) {
	api := &appAPI{Name: "a", Version: "1.3.11"}
	m := AppModel{Version: types.StringValue("latest")}
	responseToModel(api, &m)
	if m.Version.ValueString() != "latest" {
		t.Errorf("version = %v, want latest", m.Version)
	}
	m = AppModel{Version: types.StringUnknown()}
	responseToModel(api, &m)
	if m.Version.ValueString() != "1.3.11" {
		t.Errorf("unset version = %v, want 1.3.11", m.Version)
	}
	if needsUpgrade(&AppModel{Version: types.StringValue("latest")}, &AppModel{Version: types.StringValue("1.3.11")}) {
		t.Error("changing to latest must not upgrade")
	}
}

// B-150: the data source reports catalog_app.
func TestResponseToDatasourceModel_CatalogApp(t *testing.T) {
	api := &appAPI{Name: "f"}
	api.Metadata.Name = "flame"
	var m AppDatasourceModel
	responseToDatasourceModel(api, &m)
	if m.CatalogApp.ValueString() != "flame" {
		t.Errorf("catalog_app = %v, want flame", m.CatalogApp)
	}
	custom := &appAPI{Name: "c", CustomApp: true}
	custom.Metadata.Name = "custom-app"
	responseToDatasourceModel(custom, &m)
	if !m.CatalogApp.IsNull() {
		t.Errorf("custom app catalog_app = %v, want null", m.CatalogApp)
	}
}

// B-097 end to end: the Compose with YAML 1.1 values equals what app.config
// returns for it (the TrueNAS 28.0 transcript in the hunt report).
func TestReconcileComposeConfig_YAML11(t *testing.T) {
	user := "services:\n  a:\n    image: busybox:1.36\n    environment:\n      FOO: yes\n      BAR: off\n      BAZ: 0755\n      QUX: 1:30\n      F: 1.50\n"
	live := json.RawMessage(`{"services":{"a":{"image":"busybox:1.36","environment":{"FOO":true,"BAR":false,"BAZ":493,"QUX":90,"F":1.5}}}}`)
	got, equal, err := reconcileComposeConfig(user, live)
	if err != nil || !equal || got != user {
		t.Errorf("YAML 1.1 Compose reported as drift: equal=%v err=%v\n%s", equal, err, got)
	}
}

// The write-only overlay merge re-encodes the YAML 1.1 decode; numbers and
// booleans must survive as YAML scalars, not quoted strings.
func TestMergeComposeOverlay_YAML11Values(t *testing.T) {
	out, err := mergeComposeOverlay("services:\n  a:\n    environment:\n      BAZ: 0755\n      F: 1.5\n      X: yes\n      S: 'yes'\n",
		`{"services":{"a":{"environment":{"SECRET":"x"}}}}`)
	if err != nil {
		t.Fatal(err)
	}
	live := json.RawMessage(`{"services":{"a":{"environment":{"BAZ":493,"F":1.5,"X":true,"S":"yes","SECRET":"x"}}}}`)
	if _, equal, err := reconcileComposeConfig(out, live); err != nil || !equal {
		t.Errorf("merged overlay does not decode to the expected values (err=%v):\n%s", err, out)
	}
}

// B-137: the keep-from-state modifiers keep state/human_version/
// upgrade_available on a values-only update with version and running unset
// in config, and leave them unknown for an upgrade or a start/stop.
func TestKeepFromStateModifiers(t *testing.T) {
	ctx := context.Background()
	s := resourceSchema()
	typ := s.Type().TerraformType(ctx).(tftypes.Object)
	obj := func(vals map[string]tftypes.Value) tftypes.Value {
		all := map[string]tftypes.Value{}
		for k, at := range typ.AttributeTypes {
			all[k] = tftypes.NewValue(at, nil)
		}
		for k, v := range vals {
			all[k] = v
		}
		return tftypes.NewValue(typ, all)
	}
	str := func(v string) tftypes.Value { return tftypes.NewValue(tftypes.String, v) }
	bl := func(v bool) tftypes.Value { return tftypes.NewValue(tftypes.Bool, v) }
	state := tfsdk.State{Schema: s, Raw: obj(map[string]tftypes.Value{
		"name": str("a"), "version": str("1.0.0"), "running": bl(true), "state": str("RUNNING"),
		"human_version": str("1.0.0_1.0.0"), "upgrade_available": bl(false), "values": str(`{"a":1}`),
	})}
	cases := []struct {
		name string
		cfg  map[string]tftypes.Value
		keep bool
	}{
		{"values only", map[string]tftypes.Value{"name": str("a"), "values": str(`{"a":2}`)}, true},
		{"upgrade", map[string]tftypes.Value{"name": str("a"), "version": str("1.1.0")}, false},
		{"stop", map[string]tftypes.Value{"name": str("a"), "running": bl(false)}, false},
		{"same version", map[string]tftypes.Value{"name": str("a"), "version": str("1.0.0")}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tfsdk.Config{Schema: s, Raw: obj(tc.cfg)}
			plan := tfsdk.Plan{Schema: s, Raw: obj(tc.cfg)}
			sreq := planmodifier.StringRequest{Config: cfg, Plan: plan, State: state,
				StateValue: types.StringValue("RUNNING"), PlanValue: types.StringUnknown()}
			sresp := &planmodifier.StringResponse{PlanValue: types.StringUnknown()}
			keepSettledState{}.PlanModifyString(ctx, sreq, sresp)
			hresp := &planmodifier.StringResponse{PlanValue: types.StringUnknown()}
			sreq.StateValue = types.StringValue("1.0.0_1.0.0")
			keepVersionDerivedString{}.PlanModifyString(ctx, sreq, hresp)
			breq := planmodifier.BoolRequest{Config: cfg, Plan: plan, State: state,
				StateValue: types.BoolValue(false), PlanValue: types.BoolUnknown()}
			bresp := &planmodifier.BoolResponse{PlanValue: types.BoolUnknown()}
			keepVersionDerivedBool{}.PlanModifyBool(ctx, breq, bresp)
			if got := !sresp.PlanValue.IsUnknown(); got != tc.keep && tc.name != "upgrade" {
				t.Errorf("state kept = %v, want %v", got, tc.keep)
			}
			if tc.name == "upgrade" && !sresp.PlanValue.IsUnknown() {
				t.Error("state kept across an upgrade")
			}
			wantVer := tc.keep || tc.name == "stop"
			if got := !hresp.PlanValue.IsUnknown(); got != wantVer {
				t.Errorf("human_version kept = %v, want %v", got, wantVer)
			}
			if got := !bresp.PlanValue.IsUnknown(); got != wantVer {
				t.Errorf("upgrade_available kept = %v, want %v", got, wantVer)
			}
		})
	}
}
