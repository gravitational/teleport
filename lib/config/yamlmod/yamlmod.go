/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package yamlmod

import (
	"bytes"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/gravitational/trace"
	"gopkg.in/yaml.v3"
)

// segment represents one piece of a dot-path, e.g. "apps[0]" → key="apps", index=0.
type segment struct {
	key   string
	index int // -1 means no array index
}

var indexRegex = regexp.MustCompile(`^(.+)\[(\d+)\]$`)

func parsePath(path string) ([]segment, error) {
	parts := strings.Split(path, ".")
	segments := make([]segment, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			return nil, trace.BadParameter("empty segment in path %q", path)
		}
		if m := indexRegex.FindStringSubmatch(p); m != nil {
			idx, _ := strconv.Atoi(m[2])
			segments = append(segments, segment{key: m[1], index: idx})
		} else {
			segments = append(segments, segment{key: p, index: -1})
		}
	}
	return segments, nil
}

// Parse reads YAML bytes into a *yaml.Node document node. The document root must
// be a mapping, since every operation in this package addresses fields by key.
func Parse(data []byte) (*yaml.Node, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, trace.Wrap(err, "parsing YAML")
	}
	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, trace.BadParameter("expected a YAML document")
	}
	if doc.Content[0].Kind != yaml.MappingNode {
		return nil, trace.BadParameter("expected a YAML mapping at the document root")
	}
	return &doc, nil
}

// Render serializes a *yaml.Node document back to YAML bytes.
func Render(doc *yaml.Node) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, trace.Wrap(err, "rendering YAML")
	}
	if err := enc.Close(); err != nil {
		return nil, trace.Wrap(err, "closing YAML encoder")
	}
	return buf.Bytes(), nil
}

// resolveSegment finds the value node for a given segment within a mapping node.
func resolveSegment(node *yaml.Node, seg segment) (*yaml.Node, error) {
	if node.Kind != yaml.MappingNode {
		return nil, trace.BadParameter("expected mapping node, got kind %d", node.Kind)
	}
	for i := 0; i < len(node.Content)-1; i += 2 {
		if node.Content[i].Value == seg.key {
			val := node.Content[i+1]
			if seg.index >= 0 {
				if val.Kind != yaml.SequenceNode {
					return nil, trace.BadParameter("expected sequence at %q, got kind %d", seg.key, val.Kind)
				}
				if seg.index >= len(val.Content) {
					return nil, trace.BadParameter("index %d out of range for %q (len %d)", seg.index, seg.key, len(val.Content))
				}
				return val.Content[seg.index], nil
			}
			return val, nil
		}
	}
	return nil, trace.NotFound("key %q not found", seg.key)
}

// findKeyIndex returns the index of the key node in a mapping's Content slice.
// Returns -1 if not found.
func findKeyIndex(mapping *yaml.Node, key string) int {
	for i := 0; i < len(mapping.Content)-1; i += 2 {
		if mapping.Content[i].Value == key {
			return i
		}
	}
	return -1
}

// ensureMapping ensures that the given segment exists as a mapping under the
// parent. Creates it if missing. Returns the mapping node for the segment or
// an error if the existing node structure is incompatible.
func ensureMapping(parent *yaml.Node, seg segment) (*yaml.Node, error) {
	idx := findKeyIndex(parent, seg.key)
	if idx >= 0 {
		val := parent.Content[idx+1]
		if seg.index >= 0 {
			if val.Kind != yaml.SequenceNode {
				return nil, trace.BadParameter("expected sequence at %q, got kind %d", seg.key, val.Kind)
			}
			if seg.index >= len(val.Content) {
				return nil, trace.BadParameter("index %d out of range for %q (len %d)", seg.index, seg.key, len(val.Content))
			}
			return val.Content[seg.index], nil
		}
		return val, nil
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: seg.key, Tag: "!!str"}
	valNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	parent.Content = append(parent.Content, keyNode, valNode)
	if seg.index >= 0 {
		seqNode := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		mapNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		seqNode.Content = append(seqNode.Content, mapNode)
		parent.Content[len(parent.Content)-1] = seqNode
		return mapNode, nil
	}
	return valNode, nil
}

