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

	smithyxml "github.com/aws/smithy-go/encoding/xml"

	"github.com/gravitational/trace"
)

// IsXMLOfLocalName returns true if the root XML has the provided (local) name.
func IsXMLOfLocalName(data []byte, wantLocalName string) bool {
	st, err := smithyxml.FetchRootElement(xml.NewDecoder(bytes.NewReader(data)))
	if err == nil && st.Name.Local == wantLocalName {
		return true
	}

	return false
}

// UnmarshalXMLChildNode decodes the XML-encoded data and stores the child node
// with the specified name to v, where v is a pointer to an AWS SDK v1 struct.
func UnmarshalXMLChildNode(v interface{}, data []byte, childName string) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	st, err := smithyxml.FetchRootElement(decoder)
	if err != nil {
		return trace.Wrap(err)
	}
	nodeDecoder := smithyxml.WrapNodeDecoder(decoder, st)
	childElem, err := nodeDecoder.GetElement(childName)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(decoder.DecodeElement(v, &childElem))
}

// MarshalXML marshals the provided root name and a map of children in XML with
// default indent (prefix "", indent "  ").
func MarshalXML(root string, namespace string, v any) ([]byte, error) {
	var buf bytes.Buffer
	encoder := xml.NewEncoder(&buf)
	encoder.Indent("", "  ")
	err := encoder.EncodeElement(v, xml.StartElement{
		Name: xml.Name{Local: root},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "xmlns"}, Value: namespace},
		},
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return buf.Bytes(), nil
}
