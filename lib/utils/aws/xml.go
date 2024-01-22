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

package aws

import (
	"bytes"
	"encoding/xml"

	"github.com/aws/aws-sdk-go/private/protocol/xml/xmlutil"
	"github.com/gravitational/trace"
)

// IsXMLOfLocalName returns true if the root XML has the provided (local) name.
func IsXMLOfLocalName(data []byte, wantLocalName string) bool {
	var name xml.Name
	if err := xml.Unmarshal(data, &name); err == nil {
		return wantLocalName == name.Local
	}
	return false
}

// UnmarshalXMLChildNode decodes the XML-encoded data and stores the child node
// with the specified name to v, where v is a pointer to an AWS SDK v1 struct.
func UnmarshalXMLChildNode(v interface{}, data []byte, childName string) error {
	return trace.Wrap(xmlutil.UnmarshalXML(v, xml.NewDecoder(bytes.NewReader(data)), childName))
}

// MarshalXML marshals the provided root name and a map of children in XML with
// default indent (prefix "", indent "  ").
func MarshalXML(rootName xml.Name, children map[string]any) ([]byte, error) {
	var buf bytes.Buffer
	encoder := xml.NewEncoder(&buf)
	encoder.Indent("", "  ")

	err := encodeXMLNode(encoder, rootName, func() error {
		for childName, childValue := range children {
			if err := encodeXMLNodeAWSSDKV1(encoder, childName, childValue); err != nil {
				return trace.Wrap(err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if err := trace.Wrap(encoder.Flush()); err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}

func encodeXMLNode(encoder *xml.Encoder, name xml.Name, encodeChildren func() error) error {
	startElement := xml.StartElement{Name: name}
	if err := encoder.EncodeToken(startElement); err != nil {
		return trace.Wrap(err)
	}
	if err := encodeChildren(); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(encoder.EncodeToken(startElement.End()))
}

func encodeXMLNodeAWSSDKV1(encoder *xml.Encoder, name string, v any) error {
	return encodeXMLNode(encoder, xml.Name{Local: name}, func() error {
		return trace.Wrap(xmlutil.BuildXML(v, encoder))
	})
}
