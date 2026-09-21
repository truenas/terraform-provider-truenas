// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sort"
	"time"

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
	endpoint := os.Getenv("TRUENAS_ENDPOINT")
	if endpoint == "" {
		log.Fatal("set TRUENAS_ENDPOINT (e.g. wss://truenas.example.com/api/current)")
	}
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
	if section == "websharedatasetdelay" {
		pool := os.Getenv("TRUENAS_TEST_POOL")
		if pool == "" {
			pool = "tank"
		}
		dsName := pool + "/tf-probe-webshare-ds2"
		if _, err := c.Call(context.Background(), "pool.dataset.create", map[string]any{"name": dsName}); err != nil {
			log.Fatal("dataset create:", err)
		}
		defer c.Call(context.Background(), "pool.dataset.delete", dsName)

		raw, err := c.Call(context.Background(), "sharing.webshare.create", map[string]any{
			"path": "/mnt/" + dsName, "name": "tf-probe-webshare2", "enabled": true,
		})
		if err != nil {
			log.Fatal("create:", err)
		}
		var created struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(raw, &created)
		fmt.Printf("=== immediately after create ===\n%s\n", raw)

		for i, wait := range []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second} {
			time.Sleep(wait)
			raw2, err := c.Call(context.Background(), "sharing.webshare.get_instance", created.ID)
			if err != nil {
				log.Fatal("get_instance:", err)
			}
			fmt.Printf("=== get_instance poll %d (+%v) ===\n%s\n", i, wait, raw2)
		}

		if _, err := c.Call(context.Background(), "sharing.webshare.delete", created.ID); err != nil {
			log.Fatal("delete:", err)
		}
		fmt.Println("=== deleted ok ===")
	}
	if section == "webshareleftovercheck" {
		pp("webshare.config", call(c, "webshare.config"))
		pp("pool.dataset.query name~tf-acc", call(c, "pool.dataset.query", [][]any{{"id", "~", "tf-acc"}}))
		pp("sharing.webshare.query name~tf-acc", call(c, "sharing.webshare.query", [][]any{{"name", "~", "tf-acc"}}))
	}
	if section == "webshareconfigtoggle" {
		orig := call(c, "webshare.config")
		fmt.Printf("=== webshare.config (before) ===\n%v\n", orig)
		raw, err := c.Call(context.Background(), "webshare.update", map[string]any{"search": true})
		if err != nil {
			fmt.Printf("=== webshare.update({search:true}) error ===\n%v\n", err)
		} else {
			fmt.Printf("=== webshare.update({search:true}) response ===\n%s\n", raw)
		}
		raw2, err := c.Call(context.Background(), "webshare.update", map[string]any{"search": false})
		if err != nil {
			fmt.Printf("=== webshare.update({search:false}) error ===\n%v\n", err)
		} else {
			fmt.Printf("=== webshare.update({search:false}) response ===\n%s\n", raw2)
		}
	}
	if section == "dsdel" {
		name := os.Args[2]
		raw, err := c.Call(context.Background(), "pool.dataset.delete", name)
		if err != nil {
			log.Fatal("delete:", err)
		}
		fmt.Printf("=== deleted %s ===\n%s\n", name, raw)
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
	if section == "lxc" || section == "all" {
		pp("lxc.config", call(c, "lxc.config"))
		pp("lxc.bridge_choices", call(c, "lxc.bridge_choices"))
	}
	if section == "containerprobe" {
		runContainerProbe(c)
	}
	if section == "containerdeviceprobe" {
		runContainerDeviceProbe(c)
	}
	if section == "ctrdevleftovercheck" {
		pp("containers matching tf-acc-ctrdev", call(c, "container.query", [][]any{{"name", "^", "tf-acc-ctrdev"}}))
		pp("datasets matching tf-acc-ds-ctrdev", call(c, "pool.dataset.query", [][]any{{"id", "~", "tf-acc-ds-ctrdev"}}))
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
	if section == "keytabprobe" {
		// Create a throwaway kerberos keytab using a real keytab payload
		// (base64, via TRUENAS_PROBE_KEYTAB_B64), dump create + query +
		// get_instance responses to see whether "file" comes back intact,
		// redacted, or absent, then delete it.
		fileB64 := os.Getenv("TRUENAS_PROBE_KEYTAB_B64")
		if fileB64 == "" {
			log.Fatal("set TRUENAS_PROBE_KEYTAB_B64 (base64 of a real keytab)")
		}
		payload := map[string]any{
			"name": "tf-probe-keytab",
			"file": fileB64,
		}
		raw, err := c.Call(context.Background(), "kerberos.keytab.create", payload)
		if err != nil {
			log.Fatal("create:", err)
		}
		fmt.Printf("=== create response ===\n%s\n", raw)
		var created struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(raw, &created)

		raw2, err := c.Call(context.Background(), "kerberos.keytab.get_instance", created.ID)
		if err != nil {
			log.Fatal("get_instance:", err)
		}
		fmt.Printf("=== get_instance response ===\n%s\n", raw2)

		raw3, err := c.Call(context.Background(), "kerberos.keytab.query", [][]any{{"id", "=", created.ID}})
		if err != nil {
			log.Fatal("query:", err)
		}
		fmt.Printf("=== query response ===\n%s\n", raw3)

		raw4, err := c.Call(context.Background(), "kerberos.keytab.update", created.ID, map[string]any{"name": "tf-probe-keytab-renamed"})
		if err != nil {
			log.Fatal("update:", err)
		}
		fmt.Printf("=== update response (name only) ===\n%s\n", raw4)

		if _, err := c.Call(context.Background(), "kerberos.keytab.delete", created.ID); err != nil {
			log.Fatal("delete:", err)
		}
		fmt.Println("=== deleted ok ===")
	}
	if section == "authme" {
		pp("auth.me", call(c, "auth.me"))
	}
	if section == "cbprobe" {
		// Probe cloud_backup.create/query/get_instance/update/delete against a
		// throwaway S3-type cloudsync credential + a bucket name that does not
		// exist, to observe (a) whether create validates the credential/bucket
		// against the actual S3 endpoint, and (b) whether "password" is
		// returned verbatim or masked on read-back. Cleans up both the
		// cloud_backup task (if created) and the credential.
		credPayload := map[string]any{
			"name": "tf-probe-cbcreds",
			"provider": map[string]any{
				"type":              "S3",
				"access_key_id":     "AKIAFAKETESTPROBE0001",
				"secret_access_key": "fakeSecretAccessKeyForProbeTestOnly1234",
			},
		}
		rawCred, err := c.Call(context.Background(), "cloudsync.credentials.create", credPayload)
		if err != nil {
			log.Fatal("cloudsync.credentials.create:", err)
		}
		fmt.Printf("=== cloudsync.credentials.create response ===\n%s\n", rawCred)
		var cred struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(rawCred, &cred)
		defer func() {
			if _, err := c.Call(context.Background(), "cloudsync.credentials.delete", cred.ID); err != nil {
				fmt.Printf("cleanup cloudsync.credentials.delete error: %v\n", err)
			} else {
				fmt.Println("=== cloudsync credentials deleted ok ===")
			}
		}()

		cbPayload := map[string]any{
			"description": "tf-probe-cloud-backup",
			"path":        "/mnt/tank",
			"credentials": cred.ID,
			"attributes": map[string]any{
				"bucket": "tf-probe-nonexistent-bucket-xyz123",
				"folder": "tf-probe-folder",
			},
			"password":  "tf-probe-password-1234",
			"keep_last": 1,
			"enabled":   false,
		}
		rawCB, err := c.Call(context.Background(), "cloud_backup.create", cbPayload)
		if err != nil {
			fmt.Printf("=== cloud_backup.create ERROR (decisive: create validates) ===\n%v\n", err)
			return
		}
		fmt.Printf("=== cloud_backup.create response (decisive: create does NOT validate) ===\n%s\n", rawCB)

		var cb struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(rawCB, &cb)
		defer func() {
			if _, err := c.Call(context.Background(), "cloud_backup.delete", cb.ID); err != nil {
				fmt.Printf("cleanup cloud_backup.delete error: %v\n", err)
			} else {
				fmt.Println("=== cloud_backup deleted ok ===")
			}
		}()

		rawGet, err := c.Call(context.Background(), "cloud_backup.get_instance", cb.ID)
		if err != nil {
			log.Fatal("cloud_backup.get_instance:", err)
		}
		fmt.Printf("=== cloud_backup.get_instance response (password read-back?) ===\n%s\n", rawGet)

		rawQuery, err := c.Call(context.Background(), "cloud_backup.query", [][]any{{"id", "=", cb.ID}})
		if err != nil {
			log.Fatal("cloud_backup.query:", err)
		}
		fmt.Printf("=== cloud_backup.query response ===\n%s\n", rawQuery)

		rawUpd, err := c.Call(context.Background(), "cloud_backup.update", cb.ID, map[string]any{
			"description": "tf-probe-cloud-backup-renamed",
		})
		if err != nil {
			log.Fatal("cloud_backup.update:", err)
		}
		fmt.Printf("=== cloud_backup.update response ===\n%s\n", rawUpd)
	}
	if section == "dsprobe" {
		pp("directoryservices.config", call(c, "directoryservices.config"))
		pp("directoryservices.status", call(c, "directoryservices.status"))
	}
	if section == "dsupdate" {
		// Reads a directoryservices.update payload as JSON from
		// TRUENAS_PROBE_PAYLOAD and calls it via CallJob, printing the job
		// result or error. Used to iterate quickly on the exact payload
		// shape TrueNAS accepts for in-place updates while directory
		// services are already enabled, without paying the cost of a full
		// join/health-wait cycle per attempt.
		raw := os.Getenv("TRUENAS_PROBE_PAYLOAD")
		if raw == "" {
			log.Fatal("set TRUENAS_PROBE_PAYLOAD (JSON object) for dsupdate")
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(raw), &payload); err != nil {
			log.Fatal("parsing TRUENAS_PROBE_PAYLOAD: ", err)
		}
		result, err := c.CallJob(context.Background(), "directoryservices.update", payload)
		if err != nil {
			fmt.Printf("=== directoryservices.update error ===\n%v\n", err)
		} else {
			var v any
			json.Unmarshal(result, &v)
			pp("directoryservices.update result", v)
		}
	}
	if section == "fspermprobe" {
		// Probe filesystem.setperm (job) + filesystem.stat (sync) round trip
		// on a throwaway dataset directory: create dataset, setperm with
		// mode/uid/gid, stat before+after, then stat a nonexistent path to
		// see the not-found error shape.
		ds := "tank/tf-probe-fsperm"
		if _, err := c.Call(context.Background(), "pool.dataset.create", map[string]any{"name": ds}); err != nil {
			log.Fatal("dataset create:", err)
		}
		defer c.Call(context.Background(), "pool.dataset.delete", ds, map[string]any{"recursive": true})
		path := "/mnt/" + ds

		rawStat0, err := c.Call(context.Background(), "filesystem.stat", path)
		if err != nil {
			log.Fatal("stat before:", err)
		}
		fmt.Printf("=== filesystem.stat (before setperm) ===\n%s\n", rawStat0)

		result, err := c.CallJob(context.Background(), "filesystem.setperm", map[string]any{
			"path": path,
			"mode": "0750",
			"uid":  1000,
			"gid":  1000,
		})
		if err != nil {
			log.Fatal("setperm job:", err)
		}
		fmt.Printf("=== filesystem.setperm job result ===\n%s\n", result)

		rawStat1, err := c.Call(context.Background(), "filesystem.stat", path)
		if err != nil {
			log.Fatal("stat after:", err)
		}
		fmt.Printf("=== filesystem.stat (after setperm 0750, uid=gid=1000) ===\n%s\n", rawStat1)

		_, statErr := c.Call(context.Background(), "filesystem.stat", "/mnt/tank/tf-probe-fsperm-does-not-exist-xyz")
		fmt.Printf("=== filesystem.stat (nonexistent path) error ===\n%v\n", statErr)
	}
	if section == "acltprobe" {
		// List all existing (builtin) ACL templates, then create/query/
		// update/delete a throwaway NFS4 one to observe exact wire shapes.
		rawQ, err := c.Call(context.Background(), "filesystem.acltemplate.query", []any{})
		if err != nil {
			log.Fatal("acltemplate.query (all):", err)
		}
		fmt.Printf("=== filesystem.acltemplate.query (all, presumably builtins only) ===\n%s\n", rawQ)

		payload := map[string]any{
			"name":    "tf-probe-acltemplate",
			"acltype": "NFS4",
			"comment": "tf probe",
			"acl": []map[string]any{
				{
					"tag":   "owner@",
					"type":  "ALLOW",
					"perms": map[string]any{"BASIC": "FULL_CONTROL"},
					"flags": map[string]any{"BASIC": "INHERIT"},
				},
				{
					"tag":   "group@",
					"type":  "ALLOW",
					"perms": map[string]any{"BASIC": "MODIFY"},
					"flags": map[string]any{"BASIC": "INHERIT"},
				},
				{
					"tag":   "everyone@",
					"type":  "ALLOW",
					"perms": map[string]any{"BASIC": "READ"},
					"flags": map[string]any{"BASIC": "INHERIT"},
				},
			},
		}
		rawC, err := c.Call(context.Background(), "filesystem.acltemplate.create", payload)
		if err != nil {
			log.Fatal("acltemplate.create:", err)
		}
		fmt.Printf("=== filesystem.acltemplate.create response ===\n%s\n", rawC)
		var created struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(rawC, &created)
		defer func() {
			if _, err := c.Call(context.Background(), "filesystem.acltemplate.delete", created.ID); err != nil {
				fmt.Printf("cleanup delete error: %v\n", err)
			} else {
				fmt.Println("=== acltemplate deleted ok ===")
			}
		}()

		rawG, err := c.Call(context.Background(), "filesystem.acltemplate.get_instance", created.ID)
		if err != nil {
			log.Fatal("acltemplate.get_instance:", err)
		}
		fmt.Printf("=== filesystem.acltemplate.get_instance response ===\n%s\n", rawG)

		rawU, err := c.Call(context.Background(), "filesystem.acltemplate.update", created.ID, map[string]any{
			"comment": "tf probe updated",
		})
		if err != nil {
			log.Fatal("acltemplate.update:", err)
		}
		fmt.Printf("=== filesystem.acltemplate.update response (comment only) ===\n%s\n", rawU)
	}
	if section == "faclmethods" {
		raw, err := c.Call(context.Background(), "core.get_methods")
		if err != nil {
			log.Fatal(err)
		}
		var methods map[string]any
		json.Unmarshal(raw, &methods)
		for _, name := range []string{"filesystem.getacl", "filesystem.setacl"} {
			if info, ok := methods[name]; ok {
				pp(name, info)
			} else {
				fmt.Printf("=== %s: NOT FOUND ===\n", name)
			}
		}
	}
	if section == "faclprobe" {
		// Probe filesystem.getacl (sync) + filesystem.setacl (job) round
		// trip on a throwaway dataset directory: create dataset, getacl
		// on the trivial (freshly-created) state, setacl with an NFS4
		// entry set (owner@/group@/user:1000), getacl again, then setacl
		// again with options.stripacl=true to see whether it cleanly
		// reverts to a trivial/mode-based ACL (decisive for this
		// resource's Delete semantic).
		ds := "tank/tf-probe-facl"
		if _, err := c.Call(context.Background(), "pool.dataset.create", map[string]any{"name": ds, "acltype": "NFSV4", "aclmode": "PASSTHROUGH"}); err != nil {
			log.Fatal("dataset create:", err)
		}
		defer c.Call(context.Background(), "pool.dataset.delete", ds, map[string]any{"recursive": true})
		path := "/mnt/" + ds

		rawGetAcl0, err := c.Call(context.Background(), "filesystem.getacl", path)
		if err != nil {
			log.Fatal("getacl (trivial):", err)
		}
		fmt.Printf("=== filesystem.getacl (trivial, freshly-created dataset dir) ===\n%s\n", rawGetAcl0)

		nfs4Payload := map[string]any{
			"path": path,
			"dacl": []map[string]any{
				{
					"tag":   "owner@",
					"id":    nil,
					"type":  "ALLOW",
					"perms": map[string]any{"BASIC": "FULL_CONTROL"},
					"flags": map[string]any{"BASIC": "INHERIT"},
				},
				{
					"tag":   "group@",
					"id":    nil,
					"type":  "ALLOW",
					"perms": map[string]any{"BASIC": "MODIFY"},
					"flags": map[string]any{"BASIC": "INHERIT"},
				},
				{
					"tag":   "USER",
					"id":    1000,
					"type":  "ALLOW",
					"perms": map[string]any{"BASIC": "READ"},
					"flags": map[string]any{"BASIC": "INHERIT"},
				},
			},
			"options": map[string]any{
				"recursive": false,
				"traverse":  false,
				"stripacl":  false,
			},
		}
		resultSet, err := c.CallJob(context.Background(), "filesystem.setacl", nfs4Payload)
		if err != nil {
			fmt.Printf("=== filesystem.setacl (nfs4 dacl) ERROR ===\n%v\n", err)
		} else {
			fmt.Printf("=== filesystem.setacl (nfs4 dacl) job result ===\n%s\n", resultSet)
		}

		rawGetAcl1, err := c.Call(context.Background(), "filesystem.getacl", path)
		if err != nil {
			log.Fatal("getacl (after setacl nfs4):", err)
		}
		fmt.Printf("=== filesystem.getacl (after setacl nfs4 dacl) ===\n%s\n", rawGetAcl1)

		// DECISIVE: stripacl=true.
		stripPayload := map[string]any{
			"path": path,
			"dacl": []any{},
			"options": map[string]any{
				"stripacl": true,
			},
		}
		resultStrip, err := c.CallJob(context.Background(), "filesystem.setacl", stripPayload)
		if err != nil {
			fmt.Printf("=== filesystem.setacl (stripacl=true) ERROR ===\n%v\n", err)
		} else {
			fmt.Printf("=== filesystem.setacl (stripacl=true) job result ===\n%s\n", resultStrip)
		}

		rawGetAcl2, err := c.Call(context.Background(), "filesystem.getacl", path)
		if err != nil {
			log.Fatal("getacl (after stripacl):", err)
		}
		fmt.Printf("=== filesystem.getacl (after stripacl=true) ===\n%s\n", rawGetAcl2)

		rawStat, err := c.Call(context.Background(), "filesystem.stat", path)
		if err != nil {
			log.Fatal("stat (after stripacl):", err)
		}
		fmt.Printf("=== filesystem.stat (after stripacl=true) ===\n%s\n", rawStat)

		_, getAclErr := c.Call(context.Background(), "filesystem.getacl", "/mnt/tank/tf-probe-facl-does-not-exist-xyz")
		fmt.Printf("=== filesystem.getacl (nonexistent path) error ===\n%v\n", getAclErr)
	}
	if section == "faclprobeposix" {
		// POSIX1E round trip: this pool's root (tank) has acltype=POSIX
		// LOCAL (probed live), so a dataset created without an explicit
		// acltype override inherits POSIX1E - use that default here (no
		// override) to probe POSIX1E entry shapes for real via setacl.
		ds := "tank/tf-probe-faclposix"
		if _, err := c.Call(context.Background(), "pool.dataset.create", map[string]any{"name": ds}); err != nil {
			log.Fatal("dataset create:", err)
		}
		defer c.Call(context.Background(), "pool.dataset.delete", ds, map[string]any{"recursive": true})
		path := "/mnt/" + ds

		rawGetAcl0, err := c.Call(context.Background(), "filesystem.getacl", path)
		if err != nil {
			log.Fatal("getacl (trivial posix):", err)
		}
		fmt.Printf("=== filesystem.getacl (trivial POSIX1E) ===\n%s\n", rawGetAcl0)

		posixPayload := map[string]any{
			"path": path,
			"dacl": []map[string]any{
				{"tag": "USER_OBJ", "perms": map[string]any{"READ": true, "WRITE": true, "EXECUTE": true}, "default": false},
				{"tag": "GROUP_OBJ", "perms": map[string]any{"READ": true, "WRITE": false, "EXECUTE": true}, "default": false},
				{"tag": "OTHER", "perms": map[string]any{"READ": false, "WRITE": false, "EXECUTE": false}, "default": false},
				{"tag": "USER", "id": 1000, "perms": map[string]any{"READ": true, "WRITE": false, "EXECUTE": false}, "default": false},
				{"tag": "MASK", "perms": map[string]any{"READ": true, "WRITE": true, "EXECUTE": true}, "default": false},
			},
			"options": map[string]any{"recursive": false, "traverse": false, "stripacl": false},
		}
		resultSet, err := c.CallJob(context.Background(), "filesystem.setacl", posixPayload)
		if err != nil {
			fmt.Printf("=== filesystem.setacl (posix1e dacl) ERROR ===\n%v\n", err)
		} else {
			fmt.Printf("=== filesystem.setacl (posix1e dacl) job result ===\n%s\n", resultSet)
		}

		rawGetAcl1, err := c.Call(context.Background(), "filesystem.getacl", path)
		if err != nil {
			log.Fatal("getacl (after setacl posix1e):", err)
		}
		fmt.Printf("=== filesystem.getacl (after setacl posix1e dacl) ===\n%s\n", rawGetAcl1)

		stripPayload := map[string]any{
			"path":    path,
			"dacl":    []any{},
			"options": map[string]any{"stripacl": true},
		}
		resultStrip, err := c.CallJob(context.Background(), "filesystem.setacl", stripPayload)
		if err != nil {
			fmt.Printf("=== filesystem.setacl (posix1e stripacl=true) ERROR ===\n%v\n", err)
		} else {
			fmt.Printf("=== filesystem.setacl (posix1e stripacl=true) job result ===\n%s\n", resultStrip)
		}
		rawGetAcl2, err := c.Call(context.Background(), "filesystem.getacl", path)
		if err != nil {
			log.Fatal("getacl (after posix stripacl):", err)
		}
		fmt.Printf("=== filesystem.getacl (after posix1e stripacl=true) ===\n%s\n", rawGetAcl2)
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
	if section == "tnconnectmethods" {
		// Method introspection only: confirm the tn_connect namespace exists
		// at all and dump accepts/returns schemas for every tn_connect.*
		// method (Plan 19 Task 3 probe). No config-mutating calls here.
		raw, err := c.Call(context.Background(), "core.get_methods")
		if err != nil {
			log.Fatal(err)
		}
		var methods map[string]any
		json.Unmarshal(raw, &methods)
		var names []string
		for name := range methods {
			if len(name) >= 11 && name[:11] == "tn_connect." {
				names = append(names, name)
			}
		}
		sortStrings(names)
		pp("tn_connect.* method names", names)
		for _, name := range names {
			pp(name, methods[name])
		}
		if len(names) == 0 {
			fmt.Println("=== tn_connect namespace absent on this release ===")
			return
		}
		pp("tn_connect.config (read-only, current state)", call(c, "tn_connect.config"))
	}
	if section == "tnconnectdecisive" {
		// DECISIVE probe (26.0 only, per task-3 brief): with the box's
		// current tn_connect.config reporting enabled=false, can a cosmetic
		// field (e.g. "ips") be updated via tn_connect.update with ZERO side
		// effects, sending a partial payload that never includes "enabled"?
		// SAFETY: this probe NEVER sends {"enabled": true} under any
		// circumstance — it aborts entirely if the box's current config
		// already shows enabled=true (unsafe to experiment further; cloud
		// enrollment may already be active or pending) rather than risk
		// toggling it.
		before := call(c, "tn_connect.config")
		pp("tn_connect.config (before)", before)
		beforeMap, ok := before.(map[string]any)
		if !ok {
			log.Fatal("tn_connect.config did not return a JSON object")
		}
		if enabled, _ := beforeMap["enabled"].(bool); enabled {
			log.Fatal("ABORT: tn_connect.config reports enabled=true already; refusing to probe further")
		}

		origIPs, _ := beforeMap["ips"].([]any)
		fmt.Printf("=== original ips ===\n%v\n", origIPs)

		// Partial update: ONLY "ips", deliberately omitting "enabled"
		// entirely so a genuinely partial-update API leaves enabled
		// untouched (never sent, never flipped).
		probeIPs := []string{"127.0.0.1"}
		rawUpd, err := c.Call(context.Background(), "tn_connect.update", map[string]any{"ips": probeIPs})
		if err != nil {
			fmt.Printf("=== tn_connect.update({ips: [127.0.0.1]}) ERROR ===\n%v\n", err)
		} else {
			fmt.Printf("=== tn_connect.update({ips: [127.0.0.1]}) response ===\n%s\n", rawUpd)
		}

		after := call(c, "tn_connect.config")
		pp("tn_connect.config (after ips update)", after)
		if afterMap, ok := after.(map[string]any); ok {
			if enabled, _ := afterMap["enabled"].(bool); enabled {
				fmt.Println("=== !!! SIDE EFFECT: enabled flipped to true after ips-only update !!! ===")
			} else {
				fmt.Println("=== confirmed: enabled still false after ips-only update ===")
			}
		}

		// Restore ips to its original value immediately, still never
		// touching "enabled".
		restorePayload := map[string]any{"ips": origIPs}
		rawRestore, err := c.Call(context.Background(), "tn_connect.update", restorePayload)
		if err != nil {
			fmt.Printf("=== tn_connect.update (restore ips) ERROR ===\n%v\n", err)
		} else {
			fmt.Printf("=== tn_connect.update (restore ips) response ===\n%s\n", rawRestore)
		}
		pp("tn_connect.config (after restore)", call(c, "tn_connect.config"))
	}
	if section == "webshareprobe" {
		// Method introspection first: confirm the namespace exists at all
		// and dump accepts/returns schemas for the singleton + share CRUD
		// methods (Plan 19 Task 1 probe).
		raw, err := c.Call(context.Background(), "core.get_methods")
		if err != nil {
			log.Fatal(err)
		}
		var methods map[string]any
		json.Unmarshal(raw, &methods)
		var names []string
		for name := range methods {
			if len(name) >= 9 && name[:9] == "webshare." {
				names = append(names, name)
				pp(name, methods[name])
			}
			if len(name) >= 17 && name[:17] == "sharing.webshare." {
				names = append(names, name)
				pp(name, methods[name])
			}
		}
		sortStrings(names)
		pp("webshare.* and sharing.webshare.* method names", names)

		if len(names) == 0 {
			fmt.Println("=== webshare namespace absent on this release ===")
			return
		}

		// Singleton config as it currently stands on the box.
		pp("webshare.config", call(c, "webshare.config"))
		pp("webshare.bindip_choices", call(c, "webshare.bindip_choices"))

		// Share CRUD: throwaway dataset + share, dump create/get_instance/
		// update/query responses, delete both.
		pool := os.Getenv("TRUENAS_TEST_POOL")
		if pool == "" {
			pool = "tank"
		}
		dsName := pool + "/tf-probe-webshare-ds"
		if _, err := c.Call(context.Background(), "pool.dataset.create", map[string]any{
			"name": dsName,
		}); err != nil {
			log.Fatal("dataset create:", err)
		}
		defer c.Call(context.Background(), "pool.dataset.delete", dsName)

		createPayload := map[string]any{
			"path":    "/mnt/" + dsName,
			"name":    "tf-probe-webshare",
			"enabled": true,
		}
		raw, err = c.Call(context.Background(), "sharing.webshare.create", createPayload)
		if err != nil {
			log.Fatal("sharing.webshare.create:", err)
		}
		fmt.Printf("=== sharing.webshare.create response ===\n%s\n", raw)

		var created struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(raw, &created)

		raw2, err := c.Call(context.Background(), "sharing.webshare.get_instance", created.ID)
		if err != nil {
			log.Fatal("sharing.webshare.get_instance:", err)
		}
		fmt.Printf("=== sharing.webshare.get_instance response ===\n%s\n", raw2)

		rawUpd, err := c.Call(context.Background(), "sharing.webshare.update", created.ID, map[string]any{
			"enabled": false,
		})
		if err != nil {
			fmt.Printf("=== sharing.webshare.update error ===\n%v\n", err)
		} else {
			fmt.Printf("=== sharing.webshare.update response ===\n%s\n", rawUpd)
		}

		rawQuery, err := c.Call(context.Background(), "sharing.webshare.query", []any{[]any{"id", "=", created.ID}})
		if err != nil {
			log.Fatal("sharing.webshare.query:", err)
		}
		fmt.Printf("=== sharing.webshare.query response ===\n%s\n", rawQuery)

		if _, err := c.Call(context.Background(), "sharing.webshare.delete", created.ID); err != nil {
			log.Fatal("sharing.webshare.delete:", err)
		}
		fmt.Println("=== sharing.webshare.delete ok ===")
	}
	if section == "enclosuremethods" {
		// Method introspection for the enclosure*/enclosure2* namespaces
		// (Plan 20 Task 3). Per Task 1's finding that core.get_methods can
		// under-report on 25.10.4, this ALSO direct-calls the candidate
		// methods regardless of what the listing showed.
		raw, err := c.Call(context.Background(), "core.get_methods")
		if err != nil {
			log.Fatal(err)
		}
		var methods map[string]any
		json.Unmarshal(raw, &methods)
		var names []string
		for name := range methods {
			if len(name) >= 10 && name[:10] == "enclosure." {
				names = append(names, name)
			}
			if len(name) >= 11 && name[:11] == "enclosure2." {
				names = append(names, name)
			}
		}
		sortStrings(names)
		pp("enclosure*/enclosure2* method names (core.get_methods)", names)
		for _, name := range names {
			pp(name, methods[name])
		}

		fmt.Println("\n=== direct-call verification (bypassing core.get_methods listing) ===")
		pp("enclosure2.query", call(c, "enclosure2.query"))
		pp("enclosure.query", call(c, "enclosure.query"))
	}
	if section == "enclosureprobe" {
		// Read-mostly shape probe of enclosure2.query (Plan 20 Task 3), plus
		// a live round-trip of enclosure.label.set on the known enclosure id
		// "3b0ad6d1c00006c0", restoring the original label immediately
		// afterward. This is the DECISIVE probe for: (a) enclosure2.query's
		// full response shape (where does the label live — top-level? per
		// element_type/slot? both?), (b) enclosure.label.set's positional
		// arg shape (id, label), (c) whether the new label round-trips
		// through enclosure2.query afterward, and how quickly (same-call
		// synchronous vs needing a re-query).
		raw, err := c.Call(context.Background(), "enclosure2.query")
		if err != nil {
			log.Fatal("enclosure2.query:", err)
		}
		fmt.Printf("=== enclosure2.query (before) ===\n%s\n", raw)

		var encs []map[string]any
		if err := json.Unmarshal(raw, &encs); err != nil {
			log.Fatalf("enclosure2.query did not return an array: err=%v raw=%s", err, raw)
		}
		if len(encs) == 0 {
			fmt.Println("=== enclosure2.query returned an EMPTY array on this box (no enclosure hardware / not supported) ===")
			return
		}

		targetID := os.Getenv("TRUENAS_PROBE_ENCLOSURE_ID")
		if targetID == "" {
			targetID = "3b0ad6d1c00006c0"
		}
		var target map[string]any
		for _, e := range encs {
			if id, _ := e["id"].(string); id == targetID {
				target = e
				break
			}
		}
		if target == nil {
			fmt.Printf("=== enclosure id %q not found; using first result instead ===\n", targetID)
			target = encs[0]
			targetID, _ = target["id"].(string)
		}

		fmt.Printf("=== target enclosure (id=%s) top-level keys ===\n", targetID)
		var keys []string
		for k := range target {
			keys = append(keys, k)
		}
		sortStrings(keys)
		for _, k := range keys {
			fmt.Printf("  %s: %v (%T)\n", k, target[k], target[k])
		}
		origLabel, _ := target["label"].(string)
		fmt.Printf("=== original label (top-level \"label\" key) = %q ===\n", origLabel)

		fmt.Println("\n=== enclosure.get_instance(id) probe ===")
		pp("enclosure.get_instance", call(c, "enclosure.get_instance", targetID))

		fmt.Println("\n=== enclosure.label.set probe: setting a throwaway label ===")
		probeLabel := "tf-probe-enclosure-label"
		setRaw, err := c.Call(context.Background(), "enclosure.label.set", targetID, probeLabel)
		if err != nil {
			fmt.Printf("=== enclosure.label.set error (2-positional-arg form) ===\n%v\n", err)
		} else {
			fmt.Printf("=== enclosure.label.set response ===\n%s\n", setRaw)
		}

		raw2, err := c.Call(context.Background(), "enclosure2.query")
		if err != nil {
			log.Fatal("enclosure2.query (after set):", err)
		}
		fmt.Printf("=== enclosure2.query (after set) ===\n%s\n", raw2)
		var encs2 []map[string]any
		json.Unmarshal(raw2, &encs2)
		for _, e := range encs2 {
			if id, _ := e["id"].(string); id == targetID {
				fmt.Printf("=== read-back label after set = %q ===\n", e["label"])
			}
		}

		fmt.Println("\n=== RESTORING original label ===")
		restoreRaw, err := c.Call(context.Background(), "enclosure.label.set", targetID, origLabel)
		if err != nil {
			log.Fatalf("RESTORE FAILED: enclosure.label.set(%q, %q): %v -- MANUAL INTERVENTION NEEDED", targetID, origLabel, err)
		}
		fmt.Printf("=== enclosure.label.set (restore) response ===\n%s\n", restoreRaw)

		raw3, err := c.Call(context.Background(), "enclosure2.query")
		if err != nil {
			log.Fatal("enclosure2.query (after restore):", err)
		}
		var encs3 []map[string]any
		json.Unmarshal(raw3, &encs3)
		for _, e := range encs3 {
			if id, _ := e["id"].(string); id == targetID {
				fmt.Printf("=== read-back label after restore = %q (want %q) ===\n", e["label"], origLabel)
			}
		}
	}
	if section == "enclosurefilter" {
		// Read-only probe (Plan 20 Task 3): does enclosure2.query accept
		// query-filters/query-options the usual two-positional-arg way
		// (unlike ipmi.lan.query's single-"data"-object quirk)? And does
		// core.get_methods under-report the enclosure2.* namespace on a
		// different (26.0) release the same way it did for failover.*/
		// ipmi.* on 25.10.4?
		id := os.Getenv("TRUENAS_PROBE_ENCLOSURE_ID")
		if id == "" {
			id = "3b0ad6d1c00006e0"
		}
		pp("enclosure2.query([[id,=,id]])", call(c, "enclosure2.query", [][]any{{"id", "=", id}}))
		pp("enclosure2.query([[id,=,id]], {select})", call(c, "enclosure2.query", [][]any{{"id", "=", id}}, map[string]any{"select": []string{"id", "label", "name"}}))
		pp("system.version_short", call(c, "system.version_short"))
	}
	if section == "haprobe" {
		// Read-only introspection of the failover.* namespace (Plan 20 Task
		// 1 probe). Never mutates anything: no failover.update / .become_*
		// / .reboot.* calls here — those are handled by the dedicated
		// haupdateprobe / hafailover sections below, gated separately.
		raw, err := c.Call(context.Background(), "core.get_methods")
		if err != nil {
			log.Fatal(err)
		}
		var methods map[string]any
		json.Unmarshal(raw, &methods)
		var names []string
		for name := range methods {
			if len(name) >= 9 && name[:9] == "failover." {
				names = append(names, name)
			}
		}
		sortStrings(names)
		for _, name := range names {
			pp(name, methods[name])
		}
		pp("failover.* method names", names)

		if len(names) == 0 {
			fmt.Println("=== failover namespace absent on this release ===")
			return
		}

		for _, m := range []string{"failover.licensed", "failover.config", "failover.status",
			"failover.node", "failover.disabled.reasons", "failover.disabled_reasons"} {
			raw, err := c.Call(context.Background(), m)
			if err != nil {
				fmt.Printf("=== %s error ===\n%v\n", m, err)
				continue
			}
			fmt.Printf("=== %s ===\n%s\n", m, raw)
		}
	}
	if section == "haupdateprobe" {
		// Probes failover.update's writable-field semantics via a harmless
		// no-op-content round trip: reads the current "timeout" value back
		// via failover.config, then re-sends that SAME value through
		// failover.update, confirming the call succeeds and nothing else
		// changes. Never touches "disabled" or "master" (those are the
		// fields with real failover-triggering side effects). DISPOSABLE
		// HA box only — never run against a shared/production system.
		raw, err := c.Call(context.Background(), "failover.config")
		if err != nil {
			log.Fatal("failover.config:", err)
		}
		fmt.Printf("=== failover.config (before) ===\n%s\n", raw)
		var cfg struct {
			Timeout int64 `json:"timeout"`
		}
		json.Unmarshal(raw, &cfg)

		rawUpd, err := c.Call(context.Background(), "failover.update", map[string]any{
			"timeout": cfg.Timeout,
		})
		if err != nil {
			fmt.Printf("=== failover.update({timeout: %d}) error ===\n%v\n", cfg.Timeout, err)
		} else {
			fmt.Printf("=== failover.update({timeout: %d}) response ===\n%s\n", cfg.Timeout, rawUpd)
		}

		raw2, err := c.Call(context.Background(), "failover.config")
		if err != nil {
			log.Fatal("failover.config (after):", err)
		}
		fmt.Printf("=== failover.config (after) ===\n%s\n", raw2)

		// Elicit failover.update's accepts schema for "master" and
		// "disabled" WITHOUT ever sending a valid value for either: a
		// deliberately wrong-typed value fails schema validation before
		// middleware executes anything, so this is safe to run even though
		// failover.update is otherwise absent from core.get_methods
		// (privately registered). If the field name is unknown entirely,
		// middleware reports "Extra inputs are not permitted" instead of a
		// type error — that distinguishes "field exists, wrong type" from
		// "field does not exist" without ever risking a real value.
		for _, badField := range []string{"master", "disabled"} {
			_, err := c.Call(context.Background(), "failover.update", map[string]any{badField: 12345})
			fmt.Printf("=== failover.update({%q: 12345}) (deliberately wrong type) error ===\n%v\n", badField, err)
		}

		// Probe "timeout"'s bounds: negative and a very large value. If
		// either round-trips as accepted, restore to the original value
		// read above immediately afterward.
		for _, tv := range []int64{-1, 999999999} {
			raw, err := c.Call(context.Background(), "failover.update", map[string]any{"timeout": tv})
			if err != nil {
				fmt.Printf("=== failover.update({timeout: %d}) error ===\n%v\n", tv, err)
				continue
			}
			fmt.Printf("=== failover.update({timeout: %d}) response ===\n%s\n", tv, raw)
			if _, err := c.Call(context.Background(), "failover.update", map[string]any{"timeout": cfg.Timeout}); err != nil {
				log.Fatal("RESTORE FAILED after timeout bounds probe:", err)
			}
		}
		raw3, err := c.Call(context.Background(), "failover.config")
		if err != nil {
			log.Fatal("failover.config (final):", err)
		}
		fmt.Printf("=== failover.config (final, should match original) ===\n%s\n", raw3)
	}
	if section == "hahealthpoll" {
		// Continues past pollHAUntilStable's status/node stabilization
		// (which only confirms the surviving node reports MASTER, not that
		// the failed-over peer has actually rejoined the cluster): polls
		// failover.disabled.reasons every 10s until it goes empty (peer
		// fully back and heartbeating) or the budget below elapses,
		// printing every read so a persistently non-empty reasons list
		// (peer still rebooting, or a real problem) is fully visible in
		// the transcript either way.
		ctx := context.Background()
		start := time.Now()
		budget := 9 * time.Minute
		deadline := start.Add(budget)
		fmt.Println("=== POLLING failover.disabled.reasons until empty (peer node fully rejoined) ===")
		var lastReasons string
		for time.Now().Before(deadline) {
			raw, err := c.CallRead(ctx, "failover.disabled.reasons")
			elapsed := time.Since(start).Round(time.Second)
			if err != nil {
				fmt.Printf("[%s] failover.disabled.reasons poll error: %v\n", elapsed, err)
			} else {
				var reasons []string
				json.Unmarshal(raw, &reasons)
				fmt.Printf("[%s] failover.disabled.reasons=%v\n", elapsed, reasons)
				lastReasons = fmt.Sprintf("%v", reasons)
				if len(reasons) == 0 {
					break
				}
			}
			time.Sleep(10 * time.Second)
		}
		fmt.Printf("\n=== FINAL failover.disabled.reasons after %s: %s ===\n", time.Since(start).Round(time.Second), lastReasons)

		fmt.Println("\n=== FINAL HEALTH CHECK ===")
		pp("system.version_short", call(c, "system.version_short"))
		pp("failover.status", call(c, "failover.status"))
		pp("failover.node", call(c, "failover.node"))
		pp("failover.disabled.reasons", call(c, "failover.disabled.reasons"))
		pp("failover.config", call(c, "failover.config"))
	}
	if section == "hapoll" {
		// Continues monitoring an already-triggered failover (used when
		// the trigger call itself returned a transport error because the
		// node went passive mid-response — see task-1-report.md for why
		// runHAFailover's own trigger-error branch could not distinguish
		// that case from a genuine rejection). Never calls become_passive
		// itself; read-only polling plus a final health check.
		pollHAUntilStable(c, time.Now(), 10*time.Minute)
	}
	if section == "hafailover" {
		// SCRIPTED ONE-OFF: executes ONE real, controlled failover against
		// the disposable HA box (Plan 20 Task 1). NOT part of `all` or any
		// other automated section — must be invoked explicitly by name, and
		// only against the box named by TRUENAS_HA_ALLOWED_ENDPOINT.
		runHAFailover(c)
	}
	if section == "ipmimethods" {
		// Method introspection for the ipmi.* namespace. Per the Plan 20
		// Task 1 finding (25.10.4 HA box: failover.update was directly
		// callable despite being absent from core.get_methods), this ALSO
		// direct-calls every candidate method by name regardless of whether
		// core.get_methods listed it, so an under-reported method is not
		// missed.
		raw, err := c.Call(context.Background(), "core.get_methods")
		if err != nil {
			log.Fatal(err)
		}
		var methods map[string]any
		json.Unmarshal(raw, &methods)
		var names []string
		for name := range methods {
			if len(name) >= 5 && name[:5] == "ipmi." {
				names = append(names, name)
			}
		}
		sortStrings(names)
		pp("ipmi.* method names (core.get_methods)", names)
		for _, name := range names {
			pp(name, methods[name])
		}

		fmt.Println("\n=== direct-call verification (bypassing core.get_methods listing) ===")
		pp("ipmi.is_loaded", call(c, "ipmi.is_loaded"))
		pp("ipmi.lan.query", call(c, "ipmi.lan.query"))
		pp("ipmi.lan.channels", call(c, "ipmi.lan.channels"))
	}
	if section == "ipmiprobe" {
		// Safe, read-mostly probe of ipmi.lan.query/channels/update shapes
		// (Plan 20 Task 2). The ONLY mutating call is a same-value update
		// round trip on channel 1 (re-sending exactly what query already
		// reported), to observe: (a) the exact update payload shape
		// ipmi.lan.update accepts (channel-keyed? full replace vs partial?),
		// (b) whether "password" is echoed back on query/channels
		// (WriteOnly vs Sensitive evidence), (c) whether an "apply_remote"
		// (or similar) flag exists for the HA peer. Never sends a password
		// value, never changes dhcp/ipaddress/netmask/gateway/vlan to a
		// different value than what was already read.
		lanRaw, err := c.Call(context.Background(), "ipmi.lan.query")
		if err != nil {
			log.Fatal("ipmi.lan.query:", err)
		}
		fmt.Printf("=== ipmi.lan.query (all channels, before) ===\n%s\n", lanRaw)

		chansRaw, err := c.Call(context.Background(), "ipmi.lan.channels")
		if err != nil {
			fmt.Printf("=== ipmi.lan.channels error ===\n%v\n", err)
		} else {
			fmt.Printf("=== ipmi.lan.channels ===\n%s\n", chansRaw)
		}

		var channels []map[string]any
		if err := json.Unmarshal(lanRaw, &channels); err != nil || len(channels) == 0 {
			log.Fatalf("ipmi.lan.query did not return a non-empty array: err=%v raw=%s", err, lanRaw)
		}
		first := channels[0]
		fmt.Printf("=== first channel object keys ===\n")
		var keys []string
		for k := range first {
			keys = append(keys, k)
		}
		sortStrings(keys)
		for _, k := range keys {
			fmt.Printf("  %s: %v (%T)\n", k, first[k], first[k])
		}

		channelNum, ok := first["channel"]
		if !ok {
			log.Fatal("first channel object has no \"channel\" key; inspect keys above to find the real id field")
		}

		// SAME-VALUE round trip: only "vlan" (if present) is resent, since
		// per the task brief dhcp/network fields are riskier to touch even
		// with the identical value on live enterprise BMC firmware. If
		// "vlan" is absent, fall back to re-sending the whole first object
		// verbatim minus any password-shaped key, to observe the accepts
		// shape without risking a real BMC network change.
		payload := map[string]any{}
		if vlan, ok := first["vlan"]; ok {
			payload["vlan"] = vlan
		} else {
			for k, v := range first {
				if k == "channel" || k == "id" || k == "password" {
					continue
				}
				payload[k] = v
			}
		}
		fmt.Printf("=== ipmi.lan.update(%v, %v) payload ===\n", channelNum, payload)
		updRaw, err := c.Call(context.Background(), "ipmi.lan.update", channelNum, payload)
		if err != nil {
			fmt.Printf("=== ipmi.lan.update error ===\n%v\n", err)
		} else {
			fmt.Printf("=== ipmi.lan.update response ===\n%s\n", updRaw)
		}

		lanRaw2, err := c.Call(context.Background(), "ipmi.lan.query")
		if err != nil {
			log.Fatal("ipmi.lan.query (after):", err)
		}
		fmt.Printf("=== ipmi.lan.query (all channels, after) ===\n%s\n", lanRaw2)
	}
	if section == "ipmirestore" {
		// SAFETY TOOL: repeatedly resends a known-good static IPMI LAN
		// config to a channel and polls ipmi.lan.query until ip_address/
		// subnet_mask actually stick (observed live: a single
		// ipmi.lan.update call round-tripped successfully but the very
		// next ipmi.lan.query read back "0.0.0.0"/"0.0.0.0" for
		// ip_address/subnet_mask instead of the values just sent — real
		// BMC firmware settle-time behavior, not a client bug, per repeat
		// testing during Plan 20 Task 2). Args (all via env, so this can be
		// invoked with a single-line command per channel number,
		// ipaddress, netmask, gateway):
		//   TRUENAS_PROBE_CHANNEL, TRUENAS_PROBE_IPADDRESS,
		//   TRUENAS_PROBE_NETMASK, TRUENAS_PROBE_GATEWAY
		channelStr := os.Getenv("TRUENAS_PROBE_CHANNEL")
		ipaddress := os.Getenv("TRUENAS_PROBE_IPADDRESS")
		netmask := os.Getenv("TRUENAS_PROBE_NETMASK")
		gateway := os.Getenv("TRUENAS_PROBE_GATEWAY")
		if channelStr == "" || ipaddress == "" || netmask == "" || gateway == "" {
			log.Fatal("set TRUENAS_PROBE_CHANNEL, TRUENAS_PROBE_IPADDRESS, TRUENAS_PROBE_NETMASK, TRUENAS_PROBE_GATEWAY")
		}
		var channel int64
		fmt.Sscanf(channelStr, "%d", &channel)

		payload := map[string]any{
			"dhcp":      false,
			"ipaddress": ipaddress,
			"netmask":   netmask,
			"gateway":   gateway,
			"vlan":      nil,
		}

		deadline := time.Now().Add(5 * time.Minute)
		attempt := 0
		for {
			attempt++
			fmt.Printf("=== attempt %d: ipmi.lan.update(%d, %v) ===\n", attempt, channel, payload)
			raw, err := c.Call(context.Background(), "ipmi.lan.update", channel, payload)
			if err != nil {
				fmt.Printf("=== ipmi.lan.update error ===\n%v\n", err)
			} else {
				fmt.Printf("=== ipmi.lan.update response ===\n%s\n", raw)
			}

			time.Sleep(5 * time.Second)

			queryRaw, err := c.Call(context.Background(), "ipmi.lan.query", map[string]any{
				"query-filters": []any{[]any{"channel", "=", channel}},
			})
			if err != nil {
				fmt.Printf("=== ipmi.lan.query error ===\n%v\n", err)
			} else {
				fmt.Printf("=== ipmi.lan.query (attempt %d) ===\n%s\n", attempt, queryRaw)
				var results []map[string]any
				json.Unmarshal(queryRaw, &results)
				if len(results) > 0 && results[0]["ip_address"] == ipaddress && results[0]["subnet_mask"] == netmask {
					fmt.Println("=== RESTORED: ip_address/subnet_mask match target ===")
					return
				}
			}

			if time.Now().After(deadline) {
				fmt.Println("=== GAVE UP after 5 minutes: ip_address/subnet_mask still do not match target ===")
				return
			}
		}
	}
	if section == "ipmivlanclear" {
		// DECISIVE PROBE (Plan 20 Task 2 review finding): does
		// ipmi.lan.update accept an EXPLICIT "vlan": null to clear a
		// previously-set VLAN tag, or does it reject/ignore that key?
		// Captures the channel's original full config FIRST, restores it
		// (polled, not trust-on-first-response) in every exit path,
		// including any log.Fatal — this is the live disposable Enterprise
		// HA box, never left in a different state than it started in.
		chansRaw, err := c.Call(context.Background(), "ipmi.lan.channels")
		if err != nil {
			log.Fatal("ipmi.lan.channels:", err)
		}
		var channels []int64
		if err := json.Unmarshal(chansRaw, &channels); err != nil || len(channels) == 0 {
			log.Fatalf("ipmi.lan.channels did not return a non-empty array: err=%v raw=%s", err, chansRaw)
		}
		channel := channels[0]

		queryArgs := map[string]any{"query-filters": [][]any{{"channel", "=", channel}}}
		queryChannel := func() map[string]any {
			raw, err := c.Call(context.Background(), "ipmi.lan.query", queryArgs)
			if err != nil {
				log.Fatal("ipmi.lan.query:", err)
			}
			var results []map[string]any
			if err := json.Unmarshal(raw, &results); err != nil || len(results) == 0 {
				log.Fatalf("ipmi.lan.query returned no result for channel %d: err=%v raw=%s", channel, err, raw)
			}
			return results[0]
		}

		orig := queryChannel()
		pp(fmt.Sprintf("channel %d ORIGINAL config", channel), orig)
		origDHCP := orig["ip_address_source"] == "DHCP" || orig["ip_address_source"] == "dhcp"
		origIP, _ := orig["ip_address"].(string)
		origNetmask, _ := orig["subnet_mask"].(string)
		origGateway, _ := orig["default_gateway_ip_address"].(string)
		origVlan := orig["vlan_id"] // json.Unmarshal gives float64 or nil

		restorePayload := map[string]any{"dhcp": origDHCP}
		if !origDHCP {
			restorePayload["ipaddress"] = origIP
			restorePayload["netmask"] = origNetmask
			restorePayload["gateway"] = origGateway
		}
		restorePayload["vlan"] = origVlan

		restore := func() {
			fmt.Printf("=== RESTORING channel %d to original vlan=%v ===\n", channel, origVlan)
			for attempt := 1; attempt <= 8; attempt++ {
				raw, err := c.Call(context.Background(), "ipmi.lan.update", channel, restorePayload)
				if err != nil {
					fmt.Printf("=== restore attempt %d: ipmi.lan.update error: %v ===\n", attempt, err)
				} else {
					fmt.Printf("=== restore attempt %d: ipmi.lan.update response: %s ===\n", attempt, raw)
				}
				time.Sleep(3 * time.Second)
				cur := queryChannel()
				if fmt.Sprintf("%v", cur["vlan_id"]) == fmt.Sprintf("%v", origVlan) &&
					(origDHCP || (cur["ip_address"] == origIP && cur["subnet_mask"] == origNetmask)) {
					fmt.Printf("=== RESTORED: channel %d matches original (vlan_id=%v) ===\n", channel, cur["vlan_id"])
					return
				}
			}
			fmt.Printf("!!! RESTORE FAILED for channel %d: last read %+v, want vlan_id=%v !!!\n", channel, queryChannel(), origVlan)
		}
		defer restore()

		// Step 1: ensure the channel has a NON-null vlan set, so clearing it
		// is an observable transition. If it's already null, set 100 first.
		testVlan := int64(100)
		if origVlan != nil {
			if f, ok := origVlan.(float64); ok {
				testVlan = int64(f) + 1
				if testVlan > 4096 {
					testVlan = 1
				}
			}
		}
		setPayload := map[string]any{"dhcp": origDHCP}
		if !origDHCP {
			setPayload["ipaddress"] = origIP
			setPayload["netmask"] = origNetmask
			setPayload["gateway"] = origGateway
		}
		setPayload["vlan"] = testVlan
		fmt.Printf("=== STEP 1: setting vlan=%d via ipmi.lan.update(%d, %v) ===\n", testVlan, channel, setPayload)
		if raw, err := c.Call(context.Background(), "ipmi.lan.update", channel, setPayload); err != nil {
			fmt.Printf("=== STEP 1 ipmi.lan.update error: %v ===\n", err)
		} else {
			fmt.Printf("=== STEP 1 ipmi.lan.update response: %s ===\n", raw)
		}
		time.Sleep(3 * time.Second)
		afterSet := queryChannel()
		pp("STEP 1 read-back (expect vlan_id set)", afterSet)

		// Step 2: THE decisive call — send an explicit "vlan": null and see
		// whether the API accepts it and clears the tag, or rejects/ignores.
		clearPayload := map[string]any{"dhcp": origDHCP}
		if !origDHCP {
			clearPayload["ipaddress"] = origIP
			clearPayload["netmask"] = origNetmask
			clearPayload["gateway"] = origGateway
		}
		clearPayload["vlan"] = nil
		fmt.Printf("=== STEP 2 (DECISIVE): clearing via ipmi.lan.update(%d, %v) [\"vlan\": null explicit] ===\n", channel, clearPayload)
		clearRaw, clearErr := c.Call(context.Background(), "ipmi.lan.update", channel, clearPayload)
		if clearErr != nil {
			fmt.Printf("=== STEP 2 ipmi.lan.update ERROR (API REJECTS explicit vlan:null): %v ===\n", clearErr)
		} else {
			fmt.Printf("=== STEP 2 ipmi.lan.update response: %s ===\n", clearRaw)
		}
		time.Sleep(3 * time.Second)
		afterClear := queryChannel()
		pp("STEP 2 read-back (decisive: does vlan_id read back null?)", afterClear)
		if clearErr == nil && afterClear["vlan_id"] == nil {
			fmt.Println("=== VERDICT: API ACCEPTS explicit \"vlan\": null and CLEARS the tag ===")
		} else {
			fmt.Printf("=== VERDICT: API does NOT clear on explicit \"vlan\": null (call err=%v, vlan_id after=%v) ===\n", clearErr, afterClear["vlan_id"])
		}
	}
	if section == "tcmethods" {
		// Method introspection for the truecommand.* namespace (Plan 20 Task
		// 4). Per prior tasks' repeated finding that core.get_methods can
		// under-report on 25.10.4, this ALSO direct-calls truecommand.config
		// regardless of what the listing showed.
		raw, err := c.Call(context.Background(), "core.get_methods")
		if err != nil {
			log.Fatal(err)
		}
		var methods map[string]any
		json.Unmarshal(raw, &methods)
		var names []string
		for name := range methods {
			if len(name) >= 12 && name[:12] == "truecommand." {
				names = append(names, name)
			}
		}
		sortStrings(names)
		pp("truecommand.* method names (core.get_methods)", names)
		for _, name := range names {
			pp(name, methods[name])
		}

		fmt.Println("\n=== direct-call verification (bypassing core.get_methods listing) ===")
		pp("truecommand.config", call(c, "truecommand.config"))
	}
	if section == "tcprobe" {
		// DECISIVE probe (Plan 20 Task 4): with the box's current
		// truecommand.config reporting enabled=false, can a cosmetic field be
		// updated via truecommand.update with ZERO side effects, sending a
		// payload that never includes "enabled": true? SAFETY: this probe
		// NEVER sends {"enabled": true} under any circumstance — it aborts
		// entirely if the box's current config already shows enabled=true.
		before := call(c, "truecommand.config")
		pp("truecommand.config (before)", before)
		beforeMap, ok := before.(map[string]any)
		if !ok {
			log.Fatal("truecommand.config did not return a JSON object")
		}
		if enabled, _ := beforeMap["enabled"].(bool); enabled {
			log.Fatal("ABORT: truecommand.config reports enabled=true already; refusing to probe further")
		}

		// Probe truecommand.update's accepts schema WITHOUT ever sending a
		// valid "enabled" value: a deliberately wrong-typed value for a
		// field that does not exist reports "Extra inputs are not
		// permitted" (or similar) before any middleware side effect runs,
		// distinguishing "field exists" from "field absent" safely.
		for _, badField := range []string{"api_key", "enabled"} {
			_, err := c.Call(context.Background(), "truecommand.update", map[string]any{badField: 12345})
			fmt.Printf("=== truecommand.update({%q: 12345}) (deliberately wrong type) error ===\n%v\n", badField, err)
		}

		// If api_key is present in the response, dump its exact JSON
		// representation verbatim (decisive for WriteOnly vs Sensitive).
		if v, ok := beforeMap["api_key"]; ok {
			fmt.Printf("=== truecommand.config[\"api_key\"] verbatim (before) = %#v ===\n", v)
		} else {
			fmt.Println("=== truecommand.config has no \"api_key\" key at all ===")
		}

		// DECISIVE round trip: truecommand.update's own accepts schema
		// (probed above via core.get_methods) exposes EXACTLY TWO
		// properties, "enabled" and "api_key" (16-char string or null) —
		// unlike tn_connect.update on 25.10, there is no third cosmetic
		// field at all. This sends ONLY "api_key" (a fabricated but
		// schema-valid 16-char string), deliberately never touching
		// "enabled", to determine whether setting api_key alone while
		// enabled stays false is genuinely side-effect-free (no outbound
		// call to iX Portal) or itself triggers real external behavior
		// (e.g. status_reason transitioning to the "Pending Confirmation"
		// state observed in the schema's enum). Restores api_key to null
		// immediately afterward regardless of outcome.
		fakeKey := "abcd1234abcd1234" // 16 chars, satisfies minLength/maxLength
		fmt.Printf("\n=== DECISIVE: truecommand.update({\"api_key\": %q}) [enabled untouched] ===\n", fakeKey)
		rawUpd, err := c.Call(context.Background(), "truecommand.update", map[string]any{"api_key": fakeKey})
		if err != nil {
			fmt.Printf("=== truecommand.update(api_key only) ERROR ===\n%v\n", err)
		} else {
			fmt.Printf("=== truecommand.update(api_key only) response ===\n%s\n", rawUpd)
		}

		afterSet := call(c, "truecommand.config")
		pp("truecommand.config (after api_key-only update)", afterSet)
		if afterMap, ok := afterSet.(map[string]any); ok {
			if enabled, _ := afterMap["enabled"].(bool); enabled {
				fmt.Println("=== !!! SIDE EFFECT: enabled flipped to true after api_key-only update !!! ===")
			} else {
				fmt.Println("=== confirmed: enabled still false after api_key-only update ===")
			}
			fmt.Printf("=== status/status_reason after api_key-only update: %v / %v ===\n", afterMap["status"], afterMap["status_reason"])
		}

		fmt.Println("\n=== RESTORING api_key to null ===")
		rawRestore, err := c.Call(context.Background(), "truecommand.update", map[string]any{"api_key": nil})
		if err != nil {
			fmt.Printf("=== truecommand.update (restore api_key=null) ERROR ===\n%v\n", err)
		} else {
			fmt.Printf("=== truecommand.update (restore api_key=null) response ===\n%s\n", rawRestore)
		}
		pp("truecommand.config (after restore)", call(c, "truecommand.config"))
	}
	if section == "vmwaremethods" {
		// Method introspection for the vmware.* namespace (Plan 20 Task 4).
		// Direct-calls vmware.query regardless of what the core.get_methods
		// listing showed, per prior tasks' repeated 25.10.4 under-reporting
		// finding.
		raw, err := c.Call(context.Background(), "core.get_methods")
		if err != nil {
			log.Fatal(err)
		}
		var methods map[string]any
		json.Unmarshal(raw, &methods)
		var names []string
		for name := range methods {
			if len(name) >= 7 && name[:7] == "vmware." {
				names = append(names, name)
			}
		}
		sortStrings(names)
		pp("vmware.* method names (core.get_methods)", names)
		for _, name := range names {
			pp(name, methods[name])
		}

		fmt.Println("\n=== direct-call verification (bypassing core.get_methods listing) ===")
		pp("vmware.query", call(c, "vmware.query"))
	}
	if section == "vmwareprobe" {
		// DECISIVE probe (Plan 20 Task 4): does vmware.create validate the
		// hostname/username/password against a real vCenter/ESXi endpoint
		// before persisting anything? Uses an RFC 5737 TEST-NET-1 address
		// (unreachable, reserved for documentation) plus fabricated
		// credentials. If create unexpectedly succeeds (i.e. does NOT
		// validate), the created record is deleted immediately afterward.
		pool := os.Getenv("TRUENAS_TEST_POOL")
		if pool == "" {
			pool = "tank"
		}

		fmt.Println("=== vmware.query (before, should be empty or pre-existing only) ===")
		pp("vmware.query (before)", call(c, "vmware.query"))

		createPayload := map[string]any{
			"datastore":  "tf-probe-datastore",
			"filesystem": pool,
			"hostname":   "192.0.2.123",
			"password":   "tf-probe-fake-password-1234",
			"username":   "tfprobeuser",
		}
		fmt.Printf("=== vmware.create payload (DECISIVE) ===\n%v\n", createPayload)
		raw, err := c.Call(context.Background(), "vmware.create", createPayload)
		if err != nil {
			fmt.Printf("=== vmware.create ERROR (verbatim; decisive: create validates against the real endpoint) ===\n%v\n", err)
			return
		}
		fmt.Printf("=== vmware.create response (decisive: create does NOT validate) ===\n%s\n", raw)

		var created struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(raw, &created)

		rawGet, err := c.Call(context.Background(), "vmware.get_instance", created.ID)
		if err != nil {
			fmt.Printf("=== vmware.get_instance error ===\n%v\n", err)
		} else {
			fmt.Printf("=== vmware.get_instance response (password read-back?) ===\n%s\n", rawGet)
		}

		rawUpd, err := c.Call(context.Background(), "vmware.update", created.ID, map[string]any{
			"datastore": "tf-probe-datastore-renamed",
		})
		if err != nil {
			fmt.Printf("=== vmware.update error ===\n%v\n", err)
		} else {
			fmt.Printf("=== vmware.update response ===\n%s\n", rawUpd)
		}

		fmt.Println("=== cleanup: vmware.delete (create unexpectedly persisted) ===")
		if _, err := c.Call(context.Background(), "vmware.delete", created.ID); err != nil {
			fmt.Printf("vmware.delete error: %v\n", err)
		} else {
			fmt.Println("=== vmware deleted ok ===")
		}
	}
}

// waitJob polls core.get_jobs for jobID until it reaches a terminal state,
// printing progress every 5th poll. Returns the job's result on SUCCESS.
func waitJob(c *client.Client, jobID int64, label string) json.RawMessage {
	for i := 0; ; i++ {
		jobs, err := c.Call(context.Background(), "core.get_jobs", []any{[]any{"id", "=", jobID}})
		if err != nil {
			log.Fatalf("%s: polling job %d: %v", label, jobID, err)
		}
		var entries []struct {
			State  string          `json:"state"`
			Result json.RawMessage `json:"result"`
			Error  string          `json:"error"`
		}
		json.Unmarshal(jobs, &entries)
		if len(entries) > 0 {
			switch entries[0].State {
			case "SUCCESS":
				return entries[0].Result
			case "FAILED", "ABORTED":
				fmt.Printf("=== %s job %s ===\n%s\n", label, entries[0].State, entries[0].Error)
				return nil
			}
		}
		if i%5 == 0 {
			fmt.Printf("... waiting for %s job %d (poll %d)\n", label, jobID, i)
		}
		time.Sleep(2 * time.Second)
	}
}

// runContainerProbe probes container.update's accepted fields (decides
// ForceNew for the truenas_container resource), container.image.
// query_registry's version ordering, the stop-on-never-started error text,
// and the full create/get_instance response shape (idmap, capabilities_*,
// dataset, default_network, status) via one real, disposable container on
// the target box. Cleans itself up (best-effort) regardless of outcome.
func runContainerProbe(c *client.Client) {
	raw, err := c.Call(context.Background(), "core.get_methods")
	if err != nil {
		log.Fatal(err)
	}
	var methods map[string]any
	json.Unmarshal(raw, &methods)
	for _, m := range []string{"container.create", "container.update", "container.start", "container.stop", "container.delete", "container.get_instance", "container.query", "container.image.query_registry", "container.pool_choices"} {
		if info, ok := methods[m]; ok {
			pp(m+" (core.get_methods)", info)
		} else {
			fmt.Printf("[%s] not present in core.get_methods\n", m)
		}
	}

	// container.image.query_registry takes NO arguments (probed: "Too many
	// arguments (expected 0, found 1)" when a query filter is passed) — it
	// returns every image name in the registry, so the truenas_container_image
	// datasource must filter client-side by name.
	registryRaw := call(c, "container.image.query_registry")
	var alpineEntry any
	if arr, ok := registryRaw.([]any); ok {
		fmt.Printf("=== container.image.query_registry: %d total image names ===\n", len(arr))
		for _, item := range arr {
			if entry, ok := item.(map[string]any); ok {
				if entry["name"] == "alpine:3.22:amd64:default" {
					alpineEntry = entry
				}
			}
		}
	}
	pp("container.image.query_registry (alpine:3.22:amd64:default entry)", alpineEntry)

	imageVersion := os.Getenv("TRUENAS_PROBE_IMAGE_VERSION")
	if imageVersion == "" {
		// Pull the last (newest) version out of the registry response so
		// the probe container create doesn't need a hardcoded version that
		// the upstream registry may have pruned.
		if entry, ok := alpineEntry.(map[string]any); ok {
			if versions, ok := entry["versions"].([]any); ok && len(versions) > 0 {
				fmt.Printf("=== alpine:3.22:amd64:default versions (registry order, %d entries) ===\n", len(versions))
				for i, v := range versions {
					if vm, ok := v.(map[string]any); ok {
						fmt.Printf("  [%d] %v\n", i, vm["version"])
					}
				}
				if last, ok := versions[len(versions)-1].(map[string]any); ok {
					if v, ok := last["version"].(string); ok {
						imageVersion = v
					}
				}
			}
		}
	}
	if imageVersion == "" {
		log.Fatal("could not determine an image version to probe with (set TRUENAS_PROBE_IMAGE_VERSION)")
	}
	fmt.Printf("=== using image version %q ===\n", imageVersion)

	name := "tf-probe-container"
	createPayload := map[string]any{
		"name":      name,
		"pool":      "tank",
		"autostart": false,
		"image":     map[string]any{"name": "alpine:3.22:amd64:default", "version": imageVersion},
	}
	fmt.Printf("=== container.create payload ===\n%v\n", createPayload)
	raw, err = c.Call(context.Background(), "container.create", createPayload)
	if err != nil {
		log.Fatal("container.create (job submit):", err)
	}
	var jobID int64
	json.Unmarshal(raw, &jobID)
	fmt.Printf("=== container.create job id === %d\n", jobID)

	createResult := waitJob(c, jobID, "container.create")
	if createResult == nil {
		log.Fatal("container.create did not succeed; aborting probe")
	}
	fmt.Printf("=== container.create result ===\n%s\n", createResult)

	var created struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(createResult, &created)

	defer func() {
		fmt.Println("=== cleanup: container.delete ===")
		raw, err := c.Call(context.Background(), "container.delete", created.ID)
		if err != nil {
			fmt.Printf("container.delete error: %v\n", err)
			return
		}
		var delJobID int64
		if json.Unmarshal(raw, &delJobID) == nil && delJobID != 0 {
			waitJob(c, delJobID, "container.delete")
		}
		fmt.Println("=== cleanup: deleted ===")
	}()

	// Probe stop-on-never-started error text.
	raw, err = c.Call(context.Background(), "container.stop", created.ID, map[string]any{"force": false})
	if err != nil {
		fmt.Printf("=== container.stop (never started) error ===\n%v\n", err)
	} else {
		var stopJobID int64
		json.Unmarshal(raw, &stopJobID)
		waitJob(c, stopJobID, "container.stop (never started)")
	}

	// Probe container.update: description (expected to work), then name and
	// pool changes to see if they're accepted or rejected (decides
	// ForceNew for the truenas_container resource's name/pool fields).
	for _, probe := range []map[string]any{
		{"description": "probe-updated-description"},
		{"name": "tf-probe-container-renamed"},
		{"pool": "tank"},
	} {
		raw, err := c.Call(context.Background(), "container.update", created.ID, probe)
		if err != nil {
			fmt.Printf("=== container.update(%v) error ===\n%v\n", probe, err)
		} else {
			fmt.Printf("=== container.update(%v) result ===\n%s\n", probe, raw)
		}
	}

	raw, err = c.Call(context.Background(), "container.get_instance", created.ID)
	if err != nil {
		log.Fatal("container.get_instance:", err)
	}
	fmt.Printf("=== container.get_instance (final) ===\n%s\n", raw)
}

// runContainerDeviceProbe dumps the core.get_methods "accepts" JSON schema
// for the full container.device.* namespace (the discriminated device-type
// union create/update accept, plus the choices helpers), then exercises a
// real create/update/query/get_instance/delete round trip for the
// filesystem-type device against a disposable, never-started probe
// container + dataset it creates and cleans up itself.
func runContainerDeviceProbe(c *client.Client) {
	raw, err := c.Call(context.Background(), "core.get_methods")
	if err != nil {
		log.Fatal(err)
	}
	var methods map[string]any
	json.Unmarshal(raw, &methods)

	// Discover every container.device.* method verbatim, not just an assumed
	// list, in case there are helpers beyond what the design doc anticipated.
	var deviceMethods []string
	for name := range methods {
		if len(name) > len("container.device.") && name[:len("container.device.")] == "container.device." {
			deviceMethods = append(deviceMethods, name)
		}
	}
	sortStrings(deviceMethods)
	fmt.Printf("=== container.device.* methods (%d) ===\n", len(deviceMethods))
	for _, m := range deviceMethods {
		fmt.Println(" ", m)
	}
	for _, m := range deviceMethods {
		pp(m+" (core.get_methods)", methods[m])
	}

	// Dump the choices helpers' live results too (empty on a box with no
	// USB/GPU/NIC hardware exposed for passthrough tells us those types are
	// not safely live-testable here).
	for _, m := range []string{"container.device.usb_choices", "container.device.nic_attach_choices", "container.device.gpu_choices"} {
		if _, ok := methods[m]; ok {
			pp(m+" (live result)", call(c, m))
		} else {
			fmt.Printf("[%s] not present in core.get_methods\n", m)
		}
	}

	pool := os.Getenv("TRUENAS_TEST_POOL")
	if pool == "" {
		pool = "tank"
	}

	// Resolve the image version FIRST (log.Fatal's before anything is
	// created if container.image.query_registry doesn't exist, e.g. a
	// namespace-absent 25.10 box) so a version-gate skip never leaves a
	// stray dataset behind.
	registryRaw := call(c, "container.image.query_registry")
	var alpineEntry any
	if arr, ok := registryRaw.([]any); ok {
		for _, item := range arr {
			if entry, ok := item.(map[string]any); ok {
				if entry["name"] == "alpine:3.22:amd64:default" {
					alpineEntry = entry
				}
			}
		}
	}
	imageVersion := os.Getenv("TRUENAS_PROBE_IMAGE_VERSION")
	if imageVersion == "" {
		if entry, ok := alpineEntry.(map[string]any); ok {
			if versions, ok := entry["versions"].([]any); ok && len(versions) > 0 {
				if last, ok := versions[len(versions)-1].(map[string]any); ok {
					if v, ok := last["version"].(string); ok {
						imageVersion = v
					}
				}
			}
		}
	}
	if imageVersion == "" {
		log.Fatal("could not determine an image version to probe with (set TRUENAS_PROBE_IMAGE_VERSION)")
	}

	// Now safe to create the disposable dataset + probe container
	// (autostart=false, mirrors runContainerProbe) to attach devices to.
	// Never started.
	dsName := pool + "/tf-probe-container-device-ds"
	if _, err := c.Call(context.Background(), "pool.dataset.create", map[string]any{"name": dsName}); err != nil {
		log.Fatal("dataset create:", err)
	}
	defer func() {
		fmt.Println("=== cleanup: pool.dataset.delete ===")
		if _, err := c.Call(context.Background(), "pool.dataset.delete", dsName); err != nil {
			fmt.Printf("dataset delete error: %v\n", err)
		}
	}()

	createPayload := map[string]any{
		"name":      "tf-probe-container-device",
		"pool":      pool,
		"autostart": false,
		"image":     map[string]any{"name": "alpine:3.22:amd64:default", "version": imageVersion},
	}
	raw, err = c.Call(context.Background(), "container.create", createPayload)
	if err != nil {
		log.Fatal("container.create (job submit):", err)
	}
	var jobID int64
	json.Unmarshal(raw, &jobID)
	createResult := waitJob(c, jobID, "container.create")
	if createResult == nil {
		log.Fatal("container.create did not succeed; aborting probe")
	}
	var createdContainer struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	json.Unmarshal(createResult, &createdContainer)
	fmt.Printf("=== probe container created: id=%d name=%s ===\n", createdContainer.ID, createdContainer.Name)

	defer func() {
		fmt.Println("=== cleanup: container.delete ===")
		raw, err := c.Call(context.Background(), "container.delete", createdContainer.ID)
		if err != nil {
			fmt.Printf("container.delete error: %v\n", err)
			return
		}
		var delJobID int64
		if json.Unmarshal(raw, &delJobID) == nil && delJobID != 0 {
			waitJob(c, delJobID, "container.delete")
		}
		fmt.Println("=== cleanup: container deleted ===")
	}()

	// Try a FILESYSTEM-type device create using the field names discovered
	// from the accepts schema dumped above. Adjust the dtype/keys below
	// after inspecting the schema output if the first attempt is rejected.
	devicePayload := map[string]any{
		"container": createdContainer.ID,
		"attributes": map[string]any{
			"dtype":  "FILESYSTEM",
			"source": "/mnt/" + dsName,
			"target": "/data",
		},
	}
	fmt.Printf("=== container.device.create payload (attempt) ===\n%v\n", devicePayload)
	devRaw, err := c.Call(context.Background(), "container.device.create", devicePayload)
	if err != nil {
		fmt.Printf("=== container.device.create error ===\n%v\n", err)
		return
	}
	fmt.Printf("=== container.device.create response ===\n%s\n", devRaw)

	var createdDevice struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(devRaw, &createdDevice)

	getRaw, err := c.Call(context.Background(), "container.device.get_instance", createdDevice.ID)
	if err != nil {
		fmt.Printf("=== container.device.get_instance error ===\n%v\n", err)
	} else {
		fmt.Printf("=== container.device.get_instance response ===\n%s\n", getRaw)
	}

	queryRaw, err := c.Call(context.Background(), "container.device.query", [][]any{{"id", "=", createdDevice.ID}})
	if err != nil {
		fmt.Printf("=== container.device.query error ===\n%v\n", err)
	} else {
		fmt.Printf("=== container.device.query response ===\n%s\n", queryRaw)
	}

	updatePayload := map[string]any{
		"attributes": map[string]any{
			"dtype":  "FILESYSTEM",
			"source": "/mnt/" + dsName,
			"target": "/data2",
		},
	}
	updRaw, err := c.Call(context.Background(), "container.device.update", createdDevice.ID, updatePayload)
	if err != nil {
		fmt.Printf("=== container.device.update error ===\n%v\n", err)
	} else {
		fmt.Printf("=== container.device.update response ===\n%s\n", updRaw)
	}

	if _, err := c.Call(context.Background(), "container.device.delete", createdDevice.ID); err != nil {
		fmt.Printf("=== container.device.delete error ===\n%v\n", err)
	} else {
		fmt.Println("=== container.device.delete ok ===")
	}

	// Edge cases: target under /mnt/ (expect rejection, mirrors source's
	// "must reside within a pool mount point" but for the in-container
	// side), and a FILESYSTEM device with source/target omitted entirely
	// (see what the real server-applied default is, vs the odd
	// "/usr/bin/zsh" literal the accepts schema advertised above).
	badTargetPayload := map[string]any{
		"container": createdContainer.ID,
		"attributes": map[string]any{
			"dtype":  "FILESYSTEM",
			"source": "/mnt/" + dsName,
			"target": "/mnt/probe",
		},
	}
	if raw, err := c.Call(context.Background(), "container.device.create", badTargetPayload); err != nil {
		fmt.Printf("=== container.device.create (target under /mnt/) error ===\n%v\n", err)
	} else {
		fmt.Printf("=== container.device.create (target under /mnt/) response ===\n%s\n", raw)
		var d struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(raw, &d)
		c.Call(context.Background(), "container.device.delete", d.ID)
	}

	noSourceTargetPayload := map[string]any{
		"container": createdContainer.ID,
		"attributes": map[string]any{
			"dtype": "FILESYSTEM",
		},
	}
	if raw, err := c.Call(context.Background(), "container.device.create", noSourceTargetPayload); err != nil {
		fmt.Printf("=== container.device.create (no source/target) error ===\n%v\n", err)
	} else {
		fmt.Printf("=== container.device.create (no source/target) response ===\n%s\n", raw)
		var d struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(raw, &d)
		c.Call(context.Background(), "container.device.delete", d.ID)
	}

	noTargetPayload := map[string]any{
		"container": createdContainer.ID,
		"attributes": map[string]any{
			"dtype":  "FILESYSTEM",
			"source": "/mnt/" + dsName,
		},
	}
	if raw, err := c.Call(context.Background(), "container.device.create", noTargetPayload); err != nil {
		fmt.Printf("=== container.device.create (source only, no target) error ===\n%v\n", err)
	} else {
		fmt.Printf("=== container.device.create (source only, no target) response ===\n%s\n", raw)
		var d struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(raw, &d)
		c.Call(context.Background(), "container.device.delete", d.ID)
	}

	// NIC device against the truenasbr0 software bridge nic_attach_choices
	// reported above — safe, never-started container, no actual host
	// interface state changes until (if ever) the container is started.
	nicPayload := map[string]any{
		"container": createdContainer.ID,
		"attributes": map[string]any{
			"dtype":      "NIC",
			"nic_attach": "truenasbr0",
		},
	}
	if raw, err := c.Call(context.Background(), "container.device.create", nicPayload); err != nil {
		fmt.Printf("=== container.device.create (NIC) error ===\n%v\n", err)
	} else {
		fmt.Printf("=== container.device.create (NIC) response ===\n%s\n", raw)
		var d struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(raw, &d)
		c.Call(context.Background(), "container.device.delete", d.ID)
	}

	// USB device against the usb_1_4 device usb_choices reported above
	// (available:true) — same never-started-container safety rationale.
	usbPayload := map[string]any{
		"container": createdContainer.ID,
		"attributes": map[string]any{
			"dtype": "USB",
			"usb": map[string]any{
				"vendor_id":  "0x046b",
				"product_id": "0xff10",
			},
		},
	}
	if raw, err := c.Call(context.Background(), "container.device.create", usbPayload); err != nil {
		fmt.Printf("=== container.device.create (USB) error ===\n%v\n", err)
	} else {
		fmt.Printf("=== container.device.create (USB) response ===\n%s\n", raw)
		var d struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(raw, &d)
		c.Call(context.Background(), "container.device.delete", d.ID)
	}

	// GPU device with a fabricated PCI address (gpu_choices returned {} on
	// this box — no real GPU to reference), purely to see whether
	// pci_address is validated against actual host hardware.
	gpuPayload := map[string]any{
		"container": createdContainer.ID,
		"attributes": map[string]any{
			"dtype":       "GPU",
			"gpu_type":    "NVIDIA",
			"pci_address": "0000:00:99.9",
		},
	}
	if raw, err := c.Call(context.Background(), "container.device.create", gpuPayload); err != nil {
		fmt.Printf("=== container.device.create (GPU, fabricated pci_address) error ===\n%v\n", err)
	} else {
		fmt.Printf("=== container.device.create (GPU, fabricated pci_address) response ===\n%s\n", raw)
		var d struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(raw, &d)
		c.Call(context.Background(), "container.device.delete", d.ID)
	}

	// USB controller-only device (usb: null per the schema).
	usbNullPayload := map[string]any{
		"container": createdContainer.ID,
		"attributes": map[string]any{
			"dtype": "USB",
		},
	}
	if raw, err := c.Call(context.Background(), "container.device.create", usbNullPayload); err != nil {
		fmt.Printf("=== container.device.create (USB controller-only) error ===\n%v\n", err)
	} else {
		fmt.Printf("=== container.device.create (USB controller-only) response ===\n%s\n", raw)
		var d struct {
			ID int64 `json:"id"`
		}
		json.Unmarshal(raw, &d)
		c.Call(context.Background(), "container.device.delete", d.ID)
	}
}

// runHAFailover executes ONE real, controlled failover against the
// disposable HA box (Plan 20 Task 1's scripted one-off exercise).
//
// Trigger choice (decided from live probe evidence, see task-1-report.md):
// failover.become_passive, not failover.reboot.other_node. The endpoint
// (TRUENAS_ENDPOINT) resolves to 10.220.16.188, confirmed live via
// interface.query to be a VRRP failover_virtual_alias — i.e. the floating
// HA management IP, currently in vrrp_config state MASTER on the connected
// node (node B). failover.reboot.other_node, called against this
// connection, would only reboot the STANDBY node (node A) — it does not
// relinquish this node's mastership, so it would not exercise a real
// failover. failover.become_passive, called against this (master)
// connection, makes THIS node (B) relinquish mastership so node A takes
// over — a genuine controlled failover — and because 10.220.16.188 is the
// VRRP VIP, reconnecting to the SAME endpoint afterward is expected to
// land on node A once VRRP has migrated the address.
func runHAFailover(c *client.Client) {
	ctx := context.Background()
	start := time.Now()
	elapsed := func() time.Duration { return time.Since(start).Round(time.Second) }

	fmt.Println("=== HA FAILOVER EXERCISE: BEFORE STATE ===")
	pp("system.version_short (before)", call(c, "system.version_short"))
	pp("failover.status (before)", call(c, "failover.status"))
	pp("failover.node (before)", call(c, "failover.node"))
	pp("failover.config (before)", call(c, "failover.config"))
	pp("failover.disabled.reasons (before)", call(c, "failover.disabled.reasons"))

	fmt.Println("\n=== TRIGGER: failover.become_passive ===")
	raw, err := c.Call(ctx, "failover.become_passive")
	if err != nil {
		// A transport-level error here is ambiguous, not necessarily a
		// rejected trigger: become_passive uses STCNITH ("Shoot The
		// Current Node In The Head") to guarantee the node truly gets out
		// of the way, so a successful call can legitimately never return a
		// normal response — the node can go passive (or reboot) before
		// the response is written back over this same connection,
		// surfacing here as "connection reset by peer" rather than a
		// clean result. Observed live exactly this way: see
		// task-1-report.md for the verbatim error and the immediate
		// follow-up probe that confirmed the failover really had started
		// (failover.disabled.reasons briefly showed ["LOC_FAILOVER_ONGOING",
		// "NO_PONG"] on reconnect). So: log the error verbatim, but do NOT
		// treat it as "trigger rejected" — fall through to the same
		// stabilization poll a successful call would use. A genuinely
		// rejected trigger (e.g. failover already disabled) would show up
		// as a clean APIError instead, in which case the poll loop below
		// simply confirms nothing changed and exits quickly.
		fmt.Printf("failover.become_passive returned an error (verbatim, may be a transport drop caused by the "+
			"trigger itself firing, not necessarily a rejection): %v\n", err)
		fmt.Println("Proceeding to the stabilization poll to determine what actually happened; a rejected " +
			"trigger will show as no state change.")
	} else {
		fmt.Printf("failover.become_passive response: %s\n", raw)
	}
	fmt.Printf("[%s] become_passive call returned\n", elapsed())

	pollHAUntilStable(c, start, 10*time.Minute)
}

// pollHAUntilStable polls failover.status/failover.node every 5s (via
// CallRead, which auto-reconnects on transient transport failures — the
// endpoint may go fully unreachable for a stretch while VRRP migrates the
// virtual IP between nodes) until 3 consecutive non-transitional reads
// agree, or until deadline (start + budget) passes, then runs the
// post-event health check (system.version_short, failover.status,
// failover.disabled.reasons, failover.config).
func pollHAUntilStable(c *client.Client, start time.Time, budget time.Duration) {
	ctx := context.Background()
	elapsed := func() time.Duration { return time.Since(start).Round(time.Second) }

	fmt.Println("\n=== POLLING failover.status until stabilized (generous timeout: 10 minutes) ===")
	deadline := start.Add(budget)
	var lastStatus, lastNode string
	stableCount := 0
	for time.Now().Before(deadline) {
		time.Sleep(5 * time.Second)

		statusRaw, err := c.CallRead(ctx, "failover.status")
		if err != nil {
			fmt.Printf("[%s] failover.status poll error (endpoint may be mid-migration): %v\n", elapsed(), err)
			stableCount = 0
			continue
		}
		var status string
		json.Unmarshal(statusRaw, &status)

		var node string
		if nodeRaw, err := c.CallRead(ctx, "failover.node"); err == nil {
			json.Unmarshal(nodeRaw, &node)
		} else {
			fmt.Printf("[%s] failover.node poll error: %v\n", elapsed(), err)
		}

		fmt.Printf("[%s] failover.status=%s failover.node=%s\n", elapsed(), status, node)

		transitional := status == "ELECTING" || status == "IMPORTING" || status == "ERROR" || status == ""
		if !transitional && status == lastStatus && node == lastNode {
			stableCount++
		} else {
			stableCount = 1
		}
		lastStatus, lastNode = status, node

		// 3 consecutive stable, non-transitional reads (~15s of agreement)
		// before declaring the failover complete.
		if !transitional && stableCount >= 3 {
			break
		}
	}
	fmt.Printf("\n=== STABILIZED after %s: failover.status=%s failover.node=%s ===\n", elapsed(), lastStatus, lastNode)

	fmt.Println("\n=== AFTER STATE (health check) ===")
	pp("system.version_short (after)", call(c, "system.version_short"))
	pp("failover.status (after)", call(c, "failover.status"))
	pp("failover.node (after)", call(c, "failover.node"))
	pp("failover.disabled.reasons (after)", call(c, "failover.disabled.reasons"))
	pp("failover.config (after)", call(c, "failover.config"))
}
