// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package vm

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestVMSchema verifies that the resource schema has the expected attributes
// and that key attributes have the correct types/behaviors.
func TestVMSchema(t *testing.T) {
	s := resourceSchema()

	// id must be Int64Attribute and Computed.
	idAttr, ok := s.Attributes["id"]
	if !ok {
		t.Fatal("schema missing 'id' attribute")
	}
	idInt64, ok := idAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'id' attribute is %T, want schema.Int64Attribute", idAttr)
	}
	if !idInt64.IsComputed() {
		t.Error("'id' should be Computed")
	}

	// name must be Required.
	nameAttr, ok := s.Attributes["name"]
	if !ok {
		t.Fatal("schema missing 'name' attribute")
	}
	nameStr, ok := nameAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'name' attribute is %T, want schema.StringAttribute", nameAttr)
	}
	if !nameStr.IsRequired() {
		t.Error("'name' should be Required")
	}
	if len(nameStr.PlanModifiers) != 0 {
		t.Error("'name' should not have plan modifiers (vm.update accepts name changes)")
	}

	// memory must be Required Int64Attribute.
	memAttr, ok := s.Attributes["memory"]
	if !ok {
		t.Fatal("schema missing 'memory' attribute")
	}
	memInt64, ok := memAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'memory' attribute is %T, want schema.Int64Attribute", memAttr)
	}
	if !memInt64.IsRequired() {
		t.Error("'memory' should be Required")
	}

	// running must be Optional+Computed.
	runningAttr, ok := s.Attributes["running"]
	if !ok {
		t.Fatal("schema missing 'running' attribute")
	}
	runningBool, ok := runningAttr.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'running' attribute is %T, want schema.BoolAttribute", runningAttr)
	}
	if !runningBool.IsOptional() || !runningBool.IsComputed() {
		t.Error("'running' should be Optional+Computed")
	}
	if len(runningBool.PlanModifiers) == 0 {
		t.Error("'running' should have plan modifiers (UseStateForUnknown)")
	}

	// status must be Computed-only, with NO plan modifiers.
	statusAttr, ok := s.Attributes["status"]
	if !ok {
		t.Fatal("schema missing 'status' attribute")
	}
	statusStr, ok := statusAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'status' attribute is %T, want schema.StringAttribute", statusAttr)
	}
	if !statusStr.IsComputed() {
		t.Error("'status' should be Computed")
	}
	if statusStr.IsOptional() || statusStr.IsRequired() {
		t.Error("'status' should be Computed-only")
	}
	if len(statusStr.PlanModifiers) != 0 {
		t.Error("'status' should NOT have plan modifiers (value can change server-side)")
	}
}

// TestVMSchema_OptionalComputedHavePlanModifiers verifies that all Optional+Computed
// attributes (except status) have at least one plan modifier.
func TestVMSchema_OptionalComputedHavePlanModifiers(t *testing.T) {
	s := resourceSchema()
	for name, attr := range s.Attributes {
		if name == "status" {
			continue
		}
		switch a := attr.(type) {
		case schema.StringAttribute:
			if a.Optional && a.Computed && len(a.PlanModifiers) == 0 {
				t.Errorf("%s: Optional+Computed without plan modifier", name)
			}
		case schema.Int64Attribute:
			if a.Optional && a.Computed && len(a.PlanModifiers) == 0 {
				t.Errorf("%s: Optional+Computed without plan modifier", name)
			}
		case schema.BoolAttribute:
			if a.Optional && a.Computed && len(a.PlanModifiers) == 0 {
				t.Errorf("%s: Optional+Computed without plan modifier", name)
			}
		}
	}
}

// TestVMApiPayload_OmitsUnsetOptionals verifies that apiPayload does not send
// keys for null/unknown optional fields (e.g. no "vcpus": 0).
func TestVMApiPayload_OmitsUnsetOptionals(t *testing.T) {
	m := VMModel{
		Name:            types.StringValue("myvm"),
		Memory:          types.Int64Value(1073741824),
		Description:     types.StringValue("test vm"),
		MinMemory:       types.Int64Null(),
		VCPUs:           types.Int64Null(),
		Cores:           types.Int64Null(),
		Threads:         types.Int64Null(),
		Bootloader:      types.StringNull(),
		Autostart:       types.BoolNull(),
		Time:            types.StringNull(),
		ShutdownTimeout: types.Int64Null(),
		CPUMode:         types.StringNull(),
		CPUModel:        types.StringNull(),
	}

	payload := m.apiPayload()

	requiredKeys := []string{"name", "memory", "description"}
	if len(payload) != len(requiredKeys) {
		t.Errorf("payload has %d keys, want %d: %v", len(payload), len(requiredKeys), payload)
	}
	for _, k := range requiredKeys {
		if _, ok := payload[k]; !ok {
			t.Errorf("payload missing key %q", k)
		}
	}

	optionalKeys := []string{
		"vcpus", "cores", "threads", "bootloader", "autostart", "time",
		"shutdown_timeout", "cpu_mode", "cpu_model", "min_memory",
	}
	for _, k := range optionalKeys {
		if _, ok := payload[k]; ok {
			t.Errorf("payload should not contain unset optional key %q", k)
		}
	}

	if payload["name"] != "myvm" {
		t.Errorf("payload[name] = %v, want myvm", payload["name"])
	}
	if payload["memory"] != int64(1073741824) {
		t.Errorf("payload[memory] = %v, want 1073741824", payload["memory"])
	}
}

