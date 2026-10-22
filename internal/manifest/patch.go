package manifest

import (
	"bytes"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// MissingFields returns the dotted YAML keys (e.g. "pullRequest.title.maxLength")
// that are empty in existing and set in fresh, in schema order. Map entries
// are reported per key ("quality.lint"). The tija section is Tija's own
// bookkeeping and is never reported.
func MissingFields(existing, fresh Manifest) []string {
	var out []string
	missing(reflect.ValueOf(existing), reflect.ValueOf(fresh), "", &out)
	return out
}

func missing(e, f reflect.Value, prefix string, out *[]string) {
	switch f.Kind() {
	case reflect.Struct:
		t := f.Type()
		for i := range t.NumField() {
			key, _, _ := strings.Cut(t.Field(i).Tag.Get("yaml"), ",")
			if prefix == "" && key == "tija" {
				continue
			}
			missing(e.Field(i), f.Field(i), join(prefix, key), out)
		}
	case reflect.Map:
		keys := f.MapKeys()
		slices.SortFunc(keys, func(a, b reflect.Value) int { return strings.Compare(a.String(), b.String()) })
		for _, k := range keys {
			fv := f.MapIndex(k)
			if fv.IsZero() {
				continue
			}
			ev := e.MapIndex(k)
			if !ev.IsValid() {
				*out = append(*out, join(prefix, k.String()))
				continue
			}
			missing(ev, fv, join(prefix, k.String()), out)
		}
	default:
		if e.IsZero() && !f.IsZero() {
			*out = append(*out, prefix)
		}
	}
}

func join(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

// FillMissing sets each dotted field to its value in fresh inside the YAML
// document data, keeping every other key, value and comment untouched.
func FillMissing(data []byte, fresh Manifest, fields []string) ([]byte, error) {
	var src yaml.Node
	if err := src.Encode(fresh); err != nil {
		return nil, err
	}
	return patch(data, func(root *yaml.Node) {
		for _, f := range fields {
			path := strings.Split(f, ".")
			if v := lookup(&src, path); v != nil {
				set(root, path, v)
			}
		}
	})
}

// RecordInstalled sets tija.defaultsVersion and the given tija.installed
// entries inside the YAML document data, keeping everything else.
func RecordInstalled(data []byte, defaultsVersion string, installed map[string]string) ([]byte, error) {
	paths := make([]string, 0, len(installed))
	for p := range installed {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	return patch(data, func(root *yaml.Node) {
		set(root, []string{"tija", "defaultsVersion"}, str(defaultsVersion))
		for _, p := range paths {
			set(root, []string{"tija", "installed", p}, str(installed[p]))
		}
	})
}

func str(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
}

// patch decodes data into a node tree, lets edit modify the root mapping and
// re-encodes it, preserving CRLF line endings when data used them.
func patch(data []byte, edit func(root *yaml.Node)) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", Path, err)
	}
	if doc.Kind == 0 {
		doc = yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s: top level is not a mapping", Path)
	}
	edit(root)
	out, err := encode(&doc)
	if err != nil {
		return nil, err
	}
	if bytes.Contains(data, []byte("\r\n")) {
		out = bytes.ReplaceAll(out, []byte("\n"), []byte("\r\n"))
	}
	return out, nil
}

// lookup returns the value node at path inside mapping m, or nil.
func lookup(m *yaml.Node, path []string) *yaml.Node {
	for _, key := range path {
		if m.Kind != yaml.MappingNode {
			return nil
		}
		next := (*yaml.Node)(nil)
		for i := 0; i+1 < len(m.Content); i += 2 {
			if m.Content[i].Value == key {
				next = m.Content[i+1]
				break
			}
		}
		if next == nil {
			return nil
		}
		m = next
	}
	return m
}

// set stores v at path inside mapping m, creating intermediate mappings and
// replacing non-mapping intermediates (e.g. null).
func set(m *yaml.Node, path []string, v *yaml.Node) {
	for depth, key := range path {
		var val *yaml.Node
		for i := 0; i+1 < len(m.Content); i += 2 {
			if m.Content[i].Value == key {
				val = m.Content[i+1]
				if depth == len(path)-1 {
					m.Content[i+1] = v
					return
				}
				if val.Kind != yaml.MappingNode {
					val = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
					m.Content[i+1] = val
				}
				break
			}
		}
		if val == nil {
			if len(m.Content) == 0 {
				m.Style = 0 // an empty flow mapping ({}) becomes block style
			}
			k := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
			if depth == len(path)-1 {
				m.Content = append(m.Content, k, v)
				return
			}
			val = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			m.Content = append(m.Content, k, val)
		}
		m = val
	}
}
