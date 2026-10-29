// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

// CallFilesystemMutation runs a filesystem.setperm / filesystem.setacl job
// under fsMutationMu, serializing it against every other such mutation on this
// client. This works around a TrueNAS 25.10 middleware bug where concurrent
// permission/ACL mutations silently no-op (issue #45). Use it instead of
// CallJob for those two methods; everything else keeps calling CallJob.
func (c *Client) CallFilesystemMutation(ctx context.Context, method string, params ...any) (json.RawMessage, error) {
	c.fsMutationMu.Lock()
	defer c.fsMutationMu.Unlock()
	return c.CallJob(ctx, method, params...)
}

// CallJob calls a TrueNAS method that may run as a job and, when it did,
// polls until the job reaches SUCCESS or FAILED state.
//
// Whether the method is a job comes from the server's own method metadata
// (core.get_methods for the method's service, cached per client): a non-job
// method's result is returned as is, even when it is a bare integer that
// happens to equal a live job id (e.g. user.get_next_uid). Callers rely on
// this for methods that are jobs on one release and plain calls on another
// (pool.snapshottask.run is a job only on 27.0+).
//
// When the metadata is unavailable, CallJob falls back to treating a bare
// int result as a job id. If that int was really an object id,
// core.get_jobs never returns a matching entry; after maxJobPollMisses
// consecutive misses (~10s at the default interval) CallJob gives up on the
// "this was a job" theory and returns the original raw result.
//
// Polling goes through CallRead, so a connection drop while the job runs
// reconnects and keeps polling the same job id instead of failing an
// operation whose job goes on to succeed on the server.
func (c *Client) CallJob(ctx context.Context, method string, params ...any) (json.RawMessage, error) {
	raw, err := c.Call(ctx, method, params...)
	if err != nil {
		return nil, err
	}

	isJob, known := c.methodIsJob(ctx, method)
	if known && !isJob {
		return raw, nil
	}

	var jobID int64
	if err := json.Unmarshal(raw, &jobID); err != nil || jobID == 0 {
		// Method didn't return a job ID — return result directly
		return raw, nil
	}
	return c.waitJob(ctx, jobID, raw, known)
}

// waitJob polls core.get_jobs for jobID until it finishes. known says the
// server's metadata confirmed the method is a job; otherwise a job that
// never shows up is taken to be a plain integer result (see CallJob) and
// raw is returned.
func (c *Client) waitJob(ctx context.Context, jobID int64, raw json.RawMessage, known bool) (json.RawMessage, error) {
	ticker := time.NewTicker(jobPollInterval)
	defer ticker.Stop()

	misses := 0
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			jobs, err := c.CallRead(ctx, "core.get_jobs", []any{[]any{"id", "=", jobID}})
			if err != nil {
				return nil, fmt.Errorf("polling job %d: %w", jobID, err)
			}

			var entries []jobEntry
			if err := json.Unmarshal(jobs, &entries); err != nil || len(entries) == 0 {
				misses++
				if misses >= maxJobPollMisses {
					if known {
						return nil, fmt.Errorf("polling job %d: job not found in core.get_jobs", jobID)
					}
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

// methodIsJob reports whether method runs as a job on the connected server,
// from core.get_methods for the method's service (cached per service).
// known is false when the metadata could not be fetched or does not list
// the method.
func (c *Client) methodIsJob(ctx context.Context, method string) (isJob, known bool) {
	i := strings.LastIndex(method, ".")
	if i <= 0 {
		return false, false
	}
	service := method[:i]

	c.jobMethodsMu.Lock()
	defer c.jobMethodsMu.Unlock()
	flags, ok := c.jobMethods[service]
	if !ok {
		raw, err := c.CallRead(ctx, "core.get_methods", service)
		if err != nil {
			// Not cached: a later call may succeed.
			return false, false
		}
		var methods map[string]struct {
			Job bool `json:"job"`
		}
		if err := json.Unmarshal(raw, &methods); err != nil {
			return false, false
		}
		flags = make(map[string]bool, len(methods))
		for name, m := range methods {
			flags[name] = m.Job
		}
		if c.jobMethods == nil {
			c.jobMethods = make(map[string]map[string]bool)
		}
		c.jobMethods[service] = flags
	}
	isJob, known = flags[method]
	return isJob, known
}
