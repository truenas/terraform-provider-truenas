// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloud_backup

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

// baseUnsetModel returns a model with every Optional+Computed field
// null/unknown except the Required fields, for tests that only care about a
// subset.
func baseUnsetModel(path, attributesJSON, password string, credentials, keepLast int64) *CloudBackupModel {
	return &CloudBackupModel{
		Description: types.StringNull(),
		Path:        types.StringValue(path),
		Credentials: types.Int64Value(credentials),
		Attributes:  types.StringValue(attributesJSON),
		Schedule: ScheduleModel{
			Minute: types.StringNull(),
			Hour:   types.StringNull(),
			Dom:    types.StringNull(),
			Month:  types.StringNull(),
			Dow:    types.StringNull(),
		},
		PreScript:       types.StringNull(),
		PostScript:      types.StringNull(),
		Snapshot:        types.BoolNull(),
		Include:         types.ListNull(types.StringType),
		Exclude:         types.ListNull(types.StringType),
		Enabled:         types.BoolNull(),
		Password:        types.StringValue(password),
		KeepLast:        types.Int64Value(keepLast),
		TransferSetting: types.StringNull(),
		AbsolutePaths:   types.BoolNull(),
		CachePath:       types.StringNull(),
		RateLimit:       types.Int64Null(),
	}
}

// TestApiPayload_RequiredFieldsOnly verifies that with every Optional field
// null/unknown, the payload contains exactly the five Required keys —
// path, credentials, attributes, password, keep_last — matching
// cloud_backup.create's own "required" list (probed via core.get_methods),
// so TrueNAS-side defaults take effect for everything else.
func TestApiPayload_RequiredFieldsOnly(t *testing.T) {
	ctx := context.Background()
	m := baseUnsetModel("/mnt/tank/tf-acc", `{"bucket":"tf-acc-bucket","folder":"backups"}`, "s3cr3t", 5, 3)

	p, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostics errors: %v", diags)
	}

	wantKeys := []string{"path", "credentials", "attributes", "password", "keep_last"}
	if len(p) != len(wantKeys) {
		t.Fatalf("payload has %d keys (%v), want %d (%v)", len(p), p, len(wantKeys), wantKeys)
	}
	for _, k := range wantKeys {
		if _, ok := p[k]; !ok {
			t.Errorf("payload missing required key %q", k)
		}
	}
	if p["path"] != "/mnt/tank/tf-acc" {
		t.Errorf("payload[path] = %v, want /mnt/tank/tf-acc", p["path"])
	}
	if p["credentials"] != int64(5) {
		t.Errorf("payload[credentials] = %v, want 5", p["credentials"])
	}
	if p["password"] != "s3cr3t" {
		t.Errorf("payload[password] = %v, want s3cr3t", p["password"])
	}
	if p["keep_last"] != int64(3) {
		t.Errorf("payload[keep_last] = %v, want 3", p["keep_last"])
	}
	attrs, ok := p["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("payload[attributes] is %T, want map[string]any", p["attributes"])
	}
	if attrs["bucket"] != "tf-acc-bucket" {
		t.Errorf("payload[attributes][bucket] = %v, want tf-acc-bucket", attrs["bucket"])
	}
}

// TestApiPayload_OptionalFieldsIncludedWhenSet verifies every Optional field
// is included in the payload once set, alongside the schedule sub-object.
func TestApiPayload_OptionalFieldsIncludedWhenSet(t *testing.T) {
	ctx := context.Background()
	m := baseUnsetModel("/mnt/tank/tf-acc", `{"bucket":"b"}`, "pw", 1, 1)
	m.Description = types.StringValue("tf-acc-cloud-backup")
	m.Schedule = ScheduleModel{
		Minute: types.StringValue("0"),
		Hour:   types.StringValue("3"),
		Dom:    types.StringValue("*"),
		Month:  types.StringValue("*"),
		Dow:    types.StringValue("*"),
	}
	m.PreScript = types.StringValue("echo pre")
	m.PostScript = types.StringValue("echo post")
	m.Snapshot = types.BoolValue(true)
	m.Enabled = types.BoolValue(false)
	m.TransferSetting = types.StringValue("PERFORMANCE")
	m.AbsolutePaths = types.BoolValue(true)
	m.CachePath = types.StringValue("/mnt/tank/cache")
	m.RateLimit = types.Int64Value(1024)

	p, diags := m.apiPayload(ctx)
	if diags.HasError() {
		t.Fatalf("apiPayload returned diagnostics errors: %v", diags)
	}

	if p["description"] != "tf-acc-cloud-backup" {
		t.Errorf("payload[description] = %v, want tf-acc-cloud-backup", p["description"])
	}
	sched, ok := p["schedule"].(map[string]string)
	if !ok {
		t.Fatalf("payload[schedule] is %T, want map[string]string", p["schedule"])
	}
	if sched["hour"] != "3" {
		t.Errorf("payload[schedule][hour] = %v, want 3", sched["hour"])
	}
	if p["pre_script"] != "echo pre" {
		t.Errorf("payload[pre_script] = %v, want %q", p["pre_script"], "echo pre")
	}
	if p["snapshot"] != true {
		t.Errorf("payload[snapshot] = %v, want true", p["snapshot"])
	}
	if p["enabled"] != false {
		t.Errorf("payload[enabled] = %v, want false", p["enabled"])
	}
	if p["transfer_setting"] != "PERFORMANCE" {
		t.Errorf("payload[transfer_setting] = %v, want PERFORMANCE", p["transfer_setting"])
	}
	if p["absolute_paths"] != true {
		t.Errorf("payload[absolute_paths] = %v, want true", p["absolute_paths"])
	}
	if p["cache_path"] != "/mnt/tank/cache" {
		t.Errorf("payload[cache_path] = %v, want /mnt/tank/cache", p["cache_path"])
	}
	if p["rate_limit"] != int64(1024) {
		t.Errorf("payload[rate_limit] = %v, want 1024", p["rate_limit"])
	}
}

