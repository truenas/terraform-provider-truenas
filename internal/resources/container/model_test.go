// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package container

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// --- versionGateDiagnostics ------------------------------------------------

func TestVersionGateDiagnostics_BelowFloor(t *testing.T) {
	for _, version := range []string{"25.10.3.1", "25.10", "24.10.2", "1.0", ""} {
		diags := versionGateDiagnostics(version)
		if !diags.HasError() {
			t.Fatalf("version %q: expected an error diagnostic, got none", version)
		}
		if len(diags) != 1 {
			t.Fatalf("version %q: expected exactly 1 diagnostic, got %d: %v", version, len(diags), diags)
		}
		if diags[0].Detail() != "truenas_container requires TrueNAS 26.0 or later" {
			t.Errorf("version %q: detail = %q, want the documented message", version, diags[0].Detail())
		}
	}
}

func TestVersionGateDiagnostics_AtOrAboveFloor(t *testing.T) {
	for _, version := range []string{"26.0.0", "26.0.0-BETA.2", "26.0", "26.1.0", "27.0.0"} {
		diags := versionGateDiagnostics(version)
		if diags.HasError() {
			t.Errorf("version %q: unexpected error diagnostic: %v", version, diags)
		}
	}
}

// --- poolFromDataset ---------------------------------------------------------

func TestPoolFromDataset(t *testing.T) {
	tests := []struct {
		dataset string
		want    string
	}{
		{"tank/.truenas_containers/containers/my-ctr", "tank"},
		{"apool/x", "apool"},
		{"tank", "tank"}, // no slash: dataset IS the pool (defensive fallback)
		{"", ""},
	}
	for _, tt := range tests {
		if got := poolFromDataset(tt.dataset); got != tt.want {
			t.Errorf("poolFromDataset(%q) = %q, want %q", tt.dataset, got, tt.want)
		}
	}
}

// --- idmapFromAPI / idmapResponseValue ---------------------------------------

func TestIdmapFromAPI(t *testing.T) {
	t.Run("null JSON becomes null", func(t *testing.T) {
		got := idmapFromAPI(json.RawMessage("null"))
		if !got.IsNull() {
			t.Errorf("idmapFromAPI(null) = %#v, want null", got)
		}
	})
	t.Run("empty becomes null", func(t *testing.T) {
		got := idmapFromAPI(nil)
		if !got.IsNull() {
			t.Errorf("idmapFromAPI(nil) = %#v, want null", got)
		}
	})
	t.Run("object passes through verbatim", func(t *testing.T) {
		got := idmapFromAPI(json.RawMessage(`{"type":"DEFAULT"}`))
		if got.ValueString() != `{"type":"DEFAULT"}` {
			t.Errorf("idmapFromAPI = %q, want verbatim passthrough", got.ValueString())
		}
	})
}

func TestIdmapResponseValue_KnownPlanPreserved(t *testing.T) {
	// A known planned value must be preserved verbatim regardless of what
	// the API echoes back — idmap is immutable (container.update doesn't
	// accept it), so the state value must exactly equal the planned value
	// or Terraform's plan-consistency check fails.
	plan := types.StringValue(`{"type":"ISOLATED","slice":5}`)
	got := idmapResponseValue(plan, json.RawMessage(`{"type": "ISOLATED", "slice": 5}`)) // different formatting
	if got.ValueString() != `{"type":"ISOLATED","slice":5}` {
		t.Errorf("idmapResponseValue = %q, want the planned value preserved verbatim", got.ValueString())
	}
}

func TestIdmapResponseValue_UnknownPlanFallsBackToAPI(t *testing.T) {
	plan := types.StringUnknown()
	got := idmapResponseValue(plan, json.RawMessage(`{"type":"DEFAULT"}`))
	if got.ValueString() != `{"type":"DEFAULT"}` {
		t.Errorf("idmapResponseValue = %q, want the API response value", got.ValueString())
	}
}

func TestIdmapResponseValue_NullPlanFallsBackToAPI(t *testing.T) {
	// A null plan value (explicit `idmap = null` — not expected in practice
	// since the field has no meaningful "cleared" state, but exercised for
	// completeness) also falls back to the API response, since IsNull
	// values are not preserved by the "known plan" branch.
	plan := types.StringNull()
	got := idmapResponseValue(plan, json.RawMessage(`{"type":"DEFAULT"}`))
	if got.ValueString() != `{"type":"DEFAULT"}` {
		t.Errorf("idmapResponseValue = %q, want the API response value", got.ValueString())
	}
}

