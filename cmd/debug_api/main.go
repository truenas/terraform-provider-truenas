package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"

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

func lastDot(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return i
		}
	}
	return -1
}

func sortStrings(s []string) {
	sort.Strings(s)
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
	if section == "replication" || section == "all" {
		pp("replication.query (first item)", firstItem(call(c, "replication.query", []any{})))
	}
	if section == "cloudsync" || section == "all" {
		pp("cloudsync.query (first item)", firstItem(call(c, "cloudsync.query", []any{})))
		pp("cloudsync.credentials.query (first item)", firstItem(call(c, "cloudsync.credentials.query", []any{})))
	}
	if section == "interface" || section == "all" {
		pp("interface.query (first item)", firstItem(call(c, "interface.query", []any{})))
	}
	if section == "staticroute" || section == "all" {
		pp("staticroute.query (first item)", firstItem(call(c, "staticroute.query", []any{})))
	}
	if section == "alerts" || section == "all" {
		pp("alertservice.query (first item)", firstItem(call(c, "alertservice.query", []any{})))
		pp("alertclasses.config", call(c, "alertclasses.config"))
	}
	if section == "system" || section == "all" {
		pp("boot.environment.query (first item)", firstItem(call(c, "boot.environment.query", []any{})))
		pp("bootenv.query (first item)", firstItem(call(c, "bootenv.query", []any{})))
		pp("tunable.query (first item)", firstItem(call(c, "tunable.query", []any{})))
		pp("system.ntpserver.query (all)", call(c, "system.ntpserver.query", []any{}))
		pp("mail.config", call(c, "mail.config"))
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
	if section == "iscsi2" || section == "all" {
		pp("iscsi.portal.query (first item)", firstItem(call(c, "iscsi.portal.query", []any{})))
		pp("iscsi.targetextent.query (first item)", firstItem(call(c, "iscsi.targetextent.query", []any{})))
		pp("iscsi.auth.query (first item)", firstItem(call(c, "iscsi.auth.query", []any{})))
		pp("iscsi.global.config", call(c, "iscsi.global.config"))
	}
	if section == "svcconfig" || section == "all" {
		pp("ssh.config", call(c, "ssh.config"))
		pp("ftp.config", call(c, "ftp.config"))
		pp("snmp.config", call(c, "snmp.config"))
		pp("ups.config", call(c, "ups.config"))
		pp("smb.config", call(c, "smb.config"))
		pp("nfs.config", call(c, "nfs.config"))
	}
	if section == "nvmet" || section == "all" {
		pp("nvmet.global.config", call(c, "nvmet.global.config"))
		pp("nvmet.subsys.query (first item)", firstItem(call(c, "nvmet.subsys.query", []any{})))
		pp("nvmet.port.query (first item)", firstItem(call(c, "nvmet.port.query", []any{})))
		pp("nvmet.namespace.query (first item)", firstItem(call(c, "nvmet.namespace.query", []any{})))
		pp("nvmet.host.query (first item)", firstItem(call(c, "nvmet.host.query", []any{})))
		pp("nvmet.host_subsys.query (first item)", firstItem(call(c, "nvmet.host_subsys.query", []any{})))
		pp("nvmet.port_subsys.query (first item)", firstItem(call(c, "nvmet.port_subsys.query", []any{})))
	}
	if section == "syssingletons" || section == "all" {
		pp("system.general.config", call(c, "system.general.config"))
		pp("system.advanced.config", call(c, "system.advanced.config"))
		pp("network.configuration.config", call(c, "network.configuration.config"))
		pp("systemdataset.config", call(c, "systemdataset.config"))
		pp("replication.config.config", call(c, "replication.config.config"))
	}
	if section == "smbprobe" {
		// Create a throwaway dataset + share, dump create + get_instance
		// responses, delete both.
		if _, err := c.Call(context.Background(), "pool.dataset.create", map[string]any{
			"name": "tank/tf-probe-smb-ds",
		}); err != nil {
			log.Fatal("dataset create:", err)
		}
		defer c.Call(context.Background(), "pool.dataset.delete", "tank/tf-probe-smb-ds")
		payload := map[string]any{
			"path": "/mnt/tank/tf-probe-smb-ds", "name": "tf-probe-smb", "purpose": "LEGACY_SHARE",
			"options": map[string]any{
				"purpose": "LEGACY_SHARE", "hostsallow": []string{"192.168.1.0/24"},
			},
		}
		raw, err := c.Call(context.Background(), "sharing.smb.create", payload)
		if err != nil {
			log.Fatal("create:", err)
		}
		fmt.Printf("=== create response ===\n%s\n", raw)
		var created struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(raw, &created)
		raw2, err := c.Call(context.Background(), "sharing.smb.get_instance", created.ID)
		if err != nil {
			log.Fatal("get_instance:", err)
		}
		fmt.Printf("=== get_instance response ===\n%s\n", raw2)
		if _, err := c.Call(context.Background(), "sharing.smb.delete", created.ID); err != nil {
			log.Fatal("delete:", err)
		}
		fmt.Println("=== deleted ok ===")
	}
	if section == "csprobe" {
		// Create a throwaway cloudsync credential, dump create +
		// get_instance responses, delete it.
		payload := map[string]any{
			"name":     "tf-probe-cscreds",
			"provider": "STORJ_IX",
			"attributes": map[string]any{
				"access_key_id":     "x",
				"secret_access_key": "y",
			},
		}
		raw, err := c.Call(context.Background(), "cloudsync.credentials.create", payload)
		if err != nil {
			log.Fatal("create:", err)
		}
		fmt.Printf("=== create response ===\n%s\n", raw)
		var created struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(raw, &created)
		raw2, err := c.Call(context.Background(), "cloudsync.credentials.get_instance", created.ID)
		if err != nil {
			log.Fatal("get_instance:", err)
		}
		fmt.Printf("=== get_instance response ===\n%s\n", raw2)
		if _, err := c.Call(context.Background(), "cloudsync.credentials.delete", created.ID); err != nil {
			log.Fatal("delete:", err)
		}
		fmt.Println("=== deleted ok ===")
	}
	if section == "namespaces" {
		raw, err := c.Call(context.Background(), "core.get_methods")
		if err != nil {
			log.Fatal(err)
		}
		var methods map[string]any
		json.Unmarshal(raw, &methods)
		seen := map[string]bool{}
		for name := range methods {
			// namespace = everything before the last dot
			if i := lastDot(name); i > 0 {
				seen[name[:i]] = true
			}
		}
		var names []string
		for ns := range seen {
			names = append(names, ns)
		}
		sortStrings(names)
		for _, ns := range names {
			fmt.Println(ns)
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
