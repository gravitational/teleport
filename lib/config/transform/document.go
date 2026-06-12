/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

// Package transform provides YAML-node based Teleport config transforms.
package transform

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/gravitational/trace"
	"github.com/pmezard/go-difflib/difflib"
	"gopkg.in/yaml.v3"
)

// Document is a Teleport YAML config document.
type Document struct {
	node *yaml.Node
}

// RedactMode determines how a matched node is redacted.
type RedactMode int

const (
	// RedactFull replaces the full scalar value with "<redacted>".
	RedactFull RedactMode = iota
)

// RedactRule describes a scalar path to redact.
type RedactRule struct {
	Path []string
	Mode RedactMode
}

// Load parses raw YAML bytes into a node document.
func Load(raw []byte) (*Document, error) {
	var node yaml.Node
	if err := yaml.Unmarshal(raw, &node); err != nil {
		return nil, trace.Wrap(err)
	}
	if node.Kind == 0 {
		node.Kind = yaml.DocumentNode
		node.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	if node.Kind != yaml.DocumentNode {
		return nil, trace.BadParameter("expected YAML document")
	}
	if len(node.Content) == 0 {
		node.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	return &Document{node: &node}, nil
}

// Clone returns a deep copy of the document.
func (d *Document) Clone() *Document {
	return &Document{node: cloneNode(d.node)}
}

// Get returns the node at path.
func (d *Document) Get(path ...string) (*yaml.Node, bool) {
	node := d.root()
	for _, part := range path {
		if node.Kind != yaml.MappingNode {
			return nil, false
		}
		var ok bool
		node, ok = mappingValue(node, part)
		if !ok {
			return nil, false
		}
	}
	return node, true
}

// Delete removes a mapping key if present.
func (d *Document) Delete(path ...string) bool {
	if len(path) == 0 {
		return false
	}
	parent, ok := d.ensureParent(false, path[:len(path)-1]...)
	if !ok || parent.Kind != yaml.MappingNode {
		return false
	}
	return deleteMappingKey(parent, path[len(path)-1])
}

// Set creates or replaces a scalar or YAML node value at path.
func (d *Document) Set(value any, path ...string) error {
	if len(path) == 0 {
		return trace.BadParameter("missing path")
	}
	parent, ok := d.ensureParent(true, path[:len(path)-1]...)
	if !ok {
		return trace.BadParameter("failed to create parent path %q", strings.Join(path[:len(path)-1], "."))
	}
	if parent.Kind != yaml.MappingNode {
		return trace.BadParameter("path %q is not a mapping", strings.Join(path[:len(path)-1], "."))
	}

	valueNode, err := nodeFromValue(value)
	if err != nil {
		return trace.Wrap(err)
	}
	setMappingValue(parent, path[len(path)-1], valueNode)
	return nil
}

// Render marshals the document back to YAML.
func (d *Document) Render() ([]byte, error) {
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	if err := enc.Encode(d.node); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := enc.Close(); err != nil {
		return nil, trace.Wrap(err)
	}
	return out.Bytes(), nil
}

// Redact deep-copies the document and masks values matching rules.
func (d *Document) Redact(rules []RedactRule) *Document {
	out := d.Clone()
	for _, rule := range rules {
		node, ok := out.Get(rule.Path...)
		if !ok || node.Kind != yaml.ScalarNode {
			continue
		}
		switch rule.Mode {
		case RedactFull:
			node.Value = "<redacted>"
		}
		node.Tag = "!!str"
	}
	return out
}

// DiffDocuments returns a unified diff of two rendered documents with headers.
// Documents are redacted internally before rendering the diff.
func DiffDocuments(before, after *Document, beforeName, afterName string) (string, error) {
	beforeRaw, err := before.Redact(DefaultRedactionRules()).Render()
	if err != nil {
		return "", trace.Wrap(err)
	}
	afterRaw, err := after.Redact(DefaultRedactionRules()).Render()
	if err != nil {
		return "", trace.Wrap(err)
	}
	diff := difflib.UnifiedDiff{
		A:        splitLines(string(beforeRaw)),
		B:        splitLines(string(afterRaw)),
		FromFile: beforeName,
		ToFile:   afterName,
		Context:  3,
	}
	out, err := difflib.GetUnifiedDiffString(diff)
	return out, trace.Wrap(err)
}

func (d *Document) root() *yaml.Node {
	if d.node.Kind == yaml.DocumentNode && len(d.node.Content) > 0 {
		return d.node.Content[0]
	}
	return d.node
}

func (d *Document) ensureParent(create bool, path ...string) (*yaml.Node, bool) {
	node := d.root()
	for _, part := range path {
		if node.Kind == 0 {
			node.Kind = yaml.MappingNode
			node.Tag = "!!map"
		}
		if node.Kind != yaml.MappingNode {
			return nil, false
		}
		next, ok := mappingValue(node, part)
		if !ok {
			if !create {
				return nil, false
			}
			next = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			node.Content = append(node.Content, scalarKey(part), next)
		}
		node = next
	}
	return node, true
}

func nodeFromValue(value any) (*yaml.Node, error) {
	if node, ok := value.(*yaml.Node); ok {
		return cloneNode(node), nil
	}
	switch v := value.(type) {
	case string:
		return scalarString(v), nil
	case fmt.Stringer:
		return scalarString(v.String()), nil
	case bool:
		if v {
			return scalarString("yes"), nil
		}
		return scalarString("no"), nil
	default:
		var node yaml.Node
		if err := node.Encode(value); err != nil {
			return nil, trace.Wrap(err)
		}
		return &node, nil
	}
}

func mappingValue(node *yaml.Node, key string) (*yaml.Node, bool) {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return node.Content[i+1], true
		}
	}
	return nil, false
}

func setMappingValue(node *yaml.Node, key string, value *yaml.Node) {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content[i+1] = value
			return
		}
	}
	node.Content = append(node.Content, scalarKey(key), value)
}

func deleteMappingKey(node *yaml.Node, key string) bool {
	for i := 0; i+1 < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			node.Content = append(node.Content[:i], node.Content[i+2:]...)
			return true
		}
	}
	return false
}

func scalarKey(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func scalarString(value string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value}
}

func cloneNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	out := *node
	if len(node.Content) != 0 {
		out.Content = make([]*yaml.Node, len(node.Content))
		for i, child := range node.Content {
			out.Content[i] = cloneNode(child)
		}
	}
	return &out
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.SplitAfter(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
