// Copyright TrueNAS 2026
// SPDX-License-Identifier: MPL-2.0

package normalize

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// PyYAML (YAML 1.1) implicit resolvers, copied from yaml/resolver.py.
var (
	yaml11Bool  = regexp.MustCompile(`^(?:yes|Yes|YES|no|No|NO|true|True|TRUE|false|False|FALSE|on|On|ON|off|Off|OFF)$`)
	yaml11True  = regexp.MustCompile(`^(?:yes|Yes|YES|true|True|TRUE|on|On|ON)$`)
	yaml11Null  = regexp.MustCompile(`^(?:~|null|Null|NULL|)$`)
	yaml11Int   = regexp.MustCompile(`^(?:[-+]?0b[0-1_]+|[-+]?0[0-7_]+|[-+]?(?:0|[1-9][0-9_]*)|[-+]?0x[0-9a-fA-F_]+|[-+]?[1-9][0-9_]*(?::[0-5]?[0-9])+)$`)
	yaml11Float = regexp.MustCompile(`^(?:[-+]?(?:[0-9][0-9_]*)\.[0-9_]*(?:[eE][-+][0-9]+)?|\.[0-9_]+(?:[eE][-+][0-9]+)?|[-+]?[0-9][0-9_]*(?::[0-5]?[0-9])+\.[0-9_]*|[-+]?\.(?:inf|Inf|INF)|\.(?:nan|NaN|NAN))$`)
)

// yaml11Budget bounds alias expansion so a hostile document ("billion
// laughs") cannot exhaust memory.
const yaml11Budget = 1_000_000

// DecodeYAML11 decodes a YAML document the way PyYAML's safe_load (YAML 1.1)
// does, which is how TrueNAS parses Compose files: unquoted yes/no/on/off
// are booleans, 0755 is octal (493), 1:30 is base 60 (90), 0x1F is hex,
// underscores in numbers are ignored, and "<<" merge keys are applied.
// Mapping keys are turned into strings the way Python's json.dumps does
// (true -> "true", 1 -> "1", null -> "null"). Integers and finite floats come
// back as json.Number, so the result can be compared with EqualTree against a
// DecodeJSON result and marshalled with encoding/json. Timestamps are left as
// strings (PyYAML would build a datetime).
func DecodeYAML11(s string) (any, error) {
	dec := yaml.NewDecoder(strings.NewReader(s))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, err
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, err
		}
		return nil, errors.New("expected a single YAML document, found more than one")
	}
	c := yaml11Converter{budget: yaml11Budget}
	return c.convert(&doc, 0)
}

type yaml11Converter struct{ budget int }

func (c *yaml11Converter) convert(n *yaml.Node, depth int) (any, error) {
	c.budget--
	if c.budget < 0 || depth > 1000 {
		return nil, errors.New("YAML document is too large or too deeply nested after alias expansion")
	}
	switch n.Kind {
	case yaml.DocumentNode:
		if len(n.Content) == 0 {
			return nil, nil
		}
		return c.convert(n.Content[0], depth+1)
	case yaml.AliasNode:
		if n.Alias == nil {
			return nil, fmt.Errorf("line %d: unknown alias", n.Line)
		}
		return c.convert(n.Alias, depth+1)
	case yaml.ScalarNode:
		return yaml11Scalar(n)
	case yaml.SequenceNode:
		out := make([]any, 0, len(n.Content))
		for _, item := range n.Content {
			v, err := c.convert(item, depth+1)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	case yaml.MappingNode:
		return c.mapping(n, depth)
	}
	return nil, fmt.Errorf("line %d: unsupported YAML node", n.Line)
}

func (c *yaml11Converter) mapping(n *yaml.Node, depth int) (any, error) {
	out := map[string]any{}
	var merges []*yaml.Node
	for i := 0; i+1 < len(n.Content); i += 2 {
		kn, vn := n.Content[i], n.Content[i+1]
		if kn.Kind == yaml.ScalarNode && kn.Value == "<<" && kn.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle) == 0 {
			merges = append(merges, vn)
			continue
		}
		key, err := c.key(kn, depth)
		if err != nil {
			return nil, err
		}
		v, err := c.convert(vn, depth+1)
		if err != nil {
			return nil, err
		}
		out[key] = v
	}
	// Explicit keys win over merged ones; among merged mappings the first one
	// listed wins (PyYAML flatten_mapping).
	for _, m := range merges {
		srcs := []*yaml.Node{m}
		if t := deref(m); t.Kind == yaml.SequenceNode {
			srcs = t.Content
		}
		for _, src := range srcs {
			v, err := c.convert(src, depth+1)
			if err != nil {
				return nil, err
			}
			mm, ok := v.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("line %d: a merge key (<<) needs a mapping or a list of mappings", m.Line)
			}
			for k, val := range mm {
				if _, exists := out[k]; !exists {
					out[k] = val
				}
			}
		}
	}
	return out, nil
}

func deref(n *yaml.Node) *yaml.Node {
	for n.Kind == yaml.AliasNode && n.Alias != nil {
		n = n.Alias
	}
	return n
}

// key converts a mapping key to the string json.dumps would write.
func (c *yaml11Converter) key(n *yaml.Node, depth int) (string, error) {
	v, err := c.convert(n, depth+1)
	if err != nil {
		return "", err
	}
	switch k := v.(type) {
	case string:
		return k, nil
	case bool:
		return strconv.FormatBool(k), nil
	case nil:
		return "null", nil
	case json.Number:
		return string(k), nil
	case float64:
		switch {
		case math.IsInf(k, 1):
			return "Infinity", nil
		case math.IsInf(k, -1):
			return "-Infinity", nil
		}
		return "NaN", nil
	}
	return "", fmt.Errorf("line %d: mapping keys must be scalars", n.Line)
}

