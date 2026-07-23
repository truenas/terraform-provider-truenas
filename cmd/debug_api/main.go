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
