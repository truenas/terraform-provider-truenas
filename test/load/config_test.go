// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package load

import (
	"strings"
	"testing"
)

func TestGenerateConfig(t *testing.T) {
	cfg := GenerateConfig("tank", "tf-load", 3, false)
	if strings.Count(cfg, "resource \"truenas_dataset\"") != 3 {
		t.Fatalf("want 3 dataset resources, got:\n%s", cfg)
	}
	for _, want := range []string{`name = "tank/tf-load-0"`, `name = "tank/tf-load-2"`, `provider "truenas"`} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config missing %q", want)
		}
	}
	if strings.Contains(cfg, "truenas_snapshot") {
		t.Error("snapshots=false must not emit snapshot resources")
	}
	withSnap := GenerateConfig("tank", "tf-load", 2, true)
	if strings.Count(withSnap, "resource \"truenas_snapshot\"") != 2 {
		t.Errorf("want 2 snapshot resources:\n%s", withSnap)
	}
}

func TestGenerateMixedConfig(t *testing.T) {
	cfg := GenerateMixedConfig("tank", 2, 7)
	for _, res := range []string{
		"truenas_dataset", "truenas_zvol", "truenas_snapshot", "truenas_smb_share",
		"truenas_nfs_share", "truenas_user", "truenas_group", "truenas_cronjob",
	} {
		if strings.Count(cfg, `resource "`+res+`"`) != 2 {
			t.Errorf("want 2 %s resources", res)
		}
	}
	for _, want := range []string{
		`name = "tank/tf-load-ds-0"`,
		`username  = "tf-load-usr-1"`,
		"v7",                             // the variant marker
		"truenas_dataset.ds0.mountpoint", // share depends on dataset
		"truenas_dataset.ds1.name",       // snapshot depends on dataset
	} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config missing %q", want)
		}
	}
}
