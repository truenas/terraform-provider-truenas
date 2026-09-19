// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package loadtest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type ReportInput struct {
	Name  string
	Stamp string
	Wall  time.Duration
	Snap  Snapshot
	Notes map[string]string
}

func throughput(s Snapshot, wall time.Duration) float64 {
	if wall <= 0 {
		return 0
	}
	return float64(s.Attempts) / wall.Seconds()
}

// Report writes load-report-<name>-<stamp>.{md,json} into dir and returns
// their paths.
func Report(dir string, in ReportInput) (string, string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	base := fmt.Sprintf("load-report-%s-%s", in.Name, in.Stamp)
	mdPath := filepath.Join(dir, base+".md")
	jsPath := filepath.Join(dir, base+".json")
	tps := throughput(in.Snap, in.Wall)

	var b strings.Builder
	fmt.Fprintf(&b, "# Load report: %s\n\n", in.Name)
	fmt.Fprintf(&b, "- Timestamp: %s\n- Wall clock: %s\n", in.Stamp, in.Wall)
	fmt.Fprintf(&b, "- Attempts: %d\n- Rate-limit hits: %d\n- Retries: %d\n",
		in.Snap.Attempts, in.Snap.RateLimitHits, in.Snap.Retries)
	fmt.Fprintf(&b, "- Total backoff: %s\n- Throughput: %.2f ops/s\n\n",
		in.Snap.TotalBackoff, tps)
	if len(in.Snap.Failures) > 0 {
		b.WriteString("## Failures by class\n\n")
		keys := make([]string, 0, len(in.Snap.Failures))
		for k := range in.Snap.Failures {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "- %s: %d\n", k, in.Snap.Failures[k])
		}
		b.WriteString("\n")
	}
	if len(in.Snap.FailureSamples) > 0 {
		b.WriteString("## Sample error per failure class\n\n")
		keys := make([]string, 0, len(in.Snap.FailureSamples))
		for k := range in.Snap.FailureSamples {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "- %s: %s\n", k, in.Snap.FailureSamples[k])
		}
		b.WriteString("\n")
	}
	if len(in.Notes) > 0 {
		b.WriteString("## Notes\n\n")
		keys := make([]string, 0, len(in.Notes))
		for k := range in.Notes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "- %s: %s\n", k, in.Notes[k])
		}
	}
	if err := os.WriteFile(mdPath, []byte(b.String()), 0o644); err != nil {
		return "", "", err
	}

	j := map[string]any{
		"name": in.Name, "stamp": in.Stamp, "wall_seconds": in.Wall.Seconds(),
		"attempts": in.Snap.Attempts, "rate_limit_hits": in.Snap.RateLimitHits,
		"retries": in.Snap.Retries, "total_backoff_seconds": in.Snap.TotalBackoff.Seconds(),
		"throughput_ops_s": tps, "failures": in.Snap.Failures,
		"failure_samples": in.Snap.FailureSamples, "notes": in.Notes,
	}
	jb, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return "", "", err
	}
	if err := os.WriteFile(jsPath, jb, 0o644); err != nil {
		return "", "", err
	}
	return mdPath, jsPath, nil
}
