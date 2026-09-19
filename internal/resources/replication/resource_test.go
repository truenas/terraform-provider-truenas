// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package replication

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

func TestReplicationSchema(t *testing.T) {
	s := resourceSchema()

	// id: Computed Int64.
	idAttr, ok := s.Attributes["id"]
	if !ok {
		t.Fatal("schema missing 'id' attribute")
	}
	idInt64, ok := idAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'id' is %T, want schema.Int64Attribute", idAttr)
	}
	if !idInt64.IsComputed() {
		t.Error("'id' should be Computed")
	}

	// direction: Required + RequiresReplace.
	dirAttr, ok := s.Attributes["direction"]
	if !ok {
		t.Fatal("schema missing 'direction' attribute")
	}
	dirStr, ok := dirAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'direction' is %T, want schema.StringAttribute", dirAttr)
	}
	if !dirStr.IsRequired() {
		t.Error("'direction' should be Required")
	}
	if len(dirStr.PlanModifiers) == 0 {
		t.Error("'direction' should have RequiresReplace plan modifier")
	}

	// transport: Optional + Computed (default LOCAL) + RequiresReplace.
	transAttr, ok := s.Attributes["transport"]
	if !ok {
		t.Fatal("schema missing 'transport' attribute")
	}
	transStr, ok := transAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'transport' is %T, want schema.StringAttribute", transAttr)
	}
	if !transStr.IsOptional() || !transStr.IsComputed() {
		t.Error("'transport' should be Optional+Computed")
	}
	if transStr.IsRequired() {
		t.Error("'transport' should not be Required")
	}
	if transStr.StringDefaultValue() == nil {
		t.Error("'transport' should have a Default")
	}
	if len(transStr.PlanModifiers) == 0 {
		t.Error("'transport' should have RequiresReplace plan modifier")
	}
	if len(transStr.Validators) == 0 {
		t.Error("'transport' should have a LOCAL|SSH validator")
	}

	// compression: Optional String, SSH-only (no Computed/default — plain
	// nullable, unset stays null forever).
	compAttr, ok := s.Attributes["compression"]
	if !ok {
		t.Fatal("schema missing 'compression' attribute")
	}
	compStr, ok := compAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'compression' is %T, want schema.StringAttribute", compAttr)
	}
	if !compStr.IsOptional() {
		t.Error("'compression' should be Optional")
	}
	if compStr.IsComputed() {
		t.Error("'compression' should not be Computed")
	}
	if len(compStr.Validators) == 0 {
		t.Error("'compression' should have an LZ4|PIGZ|PLZIP validator")
	}

	// speed_limit: Optional Int64, SSH-only.
	slAttr, ok := s.Attributes["speed_limit"]
	if !ok {
		t.Fatal("schema missing 'speed_limit' attribute")
	}
	slInt64, ok := slAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'speed_limit' is %T, want schema.Int64Attribute", slAttr)
	}
	if !slInt64.IsOptional() {
		t.Error("'speed_limit' should be Optional")
	}
	if slInt64.IsComputed() {
		t.Error("'speed_limit' should not be Computed")
	}

	// Required fields.
	for _, field := range []string{"name", "target_dataset", "retention_policy"} {
		a, ok := s.Attributes[field]
		if !ok {
			t.Fatalf("schema missing %q attribute", field)
		}
		sa, ok := a.(schema.StringAttribute)
		if !ok {
			t.Errorf("%q is %T, want schema.StringAttribute", field, a)
			continue
		}
		if !sa.IsRequired() {
			t.Errorf("%q should be Required", field)
		}
	}

	// source_datasets: Required List[String].
	sdAttr, ok := s.Attributes["source_datasets"]
	if !ok {
		t.Fatal("schema missing 'source_datasets' attribute")
	}
	sdList, ok := sdAttr.(schema.ListAttribute)
	if !ok {
		t.Fatalf("'source_datasets' is %T, want schema.ListAttribute", sdAttr)
	}
	if !sdList.IsRequired() {
		t.Error("'source_datasets' should be Required")
	}

	// recursive, auto: Required Bool.
	for _, field := range []string{"recursive", "auto"} {
		a, ok := s.Attributes[field]
		if !ok {
			t.Fatalf("schema missing %q attribute", field)
		}
		ba, ok := a.(schema.BoolAttribute)
		if !ok {
			t.Errorf("%q is %T, want schema.BoolAttribute", field, a)
			continue
		}
		if !ba.IsRequired() {
			t.Errorf("%q should be Required", field)
		}
	}

	// ssh_credentials: Optional + Computed Int64.
	sshAttr, ok := s.Attributes["ssh_credentials"]
	if !ok {
		t.Fatal("schema missing 'ssh_credentials' attribute")
	}
	sshInt64, ok := sshAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'ssh_credentials' is %T, want schema.Int64Attribute", sshAttr)
	}
	if !sshInt64.IsOptional() || !sshInt64.IsComputed() {
		t.Error("'ssh_credentials' should be Optional+Computed")
	}
	if len(sshInt64.PlanModifiers) == 0 {
		t.Error("'ssh_credentials' should have UseStateForUnknown plan modifier")
	}

	// schedule: Optional SingleNestedAttribute with 5 required string fields.
	schedAttr, ok := s.Attributes["schedule"]
	if !ok {
		t.Fatal("schema missing 'schedule' attribute")
	}
	nested, ok := schedAttr.(schema.SingleNestedAttribute)
	if !ok {
		t.Fatalf("'schedule' is %T, want schema.SingleNestedAttribute", schedAttr)
	}
	if !nested.IsOptional() {
		t.Error("'schedule' should be Optional")
	}
	if nested.IsRequired() {
		t.Error("'schedule' should not be Required")
	}
	for _, field := range []string{"minute", "hour", "dom", "month", "dow"} {
		a, exists := nested.Attributes[field]
		if !exists {
			t.Errorf("schedule missing field %q", field)
			continue
		}
		sa, ok := a.(schema.StringAttribute)
		if !ok {
			t.Errorf("schedule.%s is %T, want schema.StringAttribute", field, a)
			continue
		}
		if !sa.IsRequired() {
			t.Errorf("schedule.%s should be Required", field)
		}
	}
}

