// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package pool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/truenas/terraform-provider-truenas/internal/resources/dataset"
)

// A pool destroy (pool.export with destroy=true) erases every dataset, zvol,
// snapshot and file on it with no way back. Until 1.6.2 the provider did that
// on any `terraform destroy` with nothing in the way. Like truenas_dataset and
// truenas_zvol, Delete now refuses while the pool holds anything unless the
// configuration says force_destroy = true. A freshly created, still-empty pool
// destroys without the flag.

// maxListedPoolContents caps how many items the refusal names.
const maxListedPoolContents = 10

// contentsCaller is the client surface poolContents needs (a fake drives the
// unit test).
type contentsCaller interface {
	Call(ctx context.Context, method string, params ...any) (json.RawMessage, error)
	CallJob(ctx context.Context, method string, params ...any) (json.RawMessage, error)
}

// poolContents lists what a destroy of pool name would take with it: child
// datasets and zvols ("name (TYPE)"), snapshots of the root dataset, and files
// or directories on the root dataset itself (a sample). Empty means the pool
// holds nothing but its empty root dataset.
func poolContents(ctx context.Context, c contentsCaller, name string) ([]string, error) {
	var out []string

	raw, err := c.Call(ctx, "pool.dataset.query",
		[]any{[]any{"pool", "=", name}},
		map[string]any{"extra": map[string]any{"flat": true, "retrieve_children": true, "properties": []string{}}, "select": []string{"name", "type"}})
	if err != nil {
		return nil, fmt.Errorf("pool.dataset.query: %w", err)
	}
	var datasets []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &datasets); err != nil {
		return nil, fmt.Errorf("parse pool.dataset.query: %w", err)
	}
	for _, d := range datasets {
		if d.Name == name {
			continue // the root dataset is the pool itself
		}
		out = append(out, fmt.Sprintf("%s (%s)", d.Name, d.Type))
	}

	raw, err = c.Call(ctx, "pool.snapshot.query",
		[]any{[]any{"dataset", "=", name}},
		map[string]any{"select": []string{"name"}})
	if err == nil { // 28.0 moved the namespace; a missing method is not a blocker on its own
		var snaps []struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(raw, &snaps); err == nil {
			for _, s := range snaps {
				out = append(out, s.Name+" (snapshot)")
			}
		}
	}

	files, err := dataset.NonEmptyContents(ctx, c, "/mnt/"+name)
	if err != nil {
		return nil, fmt.Errorf("listing /mnt/%s: %w", name, err)
	}
	for _, f := range files {
		out = append(out, f+" (file)")
	}
	return out, nil
}

// refusalMessage formats the destroy refusal.
func refusalMessage(name string, contents []string) string {
	shown := contents
	more := ""
	if len(shown) > maxListedPoolContents {
		more = fmt.Sprintf(", and %d more", len(shown)-maxListedPoolContents)
		shown = shown[:maxListedPoolContents]
	}
	return fmt.Sprintf("pool %q still holds data: %s%s. Destroying the pool erases all of it permanently. "+
		"Set force_destroy = true (and apply) to destroy the pool together with its contents, or move the "+
		"data off the pool first.", name, strings.Join(shown, ", "), more)
}
