package load

import (
	"strings"
	"testing"
)

func TestGenerateConfig(t *testing.T) {
	cfg := GenerateConfig("tank", 3, false)
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
	withSnap := GenerateConfig("tank", 2, true)
	if strings.Count(withSnap, "resource \"truenas_snapshot\"") != 2 {
		t.Errorf("want 2 snapshot resources:\n%s", withSnap)
	}
}
