// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package normalize

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
)

// DecodeJSON decodes a JSON document with numbers kept as json.Number, so
// large integers are not rounded through float64.
func DecodeJSON(s string) (any, error) {
	dec := json.NewDecoder(bytes.NewReader([]byte(s)))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("unexpected data after the JSON value")
	}
	return v, nil
}

// EqualJSON reports whether two JSON documents hold the same value: key order
// and whitespace are ignored and numbers compare by value (1 == 1.0). A
// document that does not parse is equal only to the same text.
func EqualJSON(a, b string) bool {
	if a == b {
		return true
	}
	x, err1 := DecodeJSON(a)
	y, err2 := DecodeJSON(b)
	return err1 == nil && err2 == nil && EqualTree(x, y, false)
}

// JSONSubsetOf reports whether every value in the configured document is
// present and equal in the server document, ignoring keys the server added
// (defaults, nulls). Objects recurse key by key; arrays must have the same
// length and compare item by item with the same rule (so defaults the server
// fills into list items are ignored too); scalars compare by value. A
// configured null matches a missing or null server key.
func JSONSubsetOf(configured, server string) bool {
	if configured == server {
		return true
	}
	x, err1 := DecodeJSON(configured)
	y, err2 := DecodeJSON(server)
	return err1 == nil && err2 == nil && EqualTree(x, y, true)
}

// EqualTree compares two decoded documents (map[string]any, []any, string,
// bool, nil, json.Number, float64, int types). With subset set, keys present
// only in got are ignored and a nil in want matches a missing key.
func EqualTree(want, got any, subset bool) bool {
	return EqualTreeFunc(want, got, subset, nil)
}

// LeafEqual decides equality for one pair of non-container values found under
// object key key (for list items, the key of the list). Return handled=false
// to fall back to the default comparison.
type LeafEqual func(key string, want, got any) (equal, handled bool)

// KeyEqual returns a LeafEqual that compares string values under the named
// keys with the given function, for example
// KeyEqual(map[string]func(a, b string) bool{"mac": EqualMAC}).
func KeyEqual(funcs map[string]func(a, b string) bool) LeafEqual {
	return func(key string, want, got any) (bool, bool) {
		f, ok := funcs[key]
		if !ok {
			return false, false
		}
		ws, ok1 := want.(string)
		gs, ok2 := got.(string)
		if !ok1 || !ok2 {
			return false, false
		}
		return f(ws, gs), true
	}
}

// EqualTreeFunc is EqualTree with a per-key hook for leaf values (nil for
// none). Use it for a JSON blob whose keys hold server-normalized values,
// such as a NIC "mac" inside vm_device attributes.
func EqualTreeFunc(want, got any, subset bool, leaf LeafEqual) bool {
	return equalTree("", want, got, subset, leaf)
}

func equalTree(key string, want, got any, subset bool, leaf LeafEqual) bool {
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return false
		}
		if !subset && len(w) != len(g) {
			return false
		}
		for k, wv := range w {
			gv, present := g[k]
			if !present {
				if subset && wv == nil {
					continue
				}
				return false
			}
			if !equalTree(k, wv, gv, subset, leaf) {
				return false
			}
		}
		return true
	case []any:
		g, ok := got.([]any)
		if !ok || len(w) != len(g) {
			return false
		}
		for i := range w {
			if !equalTree(key, w[i], g[i], subset, leaf) {
				return false
			}
		}
		return true
	}
	if leaf != nil {
		if eq, handled := leaf(key, want, got); handled {
			return eq
		}
	}
	switch w := want.(type) {
	case nil:
		return got == nil
	case string:
		g, ok := got.(string)
		return ok && w == g
	case bool:
		g, ok := got.(bool)
		return ok && w == g
	}
	x, ok1 := toBigFloat(want)
	y, ok2 := toBigFloat(got)
	return ok1 && ok2 && x.Cmp(y) == 0
}

func toBigFloat(v any) (*big.Float, bool) {
	f := new(big.Float).SetPrec(512)
	switch n := v.(type) {
	case json.Number:
		if _, ok := f.SetString(string(n)); !ok {
			return nil, false
		}
		return f, true
	case float64:
		if n != n { // NaN never compares equal
			return nil, false
		}
		return f.SetFloat64(n), true
	case int:
		return f.SetInt64(int64(n)), true
	case int64:
		return f.SetInt64(n), true
	case uint64:
		return f.SetUint64(n), true
	case *big.Int:
		return f.SetInt(n), true
	}
	return nil, false
}

type jsonKind struct{}

func (jsonKind) name() string                 { return "JSON" }
func (jsonKind) equal(prior, got string) bool { return EqualJSON(prior, got) }
func (jsonKind) validate(s string) error      { return validateJSON(s) }

type jsonSubsetKind struct{}

func (jsonSubsetKind) name() string                 { return "JSONSubset" }
func (jsonSubsetKind) equal(prior, got string) bool { return JSONSubsetOf(prior, got) }
func (jsonSubsetKind) validate(s string) error      { return validateJSON(s) }

func validateJSON(s string) error {
	if _, err := DecodeJSON(s); err != nil {
		return fmt.Errorf("not a valid JSON document: %v", err)
	}
	return nil
}
