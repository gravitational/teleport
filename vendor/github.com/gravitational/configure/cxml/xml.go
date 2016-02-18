/*
Copyright 2015 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cxml

import (
	"bytes"
	"encoding/xml"
	"io"

	"github.com/gravitational/trace"
)

// TransofrmFunc is a function that can transform incoming token
// into a series of outgoing tokens when traversing XML tree
type TransformFunc func(parents *NodeList, in xml.Token) []xml.Token

// ConditionFunc is accepted by some transform function generators
// so they could be applied conditionally
type ConditionFunc func(parents *NodeList, in xml.Token) bool

// TransformXML parses the XML tree, traverses it and calls TransformFunc
// on each XML token, writing the output to the writer, resulting in a
// transformed XML tree
func TransformXML(decoder *xml.Decoder, encoder *xml.Encoder, fn TransformFunc) error {
	parentNodes := &NodeList{}
	for {
		token, err := decoder.Token()
		if err != nil {
			if err != io.EOF {
				return trace.Wrap(err)
			}
			break
		}
		for _, t := range fn(parentNodes, token) {
			if err := encoder.EncodeToken(t); err != nil {
				return err
			}
		}
		switch e := token.(type) {
		case xml.StartElement:
			parentNodes.Push(e)
		case xml.EndElement:
			parentNodes.Pop()
		}
	}
	encoder.Flush()
	return nil
}

// SetAttribute is XML helper that allows to set attribute on a node
func SetAttribute(e xml.StartElement, name, value string) xml.StartElement {
	for i := range e.Attr {
		if e.Attr[i].Name.Local == name {
			e.Attr[i].Value = value
			return e
		}
	}
	e.Attr = append(e.Attr, xml.Attr{Name: xml.Name{Local: name}, Value: value})
	return e
}

// ReplaceCDATAIf replaces CDATA value of the matched node
// if the parent node name matches the name
func ReplaceCDATAIf(val xml.CharData, cond ConditionFunc) TransformFunc {
	return func(parents *NodeList, in xml.Token) []xml.Token {
		switch in.(type) {
		case xml.CharData:
			if cond(parents, in) {
				return []xml.Token{val.Copy()}
			}
		}
		return []xml.Token{in}
	}
}

// InjectNodes injects nodes at the end of the tag that matches name
func InjectNodesIf(nodes []xml.Token, cond ConditionFunc) TransformFunc {
	return func(parents *NodeList, in xml.Token) []xml.Token {
		if cond(parents, in) {
			return append(nodes, in)
		}
		return []xml.Token{in}
	}
}

// ReplaceAttribute replaces the attribute of the first node that matches the name
func ReplaceAttributeIf(attrName, attrValue string, cond ConditionFunc) TransformFunc {
	return func(parents *NodeList, in xml.Token) []xml.Token {
		switch t := in.(type) {
		case xml.StartElement:
			if cond(parents, in) {
				e := xml.StartElement(t)
				return []xml.Token{SetAttribute(e, attrName, attrValue)}
			}
		}
		return []xml.Token{in}
	}
}

// TrimSpace is a transformer function that replaces CDATA with blank
// characters with empty strings
func TrimSpace(parents *NodeList, in xml.Token) []xml.Token {
	switch t := in.(type) {
	case xml.CharData:
		return []xml.Token{xml.CharData(bytes.TrimSpace(t))}
	}
	return []xml.Token{in}
}

// Combine takes a list of TransformFuncs and converts them
// into a single transform function applying all functions one by one
func Combine(funcs ...TransformFunc) TransformFunc {
	return func(parents *NodeList, in xml.Token) []xml.Token {
		out := []xml.Token{in}
		for _, f := range funcs {
			new := []xml.Token{}
			for _, t := range out {
				new = append(new, f(parents, t)...)
			}
			out = new
		}
		return out
	}
}

// ParentIs is a functon that returns ConditionFunc checking
// if immediate parent's name matches name
func ParentIs(name xml.Name) ConditionFunc {
	return func(parents *NodeList, el xml.Token) bool {
		return parents.ParentIs(name)
	}
}

// GetAttribute returns attribute value if it's present, otherwise
// it returns an empty string
func GetAttribute(e xml.StartElement, name xml.Name) string {
	for _, a := range e.Attr {
		if NameEquals(a.Name, name) {
			return a.Value
		}
	}
	return ""
}

// HasAttribute returns true if element has an attribute with given
// name, false otherwise
func HasAttribute(e xml.StartElement, name xml.Name) bool {
	for _, a := range e.Attr {
		if NameEquals(name, a.Name) {
			return true
		}
	}
	return false
}

// Name returns an xml.Name with local part set to name
func Name(name string) xml.Name {
	return xml.Name{Local: name}
}

type NodeList struct {
	nodes []xml.StartElement
}

func (n *NodeList) Push(node xml.StartElement) {
	n.nodes = append(n.nodes, node)
}

func (n *NodeList) Pop() {
	if len(n.nodes) == 0 {
		return
	}
	n.nodes = n.nodes[:len(n.nodes)-1]
}

// ParentIs returns true if last element exists and
// it's name equals to this name
func (n *NodeList) ParentIs(a xml.Name) bool {
	if len(n.nodes) == 0 {
		return false
	}
	return NameEquals(a, n.nodes[len(n.nodes)-1].Name)
}

// NameEquals return true if both namespaces and local names of nodes
// match, and false otherwise
func NameEquals(a, b xml.Name) bool {
	return a.Local == b.Local && a.Space == b.Space
}
