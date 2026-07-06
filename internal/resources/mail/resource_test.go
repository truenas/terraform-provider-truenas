package mail

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// TestMailSchema_IDIsComputed verifies that "id" is a Computed-only
// StringAttribute, since it's a fixed singleton value never supplied by the
// user.
func TestMailSchema_IDIsComputed(t *testing.T) {
	s := resourceSchema()

	idAttr, ok := s.Attributes["id"]
	if !ok {
		t.Fatal("schema missing 'id' attribute")
	}
	idStr, ok := idAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'id' attribute is %T, want schema.StringAttribute", idAttr)
	}
	if !idStr.IsComputed() {
		t.Error("'id' should be Computed")
	}
	if idStr.IsRequired() || idStr.IsOptional() {
		t.Error("'id' should be Computed-only (not Required/Optional)")
	}
	if len(idStr.PlanModifiers) == 0 {
		t.Error("'id' should have plan modifiers (UseStateForUnknown)")
	}
}

// TestMailSchema_PassIsSensitiveWriteOnly verifies that "pass" is Optional +
// Sensitive, and specifically NOT Computed (write-only: never read back from
// TrueNAS, so it must not participate in drift detection).
func TestMailSchema_PassIsSensitiveWriteOnly(t *testing.T) {
	s := resourceSchema()

	passAttr, ok := s.Attributes["pass"]
	if !ok {
		t.Fatal("schema missing 'pass' attribute")
	}
	passStr, ok := passAttr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'pass' attribute is %T, want schema.StringAttribute", passAttr)
	}
	if !passStr.IsOptional() {
		t.Error("'pass' should be Optional")
	}
	if !passStr.IsSensitive() {
		t.Error("'pass' should be Sensitive")
	}
	if passStr.IsComputed() {
		t.Error("'pass' should NOT be Computed (write-only, never returned by the API)")
	}
	if passStr.IsRequired() {
		t.Error("'pass' should not be Required")
	}
}

// TestMailSchema_OtherFieldsAreOptionalComputed verifies that every
// non-pass, non-id config field is Optional+Computed with UseStateForUnknown
// plan modifiers, matching the "all other fields" contract in the task
// brief.
func TestMailSchema_OtherFieldsAreOptionalComputed(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"fromemail", "fromname", "outgoingserver", "security", "user"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		strAttr, ok := attr.(schema.StringAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.StringAttribute", name, attr)
		}
		if !strAttr.IsOptional() || !strAttr.IsComputed() {
			t.Errorf("%q should be Optional+Computed", name)
		}
		if strAttr.IsSensitive() {
			t.Errorf("%q should not be Sensitive", name)
		}
		if len(strAttr.PlanModifiers) == 0 {
			t.Errorf("%q should have plan modifiers (UseStateForUnknown)", name)
		}
	}

	portAttr, ok := s.Attributes["port"]
	if !ok {
		t.Fatal("schema missing 'port' attribute")
	}
	portInt, ok := portAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'port' attribute is %T, want schema.Int64Attribute", portAttr)
	}
	if !portInt.IsOptional() || !portInt.IsComputed() {
		t.Error("'port' should be Optional+Computed")
	}
	if len(portInt.PlanModifiers) == 0 {
		t.Error("'port' should have plan modifiers (UseStateForUnknown)")
	}

	smtpAttr, ok := s.Attributes["smtp"]
	if !ok {
		t.Fatal("schema missing 'smtp' attribute")
	}
	smtpBool, ok := smtpAttr.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'smtp' attribute is %T, want schema.BoolAttribute", smtpAttr)
	}
	if !smtpBool.IsOptional() || !smtpBool.IsComputed() {
		t.Error("'smtp' should be Optional+Computed")
	}
	if len(smtpBool.PlanModifiers) == 0 {
		t.Error("'smtp' should have plan modifiers (UseStateForUnknown)")
	}
}

// TestResponseToModel_PassNeverSet verifies that responseToModel never
// writes to m.Pass, regardless of its prior value: pass is write-only and
// the API never returns it.
func TestResponseToModel_PassNeverSet(t *testing.T) {
	api := &mailAPI{
		ID:             1,
		FromEmail:      "root@example.com",
		FromName:       "TrueNAS",
		OutgoingServer: "smtp.example.com",
		Port:           25,
		Security:       "PLAIN",
		SMTP:           false,
		User:           nil,
	}

	m := &MailModel{Pass: types.StringValue("super-secret")}
	diags := responseToModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.Pass.ValueString() != "super-secret" {
		t.Errorf("Pass = %q, want unchanged %q", m.Pass.ValueString(), "super-secret")
	}

	m2 := &MailModel{Pass: types.StringNull()}
	diags = responseToModel(api, m2)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if !m2.Pass.IsNull() {
		t.Errorf("Pass = %v, want still null", m2.Pass)
	}
}