func TestReplicationPayload_LocalTransport(t *testing.T) {
	ctx := context.Background()

	m := baseModel(ctx, t)
	m.Transport = types.StringValue("LOCAL")
	m.SSHCredentials = types.Int64Value(0)

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned errors: %v", diags)
	}

	v, ok := payload["ssh_credentials"]
	if !ok {
		t.Fatal("payload missing 'ssh_credentials' key")
	}
	if v != nil {
		t.Errorf("payload[ssh_credentials] = %v, want nil for LOCAL transport", v)
	}
}

func TestReplicationPayload_SSHCredentialsNonZero(t *testing.T) {
	ctx := context.Background()

	m := baseModel(ctx, t)
	m.Transport = types.StringValue("SSH")
	m.SSHCredentials = types.Int64Value(7)

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned errors: %v", diags)
	}

	v, ok := payload["ssh_credentials"]
	if !ok {
		t.Fatal("payload missing 'ssh_credentials' key")
	}
	if v != int64(7) {
		t.Errorf("payload[ssh_credentials] = %v, want 7", v)
	}
}

func TestReplicationPayload_NameRegexOmitsNamingSchema(t *testing.T) {
	ctx := context.Background()

	m := baseModel(ctx, t)
	m.NameRegex = types.StringValue("^auto-.*$")

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned errors: %v", diags)
	}

	if v, ok := payload["name_regex"]; !ok || v != "^auto-.*$" {
		t.Errorf("payload[name_regex] = %v, want ^auto-.*$", v)
	}
	if _, ok := payload["naming_schema"]; ok {
		t.Error("payload should not contain 'naming_schema' when name_regex is set")
	}
	if _, ok := payload["also_include_naming_schema"]; ok {
		t.Error("payload should not contain 'also_include_naming_schema' when name_regex is set")
	}
}

func TestReplicationPayload_NoNameRegexIncludesNamingSchema(t *testing.T) {
	ctx := context.Background()

	m := baseModel(ctx, t)
	m.NameRegex = types.StringValue("")

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned errors: %v", diags)
	}

	if _, ok := payload["name_regex"]; ok {
		t.Error("payload should not contain 'name_regex' when unset")
	}
	if _, ok := payload["naming_schema"]; !ok {
		t.Error("payload should contain 'naming_schema' when name_regex is unset")
	}
	if _, ok := payload["also_include_naming_schema"]; !ok {
		t.Error("payload should contain 'also_include_naming_schema' when name_regex is unset")
	}
}

func TestReplicationPayload_ScheduleNull(t *testing.T) {
	ctx := context.Background()

	m := baseModel(ctx, t)
	m.Schedule = types.ObjectNull(scheduleAttrTypes)

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned errors: %v", diags)
	}

	if _, ok := payload["schedule"]; ok {
		t.Error("payload should not contain 'schedule' key when Schedule is null")
	}
}

