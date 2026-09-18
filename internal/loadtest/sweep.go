// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package loadtest

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// sweepMatch reads row[field] as a string and, if it has prefix, returns the
// row's numeric id and true. Returns (0,false) on any missing/typed-wrong
// field so a malformed row is skipped, not fatal.
func sweepMatch(row map[string]any, field, prefix string) (int64, bool) {
	name, _ := row[field].(string)
	if !strings.HasPrefix(name, prefix) {
		return 0, false
	}
	idf, ok := row["id"].(float64)
	if !ok {
		return 0, false
	}
	return int64(idf), true
}

// sweepKind is a non-dataset resource type swept by matching a name-like field
// against a prefix and deleting each match by its int64 id.
type sweepKind struct {
	query    string
	del      string
	field    string
	prefix   string
	delIsJob bool
}

// Sweep deletes every load-test object under pool: datasets/zvols (and their
// snapshots) whose id begins "<pool>/tf-load-", plus SMB/NFS shares, cronjobs,
// users, and groups whose identifying field begins "tf-load-". Returns the
// total deleted. Shares/cronjobs/users/groups are swept before datasets, since
// a share's path references a dataset. Safe from t.Cleanup; errors returned.
func Sweep(ctx context.Context, c *client.Client, pool string) (int, error) {
	deleted := 0

	kinds := []sweepKind{
		{"sharing.smb.query", "sharing.smb.delete", "name", "tf-load-", true},
		{"sharing.nfs.query", "sharing.nfs.delete", "path", "/mnt/" + pool + "/tf-load-", false},
		{"cronjob.query", "cronjob.delete", "description", "tf-load-", false},
		{"user.query", "user.delete", "username", "tf-load-", false},
		{"group.query", "group.delete", "name", "tf-load-", false},
	}
	for _, k := range kinds {
		raw, err := c.CallRead(ctx, k.query)
		if err != nil {
			return deleted, fmt.Errorf("sweep query %s: %w", k.query, err)
		}
		var rows []map[string]any
		if err := json.Unmarshal(raw, &rows); err != nil {
			return deleted, fmt.Errorf("sweep decode %s: %w", k.query, err)
		}
		for _, row := range rows {
			id, ok := sweepMatch(row, k.field, k.prefix)
			if !ok {
				continue
			}
			if k.delIsJob {
				_, err = c.CallJob(ctx, k.del, id)
			} else {
				_, err = c.Call(ctx, k.del, id)
			}
			if err != nil {
				return deleted, fmt.Errorf("sweep %s %d: %w", k.del, id, err)
			}
			deleted++
		}
	}

	// Datasets/zvols last (id is a string path; recursive+force removes child
	// snapshots and zvols).
	prefix := pool + "/tf-load-"
	raw, err := c.CallRead(ctx, "pool.dataset.query", []any{[]any{"id", "^", prefix}})
	if err != nil {
		return deleted, fmt.Errorf("sweep query pool.dataset: %w", err)
	}
	var dss []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &dss); err != nil {
		return deleted, fmt.Errorf("sweep decode pool.dataset: %w", err)
	}
	for _, d := range dss {
		if _, err := c.Call(ctx, "pool.dataset.delete", d.ID,
			map[string]any{"recursive": true, "force": true}); err != nil {
			return deleted, fmt.Errorf("sweep pool.dataset.delete %s: %w", d.ID, err)
		}
		deleted++
	}
	return deleted, nil
}
