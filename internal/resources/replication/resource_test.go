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

	// transport: Required + RequiresReplace.
	transAttr, ok := s.Attributes["transport"]
	if !ok {
		t.Fatal("schema missing 'transport' attribute")
	}
	transStr, ok := transAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'transport' is %T, want schema.StringAttribute", transAttr)
	}
	if !transStr.IsRequired() {
		t.Error("'transport' should be Required")
	}
	if len(transStr.PlanModifiers) == 0 {
		t.Error("'transport' should have RequiresReplace plan modifier")
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
