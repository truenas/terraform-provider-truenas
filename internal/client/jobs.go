// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type jobEntry struct {
	ID     int64           `json:"id"`
	State  string          `json:"state"`
	Result json.RawMessage `json:"result"`
	Error  string          `json:"error"`
}

// jobPollInterval is the interval between core.get_jobs polls in CallJob.
// It's a package variable (not a const) so tests can shrink it to avoid
// slow, real-time-bound test runs.
var jobPollInterval = 2 * time.Second

// maxJobPollMisses bounds how many consecutive core.get_jobs polls may come
// back with no matching entry before CallJob concludes the "job ID" it saw
// was never a real job and bails out. See the no-job heuristic comment on
// CallJob for why this is needed.
const maxJobPollMisses = 5

// CallJob calls a TrueNAS method that returns a job ID, then polls until
// the job reaches SUCCESS or FAILED state.
//
// Heuristic for sync methods that happen to return a bare int: some
// TrueNAS methods that are NOT jobs (job:false in the API) still return a
// plain integer result — e.g. group.create/user.create return the newly
// created object's numeric id, not a job id. CallJob can't tell the
// difference up front (both look like a bare int), so it optimistically
// polls core.get_jobs for it. If that int was actually just an object id
// and not a job id, core.get_jobs will never return a matching entry, and
// without a bail-out this would poll forever. We count consecutive polls
// that return zero matching entries; after maxJobPollMisses misses (~10s at
// the default interval) we give up on the "this was a job" theory and
// return the original raw result with a nil error, i.e. treat it as a
// synchronous, non-job result. If entries do appear at any point, the miss
// counter resets and normal job polling continues.
func (c *Client) CallJob(ctx context.Context, method string, params ...any) (json.RawMessage, error) {
	raw, err := c.Call(ctx, method, params...)
	if err != nil {
		return nil, err
	}

	var jobID int64
	if err := json.Unmarshal(raw, &jobID); err != nil || jobID == 0 {
		// Method didn't return a job ID — return result directly
		return raw, nil
	}

	ticker := time.NewTicker(jobPollInterval)
	defer ticker.Stop()

	misses := 0
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			jobs, err := c.Call(ctx, "core.get_jobs", []any{[]any{"id", "=", jobID}})
			if err != nil {
				return nil, fmt.Errorf("polling job %d: %w", jobID, err)
			}

			var entries []jobEntry
			if err := json.Unmarshal(jobs, &entries); err != nil || len(entries) == 0 {
				misses++
				if misses >= maxJobPollMisses {
					// jobID never showed up in core.get_jobs despite
					// repeated polling — it was never a job id, just a
					// bare int result from a synchronous method. Return
					// the original raw result instead of polling forever.
					return raw, nil
				}
				continue
			}
			misses = 0

			job := entries[0]
			switch job.State {
			case "SUCCESS":
				return job.Result, nil
			case "FAILED", "ABORTED":
				return nil, fmt.Errorf("job %d %s: %s", jobID, job.State, job.Error)
			}
			// WAITING, RUNNING — keep polling
		}
	}
}