// navigateToParent navigates the document tree along the given path segments,
// creating intermediate mapping nodes as needed. Returns the parent node and
// the final segment that should be set/modified.
func navigateToParent(doc *yaml.Node, segs []segment) (*yaml.Node, segment, error) {
	current := doc.Content[0]
	for i, seg := range segs[:len(segs)-1] {
		// current must be a mapping to look up or create the next key. Without
		// this guard, ensureMapping/findKeyIndex would silently append children
		// to a scalar or sequence node, corrupting the document.
		if current.Kind != yaml.MappingNode {
			return nil, segment{}, trace.BadParameter("cannot descend into non-mapping value before segment %d (%q)", i, seg.key)
		}
		if seg.index >= 0 {
			idx := findKeyIndex(current, seg.key)
			if idx < 0 {
				node, err := ensureMapping(current, seg)
				if err != nil {
					return nil, segment{}, trace.Wrap(err, "at segment %d (%q)", i, seg.key)
				}
				current = node
			} else {
				val := current.Content[idx+1]
				if val.Kind != yaml.SequenceNode {
					return nil, segment{}, trace.BadParameter("expected sequence at segment %d (%q)", i, seg.key)
				}
				if seg.index >= len(val.Content) {
					return nil, segment{}, trace.BadParameter("index %d out of range at segment %d (%q)", seg.index, i, seg.key)
				}
				current = val.Content[seg.index]
			}
		} else {
			node, err := ensureMapping(current, seg)
			if err != nil {
				return nil, segment{}, trace.Wrap(err, "at segment %d (%q)", i, seg.key)
			}
			current = node
		}
	}
	// The final parent must be a mapping so the value can be set under it.
	if current.Kind != yaml.MappingNode {
		return nil, segment{}, trace.BadParameter("cannot set value under non-mapping parent")
	}
	return current, segs[len(segs)-1], nil
}

// Set sets a scalar string value at the given dot-path.
// Creates intermediate mapping nodes if they don't exist.
func Set(doc *yaml.Node, path string, value string) error {
	segs, err := parsePath(path)
	if err != nil {
		return trace.Wrap(err)
	}

	current, final, err := navigateToParent(doc, segs)
	if err != nil {
		return trace.Wrap(err)
	}

	if final.index >= 0 {
		return trace.BadParameter("cannot set a value using array index on final segment; use the full path to the scalar")
	}

	idx := findKeyIndex(current, final.key)
	if idx >= 0 {
		current.Content[idx+1].Value = value
		current.Content[idx+1].Tag = "!!str"
		current.Content[idx+1].Kind = yaml.ScalarNode
	} else {
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: final.key, Tag: "!!str"}
		valNode := &yaml.Node{Kind: yaml.ScalarNode, Value: value, Tag: "!!str"}
		current.Content = append(current.Content, keyNode, valNode)
	}

	return nil
}

// SetMap sets a mapping value at the given dot-path from a map of key-value pairs.
// Creates intermediate mapping nodes if they don't exist.
// Replaces any existing value at the path with the new mapping.
func SetMap(doc *yaml.Node, path string, values map[string]string) error {
	segs, err := parsePath(path)
	if err != nil {
		return trace.Wrap(err)
	}

	current, final, err := navigateToParent(doc, segs)
	if err != nil {
		return trace.Wrap(err)
	}

	if final.index >= 0 {
		return trace.BadParameter("cannot set a map using array index on final segment")
	}

	mapNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, k := range slices.Sorted(maps.Keys(values)) {
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: k, Tag: "!!str"}
		valNode := &yaml.Node{Kind: yaml.ScalarNode, Value: values[k], Tag: "!!str"}
		mapNode.Content = append(mapNode.Content, keyNode, valNode)
	}

	idx := findKeyIndex(current, final.key)
	if idx >= 0 {
		current.Content[idx+1] = mapNode
	} else {
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: final.key, Tag: "!!str"}
		current.Content = append(current.Content, keyNode, mapNode)
	}

	return nil
}

// Get retrieves the scalar value at the given dot-path.
func Get(doc *yaml.Node, path string) (string, error) {
	segs, err := parsePath(path)
	if err != nil {
		return "", trace.Wrap(err)
	}

	current := doc.Content[0]
	for _, seg := range segs {
		next, err := resolveSegment(current, seg)
		if err != nil {
			return "", trace.Wrap(err)
		}
		current = next
	}

	if current.Kind != yaml.ScalarNode {
		return "", trace.BadParameter("path %q does not resolve to a scalar (kind %d)", path, current.Kind)
	}
	return current.Value, nil
}