func TestReplicationPayload_ScheduleSet(t *testing.T) {
	ctx := context.Background()

	m := baseModel(ctx, t)
	schedObj, diags := types.ObjectValueFrom(ctx, scheduleAttrTypes, ScheduleModel{
		Minute: types.StringValue("0"),
		Hour:   types.StringValue("0"),
		Dom:    types.StringValue("*"),
		Month:  types.StringValue("*"),
		Dow:    types.StringValue("*"),
	})
	if diags.HasError() {
		t.Fatalf("failed to build schedule object: %v", diags)
	}
	m.Schedule = schedObj

	payload, pDiags := m.apiPayload(ctx)
	if pDiags.HasError() {
		t.Fatalf("apiPayload returned errors: %v", pDiags)
	}

	sched, ok := payload["schedule"].(map[string]string)
	if !ok {
		t.Fatalf("payload[schedule] is %T, want map[string]string", payload["schedule"])
	}
	for _, key := range []string{"minute", "hour", "dom", "month", "dow"} {
		if _, ok := sched[key]; !ok {
			t.Errorf("payload[schedule] missing key %q", key)
		}
	}
	if len(sched) != 5 {
		t.Errorf("payload[schedule] has %d keys, want 5", len(sched))
	}
}

func TestReplicationPayload_LifetimeZeroValuesBecomeNull(t *testing.T) {
	ctx := context.Background()

	m := baseModel(ctx, t)
	m.LifetimeValue = types.Int64Value(0)
	m.LifetimeUnit = types.StringValue("")

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned errors: %v", diags)
	}

	if v, ok := payload["lifetime_value"]; !ok || v != nil {
		t.Errorf("payload[lifetime_value] = %v, want nil", v)
	}
	if v, ok := payload["lifetime_unit"]; !ok || v != nil {
		t.Errorf("payload[lifetime_unit] = %v, want nil", v)
	}
}

func TestReplicationPayload_LifetimeNonZeroValuesPassThrough(t *testing.T) {
	ctx := context.Background()

	m := baseModel(ctx, t)
	m.LifetimeValue = types.Int64Value(4)
	m.LifetimeUnit = types.StringValue("WEEK")

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned errors: %v", diags)
	}

	if v := payload["lifetime_value"]; v != int64(4) {
		t.Errorf("payload[lifetime_value] = %v, want 4", v)
	}
	if v := payload["lifetime_unit"]; v != "WEEK" {
		t.Errorf("payload[lifetime_unit] = %v, want WEEK", v)
	}
}

func TestReplicationPayload_PeriodicSnapshotTasksNilBecomesEmptySlice(t *testing.T) {
	ctx := context.Background()

	m := baseModel(ctx, t)
	m.PeriodicSnapshotTasks = types.ListNull(types.Int64Type)

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned errors: %v", diags)
	}

	pst, ok := payload["periodic_snapshot_tasks"].([]int64)
	if !ok {
		t.Fatalf("payload[periodic_snapshot_tasks] is %T, want []int64", payload["periodic_snapshot_tasks"])
	}
	if pst == nil {
		t.Error("payload[periodic_snapshot_tasks] must not be nil")
	}
	if len(pst) != 0 {
		t.Errorf("payload[periodic_snapshot_tasks] = %v, want empty", pst)
	}
}

func TestSSHCredentialsID(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want int64
	}{
		{"null", nil, 0},
		{"bare int (float64)", float64(42), 42},
		{"embedded object", map[string]any{"id": float64(9)}, 9},
		{"embedded object missing id", map[string]any{"foo": "bar"}, 0},
		{"unexpected type", "garbage", 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sshCredentialsID(tc.in)
			if got != tc.want {
				t.Errorf("sshCredentialsID(%v) = %d, want %d", tc.in, got, tc.want)
			}
		})
	}
}

func TestPeriodicSnapshotTaskIDs(t *testing.T) {
	tasks := []embeddedTask{{ID: 1}, {ID: 2}, {ID: 3}}
	ids := periodicSnapshotTaskIDs(tasks)
	if len(ids) != 3 {
		t.Fatalf("periodicSnapshotTaskIDs returned %d ids, want 3", len(ids))
	}
	for i, want := range []int64{1, 2, 3} {
		if ids[i] != want {
			t.Errorf("ids[%d] = %d, want %d", i, ids[i], want)
		}
	}
}

func TestPeriodicSnapshotTaskIDs_Empty(t *testing.T) {
	ids := periodicSnapshotTaskIDs(nil)
	if ids == nil {
		t.Error("periodicSnapshotTaskIDs(nil) must not return nil")
	}
	if len(ids) != 0 {
		t.Errorf("periodicSnapshotTaskIDs(nil) = %v, want empty", ids)
	}
}

