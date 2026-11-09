// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestOriginValueAndDataset(t *testing.T) {
	if v := OriginValue(""); !v.IsNull() {
		t.Errorf("empty origin -> %v, want null", v)
	}
	if v := OriginValue("tank/g@s"); v.ValueString() != "tank/g@s" {
		t.Errorf("origin -> %v", v)
	}
	if d := OriginDataset("tank/g/x@snap-1"); d != "tank/g/x" {
		t.Errorf("OriginDataset = %q", d)
	}
}

func TestCheckOriginConfig(t *testing.T) {
	if d := CheckOriginConfig("tank/g@s", "zvol", false); d.HasError() {
		t.Errorf("valid origin rejected: %v", d)
	}
	for _, bad := range []string{"tank/g", "@s", "tank/g@"} {
		if d := CheckOriginConfig(bad, "zvol", false); !d.HasError() || !strings.Contains(d.Errors()[0].Summary(), "must name a snapshot") {
			t.Errorf("%q accepted: %v", bad, d)
		}
	}
	if d := CheckOriginConfig("tank/g@s", "dataset", true); !d.HasError() || !strings.Contains(d.Errors()[0].Summary(), "encryption") {
		t.Errorf("origin + encryption accepted: %v", d)
	}
}

func TestAnyEncryptionInput(t *testing.T) {
	var m DatasetModel
	m.Encryption, m.InheritEncryption, m.EncryptionAlgorithm = types.BoolNull(), types.BoolNull(), types.StringNull()
	m.EncryptionGenerateKey, m.EncryptionPassphrase, m.EncryptionKey = types.BoolNull(), types.StringNull(), types.StringNull()
	if m.anyEncryptionInput() {
		t.Error("all null reported as set")
	}
	m.EncryptionPassphrase = types.StringValue("secret-passphrase")
	if !m.anyEncryptionInput() {
		t.Error("passphrase not reported")
	}
}

func TestCloneSnapshotPayload(t *testing.T) {
	f := &fakeAPI{}
	if err := CloneSnapshot(context.Background(), f, "tank/g@s", "tank/c"); err != nil {
		t.Fatal(err)
	}
	if len(f.calls) != 1 || !strings.Contains(f.calls[0], "pool.snapshot.clone [map[dataset_dst:tank/c snapshot:tank/g@s]]") {
		t.Errorf("calls = %v", f.calls)
	}
}
