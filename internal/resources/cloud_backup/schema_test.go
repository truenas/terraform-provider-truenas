// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package cloud_backup

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// TestCloudBackupSchema_RequiredFields verifies "path", "credentials",
// "attributes", "password", and "keep_last" are Required, matching
// cloud_backup.create's own "required" list (probed via core.get_methods).
func TestCloudBackupSchema_RequiredFields(t *testing.T) {
	s := resourceSchema()

	stringFields := []string{"path", "attributes", "password"}
	for _, name := range stringFields {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		strAttr, ok := attr.(schema.StringAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.StringAttribute", name, attr)
		}
		if !strAttr.IsRequired() {
			t.Errorf("%q should be Required", name)
		}
	}

	intFields := []string{"credentials", "keep_last"}
	for _, name := range intFields {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		intAttr, ok := attr.(schema.Int64Attribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.Int64Attribute", name, attr)
		}
		if !intAttr.IsRequired() {
			t.Errorf("%q should be Required", name)
		}
	}
}

// TestCloudBackupSchema_IDIsComputed verifies "id" is Computed-only.
func TestCloudBackupSchema_IDIsComputed(t *testing.T) {
	s := resourceSchema()

	idAttr, ok := s.Attributes["id"]
	if !ok {
		t.Fatal("schema missing 'id' attribute")
	}
	idInt, ok := idAttr.(schema.Int64Attribute)
	if !ok {
		t.Fatalf("'id' attribute is %T, want schema.Int64Attribute", idAttr)
	}
	if !idInt.IsComputed() {
		t.Error("'id' should be Computed")
	}
	if idInt.IsRequired() || idInt.IsOptional() {
		t.Error("'id' should be Computed-only (not Required/Optional)")
	}
}

// TestCloudBackupSchema_PasswordIsSensitiveNotWriteOnly verifies "password"
// is marked Sensitive but NOT WriteOnly: source-verified (middleware's
// CloudBackupEntry.password is a pydantic Secret[NonEmptyString], read back
// unmasked to a FULL_ADMIN/CLOUD_BACKUP_WRITE-scoped session — see
// schema.go's description), so it is modeled like keychain_ssh_keypair's
// private_key rather than truenas_user's WriteOnly password.
func TestCloudBackupSchema_PasswordIsSensitiveNotWriteOnly(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["password"]
	if !ok {
		t.Fatal("schema missing 'password' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'password' attribute is %T, want schema.StringAttribute", attr)
	}
	if !strAttr.IsSensitive() {
		t.Error("'password' should be Sensitive")
	}
	if strAttr.IsWriteOnly() {
		t.Error("'password' should NOT be WriteOnly: cloud_backup.get_instance returns it unmasked to an admin-scoped session")
	}
	if !strAttr.IsRequired() {
		t.Error("'password' should be Required (cloud_backup.create requires it, minLength 1)")
	}
}

// TestCloudBackupSchema_AbsolutePathsRequiresReplace verifies "absolute_paths"
// forces replacement: cloud_backup.update's accepted fields (probed via
// core.get_methods) exclude it — it's create-only.
func TestCloudBackupSchema_AbsolutePathsRequiresReplace(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["absolute_paths"]
	if !ok {
		t.Fatal("schema missing 'absolute_paths' attribute")
	}
	boolAttr, ok := attr.(schema.BoolAttribute)
	if !ok {
		t.Fatalf("'absolute_paths' attribute is %T, want schema.BoolAttribute", attr)
	}
	if !boolAttr.IsOptional() || !boolAttr.IsComputed() {
		t.Error("'absolute_paths' should be Optional+Computed")
	}
	// boolplanmodifier.RequiresReplace() alongside UseStateForUnknown(): two
	// plan modifiers expected (cloud_backup.update's accepted fields exclude
	// absolute_paths, so any change must force a new resource).
	if len(boolAttr.PlanModifiers) != 2 {
		t.Errorf("'absolute_paths' should carry both UseStateForUnknown and RequiresReplace plan modifiers, got %d", len(boolAttr.PlanModifiers))
	}
}

// TestCloudBackupSchema_OptionalComputedFields verifies the fields carrying
// TrueNAS-side defaults are Optional+Computed.
func TestCloudBackupSchema_OptionalComputedFields(t *testing.T) {
	s := resourceSchema()

	boolFields := []string{"snapshot", "enabled", "absolute_paths"}
	for _, name := range boolFields {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		boolAttr, ok := attr.(schema.BoolAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.BoolAttribute", name, attr)
		}
		if !boolAttr.IsOptional() || !boolAttr.IsComputed() {
			t.Errorf("%q should be Optional+Computed", name)
		}
	}

	stringFields := []string{"description", "pre_script", "post_script", "transfer_setting", "cache_path"}
	for _, name := range stringFields {
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
	}

	if attr, ok := s.Attributes["rate_limit"]; !ok {
		t.Fatal("schema missing 'rate_limit' attribute")
	} else if intAttr, ok := attr.(schema.Int64Attribute); !ok {
		t.Fatalf("'rate_limit' attribute is %T, want schema.Int64Attribute", attr)
	} else if !intAttr.IsOptional() || !intAttr.IsComputed() {
		t.Error("'rate_limit' should be Optional+Computed")
	}
}

// TestCloudBackupSchema_ScheduleShape verifies the nested "schedule"
// attribute is Optional+Computed at the top level with Required string
// sub-fields, matching rsync_task's/cloudsync's convention.
func TestCloudBackupSchema_ScheduleShape(t *testing.T) {
	s := resourceSchema()

	schedAttr, ok := s.Attributes["schedule"]
	if !ok {
		t.Fatal("schema missing 'schedule' attribute")
	}
	nested, ok := schedAttr.(schema.SingleNestedAttribute)
	if !ok {
		t.Fatalf("'schedule' attribute is %T, want schema.SingleNestedAttribute", schedAttr)
	}
	if !nested.IsOptional() || !nested.IsComputed() {
		t.Error("'schedule' should be Optional+Computed")
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

// TestCloudBackupSchema_IncludeExcludeAreLists verifies "include"/"exclude"
// are Optional+Computed lists of strings.
func TestCloudBackupSchema_IncludeExcludeAreLists(t *testing.T) {
	s := resourceSchema()

	for _, name := range []string{"include", "exclude"} {
		attr, ok := s.Attributes[name]
		if !ok {
			t.Fatalf("schema missing %q attribute", name)
		}
		listAttr, ok := attr.(schema.ListAttribute)
		if !ok {
			t.Fatalf("%q attribute is %T, want schema.ListAttribute", name, attr)
		}
		if !listAttr.IsOptional() || !listAttr.IsComputed() {
			t.Errorf("%q should be Optional+Computed", name)
		}
	}
}

// TestCloudBackupSchema_TransferSettingValidator verifies "transfer_setting"
// carries an enum validator matching the choices returned by
// cloud_backup.transfer_setting_choices (probed live).
func TestCloudBackupSchema_TransferSettingValidator(t *testing.T) {
	s := resourceSchema()

	attr, ok := s.Attributes["transfer_setting"]
	if !ok {
		t.Fatal("schema missing 'transfer_setting' attribute")
	}
	strAttr, ok := attr.(schema.StringAttribute)
	if !ok {
		t.Fatalf("'transfer_setting' attribute is %T, want schema.StringAttribute", attr)
	}
	if len(strAttr.Validators) == 0 {
		t.Error("'transfer_setting' should carry a OneOf validator")
	}
}
