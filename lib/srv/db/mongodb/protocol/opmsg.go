/*
Copyright 2021 Gravitational, Inc.

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

package protocol

import (
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"go.mongodb.org/mongo-driver/x/mongo/driver/wiremessage"

	"github.com/gravitational/trace"
)

// MessageOpMsg represents parsed OP_MSG wire message.
//
// https://docs.mongodb.com/master/reference/mongodb-wire-protocol/#op-msg
type MessageOpMsg struct {
	Header   MessageHeader
	Flags    wiremessage.MsgFlag
	Sections []Section
	Checksum uint32
	// bytes is the full wire message bytes (incl. header) read from the connection.
	bytes []byte
}

// MakeOpMsg is a shorthand to create OP_MSG message from a single document.
func MakeOpMsg(document bsoncore.Document) *MessageOpMsg {
	return &MessageOpMsg{
		Sections: []Section{
			&SectionBody{
				Document: document,
			},
		},
	}
}

// GetHeader returns the wire message header.
func (m *MessageOpMsg) GetHeader() MessageHeader {
	return m.Header
}

// GetBytes returns the message raw bytes read from the connection.
func (m *MessageOpMsg) GetBytes() []byte {
	return m.bytes
}

// GetDocuments returns all documents from all sections present in the message.
func (m *MessageOpMsg) GetDocuments() (result []bsoncore.Document) {
	for _, section := range m.Sections {
		switch s := section.(type) {
		case *SectionBody:
			result = append(result, s.Document)
		case *SectionDocumentSequence:
			result = append(result, s.Documents...)
		}
	}
	return result
}

// GetDocumentsAsStrings is a convenience method to return all message bson
// documents converted to their string representations.
func (m *MessageOpMsg) GetDocumentsAsStrings() (documents []string) {
	for _, document := range m.GetDocuments() {
		documents = append(documents, document.String())
	}
	return documents
}

// GetDocument returns the message's document.
//
// It expects the message to have exactly one document and returns error otherwise.
func (m *MessageOpMsg) GetDocument() (bsoncore.Document, error) {
	documents := m.GetDocuments()
	if len(documents) != 1 {
		return nil, trace.BadParameter("expected 1 document, got: %v", documents)
	}
	return documents[0], nil
}

// GetDatabase returns name of the database for the query, or an empty string.
func (m *MessageOpMsg) GetDatabase() string {
	for _, document := range m.GetDocuments() {
		if value, ok := document.Lookup("$db").StringValueOK(); ok && value != "" {
			return value
		}
	}
	return ""
}

// MoreToCome is whether sender will send another message right after this one.
func (m *MessageOpMsg) MoreToCome(_ Message) bool {
	return m.Flags&wiremessage.MoreToCome == wiremessage.MoreToCome
}

// String returns the message string representation.
func (m *MessageOpMsg) String() string {
	var flags []string
	if m.Flags&wiremessage.ChecksumPresent == wiremessage.ChecksumPresent {
		flags = append(flags, "ChecksumPresent")
	}
	if m.Flags&wiremessage.MoreToCome == wiremessage.MoreToCome {
		flags = append(flags, "MoreToCome")
	}
	if m.Flags&wiremessage.ExhaustAllowed == wiremessage.ExhaustAllowed {
		flags = append(flags, "ExhaustAllowed")
	}
	return fmt.Sprintf("OpMsg(Flags=%v, Documents=%v)",
		strings.Join(flags, ","), m.GetDocumentsAsStrings())
}

// Section represents a single OP_MSG wire message section.
//
// https://docs.mongodb.com/manual/reference/mongodb-wire-protocol/#sections
type Section interface {
	GetType() wiremessage.SectionType
	ToWire() []byte
}

// SectionBody represents OP_MSG Body section that contains a single bson
// document.
//
// https://docs.mongodb.com/manual/reference/mongodb-wire-protocol/#kind-0--body
type SectionBody struct {
	Document bsoncore.Document
}

// GetType returns this section type.
func (s *SectionBody) GetType() wiremessage.SectionType {
	return wiremessage.SingleDocument
}

// ToWire encodes this section to wire protocol message bytes.
//
// https://docs.mongodb.com/manual/reference/mongodb-wire-protocol/#kind-0--body
func (s *SectionBody) ToWire() (dst []byte) {
	dst = wiremessage.AppendMsgSectionType(dst, wiremessage.SingleDocument)
	dst = bsoncore.AppendDocument(dst, s.Document)
	return dst
}

// SectionDocumentSequence represents OP_MSG Document Sequence section that
// contains multiple bson documents.
//
// https://docs.mongodb.com/manual/reference/mongodb-wire-protocol/#kind-1--document-sequence
type SectionDocumentSequence struct {
	Identifier string
	Documents  []bsoncore.Document
}

// GetType returns this section type.
func (s *SectionDocumentSequence) GetType() wiremessage.SectionType {
	return wiremessage.DocumentSequence
}

// ToWire encodes this section to wire protocol message bytes.
//
// https://docs.mongodb.com/manual/reference/mongodb-wire-protocol/#kind-1--document-sequence
func (s *SectionDocumentSequence) ToWire() (dst []byte) {
	dst = wiremessage.AppendMsgSectionType(dst, wiremessage.DocumentSequence)
	var idx int32
	idx, dst = bsoncore.ReserveLength(dst)
	// No helper function to append section identifier in wiremessage
	// package for some reason...
	dst = append(dst, s.Identifier...)
	dst = append(dst, 0x00)
	for _, document := range s.Documents {
		dst = bsoncore.AppendDocument(dst, document)
	}
	return bsoncore.UpdateLength(dst, idx, int32(len(dst[idx:])))
}

// readOpMsg converts OP_MSG wire message bytes to a structured form.
//
// https://docs.mongodb.com/manual/reference/mongodb-wire-protocol/#op_msg
func readOpMsg(header MessageHeader, payload []byte) (*MessageOpMsg, error) {
	flags, rem, ok := wiremessage.ReadMsgFlags(payload)
	if !ok {
		return nil, trace.BadParameter("failed to read OP_MSG flags %v", payload)
	}
	var sections []Section
	var checksum uint32
	for len(rem) > 0 {
		// Checksum is the optional last part of the message.
		if flags&wiremessage.ChecksumPresent == wiremessage.ChecksumPresent && len(rem) == 4 {
			checksum, _, ok = wiremessage.ReadMsgChecksum(rem)
			if !ok {
				return nil, trace.BadParameter("failed to read OP_MSG checksum %v", payload)
			}
			break
		}
		var sectionType wiremessage.SectionType
		sectionType, rem, ok = wiremessage.ReadMsgSectionType(rem)
		if !ok {
			return nil, trace.BadParameter("failed to read OP_MSG section type %v", payload)
		}
		switch sectionType {
		// https://docs.mongodb.com/manual/reference/mongodb-wire-protocol/#kind-0--body
		case wiremessage.SingleDocument:
			var doc bsoncore.Document
			doc, rem, ok = wiremessage.ReadMsgSectionSingleDocument(rem)
			if !ok {
				return nil, trace.BadParameter("failed to read OP_MSG body section %v", payload)
			}
			sections = append(sections, &SectionBody{
				Document: doc,
			})
		// https://docs.mongodb.com/manual/reference/mongodb-wire-protocol/#kind-1--document-sequence
		case wiremessage.DocumentSequence:
			var id string
			var docs []bsoncore.Document
			id, docs, rem, ok = wiremessage.ReadMsgSectionDocumentSequence(rem)
			if !ok {
				return nil, trace.BadParameter("failed to read OP_MSG document sequence section %v", payload)
			}
			sections = append(sections, &SectionDocumentSequence{
				Identifier: id,
				Documents:  docs,
			})
		}
	}
	return &MessageOpMsg{
		Header:   header,
		Flags:    flags,
		Sections: sections,
		Checksum: checksum,
		bytes:    append(header.bytes[:], payload...),
	}, nil
}

// ToWire converts this message to wire protocol message bytes.
//
// https://docs.mongodb.com/manual/reference/mongodb-wire-protocol/#op_msg
func (m *MessageOpMsg) ToWire(responseTo int32) (dst []byte) {
	var idx int32
	idx, dst = wiremessage.AppendHeaderStart(dst, m.Header.RequestID, responseTo, wiremessage.OpMsg)
	dst = wiremessage.AppendMsgFlags(dst, m.Flags)
	for _, section := range m.Sections {
		dst = append(dst, section.ToWire()...)
	}
	if m.Flags&wiremessage.ChecksumPresent == wiremessage.ChecksumPresent {
		dst = bsoncore.AppendInt32(dst, int32(m.Checksum))
	}
	return bsoncore.UpdateLength(dst, idx, int32(len(dst[idx:])))
}