// TestVMApiPayload_IncludesSetOptionals verifies that apiPayload includes keys
// for optional fields that are explicitly set (including zero values like
// vcpus: 0, since VCPUs/Cores/Threads have no special zero-omission rule).
func TestVMApiPayload_IncludesSetOptionals(t *testing.T) {
	m := VMModel{
		Name:            types.StringValue("myvm"),
		Memory:          types.Int64Value(1073741824),
		Description:     types.StringValue(""),
		VCPUs:           types.Int64Value(2),
		Cores:           types.Int64Value(1),
		Threads:         types.Int64Value(1),
		Bootloader:      types.StringValue("UEFI"),
		Autostart:       types.BoolValue(true),
		Time:            types.StringValue("LOCAL"),
		ShutdownTimeout: types.Int64Value(90),
		CPUMode:         types.StringValue("CUSTOM"),
		CPUModel:        types.StringValue("EPYC"),
		MinMemory:       types.Int64Value(536870912),
	}

	payload := m.apiPayload()

	expect := map[string]any{
		"name":             "myvm",
		"memory":           int64(1073741824),
		"description":      "",
		"vcpus":            int64(2),
		"cores":            int64(1),
		"threads":          int64(1),
		"bootloader":       "UEFI",
		"autostart":        true,
		"time":             "LOCAL",
		"shutdown_timeout": int64(90),
		"cpu_mode":         "CUSTOM",
		"cpu_model":        "EPYC",
		"min_memory":       int64(536870912),
	}
	if len(payload) != len(expect) {
		t.Errorf("payload has %d keys, want %d: %v", len(payload), len(expect), payload)
	}
	for k, v := range expect {
		if payload[k] != v {
			t.Errorf("payload[%q] = %v, want %v", k, payload[k], v)
		}
	}
}

// TestVMApiPayload_MinMemoryZeroOmitted verifies that min_memory is omitted
// from the payload when it is explicitly set to 0.
func TestVMApiPayload_MinMemoryZeroOmitted(t *testing.T) {
	m := VMModel{
		Name:        types.StringValue("myvm"),
		Memory:      types.Int64Value(1073741824),
		Description: types.StringValue(""),
		MinMemory:   types.Int64Value(0),
	}

	payload := m.apiPayload()
	if _, ok := payload["min_memory"]; ok {
		t.Error("payload should not contain min_memory when set to 0")
	}
}

// TestVMApiPayload_CPUModelEmptyOmitted verifies that cpu_model is omitted
// from the payload when it is explicitly set to "".
func TestVMApiPayload_CPUModelEmptyOmitted(t *testing.T) {
	m := VMModel{
		Name:        types.StringValue("myvm"),
		Memory:      types.Int64Value(1073741824),
		Description: types.StringValue(""),
		CPUModel:    types.StringValue(""),
	}

	payload := m.apiPayload()
	if _, ok := payload["cpu_model"]; ok {
		t.Error("payload should not contain cpu_model when set to \"\"")
	}
}

// TestResponseToModel_NilHandling verifies that a nil MinMemory maps to 0 and
// a nil CPUModel maps to "".
func TestResponseToModel_NilHandling(t *testing.T) {
	api := &vmAPI{
		ID:              1,
		Name:            "myvm",
		Description:     "",
		Memory:          1073741824,
		MinMemory:       nil,
		VCPUs:           2,
		Cores:           1,
		Threads:         1,
		Bootloader:      "UEFI",
		Autostart:       false,
		Time:            "LOCAL",
		ShutdownTimeout: 90,
		CPUMode:         "HOST-MODEL",
		CPUModel:        nil,
		Status:          vmStatusAPI{State: "STOPPED"},
	}

	var m VMModel
	responseToModel(api, &m)

	if m.MinMemory.ValueInt64() != 0 {
		t.Errorf("MinMemory = %v, want 0", m.MinMemory.ValueInt64())
	}
	if m.CPUModel.ValueString() != "" {
		t.Errorf("CPUModel = %q, want \"\"", m.CPUModel.ValueString())
	}
}