func TestResponseToModel_ScheduleNilAndLifetimeNil(t *testing.T) {
	ctx := context.Background()

	api := &replicationAPI{
		ID:              1,
		Name:            "task1",
		Direction:       "PUSH",
		Transport:       "LOCAL",
		SSHCredentials:  nil,
		SourceDatasets:  []string{"tank/data"},
		TargetDataset:   "backup/data",
		RetentionPolicy: "SOURCE",
		Schedule:        nil,
		LifetimeValue:   nil,
		LifetimeUnit:    nil,
	}

	var m ReplicationModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned errors: %v", diags)
	}

	if !m.Schedule.IsNull() {
		t.Error("Schedule should be null when API schedule is nil")
	}
	if m.LifetimeValue.ValueInt64() != 0 {
		t.Errorf("LifetimeValue = %v, want 0", m.LifetimeValue.ValueInt64())
	}
	if m.LifetimeUnit.ValueString() != "" {
		t.Errorf("LifetimeUnit = %q, want empty", m.LifetimeUnit.ValueString())
	}
	if m.NameRegex.ValueString() != "" {
		t.Errorf("NameRegex = %q, want empty", m.NameRegex.ValueString())
	}
	if m.SSHCredentials.ValueInt64() != 0 {
		t.Errorf("SSHCredentials = %v, want 0", m.SSHCredentials.ValueInt64())
	}
}

func TestResponseToModel_ScheduleSetAndEmbeddedTasks(t *testing.T) {
	ctx := context.Background()

	lifetimeVal := int64(2)
	lifetimeUnit := "WEEK"
	regex := "^auto-.*$"

	api := &replicationAPI{
		ID:             2,
		Name:           "task2",
		Direction:      "PUSH",
		Transport:      "SSH",
		SSHCredentials: map[string]any{"id": float64(5)},
		SourceDatasets: []string{"tank/data"},
		TargetDataset:  "backup/data",
		PeriodicSnapshotTasks: []embeddedTask{
			{ID: 10}, {ID: 20},
		},
		NameRegex:       &regex,
		RetentionPolicy: "CUSTOM",
		Schedule: &struct {
			Minute string `json:"minute"`
			Hour   string `json:"hour"`
			Dom    string `json:"dom"`
			Month  string `json:"month"`
			Dow    string `json:"dow"`
		}{
			Minute: "0", Hour: "3", Dom: "*", Month: "*", Dow: "*",
		},
		LifetimeValue: &lifetimeVal,
		LifetimeUnit:  &lifetimeUnit,
	}

	var m ReplicationModel
	diags := responseToModel(ctx, api, &m)
	if diags.HasError() {
		t.Fatalf("responseToModel returned errors: %v", diags)
	}

	if m.Schedule.IsNull() {
		t.Fatal("Schedule should not be null when API schedule is set")
	}
	var sched ScheduleModel
	diags2 := m.Schedule.As(ctx, &sched, basetypes.ObjectAsOptions{})
	if diags2.HasError() {
		t.Fatalf("Schedule.As returned errors: %v", diags2)
	}
	if sched.Hour.ValueString() != "3" {
		t.Errorf("Schedule.Hour = %q, want 3", sched.Hour.ValueString())
	}

	if m.SSHCredentials.ValueInt64() != 5 {
		t.Errorf("SSHCredentials = %v, want 5", m.SSHCredentials.ValueInt64())
	}

	var taskIDs []int64
	if d := m.PeriodicSnapshotTasks.ElementsAs(ctx, &taskIDs, false); d.HasError() {
		t.Fatalf("PeriodicSnapshotTasks.ElementsAs failed: %v", d)
	}
	if len(taskIDs) != 2 || taskIDs[0] != 10 || taskIDs[1] != 20 {
		t.Errorf("PeriodicSnapshotTasks = %v, want [10 20]", taskIDs)
	}

	if m.LifetimeValue.ValueInt64() != 2 {
		t.Errorf("LifetimeValue = %v, want 2", m.LifetimeValue.ValueInt64())
	}
	if m.LifetimeUnit.ValueString() != "WEEK" {
		t.Errorf("LifetimeUnit = %q, want WEEK", m.LifetimeUnit.ValueString())
	}
	if m.NameRegex.ValueString() != regex {
		t.Errorf("NameRegex = %q, want %q", m.NameRegex.ValueString(), regex)
	}
}

func TestReplicationApiPayload_OmitsUnsetOptionals(t *testing.T) {
	ctx := context.Background()

	m := ReplicationModel{
		Name:                    types.StringValue("test-task"),
		Direction:               types.StringValue("PUSH"),
		Transport:               types.StringValue("LOCAL"),
		SSHCredentials:          types.Int64Null(),
		Sudo:                    types.BoolNull(),
		Compression:             types.StringNull(),
		SpeedLimit:              types.Int64Null(),
		SourceDatasets:          types.ListValueMust(types.StringType, []attr.Value{types.StringValue("tank/data")}),
		TargetDataset:           types.StringValue("backup/data"),
		Recursive:               types.BoolValue(true),
		Exclude:                 types.ListNull(types.StringType),
		Properties:              types.BoolNull(),
		Replicate:               types.BoolNull(),
		PeriodicSnapshotTasks:   types.ListNull(types.Int64Type),
		NamingSchema:            types.ListNull(types.StringType),
		AlsoIncludeNamingSchema: types.ListNull(types.StringType),
		NameRegex:               types.StringNull(),
		Auto:                    types.BoolValue(true),
		Schedule:                types.ObjectNull(scheduleAttrTypes),
		RetentionPolicy:         types.StringValue("NONE"),
		LifetimeValue:           types.Int64Null(),
		LifetimeUnit:            types.StringNull(),
		Readonly:                types.StringNull(),
		Enabled:                 types.BoolNull(),
		Retries:                 types.Int64Null(),
	}

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned errors: %v", diags)
	}

	for _, key := range []string{"sudo", "properties", "replicate", "readonly", "enabled", "retries"} {
		if _, ok := payload[key]; ok {
			t.Errorf("payload should not contain unset optional key %q, got %v", key, payload[key])
		}
	}
}

