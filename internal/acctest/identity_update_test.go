// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package acctest_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestResourceIdentitySetInUpdate guards against the class of bug reported in
// issue #20: a resource that declares an identity schema (ResourceWithIdentity)
// but does not populate resp.Identity in its Update method fails every
// in-place update with "unexpectedly returned no resource identity data after
// having no errors in the resource update". This is a plain source scan (no
// TF_ACC): for every resource whose Update can succeed, its Update must call
// SetIdentity.
func TestResourceIdentitySetInUpdate(t *testing.T) {
	root := repoRoot(t)
	files, err := filepath.Glob(filepath.Join(root, "internal", "resources", "*", "resource.go"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	updateRe := regexp.MustCompile(`(?s)func \(r \*[A-Za-z0-9_]+Resource\) Update\(.*?\n}\n`)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		src := string(b)
		if !strings.Contains(src, "ResourceWithIdentity") {
			continue
		}
		m := updateRe.FindString(src)
		if m == "" {
			t.Errorf("%s: declares identity but no Update method found", res(f))
			continue
		}
		// An immutable resource whose Update only errors never returns success
		// without identity, so it is exempt.
		if strings.Contains(m, "Update not supported") || strings.Contains(m, "immutable") {
			continue
		}
		// listing.SetIdentity, or dataset.SetGuidIdentity for the guid-keyed
		// dataset/zvol (F8).
		if !strings.Contains(m, "SetIdentity") && !strings.Contains(m, "SetGuidIdentity") {
			t.Errorf("%s: Update does not call SetIdentity — in-place updates will fail with "+
				"\"no resource identity data after update\" (issue #20). Mirror this resource's Create.", res(f))
		}
	}
}

func res(path string) string { return filepath.Base(filepath.Dir(path)) }
