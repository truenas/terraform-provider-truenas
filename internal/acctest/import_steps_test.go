// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest

import (
	"strings"
	"testing"
)

func TestRemovedConfig(t *testing.T) {
	cfg := `
provider "truenas" {}

resource "truenas_dataset" "parent" {
  name = "tank/p"
}

resource "truenas_snapshot" "test" {
  dataset = truenas_dataset.parent.name
  name    = "s1"
  nested {
    x = "}"
  }
}

resource "truenas_snapshot" "other" {
  dataset = "tank/q"
  name    = "s1"
}
`
	got := removedConfig(cfg, "truenas_snapshot.test")
	if strings.Contains(got, `"truenas_snapshot" "test"`) {
		t.Fatalf("resource block not removed:\n%s", got)
	}
	for _, want := range []string{
		`"truenas_dataset" "parent"`,
		`"truenas_snapshot" "other"`,
		"removed {\n  from = truenas_snapshot.test\n  lifecycle {\n    destroy = false\n  }\n}",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
}

func TestRemovedConfig_MissingBlockPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for a missing resource block")
		}
	}()
	removedConfig(`resource "truenas_snapshot" "a" {}`, "truenas_snapshot.b")
}