// --- isContainerAlreadyStopped ----------------------------------------------------

func TestIsContainerNotStarted(t *testing.T) {
	t.Run("nil error", func(t *testing.T) {
		if isContainerAlreadyStopped(nil) {
			t.Error("nil error should not be treated as not-started")
		}
	})
	t.Run("exact probed job-failure text: never started", func(t *testing.T) {
		// CallJob wraps a FAILED job's error as: fmt.Errorf("job %d %s: %s",
		// jobID, job.State, job.Error) — job.Error is the raw libvirt text
		// probed live: "Domain 'tf-probe-container' does not exist".
		err := errors.New(`job 21386 FAILED: Domain 'tf-probe-container' does not exist`)
		if !isContainerAlreadyStopped(err) {
			t.Error("expected the probed never-started error text to be recognized")
		}
	})
	t.Run("exact probed job-failure text: already stopped", func(t *testing.T) {
		// Probed live: stopping a container that WAS started at least once
		// but is already stopped (e.g. Delete's best-effort stop right
		// after an apply that left running=false) fails with this
		// different libvirt text, not "does not exist".
		err := errors.New(`job 21640 FAILED: Domain 'tf-acc-container-9c96e959' is not active`)
		if !isContainerAlreadyStopped(err) {
			t.Error("expected the probed already-stopped error text to be recognized")
		}
	})
	t.Run("case insensitive", func(t *testing.T) {
		err := errors.New("Domain 'x' Does Not Exist")
		if !isContainerAlreadyStopped(err) {
			t.Error("expected case-insensitive match")
		}
	})
	t.Run("unrelated error not matched", func(t *testing.T) {
		err := errors.New("job 1 FAILED: permission denied")
		if isContainerAlreadyStopped(err) {
			t.Error("unrelated error should not be treated as not-started")
		}
	})
}

// --- responseToModel / responseToDataSourceModel ------------------------------

func fullAPI() *containerAPI {
	uuid := "4532f4f9-7570-4dbd-a711-b2f89aab450f"
	network := "truenasbr0"
	return &containerAPI{
		ID:                 2,
		UUID:               &uuid,
		Name:               "tf-probe-container",
		Description:        "a description",
		Cpuset:             nil,
		Autostart:          false,
		Time:               "LOCAL",
		ShutdownTimeout:    90,
		Dataset:            "tank/.truenas_containers/containers/tf-probe-container",
		Init:               "/sbin/init",
		InitDir:            nil,
		InitEnv:            map[string]string{},
		InitUser:           nil,
		InitGroup:          nil,
		Idmap:              json.RawMessage(`{"type":"DEFAULT"}`),
		CapabilitiesPolicy: "DEFAULT",
		CapabilitiesState:  map[string]bool{},
		DefaultNetwork:     &network,
		Status:             containerStatusAPI{State: "STOPPED"},
	}
}

func TestResponseToModel(t *testing.T) {
	api := fullAPI()
	m := &ContainerModel{}
	diags := responseToModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueInt64() != 2 {
		t.Errorf("ID = %d, want 2", m.ID.ValueInt64())
	}
	if m.UUID.ValueString() != "4532f4f9-7570-4dbd-a711-b2f89aab450f" {
		t.Errorf("UUID = %q", m.UUID.ValueString())
	}
	if m.Name.ValueString() != "tf-probe-container" {
		t.Errorf("Name = %q", m.Name.ValueString())
	}
	if m.Pool.ValueString() != "tank" {
		t.Errorf("Pool = %q, want %q (derived from dataset)", m.Pool.ValueString(), "tank")
	}
	if m.Description.ValueString() != "a description" {
		t.Errorf("Description = %q", m.Description.ValueString())
	}
	if !m.Cpuset.IsNull() {
		t.Errorf("Cpuset = %#v, want null", m.Cpuset)
	}
	if m.Status.ValueString() != "STOPPED" {
		t.Errorf("Status = %q, want STOPPED", m.Status.ValueString())
	}
	if m.Running.ValueBool() {
		t.Error("Running should be false when status.state is STOPPED")
	}
	if m.DefaultNetwork.ValueString() != "truenasbr0" {
		t.Errorf("DefaultNetwork = %q", m.DefaultNetwork.ValueString())
	}
	if m.Dataset.ValueString() != "tank/.truenas_containers/containers/tf-probe-container" {
		t.Errorf("Dataset = %q", m.Dataset.ValueString())
	}
}

