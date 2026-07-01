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

// CallJob calls a TrueNAS method that returns a job ID, then polls until
// the job reaches SUCCESS or FAILED state.
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

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

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
				continue
			}

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
