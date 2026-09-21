// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package pool

import (
	"context"
	"encoding/json"
	"path"
	"strings"

	"github.com/truenas/terraform-provider-truenas/internal/client"
)

// diskIdentity holds the identity fields of one physical disk, as reported by
// disk.query. Probed live (TrueNAS 26.0, wss://192.168.1.68):
//
//		{"name": "sde", "devname": "sde",
//		 "serial": "SMC0515D90925BQ85185",
//		 "identifier": "{serial_lunid}SMC0515D90925BQ85185_515d90925b000185"}
//
//	  - Name is the CURRENT kernel device name ("sdX"). It is NOT stable across
//	    reboots — the whole reason for issue #9.
//	  - Serial is stable and human-readable (matches the printed disk label).
//	  - Identifier is TrueNAS's own canonical handle, of the form
//	    "{serial_lunid}<serial>_<lunid>", "{serial}<serial>", or "{uuid}<uuid>".
type diskIdentity struct {
	Name       string `json:"name"`
	Devname    string `json:"devname"`
	Serial     string `json:"serial"`
	Identifier string `json:"identifier"`
}

// diskResolver translates between the several ways a disk can be named
// (kernel sdX, serial, TrueNAS identifier, /dev/disk/by-id path) and the two
// canonical forms this provider uses: the SERIAL for Terraform state (stable),
// and the current kernel device NAME for the pool.create payload (what the API
// consumes). It is built once per operation from a single disk.query call.
type diskResolver struct {
	all []diskIdentity
	// Lookups keyed by each known form to the owning disk. Keys are matched
	// case-sensitively except where noted in the resolve helpers.
	byName       map[string]*diskIdentity
	bySerial     map[string]*diskIdentity
	byIdentifier map[string]*diskIdentity
}

// newDiskResolver builds a resolver from disk.query. A caller that cannot
// reach disk.query still gets a non-nil resolver (with empty tables) so every
// resolve call falls back to returning its input unchanged, rather than the
// provider hard-failing an otherwise-valid operation over a disk-naming
// convenience.
func newDiskResolver(ctx context.Context, c *client.Client) (*diskResolver, error) {
	r := &diskResolver{
		byName:       map[string]*diskIdentity{},
		bySerial:     map[string]*diskIdentity{},
		byIdentifier: map[string]*diskIdentity{},
	}
	raw, err := c.CallRead(ctx, "disk.query")
	if err != nil {
		return r, err
	}
	if err := json.Unmarshal(raw, &r.all); err != nil {
		return r, err
	}
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
	return r, nil
}

// lookup finds the disk that a user- or API-supplied name refers to, in any
// accepted form. It returns nil when the name resolves to no known disk, in
// which case callers fall back to using the raw string.
func (r *diskResolver) lookup(name string) *diskIdentity {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if d, ok := r.bySerial[name]; ok {
		return d
	}
	if d, ok := r.byIdentifier[name]; ok {
		return d
	}
	if d, ok := r.byName[name]; ok {
		return d
	}
	// A "{...}serial..." identifier whose exact string we did not index (e.g.
	// the lunid drifted): pull the serial out of the "{tag}serial[_lunid]"
	// shape and match on that.
	if s := serialFromIdentifier(name); s != "" {
		if d, ok := r.bySerial[s]; ok {
			return d
		}
	}
	// A device or by-id path: "/dev/sdb" -> "sdb"; "/dev/disk/by-id/ata-<model>_<serial>"
	// (and wwn-/scsi-/nvme- variants) -> the disk whose serial is embedded in
	// the last path segment.
	if strings.HasPrefix(name, "/dev/") {
		base := path.Base(name)
		if d, ok := r.byName[base]; ok {
			return d
		}
		if d := r.matchBySerialSubstring(base); d != nil {
			return d
		}
	}
	return nil
}

// matchBySerialSubstring resolves a /dev/disk/by-id last segment (e.g.
// "ata-WDC_WD40EFRX-68N32N0_WD-WCC7K5PACL0V") by finding the single known disk
// whose serial appears within it. disk.query exposes no by-id path field, so
// this substring match is the available bridge; it resolves only when exactly
// one disk matches, to avoid guessing.
func (r *diskResolver) matchBySerialSubstring(segment string) *diskIdentity {
	seg := strings.ToUpper(segment)
	var match *diskIdentity
	for i := range r.all {
		s := strings.ToUpper(r.all[i].Serial)
		if s == "" {
			continue
		}
		if strings.Contains(seg, s) {
			if match != nil {
				return nil // ambiguous — refuse to guess
			}
			match = &r.all[i]
		}
	}
	return match
}

// serialFor returns the stable serial for a disk named in any accepted form,
// for storing in Terraform state. When the disk cannot be resolved (unknown,
// or disk.query unavailable) it returns the input unchanged, so state still
// round-trips rather than losing the reference.
func (r *diskResolver) serialFor(name string) string {
	if d := r.lookup(name); d != nil && d.Serial != "" {
		return d.Serial
	}
	return strings.TrimSpace(name)
}

// deviceFor returns the current kernel device name (sdX) for a disk named in
// any accepted form, for the pool.create payload. Falls back to the input when
// unresolvable, preserving the pre-#9 behavior of passing a device name
// straight through.
func (r *diskResolver) deviceFor(name string) string {
	if d := r.lookup(name); d != nil && d.Name != "" {
		return d.Name
	}
	return strings.TrimSpace(name)
}

// sameDisk reports whether two names (each in any accepted form) refer to the
// same physical disk. It is the identity test ModifyPlan uses to tell a
// harmless rename (sdX renumber, or a serial written where sdX is stored)
// apart from a real disk swap. When either name cannot be resolved it falls
// back to a trimmed string compare.
func (r *diskResolver) sameDisk(a, b string) bool {
	da, db := r.lookup(a), r.lookup(b)
	if da != nil && db != nil {
		return da == db
	}
	return strings.TrimSpace(a) == strings.TrimSpace(b)
}

// serialFromIdentifier extracts the serial from a TrueNAS disk identifier of
// the form "{serial_lunid}<serial>_<lunid>" or "{serial}<serial>". It returns
// "" for any other shape (e.g. "{uuid}...", "{devicename}sdb"), where the
// remainder is not a serial.
func serialFromIdentifier(id string) string {
	if !strings.HasPrefix(id, "{") {
		return ""
	}
	close := strings.IndexByte(id, '}')
	if close < 0 {
		return ""
	}
	tag := id[1:close]
	rest := id[close+1:]
	switch tag {
	case "serial_lunid":
		if i := strings.LastIndexByte(rest, '_'); i >= 0 {
			return rest[:i]
		}
		return rest
	case "serial":
		return rest
	default:
		return ""
	}
}