// TestResponseToModel_UserNilBecomesEmptyString verifies that a nil User
// from the API maps to an empty string in the model (not null).
func TestResponseToModel_UserNilBecomesEmptyString(t *testing.T) {
	api := &mailAPI{
		ID:             1,
		FromEmail:      "root@example.com",
		FromName:       "TrueNAS",
		OutgoingServer: "smtp.example.com",
		Port:           25,
		Security:       "PLAIN",
		SMTP:           false,
		User:           nil,
	}

	m := &MailModel{}
	diags := responseToModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.User.IsNull() {
		t.Error("User should not be null when API returns nil, want empty string")
	}
	if m.User.ValueString() != "" {
		t.Errorf("User = %q, want empty string", m.User.ValueString())
	}
}

// TestResponseToModel_UserSet verifies that a non-nil User from the API is
// carried through as-is.
func TestResponseToModel_UserSet(t *testing.T) {
	user := "smtp-user"
	api := &mailAPI{
		ID:             1,
		FromEmail:      "root@example.com",
		FromName:       "TrueNAS",
		OutgoingServer: "smtp.example.com",
		Port:           25,
		Security:       "PLAIN",
		SMTP:           true,
		User:           &user,
	}

	m := &MailModel{}
	diags := responseToModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.User.ValueString() != user {
		t.Errorf("User = %q, want %q", m.User.ValueString(), user)
	}
	if m.ID.ValueString() != mailResourceID {
		t.Errorf("ID = %q, want %q", m.ID.ValueString(), mailResourceID)
	}
}

// TestResponseToDataSourceModel_UserNilBecomesEmptyString mirrors the same
// nil-to-empty-string handling for the datasource model.
func TestResponseToDataSourceModel_UserNilBecomesEmptyString(t *testing.T) {
	api := &mailAPI{
		ID:             1,
		FromEmail:      "root@example.com",
		FromName:       "TrueNAS",
		OutgoingServer: "smtp.example.com",
		Port:           25,
		Security:       "PLAIN",
		SMTP:           false,
		User:           nil,
	}

	m := &MailDataSourceModel{}
	diags := responseToDataSourceModel(api, m)
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if m.User.ValueString() != "" {
		t.Errorf("User = %q, want empty string", m.User.ValueString())
	}
}

// TestUpdatePayload_OnlyKnownFieldsSent verifies that updatePayload omits
// any field whose model value is null or unknown, for every guarded field
// except user/pass (which have their own tests below).
func TestUpdatePayload_OnlyKnownFieldsSent(t *testing.T) {
	m := &MailModel{
		FromEmail:      types.StringNull(),
		FromName:       types.StringValue("TrueNAS"),
		OutgoingServer: types.StringUnknown(),
		Port:           types.Int64Value(587),
		Security:       types.StringNull(),
		SMTP:           types.BoolUnknown(),
		User:           types.StringNull(),
		Pass:           types.StringNull(),
	}

	p := m.updatePayload()

	if _, ok := p["fromemail"]; ok {
		t.Error("expected 'fromemail' to be omitted (null)")
	}
	if v, ok := p["fromname"]; !ok || v != "TrueNAS" {
		t.Errorf("expected 'fromname' = 'TrueNAS', got %v (present=%v)", v, ok)
	}
	if _, ok := p["outgoingserver"]; ok {
		t.Error("expected 'outgoingserver' to be omitted (unknown)")
	}
	if v, ok := p["port"]; !ok || v != int64(587) {
		t.Errorf("expected 'port' = 587, got %v (present=%v)", v, ok)
	}
	if _, ok := p["security"]; ok {
		t.Error("expected 'security' to be omitted (null)")
	}
	if _, ok := p["smtp"]; ok {
		t.Error("expected 'smtp' to be omitted (unknown)")
	}
}