// TestNameRegexConflict verifies the mutual-exclusion boolean logic used by
// ReplicationResource.ValidateConfig: name_regex conflicts with a non-empty
// naming_schema and/or also_include_naming_schema, but not when only one of
// the two attribute groups is set, and unknown values (not yet known at
// plan time) are treated as absent so partially-unknown configs don't
// falsely trip the check.
func TestNameRegexConflict(t *testing.T) {
	strList := func(vals ...string) types.List {
		elems := make([]attr.Value, 0, len(vals))
		for _, v := range vals {
			elems = append(elems, types.StringValue(v))
		}
		return types.ListValueMust(types.StringType, elems)
	}
	emptyList := types.ListValueMust(types.StringType, []attr.Value{})
	nullList := types.ListNull(types.StringType)
	unknownList := types.ListUnknown(types.StringType)

	cases := []struct {
		name                    string
		nameRegex               types.String
		namingSchema            types.List
		alsoIncludeNamingSchema types.List
		want                    bool
	}{
		{"regex alone", types.StringValue("^auto-.*$"), nullList, nullList, false},
		{"naming_schema alone", types.StringValue(""), strList("auto-%Y"), nullList, false},
		{"also_include alone", types.StringValue(""), nullList, strList("auto-%Y"), false},
		{"regex + naming_schema", types.StringValue("^auto-.*$"), strList("auto-%Y"), nullList, true},
		{"regex + also_include", types.StringValue("^auto-.*$"), nullList, strList("auto-%Y"), true},
		{"regex + both", types.StringValue("^auto-.*$"), strList("auto-%Y"), strList("auto-%Y"), true},
		{"regex + empty lists", types.StringValue("^auto-.*$"), emptyList, emptyList, false},
		{"regex null", types.StringNull(), strList("auto-%Y"), nullList, false},
		{"regex empty string", types.StringValue(""), strList("auto-%Y"), nullList, false},
		{"regex unknown", types.StringUnknown(), strList("auto-%Y"), nullList, false},
		{"naming_schema unknown", types.StringValue("^auto-.*$"), unknownList, nullList, false},
		{"also_include unknown", types.StringValue("^auto-.*$"), nullList, unknownList, false},
		{"nothing set", types.StringValue(""), nullList, nullList, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := &ReplicationModel{
				NameRegex:               tc.nameRegex,
				NamingSchema:            tc.namingSchema,
				AlsoIncludeNamingSchema: tc.alsoIncludeNamingSchema,
			}
			if got := nameRegexConflict(config); got != tc.want {
				t.Errorf("nameRegexConflict() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestReplicationPayload_CompressionSpeedLimitSet verifies compression and
// speed_limit pass through to the payload when set (SSH-only fields).
func TestReplicationPayload_CompressionSpeedLimitSet(t *testing.T) {
	ctx := context.Background()

	m := baseModel(ctx, t)
	m.Compression = types.StringValue("LZ4")
	m.SpeedLimit = types.Int64Value(1048576)

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned errors: %v", diags)
	}

	if v := payload["compression"]; v != "LZ4" {
		t.Errorf("payload[compression] = %v, want LZ4", v)
	}
	if v := payload["speed_limit"]; v != int64(1048576) {
		t.Errorf("payload[speed_limit] = %v, want 1048576", v)
	}
}

// TestReplicationPayload_CompressionSpeedLimitNullBecomesNil verifies the
// unset (null) case always sends an explicit nil rather than omitting the
// key — matching the lifetime_value/lifetime_unit nil-clearing pattern, so
// an in-place update can clear a previously-set compression/speed_limit.
func TestReplicationPayload_CompressionSpeedLimitNullBecomesNil(t *testing.T) {
	ctx := context.Background()

	m := baseModel(ctx, t)
	m.Compression = types.StringNull()
	m.SpeedLimit = types.Int64Null()

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned errors: %v", diags)
	}

	if v, ok := payload["compression"]; !ok || v != nil {
		t.Errorf("payload[compression] = %v, want nil", v)
	}
	if v, ok := payload["speed_limit"]; !ok || v != nil {
		t.Errorf("payload[speed_limit] = %v, want nil", v)
	}
}

// TestTransportSSHCredentialsMismatch exercises the preflight enforced in
// ValidateConfig: transport = "SSH" requires ssh_credentials, transport =
// "LOCAL" (explicit or defaulted from an unset/unknown config value)
// forbids it.
func TestTransportSSHCredentialsMismatch(t *testing.T) {
	cases := []struct {
		name      string
		transport types.String
		sshCreds  types.Int64
		wantBad   bool
	}{
		{"SSH with credentials", types.StringValue("SSH"), types.Int64Value(7), false},
		{"SSH without credentials", types.StringValue("SSH"), types.Int64Null(), true},
		{"SSH with zero credentials", types.StringValue("SSH"), types.Int64Value(0), true},
		{"SSH with unknown credentials", types.StringValue("SSH"), types.Int64Unknown(), false},
		{"LOCAL without credentials", types.StringValue("LOCAL"), types.Int64Null(), false},
		{"LOCAL with credentials", types.StringValue("LOCAL"), types.Int64Value(7), true},
		{"unset transport (defaults LOCAL) without credentials", types.StringNull(), types.Int64Null(), false},
		{"unset transport (defaults LOCAL) with credentials", types.StringNull(), types.Int64Value(7), true},
		{"unknown transport (not yet knowable) skips validation", types.StringUnknown(), types.Int64Value(7), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := &ReplicationModel{Transport: tc.transport, SSHCredentials: tc.sshCreds}
			got := transportSSHCredentialsMismatch(config) != ""
			if got != tc.wantBad {
				t.Errorf("transportSSHCredentialsMismatch() bad=%v, want %v", got, tc.wantBad)
			}
		})
	}
}

// TestSSHOnlyFieldsWithoutSSH exercises the preflight enforced in
// ValidateConfig: compression/speed_limit are only valid for transport =
// "SSH".
func TestSSHOnlyFieldsWithoutSSH(t *testing.T) {
	cases := []struct {
		name        string
		transport   types.String
		compression types.String
		speedLimit  types.Int64
		wantBad     bool
	}{
		{"SSH with both set", types.StringValue("SSH"), types.StringValue("LZ4"), types.Int64Value(100), false},
		{"SSH with neither set", types.StringValue("SSH"), types.StringNull(), types.Int64Null(), false},
		{"LOCAL with neither set", types.StringValue("LOCAL"), types.StringNull(), types.Int64Null(), false},
		{"LOCAL with compression set", types.StringValue("LOCAL"), types.StringValue("LZ4"), types.Int64Null(), true},
		{"LOCAL with speed_limit set", types.StringValue("LOCAL"), types.StringNull(), types.Int64Value(100), true},
		{"unset transport (defaults LOCAL) with compression set", types.StringNull(), types.StringValue("LZ4"), types.Int64Null(), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := &ReplicationModel{
				Transport:   tc.transport,
				Compression: tc.compression,
				SpeedLimit:  tc.speedLimit,
			}
			got := sshOnlyFieldsWithoutSSH(config) != ""
			if got != tc.wantBad {
				t.Errorf("sshOnlyFieldsWithoutSSH() bad=%v, want %v", got, tc.wantBad)
			}
		})
	}
}

// TestResponseToModel_CompressionSpeedLimit verifies both the nil (LOCAL
// task, or SSH task with neither set) and populated (SSH task) cases decode
// to null / concrete values respectively.
func TestResponseToModel_CompressionSpeedLimit(t *testing.T) {
	ctx := context.Background()

	api := &replicationAPI{
		ID: 1, Name: "t", Direction: "PUSH", Transport: "LOCAL",
		SourceDatasets: []string{"tank/data"}, TargetDataset: "backup/data", RetentionPolicy: "SOURCE",
	}
	var m ReplicationModel
	if diags := responseToModel(ctx, api, &m); diags.HasError() {
		t.Fatalf("responseToModel returned errors: %v", diags)
	}
	if !m.Compression.IsNull() {
		t.Error("Compression should be null when API compression is nil")
	}
	if !m.SpeedLimit.IsNull() {
		t.Error("SpeedLimit should be null when API speed_limit is nil")
	}

	compression := "PIGZ"
	speedLimit := int64(2048)
	api2 := &replicationAPI{
		ID: 2, Name: "t2", Direction: "PUSH", Transport: "SSH",
		SourceDatasets: []string{"tank/data"}, TargetDataset: "backup/data", RetentionPolicy: "SOURCE",
		Compression: &compression, SpeedLimit: &speedLimit,
	}
	var m2 ReplicationModel
	if diags := responseToModel(ctx, api2, &m2); diags.HasError() {
		t.Fatalf("responseToModel returned errors: %v", diags)
	}
	if m2.Compression.ValueString() != "PIGZ" {
		t.Errorf("Compression = %q, want PIGZ", m2.Compression.ValueString())
	}
	if m2.SpeedLimit.ValueInt64() != 2048 {
		t.Errorf("SpeedLimit = %v, want 2048", m2.SpeedLimit.ValueInt64())
	}
}

// TestReplicationPayload_NetcatFieldsSet verifies the netcat_* attributes
// pass through to the payload when set (SSH+NETCAT transport).
func TestReplicationPayload_NetcatFieldsSet(t *testing.T) {
	ctx := context.Background()

	m := baseModel(ctx, t)
	m.Transport = types.StringValue("SSH+NETCAT")
	m.NetcatActiveSide = types.StringValue("LOCAL")
	m.NetcatActiveSideListenAddress = types.StringValue("0.0.0.0")
	m.NetcatActiveSidePortMin = types.Int64Value(20000)
	m.NetcatActiveSidePortMax = types.Int64Value(20100)
	m.NetcatPassiveSideConnectAddress = types.StringValue("192.0.2.10")

	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned errors: %v", diags)
	}
	if v := payload["netcat_active_side"]; v != "LOCAL" {
		t.Errorf("payload[netcat_active_side] = %v, want LOCAL", v)
	}
	if v := payload["netcat_active_side_listen_address"]; v != "0.0.0.0" {
		t.Errorf("payload[netcat_active_side_listen_address] = %v, want 0.0.0.0", v)
	}
	if v := payload["netcat_active_side_port_min"]; v != int64(20000) {
		t.Errorf("payload[netcat_active_side_port_min] = %v, want 20000", v)
	}
	if v := payload["netcat_active_side_port_max"]; v != int64(20100) {
		t.Errorf("payload[netcat_active_side_port_max] = %v, want 20100", v)
	}
	if v := payload["netcat_passive_side_connect_address"]; v != "192.0.2.10" {
		t.Errorf("payload[netcat_passive_side_connect_address] = %v, want 192.0.2.10", v)
	}
}

