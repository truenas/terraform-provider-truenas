// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package app

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/truenas/terraform-provider-truenas/internal/normalize"
	yaml "gopkg.in/yaml.v3"
)

// reconcileComposeConfig compares the user's Compose YAML against the live
// Compose document for a custom app. For a custom app, app.config returns the
// Compose document verbatim as an object (no chart defaults or ix_* keys), so
// unlike catalog `values` no projection is needed — the whole document is the
// user's, and additions/removals made out of band are real drift. The
// comparison is semantic (both sides canonicalized to sorted JSON), so
// formatting, comments, key order, and YAML-vs-JSON number spelling are not
// treated as drift (#34).
//
// It returns the string to store in state, whether the two are equivalent, and
// an error. When equivalent, the user's original YAML is preserved (no spurious
// diff). When they differ — real drift, or an initial populate on import where
// the prior string is empty — the live document is returned as YAML so the diff
// is readable. An unparseable user string is treated as "not equivalent" so the
// live value replaces it. A non-error nil is returned only on success; the error
// carries a generic message and never Compose contents (they may hold secrets).
func reconcileComposeConfig(previous string, liveRaw json.RawMessage) (string, bool, error) {
	// Decode with UseNumber so large integers keep exact precision instead of
	// being rounded through float64, which would otherwise conceal or invent
	// drift on big numeric values.
	live, err := decodeJSONExact(liveRaw)
	if err != nil {
		return "", false, errors.New("app.config did not return a valid Compose object; the previous Compose state has been preserved")
	}
	if m, ok := live.(map[string]any); !ok {
		return "", false, errors.New("app.config did not return a Compose object; the previous Compose state has been preserved")
	} else if _, ok := m["services"].(map[string]any); !ok {
		return "", false, errors.New("app.config did not return a Compose object with a services mapping; the previous Compose state has been preserved")
	}

	liveCanon, err := canonicalJSON(live)
	if err != nil {
		return "", false, errors.New("could not encode the Compose configuration returned by app.config; the previous Compose state has been preserved")
	}

	if userVal, perr := decodeYAMLExact(previous); perr == nil && normalize.EqualTree(userVal, live, false) {
		return previous, true, nil // equivalent: preserve the user's exact YAML
	}

	// Different (or prior unparseable / empty): emit the live document as YAML.
	// Serialize from a plainly-decoded copy so json.Number values render as
	// numbers, not quoted strings.
	var display any
	if json.Unmarshal([]byte(liveCanon), &display) == nil {
		if out, merr := yaml.Marshal(display); merr == nil {
			return string(out), false, nil
		}
	}
	return liveCanon, false, nil // fall back to canonical JSON (valid YAML)
}

// mergeComposeOverlay deep-merges the write-only secret overlay (JSON or YAML)
// into the base Compose YAML and returns the merged document as YAML to send to
// the API. Overlay leaves override/add to the base (e.g. a nested
// services.<svc>.environment.<KEY> secret). The overlay is never stored. (#34)
func mergeComposeOverlay(baseYAML, overlay string) (string, error) {
	if baseYAML == "" {
		baseYAML = "{}"
	}
	base, err := decodeYAMLExact(baseYAML)
	if err != nil {
		return "", err
	}
	ov, err := decodeYAMLExact(overlay) // JSON is valid YAML, so this accepts both
	if err != nil {
		return "", err
	}
	bm, ok1 := base.(map[string]any)
	om, ok2 := ov.(map[string]any)
	if !ok1 || !ok2 {
		return baseYAML, nil // non-object base/overlay: nothing sensible to merge
	}
	merged := deepMerge(bm, om)
	out, err := yaml.Marshal(yamlNumbers(merged))
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// deepMerge recursively merges overlay into base (overlay wins at the leaves;
// nested maps are merged, not replaced). Inputs are not mutated beyond base.
func deepMerge(base, overlay map[string]any) map[string]any {
	for k, ov := range overlay {
		if bv, ok := base[k]; ok {
			if bm, ok1 := bv.(map[string]any); ok1 {
				if om, ok2 := ov.(map[string]any); ok2 {
					base[k] = deepMerge(bm, om)
					continue
				}
			}
		}
		base[k] = ov
	}
	return base
}

// reconcileComposeProjected is the write-only variant of reconcileComposeConfig:
// the live Compose is projected onto only the keys present in the base string,
// so the overlay's secret keys (present live, absent from the base) are never
// absorbed into state. Drift is detected only on the base (non-secret) keys.
func reconcileComposeProjected(previous string, liveRaw json.RawMessage) (string, bool, error) {
	live, err := decodeJSONExact(liveRaw)
	if err != nil {
		return "", false, errors.New("app.config did not return a valid Compose object; the previous Compose state has been preserved")
	}
	liveMap, ok := live.(map[string]any)
	if !ok {
		return "", false, errors.New("app.config did not return a Compose object; the previous Compose state has been preserved")
	}
	base, err := decodeYAMLExact(previous)
	if err != nil {
		return previous, true, nil // unparseable base: keep as-is
	}
	baseMap, ok := base.(map[string]any)
	if !ok {
		return previous, true, nil
	}
	projected := projectComposeKeys(liveMap, baseMap)
	if normalize.EqualTree(baseMap, projected, false) {
		return previous, true, nil // base keys unchanged: preserve the user's YAML
	}
	out, err := yaml.Marshal(yamlNumbers(projected))
	if err != nil {
		return previous, false, nil
	}
	return string(out), false, nil
}

// projectComposeKeys keeps, from live, only the keys present in base (recursing
// into nested maps), so secret keys the user put only in the write-only overlay
// are dropped and never reach state.
func projectComposeKeys(live, base map[string]any) map[string]any {
	out := map[string]any{}
	for k, bv := range base {
		lv, ok := live[k]
		if !ok {
			continue
		}
		if bm, ok1 := bv.(map[string]any); ok1 {
			if lm, ok2 := lv.(map[string]any); ok2 {
				out[k] = projectComposeKeys(lm, bm)
				continue
			}
		}
		out[k] = lv
	}
	return out
}

// decodeJSONExact decodes JSON into a generic value, keeping numbers as
// json.Number (no float64 rounding).
func decodeJSONExact(raw []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// decodeYAMLExact decodes exactly one YAML document into a generic value,
// normalized to JSON-compatible types. TrueNAS parses Compose with PyYAML
// (YAML 1.1: unquoted yes/off are booleans, 0755 is octal, 1:30 is 90), so the
// user's document is decoded with the same rules; with YAML 1.2 rules a
// Compose using them never compared equal to app.config (B-097).
func decodeYAMLExact(s string) (any, error) {
	v, err := normalize.DecodeYAML11(s)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, errors.New("empty document")
	}
	return v, nil
}

// canonicalJSON marshals a value to JSON with map keys sorted (encoding/json
// sorts them), giving a stable canonical form. json.Number marshals to its
// exact token, so integer precision is preserved.
func canonicalJSON(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// yamlNumbers converts json.Number values (from the YAML 1.1 and JSON
// decoders) to int64 or float64, recursively, so yaml.Marshal writes them as
// numbers instead of quoted strings.
func yamlNumbers(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			t[k] = yamlNumbers(val)
		}
	case []any:
		for i, val := range t {
			t[i] = yamlNumbers(val)
		}
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i
		}
		if f, err := t.Float64(); err == nil {
			return f
		}
	}
	return v
}
