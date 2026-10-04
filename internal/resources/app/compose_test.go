// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestReconcileCompose(t *testing.T) {
	const liveJSON = `{"services":{"test":{"command":["sleep","infinity"],"environment":{"ENABLED":true,"LARGE":9007199254740993,"PORT":8080,"TOKEN":"synthetic-token"},"image":"busybox:1.37","scale":1.0}},"volumes":{"data":null}}`
	yaml := `# user comment
volumes:
  data: null
services:
  test:
    scale: 1
    image: 'busybox:1.37'
    command: [sleep, infinity]
    environment:
      TOKEN: synthetic-token
      PORT: 8080
      LARGE: 9007199254740993
      ENABLED: true
`
	for _, tc := range []struct {
		name, previous string
		unchanged      bool
	}{
		{"formatting and numeric representation", yaml, true},
		{"JSON", liveJSON, true},
		{"import", "", false},
		{"image drift", strings.Replace(yaml, "1.37", "1.36", 1), false},
		{"large integer drift", strings.Replace(yaml, "9007199254740993", "9007199254740992", 1), false},
		{"string versus boolean", strings.Replace(yaml, "ENABLED: true", `ENABLED: "true"`, 1), false},
		{"added key", strings.Replace(yaml, "      PORT: 8080\n", "", 1), false},
		{"deleted key", yaml + "      REMOVED: value\n", false},
		{"invalid old state", "services: [", false},
		{"extra document", yaml + "---\nservices: {}\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := reconcileCompose(tc.previous, json.RawMessage(liveJSON))
			if err != nil {
				t.Fatal(err)
			}
			if tc.unchanged {
				if got != tc.previous {
					t.Fatal("equivalent document lost its original formatting")
				}
			} else {
				expected := liveJSON
				if got != string(expected) {
					t.Fatal("changed or imported state did not contain the full live configuration")
				}
			}
		})
	}
}

func TestReconcileComposeInvalidResponse(t *testing.T) {
	for _, live := range []string{
		`null`, `{}`, `{"services": null}`, `{"services": []}`,
		`{"secret":`, `[]`, `"secret"`,
	} {
		if _, err := reconcileCompose("services: {}", json.RawMessage(live)); err == nil {
			t.Fatal("invalid API response must fail instead of retaining stale state silently")
		} else if strings.Contains(err.Error(), "secret") {
			t.Fatal("diagnostic exposed configuration contents")
		}
	}
}

func TestComposeJSON(t *testing.T) {
	for _, document := range []string{"", "null", "[]", "services: [secret", "services: {}\n---\n", "services: {test: {environment: {true: secret}}}"} {
		if _, err := composeJSON(document); err == nil {
			t.Errorf("expected invalid document to fail")
		} else if strings.Contains(err.Error(), "secret") {
			t.Fatal("parser diagnostic exposed contents")
		}
	}
	const aliases = "x-base: &base\n  image: busybox:1.37\nservices:\n  test:\n    <<: *base\n"
	got, err := composeJSON(aliases)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"services":{"test":{"image":"busybox:1.37"}},"x-base":{"image":"busybox:1.37"}}` {
		t.Fatal("YAML aliases were not resolved")
	}
}