func yaml11Scalar(n *yaml.Node) (any, error) {
	s := n.Value
	if n.Style&yaml.TaggedStyle != 0 {
		switch n.Tag {
		case "!!str", "!", "tag:yaml.org,2002:str":
			return s, nil
		case "!!int", "!!float", "!!bool", "!!null":
			v, err := yaml11Plain(s)
			if err != nil {
				return nil, err
			}
			if !yaml11TagMatches(n.Tag, v) {
				return nil, fmt.Errorf("line %d: %q is not a valid %s", n.Line, s, n.Tag)
			}
			return v, nil
		}
		return s, nil // other tags (!!binary, !!timestamp, custom): keep the text
	}
	if n.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle|yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		return s, nil
	}
	return yaml11Plain(s)
}

func yaml11TagMatches(tag string, v any) bool {
	switch n := v.(type) {
	case bool:
		return tag == "!!bool"
	case nil:
		return tag == "!!null"
	case json.Number:
		return tag == "!!float" || (tag == "!!int" && !strings.ContainsAny(string(n), ".eE"))
	case float64:
		return tag == "!!float"
	}
	return false
}

// yaml11Plain resolves an unquoted scalar with PyYAML's implicit resolvers.
func yaml11Plain(s string) (any, error) {
	switch {
	case yaml11Null.MatchString(s):
		return nil, nil
	case yaml11Bool.MatchString(s):
		return yaml11True.MatchString(s), nil
	case yaml11Int.MatchString(s):
		return yaml11ParseInt(s)
	case yaml11Float.MatchString(s):
		return yaml11ParseFloat(s)
	}
	return s, nil
}

func yaml11ParseInt(s string) (any, error) {
	v := strings.ReplaceAll(s, "_", "")
	neg := false
	if v != "" && (v[0] == '-' || v[0] == '+') {
		neg = v[0] == '-'
		v = v[1:]
	}
	n := new(big.Int)
	var ok bool
	switch {
	case v == "0":
		ok = true
	case strings.HasPrefix(v, "0b"):
		_, ok = n.SetString(v[2:], 2)
	case strings.HasPrefix(v, "0x"):
		_, ok = n.SetString(v[2:], 16)
	case strings.HasPrefix(v, "0"):
		_, ok = n.SetString(v[1:], 8)
	case strings.Contains(v, ":"):
		ok = true
		for _, d := range strings.Split(v, ":") {
			x, err := strconv.Atoi(d)
			if err != nil {
				ok = false
				break
			}
			n.Mul(n, big.NewInt(60)).Add(n, big.NewInt(int64(x)))
		}
	default:
		_, ok = n.SetString(v, 10)
	}
	if !ok {
		return nil, fmt.Errorf("%q is not a valid YAML 1.1 integer", s)
	}
	if neg {
		n.Neg(n)
	}
	return json.Number(n.String()), nil
}

func yaml11ParseFloat(s string) (any, error) {
	v := strings.ToLower(strings.ReplaceAll(s, "_", ""))
	sign := 1.0
	if v != "" && (v[0] == '-' || v[0] == '+') {
		if v[0] == '-' {
			sign = -1
		}
		v = v[1:]
	}
	switch {
	case v == ".inf":
		return math.Inf(int(sign)), nil
	case v == ".nan":
		return math.NaN(), nil
	case strings.Contains(v, ":"):
		parts := strings.Split(v, ":")
		total := 0.0
		for i, p := range parts {
			var x float64
			var err error
			if i == len(parts)-1 {
				x, err = strconv.ParseFloat(p, 64)
			} else {
				var xi int
				xi, err = strconv.Atoi(p)
				x = float64(xi)
			}
			if err != nil {
				return nil, fmt.Errorf("%q is not a valid YAML 1.1 float", s)
			}
			total = total*60 + x
		}
		return json.Number(strconv.FormatFloat(sign*total, 'g', -1, 64)), nil
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return nil, fmt.Errorf("%q is not a valid YAML 1.1 float", s)
	}
	return json.Number(strconv.FormatFloat(sign*f, 'g', -1, 64)), nil
}

// EqualYAML11 reports whether two YAML documents hold the same value under
// YAML 1.1 (PyYAML) rules. A side that is valid JSON is decoded as JSON, so a
// server document stored as JSON compares cleanly against the user's YAML.
// A document that does not parse is equal only to the same text.
func EqualYAML11(a, b string) bool {
	if a == b {
		return true
	}
	x, err1 := decodeYAMLOrJSON(a)
	y, err2 := decodeYAMLOrJSON(b)
	return err1 == nil && err2 == nil && EqualTree(x, y, false)
}

func decodeYAMLOrJSON(s string) (any, error) {
	if v, err := DecodeJSON(s); err == nil {
		return v, nil
	}
	return DecodeYAML11(s)
}

type yamlKind struct{}

func (yamlKind) name() string                 { return "YAML" }
func (yamlKind) equal(prior, got string) bool { return EqualYAML11(prior, got) }
func (yamlKind) validate(s string) error {
	if _, err := decodeYAMLOrJSON(s); err != nil {
		return fmt.Errorf("not a valid YAML document: %v", err)
	}
	return nil
}