func TestResponseToModel_RunningWhenStateIsRunning(t *testing.T) {
	api := fullAPI()
	api.Status.State = "RUNNING"
	m := &ContainerModel{}
	responseToModel(context.Background(), api, m)
	if !m.Running.ValueBool() {
		t.Error("Running should be true when status.state is RUNNING")
	}
	if m.Status.ValueString() != "RUNNING" {
		t.Errorf("Status = %q, want RUNNING", m.Status.ValueString())
	}
}

func TestResponseToModel_DoesNotTouchImageOrIdmap(t *testing.T) {
	// responseToModel must never overwrite Image or Idmap — both are
	// immutable, write-only-from-the-API's-perspective fields the caller
	// is responsible for preserving (see doc comments in model.go).
	api := fullAPI()
	sentinel := types.StringValue("sentinel-idmap-value")
	m := &ContainerModel{Idmap: sentinel}
	responseToModel(context.Background(), api, m)
	if !m.Idmap.Equal(sentinel) {
		t.Errorf("Idmap changed to %#v, want untouched sentinel %#v", m.Idmap, sentinel)
	}
}

func TestResponseToModel_NilOptionalPointersBecomeNull(t *testing.T) {
	api := fullAPI()
	api.UUID = nil
	api.DefaultNetwork = nil
	m := &ContainerModel{}
	responseToModel(context.Background(), api, m)
	if !m.UUID.IsNull() {
		t.Errorf("UUID = %#v, want null", m.UUID)
	}
	if !m.DefaultNetwork.IsNull() {
		t.Errorf("DefaultNetwork = %#v, want null", m.DefaultNetwork)
	}
}

