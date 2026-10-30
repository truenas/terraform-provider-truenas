// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package dataset

import "testing"

// Baselines measured live on 25.10/26.0/28.0: a fresh zvol reports 28672
// (plain) or 41472 (encrypted) regardless of volblocksize; one written 16K
// block takes a plain one to 45056.
func TestZvolHoldsData(t *testing.T) {
	cases := []struct {
		lr, vbs   int64
		encrypted bool
		want      bool
	}{
		{28672, 16384, false, false},  // fresh plain
		{45056, 16384, false, true},   // one 16K block written
		{28672, 131072, false, false}, // fresh plain, 128K blocks
		{28672 + 131072, 131072, false, true},
		{41472, 16384, true, false},  // fresh encrypted
		{41472, 131072, true, false}, // fresh encrypted, 128K blocks
		{41472 + 16384, 16384, true, true},
		{45056, 0, false, true}, // unknown volblocksize falls back to 16K
	}
	for _, c := range cases {
		if got := zvolHoldsData(c.lr, c.vbs, c.encrypted); got != c.want {
			t.Errorf("lr=%d vbs=%d enc=%v: got %v want %v", c.lr, c.vbs, c.encrypted, got, c.want)
		}
	}
}
