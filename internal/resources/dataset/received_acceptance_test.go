// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/truenas/terraform-provider-truenas/internal/acctest"
)

// TestAccDataset_receivedProperties imports a real ZFS replication target.
// Received values must survive refresh without replacing the dataset or
// rewriting properties which already match the configuration.
func TestAccDataset_receivedProperties(t *testing.T) {
	acctest.PreCheck(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	c := acctest.Client()
	prefix := acctest.RandName("tf-acc-received")
	source := acctest.TestPool() + "/" + prefix + "-source"
	target := acctest.TestPool() + "/" + prefix + "-target"
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), time.Minute)
		defer done()
		for _, name := range []string{target, source} {
			raw, err := c.CallRead(cleanup, "pool.dataset.query", []any{[]any{"id", "=", name}})
			if err != nil {
				t.Errorf("query cleanup %s: %v", name, err)
				continue
			}
			var datasets []json.RawMessage
			if err := json.Unmarshal(raw, &datasets); err != nil {
				t.Error(err)
				continue
			}
			if len(datasets) > 0 {
				if _, err := c.CallJob(cleanup, "pool.dataset.delete", name, map[string]any{"recursive": true}); err != nil {
					t.Errorf("delete %s: %v", name, err)
				}
			}
		}
	})
	if _, err := c.Call(ctx, "pool.dataset.create", map[string]any{
		"name": source, "acltype": "POSIX", "compression": "LZ4", "atime": "OFF", "recordsize": "128K", "special_small_block_size": 0,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Call(ctx, "pool.snapshot.create", map[string]any{"dataset": source, "name": prefix, "recursive": true}); err != nil {
		t.Fatal(err)
	}
	raw, err := c.Call(ctx, "replication.create", map[string]any{
		"name": prefix, "direction": "PUSH", "transport": "LOCAL", "source_datasets": []string{source}, "target_dataset": target,
		"recursive": true, "replicate": true, "auto": false, "retention_policy": "SOURCE", "name_regex": prefix, "readonly": "IGNORE",
	})
	if err != nil {
		t.Fatal(err)
	}
	var task struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(raw, &task); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanup, done := context.WithTimeout(context.Background(), time.Minute)
		defer done()
		raw, err := c.CallRead(cleanup, "replication.query", []any{[]any{"id", "=", task.ID}})
		if err != nil {
			t.Error(err)
			return
		}
		var tasks []json.RawMessage
		if err := json.Unmarshal(raw, &tasks); err != nil {
			t.Error(err)
			return
		}
		if len(tasks) == 0 {
			return
		}
		if _, err := c.Call(cleanup, "replication.delete", task.ID); err != nil {
			t.Error(err)
		}
	})
	if _, err := c.CallJob(ctx, "replication.run", task.ID); err != nil {
		t.Fatal(err)
	}
	// Detach the completed task before Terraform destroys its target fixture.
	if _, err := c.Call(ctx, "replication.delete", task.ID); err != nil {
		t.Fatal(err)
	}
	raw, err = c.CallRead(ctx, "pool.dataset.get_instance", target)
	if err != nil {
		t.Fatal(err)
	}
	var received struct {
		ACLType struct {
			Source string `json:"source"`
		} `json:"acltype"`
	}
	if err := json.Unmarshal(raw, &received); err != nil {
		t.Fatal(err)
	}
	if received.ACLType.Source != "RECEIVED" {
		t.Fatalf("expected a received fixture, got %s", received.ACLType.Source)
	}
	config := acctest.ProviderConfig() + fmt.Sprintf(`
import {
 to = truenas_dataset.test
 id = %q
}
resource "truenas_dataset" "test" {
 name = %q
 acltype = "posix"
 compression = "lz4"
 atime = "OFF"
 recordsize = "128K"
 special_small_block_size = 0
}
`, target, target)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { acctest.PreCheck(t) },
		ProtoV6ProviderFactories: acctest.ProtoV6ProviderFactories,
		Steps: []resource.TestStep{{
			Config: config,
			ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
				plancheck.ExpectResourceAction("truenas_dataset.test", plancheck.ResourceActionNoop),
			}},
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("truenas_dataset.test", "acltype", "posix"),
				resource.TestCheckResourceAttr("truenas_dataset.test", "compression", "lz4"),
				resource.TestCheckResourceAttr("truenas_dataset.test", "atime", "OFF"),
				resource.TestCheckResourceAttr("truenas_dataset.test", "recordsize", "128K"),
				resource.TestCheckResourceAttr("truenas_dataset.test", "special_small_block_size", "0"),
			),
		}},
	})
}
