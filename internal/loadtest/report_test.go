// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package loadtest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReport_WritesMarkdownAndJSON(t *testing.T) {
	dir := t.TempDir()
	in := ReportInput{
		Name:  "authburst",
		Stamp: "20260812T101500Z",
		Wall:  10 * time.Second,
		Snap: Snapshot{
			Attempts: 40, RateLimitHits: 20, Retries: 20,
			TotalBackoff: 100 * time.Second,
			Failures:     map[string]int{"calljob": 3},
		},
		Notes: map[string]string{"observed_login_ceiling": "~20/min"},
	}
	md, js, err := Report(dir, in)
	if err != nil {
		t.Fatalf("Report: %v", err)
	}
	if md != filepath.Join(dir, "load-report-authburst-20260812T101500Z.md") {
		t.Fatalf("md path = %q", md)
	}
	mdBody, _ := os.ReadFile(md)
	for _, want := range []string{"authburst", "40", "calljob", "observed_login_ceiling", "4.00"} {
		if !strings.Contains(string(mdBody), want) {
			t.Errorf("markdown missing %q", want)
		}
	}
	jsBody, _ := os.ReadFile(js)
	var got map[string]any
	if err := json.Unmarshal(jsBody, &got); err != nil {
		t.Fatalf("json invalid: %v", err)
	}
	if got["attempts"].(float64) != 40 {
		t.Fatalf("json attempts = %v", got["attempts"])
	}
}
