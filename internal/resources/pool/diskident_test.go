// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package pool

import "testing"

// testResolver builds a diskResolver directly from identities, bypassing the
// disk.query call, to unit-test the pure resolution logic.
func testResolver(disks ...diskIdentity) *diskResolver {
	r := &diskResolver{
		byName:       map[string]*diskIdentity{},
		bySerial:     map[string]*diskIdentity{},
		byIdentifier: map[string]*diskIdentity{},
	}
	r.all = disks
	for i := range r.all {
		d := &r.all[i]
		if d.Name != "" {
			r.byName[d.Name] = d
		}
		if d.Devname != "" {
			r.byName[d.Devname] = d
		}
		if d.Serial != "" {
			r.bySerial[d.Serial] = d
		}
		if d.Identifier != "" {
			r.byIdentifier[d.Identifier] = d
		}
	}
	return r
}

var (
	diskA = diskIdentity{Name: "sdb", Devname: "sdb", Serial: "WD-WCC7K5PACL0V", Identifier: "{serial_lunid}WD-WCC7K5PACL0V_50014ee2bfd12345"}
	diskB = diskIdentity{Name: "sdc", Devname: "sdc", Serial: "SMC0515D90925BQ85185", Identifier: "{serial_lunid}SMC0515D90925BQ85185_515d90925b000185"}
	diskC = diskIdentity{Name: "sdd", Devname: "sdd", Serial: "S3Z1NB0K", Identifier: "{serial}S3Z1NB0K"}
)

func TestSerialFor(t *testing.T) {
	r := testResolver(diskA, diskB, diskC)
	cases := map[string]string{
		"sdb":             "WD-WCC7K5PACL0V", // kernel name -> serial
		"WD-WCC7K5PACL0V": "WD-WCC7K5PACL0V", // serial -> serial
		"{serial_lunid}WD-WCC7K5PACL0V_50014ee2bfd12345": "WD-WCC7K5PACL0V", // identifier -> serial
		"{serial}S3Z1NB0K": "S3Z1NB0K",             // {serial} identifier
		"/dev/sdc":         "SMC0515D90925BQ85185", // /dev/sdX path
		"/dev/disk/by-id/ata-WDC_WD40EFRX_WD-WCC7K5PACL0V": "WD-WCC7K5PACL0V", // by-id embeds serial
		"sdz":     "sdz",             // unknown -> unchanged
		"  sdb  ": "WD-WCC7K5PACL0V", // trimmed
	}
	for in, want := range cases {
		if got := r.serialFor(in); got != want {
			t.Errorf("serialFor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDeviceFor(t *testing.T) {
	r := testResolver(diskA, diskB, diskC)
	cases := map[string]string{
		"WD-WCC7K5PACL0V": "sdb", // serial -> current kernel name
		"{serial_lunid}SMC0515D90925BQ85185_515d90925b000185": "sdc", // identifier -> name
		"sdb":                                  "sdb", // name -> name
		"/dev/disk/by-id/ata-Samsung_S3Z1NB0K": "sdd", // by-id -> name
		"sdz":                                  "sdz", // unknown -> unchanged
	}
	for in, want := range cases {
		if got := r.deviceFor(in); got != want {
			t.Errorf("deviceFor(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSameDisk(t *testing.T) {
	r := testResolver(diskA, diskB, diskC)
	// Same physical disk, different naming forms.
	if !r.sameDisk("sdb", "WD-WCC7K5PACL0V") {
		t.Error("sdb and its serial should be the same disk")
	}
	if !r.sameDisk("WD-WCC7K5PACL0V", "{serial_lunid}WD-WCC7K5PACL0V_50014ee2bfd12345") {
		t.Error("serial and identifier of one disk should match")
	}
	// A kernel renumber: state stored sdb, but sdb now refers to a DIFFERENT
	// physical disk than the one whose serial the config pins. Must be seen as
	// a real change.
	if r.sameDisk("WD-WCC7K5PACL0V", "sdc") {
		t.Error("different physical disks must not be equal")
	}
	// Unknown names fall back to string compare.
	if !r.sameDisk("sdx1", "sdx1") {
		t.Error("identical unknown names should be equal")
	}
	if r.sameDisk("sdx1", "sdx2") {
		t.Error("different unknown names should not be equal")
	}
}

func TestSerialFromIdentifier(t *testing.T) {
	cases := map[string]string{
		"{serial_lunid}ABC123_deadbeef": "ABC123",
		"{serial}ABC123":                "ABC123",
		"{serial_lunid}ABC123":          "ABC123", // no lunid separator
		"{uuid}1234-5678":               "",       // not a serial-bearing form
		"{devicename}sdb":               "",
		"plainstring":                   "",
		"":                              "",
	}
	for in, want := range cases {
		if got := serialFromIdentifier(in); got != want {
			t.Errorf("serialFromIdentifier(%q) = %q, want %q", in, got, want)
		}
	}
}