// TestResponseToModel_NonNilHandling verifies that non-nil MinMemory/CPUModel
// pointers are dereferenced correctly.
func TestResponseToModel_NonNilHandling(t *testing.T) {
	minMem := int64(536870912)
	cpuModel := "EPYC"
	api := &vmAPI{
		ID:        2,
		Name:      "myvm2",
		Memory:    2147483648,
		MinMemory: &minMem,
		CPUModel:  &cpuModel,
		Status:    vmStatusAPI{State: "RUNNING"},
	}

	var m VMModel
	responseToModel(api, &m)

	if m.MinMemory.ValueInt64() != 536870912 {
		t.Errorf("MinMemory = %v, want 536870912", m.MinMemory.ValueInt64())
	}
	if m.CPUModel.ValueString() != "EPYC" {
		t.Errorf("CPUModel = %q, want EPYC", m.CPUModel.ValueString())
	}
}

// TestResponseToModel_RunningDerivedFromStatus verifies that Running is
// derived from Status.State == "RUNNING".
func TestResponseToModel_RunningDerivedFromStatus(t *testing.T) {
	cases := []struct {
		state   string
		running bool
	}{
		{"RUNNING", true},
		{"STOPPED", false},
		{"", false},
	}

	for _, tc := range cases {
		api := &vmAPI{
			ID:     1,
			Name:   "myvm",
			Status: vmStatusAPI{State: tc.state},
		}
		var m VMModel
		responseToModel(api, &m)

		if m.Running.ValueBool() != tc.running {
			t.Errorf("state=%q: Running=%v, want %v", tc.state, m.Running.ValueBool(), tc.running)
		}
		if m.Status.ValueString() != tc.state {
			t.Errorf("state=%q: Status=%v, want %v", tc.state, m.Status.ValueString(), tc.state)
		}
	}
}

// TestResponseToModel_AllFields verifies that all remaining fields are mapped
// correctly from vmAPI to VMModel.
func TestResponseToModel_AllFields(t *testing.T) {
	api := &vmAPI{
		ID:              42,
		Name:            "myvm",
		Description:     "a test vm",
		Memory:          4294967296,
		VCPUs:           4,
		Cores:           2,
		Threads:         2,
		Bootloader:      "UEFI_CSM",
		Autostart:       true,
		Time:            "UTC",
		ShutdownTimeout: 120,
		CPUMode:         "HOST-PASSTHROUGH",
		Status:          vmStatusAPI{State: "RUNNING"},
	}

	var m VMModel
	responseToModel(api, &m)

	if m.ID.ValueInt64() != 42 {
		t.Errorf("ID = %v, want 42", m.ID.ValueInt64())
	}
	if m.Name.ValueString() != "myvm" {
		t.Errorf("Name = %v, want myvm", m.Name.ValueString())
	}
	if m.Description.ValueString() != "a test vm" {
		t.Errorf("Description = %v, want 'a test vm'", m.Description.ValueString())
	}
	if m.Memory.ValueInt64() != 4294967296 {
		t.Errorf("Memory = %v, want 4294967296", m.Memory.ValueInt64())
	}
	if m.VCPUs.ValueInt64() != 4 {
		t.Errorf("VCPUs = %v, want 4", m.VCPUs.ValueInt64())
	}
	if m.Cores.ValueInt64() != 2 {
		t.Errorf("Cores = %v, want 2", m.Cores.ValueInt64())
	}
	if m.Threads.ValueInt64() != 2 {
		t.Errorf("Threads = %v, want 2", m.Threads.ValueInt64())
	}
	if m.Bootloader.ValueString() != "UEFI_CSM" {
		t.Errorf("Bootloader = %v, want UEFI_CSM", m.Bootloader.ValueString())
	}
	if !m.Autostart.ValueBool() {
		t.Error("Autostart = false, want true")
	}
	if m.Time.ValueString() != "UTC" {
		t.Errorf("Time = %v, want UTC", m.Time.ValueString())
	}
	if m.ShutdownTimeout.ValueInt64() != 120 {
		t.Errorf("ShutdownTimeout = %v, want 120", m.ShutdownTimeout.ValueInt64())
	}
	if m.CPUMode.ValueString() != "HOST-PASSTHROUGH" {
		t.Errorf("CPUMode = %v, want HOST-PASSTHROUGH", m.CPUMode.ValueString())
	}
}
