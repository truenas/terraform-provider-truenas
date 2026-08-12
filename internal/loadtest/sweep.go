// Copyright iXsystems, Inc. 2026
// SPDX-License-Identifier: MPL-2.0

package loadtest

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// Sweep deletes every dataset (and its snapshots) under pool whose name
// component begins with "tf-load-". Returns the count deleted. Safe to call
// from t.Cleanup; errors are returned, not panicked.
func Sweep(ctx context.Context, c *client.Client, pool string) (int, error) {
	// Datasets whose id starts with "<pool>/tf-load-".
	prefix := pool + "/tf-load-"
	filter := []any{[]any{"id", "^", prefix}}
	raw, err := c.CallRead(ctx, "pool.dataset.query", filter)
	if err != nil {
		return 0, fmt.Errorf("sweep query: %w", err)
	}
	var rows []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &rows); err != nil {
		return 0, fmt.Errorf("sweep decode: %w", err)
	}
	deleted := 0
	for _, r := range rows {
		// recursive+force delete removes child snapshots too.
		if _, err := c.Call(ctx, "pool.dataset.delete", r.ID,
			map[string]any{"recursive": true, "force": true}); err != nil {
			return deleted, fmt.Errorf("sweep delete %s: %w", r.ID, err)
		}
		deleted++
	}
	return deleted, nil
}
