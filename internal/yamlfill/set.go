package yamlfill

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Set returns src with the scalar at path (mapping keys from the top) replaced by value, and
// nothing else changed: the bytes around it (comments, layout, the other values) stay. The key
// must be there (Fill adds it); a value that is a block scalar, a collection or spans lines is
// an error. value is written plain when YAML reads it back as the same string, else quoted.
func Set(src []byte, path []string, value string) ([]byte, error) {
	return set(src, path, value, false)
}

// SetPlain is Set with value written as it is, for a scalar of another type than a string (10,
// none, true): letters, digits and . _ - only.
func SetPlain(src []byte, path []string, value string) ([]byte, error) {
	if value == "" || strings.Trim(value, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789._-") != "" {
		return nil, fmt.Errorf("%q is not a plain scalar", value)
	}
	return set(src, path, value, true)
}

func set(src []byte, path []string, value string, plain bool) ([]byte, error) {
	if len(path) == 0 {
		return nil, errors.New("no path")
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("not a mapping")
	}
	f := &filler{src: src, lines: []int{0}}
	for i, c := range src {
		if c == '\n' {
			f.lines = append(f.lines, i+1)
		}
	}
	m, key := doc.Content[0], (*yaml.Node)(nil)
	var v *yaml.Node
	inFlow := false
	for i, k := range path {
		if m.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("%s is not a mapping", strings.Join(path[:i], "."))
		}
		inFlow = inFlow || m.Style&yaml.FlowStyle != 0
		key, v = nil, nil
		for j := 0; j+1 < len(m.Content); j += 2 {
			if m.Content[j].Value == k {
				key, v = m.Content[j], m.Content[j+1]
			}
		}
		if v == nil {
			return nil, fmt.Errorf("no %s", strings.Join(path[:i+1], "."))
		}
		m = v
	}
	p := strings.Join(path, ".")
	text := value
	if !plain {
		var err error
		if text, err = scalarText(value, inFlow); err != nil {
			return nil, err
		}
	}
	if v.Kind != yaml.ScalarNode || v.Style&(yaml.LiteralStyle|yaml.FoldedStyle) != 0 {
		return nil, fmt.Errorf("%s is not a one-line value", p)
	}
	var start, end int
	switch {
	case v.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle) != 0:
		start, end = f.offset(v.Line, v.Column), f.closingQuote(v)+1
	case v.Tag == "!!null" && v.Value == "":
		// `key:` with nothing after it: the value goes after the colon
		start = strings.IndexByte(string(src[f.offset(key.Line, key.Column):]), ':')
		if start < 0 {
			return nil, fmt.Errorf("%s: no colon", p)
		}
		start += f.offset(key.Line, key.Column) + 1
		end, text = start, " "+text
	default:
		start = f.offset(v.Line, v.Column)
		end = start + len(v.Value)
		if end > len(src) || string(src[start:end]) != v.Value {
			return nil, fmt.Errorf("%s is not a one-line value", p)
		}
	}
	if end > len(src) || strings.ContainsRune(string(src[start:end]), '\n') {
		return nil, fmt.Errorf("%s is not a one-line value", p)
	}
	out := make([]byte, 0, len(src)+len(text))
	out = append(append(append(out, src[:start]...), text...), src[end:]...)
	return out, nil
}

// scalarText is value as YAML text on one line: plain when it reads back as the same string (in a
// flow collection, also when it has none of the flow indicators), else double-quoted.
func scalarText(value string, inFlow bool) (string, error) {
	if strings.ContainsAny(value, "\n\r") {
		return "", errors.New("a value on one line")
	}
	n := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
	if inFlow && strings.ContainsAny(value, ",[]{}") {
		n.Style = yaml.DoubleQuotedStyle
	}
	b, err := yaml.Marshal(n)
	if err != nil {
		return "", err
	}
	text := strings.TrimSuffix(string(b), "\n")
	if strings.Contains(text, "\n") {
		return "", errors.New("a value on one line")
	}
	return text, nil
}
