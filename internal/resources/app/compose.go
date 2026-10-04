// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

// reconcileCompose preserves the user's exact YAML when the live document is
// equivalent, so comments, key order, and whitespace do not cause refresh drift.
// Custom apps own the whole document: unlike catalog values, new and deleted
// keys must be retained to detect out-of-band additions and removals as well.
// JSON is valid YAML and gives imports and drift a stable representation.
func reconcileCompose(previous string, raw json.RawMessage) (string, error) {
	// Decode numbers without float64 rounding so large numeric values cannot
	// lose precision or conceal drift. Keep catalog values decoding unchanged.
	var live map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&live); err != nil {
		return "", errors.New("app.config did not return a valid Compose object; the previous Compose state has been preserved")
	}
	if _, ok := live["services"].(map[string]any); !ok {
		return "", errors.New("app.config did not return a Compose object with a services mapping; the previous Compose state has been preserved")
	}
	current, err := json.Marshal(live)
	if err != nil {
		return "", errors.New("could not encode the Compose configuration returned by app.config; the previous Compose state has been preserved")
	}
	if old, err := composeJSON(previous); err == nil {
		// Normalize both encodings alike (e.g. JSON 1.0 and YAML 1).
		if normalized, err := composeJSON(string(current)); err == nil && bytes.Equal(old, normalized) {
			return previous, nil
		}
	}
	return string(current), nil
}

// composeJSON decodes exactly one YAML document. Errors intentionally omit
// parser details, which can contain environment values or other credentials.
func composeJSON(document string) ([]byte, error) {
	decoder := yaml.NewDecoder(strings.NewReader(document))
	var value map[string]any
	if err := decoder.Decode(&value); err != nil || value == nil {
		return nil, errors.New("invalid Compose YAML object")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, errors.New("expected one Compose YAML document")
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, errors.New("compose YAML cannot be represented as JSON")
	}
	return encoded, nil
}
