// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package pool

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestVdevDisks_spareActive verifies that a data mirror whose faulted member
// has been covered by a hot spare reads back with its ORIGINAL configured
// members, not the transient SPARE repair vdev. This is the parse that keeps a
// spare activation from planning a destroy/recreate of a degraded pool: the
// live pool.query shape (captured from a real spare activation on the test VM)
// nests a "SPARE" vdev whose first child is the original faulted disk and whose
// second child is the spare. vdevDisks must represent the slot by the original.
func TestVdevDisks_spareActive(t *testing.T) {
	// Trimmed from a real pool.query during an active hot spare (mirror-0 with
	// sdd ONLINE and sde faulted, covered by spare sdg).
	const raw = `{
	  "type": "MIRROR",
	  "children": [
	    { "type": "DISK", "disk": "sdd", "children": [] },
	    { "type": "SPARE", "disk": "", "children": [
	        { "type": "DISK", "disk": "sde", "children": [] },
	        { "type": "DISK", "disk": "sdg", "children": [] }
	    ]}
	  ]
	}`
	var v poolVdev
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	got := vdevDisks(v)
	want := []string{"sdd", "sde"} // original members, spare (sdg) not surfaced
	if !reflect.DeepEqual(got, want) {
		t.Errorf("vdevDisks(spare-active mirror) = %v, want %v", got, want)
	}
}

// TestVdevDisks_healthyAndSingle covers the ordinary shapes still work: a
// multi-disk mirror lists its leaf disks, and a single-disk vdev reports its
// device at the vdev top level.
func TestVdevDisks_healthyAndSingle(t *testing.T) {
	mirror := poolVdev{Type: "MIRROR", Children: []poolDisk{{Disk: "sda"}, {Disk: "sdb"}}}
	if got := vdevDisks(mirror); !reflect.DeepEqual(got, []string{"sda", "sdb"}) {
		t.Errorf("healthy mirror = %v, want [sda sdb]", got)
	}
	single := poolVdev{Type: "DISK", Disk: "sdc"}
	if got := vdevDisks(single); !reflect.DeepEqual(got, []string{"sdc"}) {
		t.Errorf("single-disk vdev = %v, want [sdc]", got)
	}
}