// TestReplicationPayload_NetcatNullBecomesNil verifies unset netcat_* fields
// always send explicit nil (so switching away from SSH+NETCAT clears them).
func TestReplicationPayload_NetcatNullBecomesNil(t *testing.T) {
	ctx := context.Background()

	m := baseModel(ctx, t) // netcat fields default to null
	payload, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned errors: %v", diags)
	}
	for _, key := range []string{
		"netcat_active_side", "netcat_active_side_listen_address",
		"netcat_active_side_port_min", "netcat_active_side_port_max",
		"netcat_passive_side_connect_address",
	} {
		if v, ok := payload[key]; !ok || v != nil {
			t.Errorf("payload[%s] = %v (ok=%v), want nil", key, v, ok)
		}
	}
}

// TestResponseToModel_NetcatFields verifies the nil (non-netcat task) and
// populated (SSH+NETCAT task) cases decode to null / concrete values.
func TestResponseToModel_NetcatFields(t *testing.T) {
	ctx := context.Background()

	api := &replicationAPI{
		ID: 1, Name: "t", Direction: "PUSH", Transport: "LOCAL",
		SourceDatasets: []string{"tank/data"}, TargetDataset: "backup/data", RetentionPolicy: "SOURCE",
	}
	var m ReplicationModel
	if diags := responseToModel(ctx, api, &m); diags.HasError() {
		t.Fatalf("responseToModel returned errors: %v", diags)
	}
	if !m.NetcatActiveSide.IsNull() || !m.NetcatActiveSidePortMin.IsNull() {
		t.Error("netcat fields should be null when API returns nil")
	}

	side := "REMOTE"
	listen := "0.0.0.0"
	connect := "192.0.2.10"
	pmin := int64(20000)
	pmax := int64(20100)
	api2 := &replicationAPI{
		ID: 2, Name: "t2", Direction: "PUSH", Transport: "SSH+NETCAT",
		SourceDatasets: []string{"tank/data"}, TargetDataset: "backup/data", RetentionPolicy: "SOURCE",
		NetcatActiveSide: &side, NetcatActiveSideListenAddress: &listen,
		NetcatActiveSidePortMin: &pmin, NetcatActiveSidePortMax: &pmax,
		NetcatPassiveSideConnectAddress: &connect,
	}
	var m2 ReplicationModel
	if diags := responseToModel(ctx, api2, &m2); diags.HasError() {
		t.Fatalf("responseToModel returned errors: %v", diags)
	}
	if m2.NetcatActiveSide.ValueString() != "REMOTE" {
		t.Errorf("NetcatActiveSide = %q, want REMOTE", m2.NetcatActiveSide.ValueString())
	}
	if m2.NetcatActiveSidePortMax.ValueInt64() != 20100 {
		t.Errorf("NetcatActiveSidePortMax = %v, want 20100", m2.NetcatActiveSidePortMax.ValueInt64())
	}
}

