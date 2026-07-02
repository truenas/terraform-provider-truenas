package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/truenas/terraform-provider-truenas/internal/client"
)

func pp(label string, v any) {
	out, _ := json.MarshalIndent(v, "", "  ")
	fmt.Printf("\n=== %s ===\n%s\n", label, out)
}

func call(c *client.Client, method string, params ...any) any {
	raw, err := c.Call(context.Background(), method, params...)
	if err != nil {
		fmt.Printf("[%s] error: %v\n", method, err)
		return nil
	}
	var v any
	json.Unmarshal(raw, &v)
	return v
}

func firstItem(v any) any {
	if arr, ok := v.([]any); ok && len(arr) > 0 {
		return arr[0]
	}
	return v
}

func main() {
	endpoint := "wss://192.168.1.68/websocket"
	apiKey := os.Getenv("TRUENAS_API_KEY")
	if apiKey == "" {
		log.Fatal("set TRUENAS_API_KEY")
	}

	tlsCfg, err := client.BuildTLSConfig(true, "")
	if err != nil {
		log.Fatal(err)
	}

	c := client.New(endpoint, tlsCfg)
	authFn := func(ctx context.Context) error { return client.AuthAPIKey(ctx, c, apiKey) }
	if err := c.Connect(context.Background(), authFn); err != nil {
		log.Fatal("connect:", err)
	}

	section := "all"
	if len(os.Args) > 1 {
		section = os.Args[1]
	}

	if section == "pool" || section == "all" {
		pp("pool.query (first item)", firstItem(call(c, "pool.query", []any{})))
	}
	if section == "snapshot" || section == "all" {
		pp("pool.snapshot.query (first item)", firstItem(call(c, "pool.snapshot.query", []any{}, map[string]any{"limit": 1})))
	}
	if section == "snapshottask" || section == "all" {
		pp("pool.snapshottask.query (first item)", firstItem(call(c, "pool.snapshottask.query", []any{})))
	}
	if section == "zvol" || section == "all" {
		pp("pool.dataset.query type=VOLUME (first item)", firstItem(call(c, "pool.dataset.query", [][]any{{"type", "=", "VOLUME"}})))
	}
	if section == "dataset" {
		name := "tank/mydata"
		if len(os.Args) > 2 {
			name = os.Args[2]
		}
		pp("pool.dataset.get_instance", call(c, "pool.dataset.get_instance", name))
	}
	if section == "smb" || section == "all" {
		pp("sharing.smb.query (first item)", firstItem(call(c, "sharing.smb.query", []any{})))
	}
	if section == "iscsi_target" || section == "all" {
		pp("iscsi.target.query (first item)", firstItem(call(c, "iscsi.target.query", []any{})))
	}
	if section == "iscsi_extent" || section == "all" {
		pp("iscsi.extent.query (first item)", firstItem(call(c, "iscsi.extent.query", []any{})))
	}
	if section == "iscsi_initiator" || section == "all" {
		pp("iscsi.initiator.query (first item)", firstItem(call(c, "iscsi.initiator.query", []any{})))
	}
	if section == "service" || section == "all" {
		pp("service.query (all)", call(c, "service.query", []any{}))
	}
	if section == "user" || section == "all" {
		pp("user.query (first item)", firstItem(call(c, "user.query", []any{}, map[string]any{"limit": 1})))
	}
	if section == "group" || section == "all" {
		pp("group.query (first item)", firstItem(call(c, "group.query", []any{}, map[string]any{"limit": 1})))
	}
	if section == "app" || section == "all" {
		pp("app.query (first item)", firstItem(call(c, "app.query", []any{})))
		pp("catalog.config", call(c, "catalog.config"))
	}
	if section == "vm" || section == "all" {
		pp("vm.query (first item)", firstItem(call(c, "vm.query", []any{})))
		pp("vm.device.query (first item)", firstItem(call(c, "vm.device.query", []any{})))
	}
	if section == "methods" {
		prefix := "vm."
		if len(os.Args) > 2 {
			prefix = os.Args[2]
		}
		raw, err := c.Call(context.Background(), "core.get_methods")
		if err != nil {
			log.Fatal(err)
		}
		var methods map[string]any
		json.Unmarshal(raw, &methods)
		for name, info := range methods {
			if len(name) >= len(prefix) && name[:len(prefix)] == prefix {
				pp(name, info)
			}
		}
	}
	if section == "appmethods" {
		raw, err := c.Call(context.Background(), "core.get_methods")
		if err != nil {
			log.Fatal(err)
		}
		var methods map[string]any
		json.Unmarshal(raw, &methods)
		var names []string
		for name := range methods {
			if len(name) >= 4 && name[:4] == "app." {
				names = append(names, name)
			}
			if len(name) >= 3 && name[:3] == "vm." {
				names = append(names, name)
			}
		}
		pp("app.* and vm.* method names", names)
	}
}