// TestUpdatePayload_AllKnownFieldsSent verifies that updatePayload includes
// every guarded field when its model value is known.
func TestUpdatePayload_AllKnownFieldsSent(t *testing.T) {
	m := &MailModel{
		FromEmail:      types.StringValue("root@example.com"),
		FromName:       types.StringValue("TrueNAS"),
		OutgoingServer: types.StringValue("smtp.example.com"),
		Port:           types.Int64Value(587),
		Security:       types.StringValue("TLS"),
		SMTP:           types.BoolValue(true),
		User:           types.StringNull(),
		Pass:           types.StringNull(),
	}

	p := m.updatePayload()

	want := map[string]any{
		"fromemail":      "root@example.com",
		"fromname":       "TrueNAS",
		"outgoingserver": "smtp.example.com",
		"port":           int64(587),
		"security":       "TLS",
		"smtp":           true,
	}
	for k, v := range want {
		if p[k] != v {
			t.Errorf("payload[%q] = %v, want %v", k, p[k], v)
		}
	}
	if len(p) != len(want) {
		t.Errorf("payload has %d keys (%v), want %d (user/pass should be omitted)", len(p), p, len(want))
	}
}

// TestUpdatePayload_PassOnlyWhenSet verifies that pass is included only when
// it has a known, non-null value, and is otherwise omitted entirely (never
// sent as an empty string or null).
func TestUpdatePayload_PassOnlyWhenSet(t *testing.T) {
	base := func() *MailModel {
		return &MailModel{
			FromEmail:      types.StringValue("root@example.com"),
			FromName:       types.StringValue("TrueNAS"),
			OutgoingServer: types.StringValue("smtp.example.com"),
			Port:           types.Int64Value(587),
			Security:       types.StringValue("TLS"),
			SMTP:           types.BoolValue(true),
			User:           types.StringNull(),
		}
	}

	m := base()
	m.Pass = types.StringValue("hunter2")
	p := m.updatePayload()
	if v, ok := p["pass"]; !ok || v != "hunter2" {
		t.Errorf("expected 'pass' = 'hunter2', got %v (present=%v)", v, ok)
	}

	m2 := base()
	m2.Pass = types.StringNull()
	p2 := m2.updatePayload()
	if _, ok := p2["pass"]; ok {
		t.Error("expected 'pass' to be omitted when null")
	}

	m3 := base()
	m3.Pass = types.StringUnknown()
	p3 := m3.updatePayload()
	if _, ok := p3["pass"]; ok {
		t.Error("expected 'pass' to be omitted when unknown")
	}
}

// TestUpdatePayload_UserOnlyWhenNonEmpty verifies that user is omitted when
// null/unknown, sent as nil when explicitly cleared to an empty string (so a
// previously set user can be cleared), and sent as-is when non-empty.
func TestUpdatePayload_UserOnlyWhenNonEmpty(t *testing.T) {
	base := func() *MailModel {
		return &MailModel{
			FromEmail:      types.StringValue("root@example.com"),
			FromName:       types.StringValue("TrueNAS"),
			OutgoingServer: types.StringValue("smtp.example.com"),
			Port:           types.Int64Value(587),
			Security:       types.StringValue("TLS"),
			SMTP:           types.BoolValue(true),
			Pass:           types.StringNull(),
		}
	}

	m := base()
	m.User = types.StringValue("smtp-user")
	p := m.updatePayload()
	if v, ok := p["user"]; !ok || v != "smtp-user" {
		t.Errorf("expected 'user' = 'smtp-user', got %v (present=%v)", v, ok)
	}

	m2 := base()
	m2.User = types.StringValue("")
	p2 := m2.updatePayload()
	if v, ok := p2["user"]; !ok {
		t.Error("expected 'user' to be present (nil) when explicitly set to empty string")
	} else if v != nil {
		t.Errorf("expected 'user' = nil when explicitly set to empty string, got %v", v)
	}

	m3 := base()
	m3.User = types.StringNull()
	p3 := m3.updatePayload()
	if _, ok := p3["user"]; ok {
		t.Error("expected 'user' to be omitted when null")
	}

	m4 := base()
	m4.User = types.StringUnknown()
	p4 := m4.updatePayload()
	if _, ok := p4["user"]; ok {
		t.Error("expected 'user' to be omitted when unknown")
	}
}

// TestDeleteWarningDiagnostics verifies that Delete's diagnostic builder
// returns exactly one warning (no errors) and does not require or touch a
// client — this is what makes "Delete makes no client calls" verifiable:
// MailResource.Delete calls only this pure function.
func TestDeleteWarningDiagnostics(t *testing.T) {
	diags := deleteWarningDiagnostics()
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	if len(diags) != 1 {
		t.Fatalf("expected exactly 1 diagnostic, got %d: %v", len(diags), diags)
	}
	summary := diags[0].Summary()
	detail := diags[0].Detail()
	if summary == "" || detail == "" {
		t.Error("expected non-empty summary and detail")
	}
	if detail != "mail configuration left in place; removed from Terraform state only" {
		t.Errorf("detail = %q, want the documented warning text", detail)
	}
}