// Exists returns true if the given dot-path exists in the tree.
func Exists(doc *yaml.Node, path string) bool {
	segs, err := parsePath(path)
	if err != nil {
		return false
	}

	current := doc.Content[0]
	for _, seg := range segs {
		next, err := resolveSegment(current, seg)
		if err != nil {
			return false
		}
		current = next
	}
	return true
}

// Delete removes the key (or array element) at the given dot-path.
func Delete(doc *yaml.Node, path string) error {
	segs, err := parsePath(path)
	if err != nil {
		return trace.Wrap(err)
	}

	var parent *yaml.Node
	var finalSeg segment

	if len(segs) == 1 {
		parent = doc.Content[0]
		finalSeg = segs[0]
	} else {
		parent = doc.Content[0]
		for _, seg := range segs[:len(segs)-1] {
			next, err := resolveSegment(parent, seg)
			if err != nil {
				return trace.Wrap(err)
			}
			parent = next
		}
		finalSeg = segs[len(segs)-1]
	}

	if finalSeg.index >= 0 {
		if parent.Kind != yaml.MappingNode {
			return trace.BadParameter("parent of %q is not a mapping", path)
		}
		idx := findKeyIndex(parent, finalSeg.key)
		if idx < 0 {
			return trace.NotFound("path %q not found", path)
		}
		seqNode := parent.Content[idx+1]
		if seqNode.Kind != yaml.SequenceNode {
			return trace.BadParameter("expected sequence at %q, got kind %d", finalSeg.key, seqNode.Kind)
		}
		if finalSeg.index >= len(seqNode.Content) {
			return trace.BadParameter("index %d out of range for %q (len %d)", finalSeg.index, finalSeg.key, len(seqNode.Content))
		}
		seqNode.Content = append(seqNode.Content[:finalSeg.index], seqNode.Content[finalSeg.index+1:]...)
		return nil
	}

	if parent.Kind != yaml.MappingNode {
		return trace.BadParameter("parent of %q is not a mapping", path)
	}

	// Remove every occurrence of the key, not just the first: a (malformed)
	// document may carry duplicate keys, and a delete should leave none behind.
	found := false
	for {
		idx := findKeyIndex(parent, finalSeg.key)
		if idx < 0 {
			break
		}
		parent.Content = append(parent.Content[:idx], parent.Content[idx+2:]...)
		found = true
	}
	if !found {
		return trace.NotFound("path %q not found", path)
	}
	return nil
}

// Merge inserts a top-level key with the provided subtree as its value.
// The src should be a parsed document whose root content becomes the value.
// No-op if the key already exists in dst.
func Merge(dst *yaml.Node, key string, src *yaml.Node) error {
	root := dst.Content[0]
	if root.Kind != yaml.MappingNode {
		return trace.BadParameter("destination root is not a mapping")
	}

	if findKeyIndex(root, key) >= 0 {
		return nil
	}

	srcRoot := src.Content[0]
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: key, Tag: "!!str"}
	root.Content = append(root.Content, keyNode, srcRoot)
	return nil
}

// SetBool sets a quoted "yes" or "no" string at the given dot-path, matching Teleport's config convention for enabled/disabled flags (not a YAML boolean).
func SetBool(doc *yaml.Node, path string, value bool) error {
	v := "no"
	if value {
		v = "yes"
	}
	segs, err := parsePath(path)
	if err != nil {
		return trace.Wrap(err)
	}

	current, final, err := navigateToParent(doc, segs)
	if err != nil {
		return trace.Wrap(err)
	}

	idx := findKeyIndex(current, final.key)
	if idx >= 0 {
		current.Content[idx+1].Value = v
		current.Content[idx+1].Tag = "!!str"
		current.Content[idx+1].Kind = yaml.ScalarNode
		current.Content[idx+1].Style = yaml.DoubleQuotedStyle
	} else {
		keyNode := &yaml.Node{Kind: yaml.ScalarNode, Value: final.key, Tag: "!!str"}
		valNode := &yaml.Node{Kind: yaml.ScalarNode, Value: v, Tag: "!!str", Style: yaml.DoubleQuotedStyle}
		current.Content = append(current.Content, keyNode, valNode)
	}

	return nil
}
