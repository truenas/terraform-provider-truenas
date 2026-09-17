// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package enclosure_label

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
)

// apiShape mirrors the live TrueNAS 25.10.4 Enterprise HA probe of the single
// shared H-series chassis enclosure (see the sibling internal/resources/
// enclosure package's model.go for the full probed top-level shape; this
// resource only decodes "id"/"label"/"name").
func apiShape() *enclosureLabelAPI {
	return &enclosureLabelAPI{
		ID:    "3b0ad6d1c00006e0",
		Label: "BROADCOM VirtualSES 03",
		Name:  "BROADCOM VirtualSES 03",
	}
}

func TestResponseToModel(t *testing.T) {
	m := &EnclosureLabelModel{}
	responseToModel(apiShape(), m)

	if m.ID.ValueString() != "3b0ad6d1c00006e0" {
		t.Errorf("ID = %q, want 3b0ad6d1c00006e0", m.ID.ValueString())
	}
	if m.Label.ValueString() != "BROADCOM VirtualSES 03" {
		t.Errorf("Label = %q, want BROADCOM VirtualSES 03", m.Label.ValueString())
	}
	if m.Name.ValueString() != "BROADCOM VirtualSES 03" {
		t.Errorf("Name = %q, want BROADCOM VirtualSES 03", m.Name.ValueString())
	}
}

func TestResponseToModel_LabelIndependentOfName(t *testing.T) {
	api := apiShape()
	api.Label = "tf-acc-enclosure-abc123"

	m := &EnclosureLabelModel{}
	responseToModel(api, m)
	if m.Label.ValueString() != "tf-acc-enclosure-abc123" {
		t.Errorf("Label = %q, want tf-acc-enclosure-abc123", m.Label.ValueString())
	}
	if m.Name.ValueString() != "BROADCOM VirtualSES 03" {
		t.Errorf("Name = %q, want BROADCOM VirtualSES 03 (unchanged)", m.Name.ValueString())
	}
}

// --- enclosureQueryArgs --------------------------------------------------

func TestEnclosureQueryArgs(t *testing.T) {
	args := enclosureQueryArgs("3b0ad6d1c00006e0")
	if len(args) != 1 {
		t.Fatalf("enclosureQueryArgs returned %d args, want 1 (filters only)", len(args))
	}
	filters, ok := args[0].([][]any)
	if !ok || len(filters) != 1 {
		t.Fatalf("enclosureQueryArgs()[0] = %#v, want a single-filter [][]any", args[0])
	}
	filter := filters[0]
	if len(filter) != 3 || filter[0] != "id" || filter[1] != "=" || filter[2] != "3b0ad6d1c00006e0" {
		t.Errorf("filter = %#v, want [id = 3b0ad6d1c00006e0]", filter)
	}
}

// --- captureOriginalLabel / privateSetter ---------------------------------

// fakePrivate is a minimal privateSetter test double: it records the
// key/value SetKey was called with, so tests can assert what
// captureOriginalLabel actually stores without needing a real
// *privatestate.ProviderData (an unexported terraform-plugin-framework
// type this package cannot construct directly).
type fakePrivate struct {
	key   string
	value []byte
	err   diag.Diagnostics
}

func (f *fakePrivate) SetKey(_ context.Context, key string, value []byte) diag.Diagnostics {
	f.key, f.value = key, value
	return f.err
}

func TestCaptureOriginalLabel_StoresJSONEncodedLabel(t *testing.T) {
	ctx := context.Background()
	p := &fakePrivate{}
	api := apiShape()

	if err := captureOriginalLabel(ctx, p, api); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.key != originalLabelPrivateKey {
		t.Errorf("SetKey called with key %q, want %q", p.key, originalLabelPrivateKey)
	}

	var got string
	if err := json.Unmarshal(p.value, &got); err != nil {
		t.Fatalf("stored value is not valid JSON: %v (value=%s)", err, p.value)
	}
	if got != api.Label {
		t.Errorf("stored original label = %q, want %q", got, api.Label)
	}
}

func TestCaptureOriginalLabel_PropagatesSetKeyError(t *testing.T) {
	ctx := context.Background()
	p := &fakePrivate{err: diag.Diagnostics{diag.NewErrorDiagnostic("boom", "boom detail")}}

	if err := captureOriginalLabel(ctx, p, apiShape()); err == nil {
		t.Error("expected an error when SetKey returns error diagnostics")
	}
}

// TestCaptureOriginalLabel_HandlesSpecialCharacters verifies a label
// containing characters that are meaningful in raw JSON (quotes, backslash,
// unicode) round-trips correctly through json.Marshal/Unmarshal — private
// state values must be valid JSON+UTF-8 (framework requirement), so this
// pins that captureOriginalLabel never hand-builds the JSON string itself.
func TestCaptureOriginalLabel_HandlesSpecialCharacters(t *testing.T) {
	ctx := context.Background()
	p := &fakePrivate{}
	api := &enclosureLabelAPI{ID: "x", Label: `weird "label" \ with unicode 日本語`, Name: "x"}

	if err := captureOriginalLabel(ctx, p, api); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got string
	if err := json.Unmarshal(p.value, &got); err != nil {
		t.Fatalf("stored value is not valid JSON: %v (value=%s)", err, p.value)
	}
	if got != api.Label {
		t.Errorf("stored original label = %q, want %q", got, api.Label)
	}
}