// TestTransportSSHCredentialsMismatch_Netcat verifies SSH+NETCAT requires
// ssh_credentials, the same as SSH.
func TestTransportSSHCredentialsMismatch_Netcat(t *testing.T) {
	cases := []struct {
		name     string
		sshCreds types.Int64
		wantBad  bool
	}{
		{"SSH+NETCAT with credentials", types.Int64Value(7), false},
		{"SSH+NETCAT without credentials", types.Int64Null(), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := &ReplicationModel{Transport: types.StringValue("SSH+NETCAT"), SSHCredentials: tc.sshCreds}
			if got := transportSSHCredentialsMismatch(config) != ""; got != tc.wantBad {
				t.Errorf("transportSSHCredentialsMismatch() bad=%v, want %v", got, tc.wantBad)
			}
		})
	}
}

// TestNetcatFieldsMismatch exercises the SSH+NETCAT pairing: netcat_* fields
// are only valid for SSH+NETCAT, which in turn requires netcat_active_side.
func TestNetcatFieldsMismatch(t *testing.T) {
	cases := []struct {
		name       string
		transport  types.String
		activeSide types.String
		portMin    types.Int64
		wantBad    bool
	}{
		{"netcat on SSH+NETCAT with active side", types.StringValue("SSH+NETCAT"), types.StringValue("LOCAL"), types.Int64Value(20000), false},
		{"SSH+NETCAT missing active side", types.StringValue("SSH+NETCAT"), types.StringNull(), types.Int64Null(), true},
		{"netcat field set on SSH", types.StringValue("SSH"), types.StringNull(), types.Int64Value(20000), true},
		{"netcat field set on LOCAL", types.StringValue("LOCAL"), types.StringNull(), types.Int64Value(20000), true},
		{"active side set on SSH", types.StringValue("SSH"), types.StringValue("LOCAL"), types.Int64Null(), true},
		{"no netcat on SSH", types.StringValue("SSH"), types.StringNull(), types.Int64Null(), false},
		{"unknown transport skips", types.StringUnknown(), types.StringValue("LOCAL"), types.Int64Null(), false},
		{"SSH+NETCAT unknown active side skips", types.StringValue("SSH+NETCAT"), types.StringUnknown(), types.Int64Null(), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			config := &ReplicationModel{
				Transport:               tc.transport,
				NetcatActiveSide:        tc.activeSide,
				NetcatActiveSidePortMin: tc.portMin,
			}
			if got := netcatFieldsMismatch(config) != ""; got != tc.wantBad {
				t.Errorf("netcatFieldsMismatch() bad=%v, want %v", got, tc.wantBad)
			}
		})
	}
}