// TestUpdatePayload_ExcludesAbsolutePaths verifies updatePayload strips
// "absolute_paths" even when set on the model: cloud_backup.update's
// accepted fields (probed via core.get_methods, mirrored by middlewared's
// CloudBackupUpdate model) exclude it — it's create-only.
func TestUpdatePayload_ExcludesAbsolutePaths(t *testing.T) {
	ctx := context.Background()
	m := baseUnsetModel("/mnt/tank/tf-acc", `{"bucket":"b"}`, "pw", 1, 1)
	m.AbsolutePaths = types.BoolValue(true)

	p, diags := m.updatePayload(ctx)
	if diags.HasError() {
		t.Fatalf("updatePayload returned diagnostics errors: %v", diags)
	}
	if _, ok := p["absolute_paths"]; ok {
		t.Error("updatePayload should never include absolute_paths (create-only field)")
	}
}

// TestDecodeCredentialsID_EmbeddedObject verifies the shape actually
// probed live from cloudsync.credentials.create/cloud_backup responses: an
// embedded CloudCredentialEntry object.
func TestDecodeCredentialsID_EmbeddedObject(t *testing.T) {
	raw := []byte(`{"id": 5, "name": "tf-probe-cbcreds", "provider": {"type": "S3"}}`)
	id, err := decodeCredentialsID(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 5 {
		t.Errorf("id = %v, want 5", id)
	}
}

// TestDecodeCredentialsID_BareInteger verifies defensive support for a bare
// integer, in case a future API version returns one directly.
func TestDecodeCredentialsID_BareInteger(t *testing.T) {
	id, err := decodeCredentialsID([]byte("9"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != 9 {
		t.Errorf("id = %v, want 9", id)
	}
}

// TestDecodeCredentialsID_Null verifies a JSON null is rejected: credentials
// is Required on cloud_backup.create, so a null in a response indicates a
// malformed/unexpected API shape, not a legitimate "unset" state.
func TestDecodeCredentialsID_Null(t *testing.T) {
	if _, err := decodeCredentialsID([]byte("null")); err == nil {
		t.Fatal("expected an error for a null credentials field")
	}
}

// TestDecodeCredentialsID_Invalid verifies an undecodable shape returns an
// error rather than silently defaulting to 0.
func TestDecodeCredentialsID_Invalid(t *testing.T) {
	if _, err := decodeCredentialsID([]byte(`"not an object or integer"`)); err == nil {
		t.Fatal("expected an error for an undecodable credentials shape")
	}
}

// TestResponseToModel_FullShape verifies responseToModel against the field
// shape documented by core.get_methods (cloud_backup.create/get_instance
// returns, live-probed on TrueNAS 25.10) and middlewared's
// api/v25_10_2/cloud_backup.py CloudBackupEntry model: credentials embedded,
// nullable cache_path/rate_limit, password returned verbatim.
func TestResponseToModel_FullShape(t *testing.T) {
	ctx := context.Background()
	cachePath := "/mnt/tank/cache"
	rateLimit := int64(2048)
	api := &cloudBackupAPI{
		ID:              1,
		Description:     "tf-acc-cloud-backup",
		Path:            "/mnt/tank",
		Credentials:     []byte(`{"id": 5, "name": "tf-acc-creds", "provider": {"type": "S3"}}`),
		Attributes:      map[string]any{"bucket": "tf-acc-bucket", "folder": "backups"},
		Schedule:        scheduleAPI{Minute: "0", Hour: "3", Dom: "*", Month: "*", Dow: "*"},
		PreScript:       "echo pre",
		PostScript:      "echo post",
		Snapshot:        true,
		Include:         nil,
		Exclude:         []string{"*.tmp"},
		Enabled:         false,
		Password:        "s3cr3t-readback",
		KeepLast:        3,
		TransferSetting: "PERFORMANCE",
		AbsolutePaths:   false,
		CachePath:       &cachePath,
		RateLimit:       &rateLimit,
	}

	m := &CloudBackupModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	if m.ID.ValueInt64() != 1 {
		t.Errorf("ID = %v, want 1", m.ID)
	}
	if m.Credentials.ValueInt64() != 5 {
		t.Errorf("Credentials = %v, want 5", m.Credentials)
	}
	if m.Password.ValueString() != "s3cr3t-readback" {
		t.Errorf("Password = %q, want it returned verbatim (not masked)", m.Password.ValueString())
	}
	if m.KeepLast.ValueInt64() != 3 {
		t.Errorf("KeepLast = %v, want 3", m.KeepLast)
	}
	if m.TransferSetting.ValueString() != "PERFORMANCE" {
		t.Errorf("TransferSetting = %q, want PERFORMANCE", m.TransferSetting.ValueString())
	}
	if m.CachePath.ValueString() != cachePath {
		t.Errorf("CachePath = %q, want %q", m.CachePath.ValueString(), cachePath)
	}
	if m.RateLimit.ValueInt64() != rateLimit {
		t.Errorf("RateLimit = %v, want %v", m.RateLimit, rateLimit)
	}
	if m.Include.IsNull() {
		t.Error("Include should be an empty list, not null, when API returns nil")
	}
	var include []string
	diags = m.Include.ElementsAs(ctx, &include, false)
	if diags.HasError() {
		t.Fatalf("reading back include: %v", diags)
	}
	if len(include) != 0 {
		t.Errorf("Include = %v, want empty", include)
	}
}

// TestResponseToModel_NullableFieldsNull verifies cache_path and rate_limit
// decode to true Terraform null (not zero values) when the API returns null,
// matching the probed nullable shape.
func TestResponseToModel_NullableFieldsNull(t *testing.T) {
	ctx := context.Background()
	api := &cloudBackupAPI{
		ID:          2,
		Credentials: []byte(`{"id": 1, "name": "c", "provider": {"type": "S3"}}`),
		Attributes:  map[string]any{"bucket": "b"},
		Schedule:    scheduleAPI{Minute: "0", Hour: "*", Dom: "*", Month: "*", Dow: "*"},
		Password:    "pw",
		KeepLast:    1,
		CachePath:   nil,
		RateLimit:   nil,
	}

	m := &CloudBackupModel{}
	diags := responseToModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !m.CachePath.IsNull() {
		t.Errorf("CachePath = %v, want null", m.CachePath)
	}
	if !m.RateLimit.IsNull() {
		t.Errorf("RateLimit = %v, want null", m.RateLimit)
	}
}

// TestAttributesDrifted_UserKeyChanged verifies attributesDrifted reports
// drift when a user-set key's value differs between state and the API.
func TestAttributesDrifted_UserKeyChanged(t *testing.T) {
	state := map[string]any{"bucket": "tf-acc-bucket"}
	apiAttrs := map[string]any{"bucket": "some-other-bucket", "region": ""}
	if !attributesDrifted(state, apiAttrs) {
		t.Error("expected drift when a user-set key's value differs")
	}
}

// TestAttributesDrifted_APIAddedKeyIgnored verifies attributesDrifted does
// NOT report drift when the API merely adds default keys the user never set.
func TestAttributesDrifted_APIAddedKeyIgnored(t *testing.T) {
	state := map[string]any{"bucket": "tf-acc-bucket"}
	apiAttrs := map[string]any{"bucket": "tf-acc-bucket", "region": "", "storage_class": ""}
	if attributesDrifted(state, apiAttrs) {
		t.Error("expected no drift when the API only adds default keys")
	}
}

// TestResponseToDataSourceModel_FullShape mirrors
// TestResponseToModel_FullShape for the datasource model, and additionally
// verifies Attributes is populated as canonical JSON (the datasource has no
// plan to preserve write-what-you-said against, unlike the resource).
func TestResponseToDataSourceModel_FullShape(t *testing.T) {
	ctx := context.Background()
	api := &cloudBackupAPI{
		ID:          1,
		Description: "tf-acc-cloud-backup",
		Path:        "/mnt/tank",
		Credentials: []byte(`{"id": 5, "name": "tf-acc-creds", "provider": {"type": "S3"}}`),
		Attributes:  map[string]any{"bucket": "tf-acc-bucket"},
		Schedule:    scheduleAPI{Minute: "0", Hour: "3", Dom: "*", Month: "*", Dow: "*"},
		Password:    "s3cr3t-readback",
		KeepLast:    3,
	}

	m := &CloudBackupDataSourceModel{}
	diags := responseToDataSourceModel(ctx, api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.Attributes.ValueString() != `{"bucket":"tf-acc-bucket"}` {
		t.Errorf("Attributes = %q, want canonical JSON of the API attributes map", m.Attributes.ValueString())
	}
	if m.Description.ValueString() != "tf-acc-cloud-backup" {
		t.Errorf("Description = %q, want tf-acc-cloud-backup", m.Description.ValueString())
	}
}