func TestResponseToDataSourceModel(t *testing.T) {
	api := fullAPI()
	m := &ContainerDataSourceModel{}
	diags := responseToDataSourceModel(context.Background(), api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.Pool.ValueString() != "tank" {
		t.Errorf("Pool = %q, want tank", m.Pool.ValueString())
	}
	// Unlike responseToModel, the datasource mapper DOES populate Idmap
	// directly from the API response — a datasource has no prior planned
	// value to stay consistent with.
	if m.Idmap.ValueString() != `{"type":"DEFAULT"}` {
		t.Errorf("Idmap = %q, want the raw API response text", m.Idmap.ValueString())
	}
}

// --- createPayload / updatePayload --------------------------------------------

func modelWithImage(t *testing.T, imageName, imageVersion string) ContainerModel {
	t.Helper()
	imgObj, diags := types.ObjectValueFrom(context.Background(), imageAttrTypes, ImageModel{
		Name:    types.StringValue(imageName),
		Version: types.StringValue(imageVersion),
	})
	if diags.HasError() {
		t.Fatalf("building image object: %v", diags)
	}
	return ContainerModel{
		Name:  types.StringValue("tf-probe-container"),
		Pool:  types.StringValue("tank"),
		Image: imgObj,
	}
}

func TestCreatePayload_RequiredFields(t *testing.T) {
	m := modelWithImage(t, "alpine:3.22:amd64:default", "20260722_17:16")
	// Every Optional+Computed field left Unknown (zero-value types.String{}
	// etc. IS the unknown/null state for an un-set struct field — but to be
	// explicit and avoid relying on Go zero values, set them all Unknown.
	m.Description = types.StringUnknown()
	m.Autostart = types.BoolUnknown()
	m.Cpuset = types.StringUnknown()
	m.Time = types.StringUnknown()
	m.ShutdownTimeout = types.Int64Unknown()
	m.Init = types.StringUnknown()
	m.InitDir = types.StringUnknown()
	m.InitEnv = types.MapUnknown(types.StringType)
	m.InitUser = types.StringUnknown()
	m.InitGroup = types.StringUnknown()
	m.Idmap = types.StringUnknown()
	m.CapabilitiesPolicy = types.StringUnknown()
	m.CapabilitiesState = types.MapUnknown(types.BoolType)

	p, diags := m.createPayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if p["name"] != "tf-probe-container" {
		t.Errorf(`p["name"] = %#v, want "tf-probe-container"`, p["name"])
	}
	if p["pool"] != "tank" {
		t.Errorf(`p["pool"] = %#v, want "tank"`, p["pool"])
	}
	img, ok := p["image"].(map[string]any)
	if !ok {
		t.Fatalf(`p["image"] = %#v, want map[string]any`, p["image"])
	}
	if img["name"] != "alpine:3.22:amd64:default" || img["version"] != "20260722_17:16" {
		t.Errorf("p[image] = %#v, want the alpine image", img)
	}

	for _, key := range []string{"description", "autostart", "cpuset", "time", "shutdown_timeout", "init",
		"initdir", "initenv", "inituser", "initgroup", "idmap", "capabilities_policy", "capabilities_state"} {
		if _, present := p[key]; present {
			t.Errorf("p[%q] should be omitted when unknown, got %#v", key, p[key])
		}
	}
}

func TestCreatePayload_ThreeWayNullableFields(t *testing.T) {
	m := modelWithImage(t, "alpine:3.22:amd64:default", "20260722_17:16")
	m.Cpuset = types.StringNull()
	m.InitDir = types.StringValue("/tmp")
	m.InitUser = types.StringUnknown()
	m.InitGroup = types.StringNull()

	p, diags := m.createPayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	cpuset, present := p["cpuset"]
	if !present || cpuset != nil {
		t.Errorf(`p["cpuset"] = %#v (present=%v), want explicit nil`, cpuset, present)
	}
	if p["initdir"] != "/tmp" {
		t.Errorf(`p["initdir"] = %#v, want "/tmp"`, p["initdir"])
	}
	if _, present := p["inituser"]; present {
		t.Errorf(`p["inituser"] should be omitted when unknown, got %#v`, p["inituser"])
	}
	initgroup, present := p["initgroup"]
	if !present || initgroup != nil {
		t.Errorf(`p["initgroup"] = %#v (present=%v), want explicit nil`, initgroup, present)
	}
}

func TestCreatePayload_IdmapParsedFromJSON(t *testing.T) {
	m := modelWithImage(t, "alpine:3.22:amd64:default", "20260722_17:16")
	m.Idmap = types.StringValue(`{"type":"ISOLATED","slice":5}`)

	p, diags := m.createPayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	idmap, ok := p["idmap"].(map[string]any)
	if !ok {
		t.Fatalf(`p["idmap"] = %#v, want map[string]any`, p["idmap"])
	}
	if idmap["type"] != "ISOLATED" || idmap["slice"] != float64(5) {
		t.Errorf("p[idmap] = %#v, want ISOLATED/slice=5", idmap)
	}
}

func TestCreatePayload_InvalidIdmapJSONErrors(t *testing.T) {
	m := modelWithImage(t, "alpine:3.22:amd64:default", "20260722_17:16")
	m.Idmap = types.StringValue(`not valid json`)

	_, diags := m.createPayload(context.Background())
	if !diags.HasError() {
		t.Fatal("expected an error diagnostic for invalid idmap JSON")
	}
}

func TestCreatePayload_MapsAndScalars(t *testing.T) {
	m := modelWithImage(t, "alpine:3.22:amd64:default", "20260722_17:16")
	m.Description = types.StringValue("hello")
	m.Autostart = types.BoolValue(true)
	m.Time = types.StringValue("UTC")
	m.ShutdownTimeout = types.Int64Value(120)
	m.Init = types.StringValue("/bin/sh")
	m.CapabilitiesPolicy = types.StringValue("ALLOW")

	env, diags := types.MapValueFrom(context.Background(), types.StringType, map[string]string{"FOO": "bar"})
	if diags.HasError() {
		t.Fatalf("building initenv: %v", diags)
	}
	m.InitEnv = env

	capState, diags := types.MapValueFrom(context.Background(), types.BoolType, map[string]bool{"sys_admin": true})
	if diags.HasError() {
		t.Fatalf("building capabilities_state: %v", diags)
	}
	m.CapabilitiesState = capState

	p, diags := m.createPayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if p["description"] != "hello" {
		t.Errorf(`p["description"] = %#v`, p["description"])
	}
	if p["autostart"] != true {
		t.Errorf(`p["autostart"] = %#v`, p["autostart"])
	}
	if p["time"] != "UTC" {
		t.Errorf(`p["time"] = %#v`, p["time"])
	}
	if p["shutdown_timeout"] != int64(120) {
		t.Errorf(`p["shutdown_timeout"] = %#v`, p["shutdown_timeout"])
	}
	if p["init"] != "/bin/sh" {
		t.Errorf(`p["init"] = %#v`, p["init"])
	}
	if p["capabilities_policy"] != "ALLOW" {
		t.Errorf(`p["capabilities_policy"] = %#v`, p["capabilities_policy"])
	}
	envMap, ok := p["initenv"].(map[string]string)
	if !ok || envMap["FOO"] != "bar" {
		t.Errorf(`p["initenv"] = %#v`, p["initenv"])
	}
	capMap, ok := p["capabilities_state"].(map[string]bool)
	if !ok || !capMap["sys_admin"] {
		t.Errorf(`p["capabilities_state"] = %#v`, p["capabilities_state"])
	}
}

func TestUpdatePayload_NoPoolImageOrIdmap(t *testing.T) {
	// container.update does not accept pool/image/idmap (probed live) —
	// updatePayload must never include them, regardless of what's set on
	// the model (pool/image are Required so always set; idmap may be set
	// too).
	m := modelWithImage(t, "alpine:3.22:amd64:default", "20260722_17:16")
	m.Idmap = types.StringValue(`{"type":"DEFAULT"}`)
	m.Description = types.StringUnknown()
	m.Autostart = types.BoolUnknown()
	m.Cpuset = types.StringUnknown()
	m.Time = types.StringUnknown()
	m.ShutdownTimeout = types.Int64Unknown()
	m.Init = types.StringUnknown()
	m.InitDir = types.StringUnknown()
	m.InitEnv = types.MapUnknown(types.StringType)
	m.InitUser = types.StringUnknown()
	m.InitGroup = types.StringUnknown()
	m.CapabilitiesPolicy = types.StringUnknown()
	m.CapabilitiesState = types.MapUnknown(types.BoolType)

	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	for _, key := range []string{"pool", "image", "idmap"} {
		if _, present := p[key]; present {
			t.Errorf("p[%q] should never be present in an update payload, got %#v", key, p[key])
		}
	}
	if p["name"] != "tf-probe-container" {
		t.Errorf(`p["name"] = %#v, want "tf-probe-container" (name IS mutable, probed live)`, p["name"])
	}
}

func TestUpdatePayload_NameChangeIncluded(t *testing.T) {
	m := modelWithImage(t, "alpine:3.22:amd64:default", "20260722_17:16")
	m.Name = types.StringValue("renamed")
	p, diags := m.updatePayload(context.Background())
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if p["name"] != "renamed" {
		t.Errorf(`p["name"] = %#v, want "renamed"`, p["name"])
	}
}

// --- threeWayString ------------------------------------------------------

func TestThreeWayString(t *testing.T) {
	t.Run("unknown omits", func(t *testing.T) {
		p := map[string]any{}
		threeWayString(p, "k", types.StringUnknown())
		if _, present := p["k"]; present {
			t.Errorf("expected omitted, got %#v", p["k"])
		}
	})
	t.Run("null sends nil", func(t *testing.T) {
		p := map[string]any{}
		threeWayString(p, "k", types.StringNull())
		v, present := p["k"]
		if !present || v != nil {
			t.Errorf("expected explicit nil, got present=%v value=%#v", present, v)
		}
	})
	t.Run("value sends string", func(t *testing.T) {
		p := map[string]any{}
		threeWayString(p, "k", types.StringValue("v"))
		if p["k"] != "v" {
			t.Errorf(`expected "v", got %#v`, p["k"])
		}
	})
}

// --- nonNilStringMap / nonNilBoolMap ------------------------------------------

func TestNonNilStringMap(t *testing.T) {
	if got := nonNilStringMap(nil); got == nil || len(got) != 0 {
		t.Errorf("nonNilStringMap(nil) = %#v, want non-nil empty map", got)
	}
	in := map[string]string{"a": "b"}
	if got := nonNilStringMap(in); len(got) != 1 || got["a"] != "b" {
		t.Errorf("nonNilStringMap(%#v) = %#v, want unchanged", in, got)
	}
}

func TestNonNilBoolMap(t *testing.T) {
	if got := nonNilBoolMap(nil); got == nil || len(got) != 0 {
		t.Errorf("nonNilBoolMap(nil) = %#v, want non-nil empty map", got)
	}
}