// baseModel builds a fully-populated, valid ReplicationModel for payload tests.
func baseModel(ctx context.Context, t *testing.T) ReplicationModel {
	t.Helper()
	return ReplicationModel{
		ID:                      types.Int64Value(1),
		Name:                    types.StringValue("test-task"),
		Direction:               types.StringValue("PUSH"),
		Transport:               types.StringValue("SSH"),
		SSHCredentials:          types.Int64Value(1),
		Sudo:                    types.BoolValue(false),
		Compression:             types.StringNull(),
		SpeedLimit:              types.Int64Null(),
		SourceDatasets:          types.ListValueMust(types.StringType, []attr.Value{types.StringValue("tank/data")}),
		TargetDataset:           types.StringValue("backup/data"),
		Recursive:               types.BoolValue(true),
		Exclude:                 types.ListValueMust(types.StringType, []attr.Value{}),
		Properties:              types.BoolValue(true),
		Replicate:               types.BoolValue(false),
		PeriodicSnapshotTasks:   types.ListValueMust(types.Int64Type, []attr.Value{}),
		NamingSchema:            types.ListValueMust(types.StringType, []attr.Value{types.StringValue("auto-%Y-%m-%d")}),
		AlsoIncludeNamingSchema: types.ListValueMust(types.StringType, []attr.Value{}),
		NameRegex:               types.StringValue(""),
		Auto:                    types.BoolValue(true),
		Schedule:                types.ObjectNull(scheduleAttrTypes),
		RetentionPolicy:         types.StringValue("SOURCE"),
		LifetimeValue:           types.Int64Value(0),
		LifetimeUnit:            types.StringValue(""),
		Readonly:                types.StringValue("SET"),
		Enabled:                 types.BoolValue(true),
		Retries:                 types.Int64Value(5),
	}
}
