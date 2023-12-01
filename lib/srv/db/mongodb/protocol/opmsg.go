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

package protocol

import (
	"fmt"
	"strings"

	"github.com/gravitational/trace"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"go.mongodb.org/mongo-driver/x/mongo/driver/wiremessage"
)

// MessageOpMsg represents parsed OP_MSG wire message.
//
// https://docs.mongodb.com/master/reference/mongodb-wire-protocol/#op-msg
//
//	OP_MSG {
//	    MsgHeader header;          // standard message header
//	    uint32 flagBits;           // message flags
//	    Sections[] sections;       // data sections
//	    optional<uint32> checksum; // optional CRC-32C checksum
//	}
type MessageOpMsg struct {
	Header                   MessageHeader
	Flags                    wiremessage.MsgFlag
	BodySection              SectionBody
	DocumentSequenceSections []SectionDocumentSequence
	Checksum                 uint32
	// bytes is the full wire message bytes (incl. header) read from the connection.
	bytes []byte
}

// MakeOpMsg is a shorthand to create OP_MSG message from a single document.
func MakeOpMsg(document bsoncore.Document) *MessageOpMsg {
	return &MessageOpMsg{
		BodySection: SectionBody{
			Document: document,
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

// GetDatabase returns the message's database.
func (m *MessageOpMsg) GetDatabase() (string, error) {
	// Database name must be present in the body section of client messages:
	// https://github.com/mongodb/specifications/blob/9946950/source/message/OP_MSG.rst#global-command-arguments
	// Do a sanity check to make sure there's exactly one "$db" key.
	elements, err := m.BodySection.Document.Elements()
	if err != nil {
		return "", trace.Wrap(err)
	}
	var dbElements []bsoncore.Element
	for _, element := range elements {
		if element.Key() == "$db" {
			dbElements = append(dbElements, element)
		}
	}
	if len(dbElements) != 1 {
		return "", trace.BadParameter("malformed OP_MSG: expected single $db key: %s", elements)
	}
	val, err := dbElements[0].ValueErr()
	if err != nil {
		return "", trace.Wrap(err)
	}
	str, ok := val.StringValueOK()
	if !ok {
		return "", trace.BadParameter("malformed OP_MSG: non-string $db value: %s", elements)
	}
	if len(str) == 0 {
		return "", trace.BadParameter("malformed OP_MSG: empty $db value: %s", elements)
	}
	return str, nil
}

// GetCommand returns the message's command.
func (m *MessageOpMsg) GetCommand() (string, error) {
	// Command is the first element of the body document e.g.
	// { "authenticate": 1, "mechanism": ... }
	cmd, err := m.BodySection.Document.IndexErr(0)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return cmd.Key(), nil
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
	return fmt.Sprintf("OpMsg(Body=%s, Documents=%s, Flags=%v)",
		m.BodySection, m.DocumentSequenceSections, strings.Join(flags, ","))
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

// String returns the section's string representation.
func (s SectionBody) String() string {
	return s.Document.String()
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

// String returns the section's string representation.
func (s SectionDocumentSequence) String() string {
	docs := make([]string, 0, len(s.Documents))
	for _, doc := range s.Documents {
		docs = append(docs, doc.String())
	}
	return strings.Join(docs, ", ")
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
		return nil, trace.BadParameter("malformed OP_MSG: missing flags %v", payload)
	}
	var (
		bodySection              *SectionBody
		documentSequenceSections []SectionDocumentSequence
		checksum                 uint32
	)
	for len(rem) > 0 {
		// Checksum is the optional last part of the message.
		if flags&wiremessage.ChecksumPresent == wiremessage.ChecksumPresent && len(rem) == 4 {
			checksum, _, ok = wiremessage.ReadMsgChecksum(rem)
			if !ok {
				return nil, trace.BadParameter("malformed OP_MSG: missing checksum %v", payload)
			}
			break
		}
		var sectionType wiremessage.SectionType
		sectionType, rem, ok = wiremessage.ReadMsgSectionType(rem)
		if !ok {
			return nil, trace.BadParameter("malformed OP_MSG: missing section type %v", payload)
		}
		switch sectionType {
		// https://docs.mongodb.com/manual/reference/mongodb-wire-protocol/#kind-0--body
		case wiremessage.SingleDocument:
			// Valid OP_MSG must have exactly one body section:
			// https://github.com/mongodb/specifications/blob/9946950/source/message/OP_MSG.rst#sections
			if bodySection != nil {
				return nil, trace.BadParameter("malformed OP_MSG: expected exactly 1 body section %v", payload)
			}
			var doc bsoncore.Document
			doc, rem, ok = wiremessage.ReadMsgSectionSingleDocument(rem)
			if !ok {
				return nil, trace.BadParameter("malformed OP_MSG: missing body section %v", payload)
			}
			bodySection = &SectionBody{
				Document: doc,
			}
		// https://docs.mongodb.com/manual/reference/mongodb-wire-protocol/#kind-1--document-sequence
		case wiremessage.DocumentSequence:
			var id string
			var docs []bsoncore.Document

			if err := validateDocumentSize(rem); err != nil {
				return nil, trace.BadParameter("malformed OP_MSG: %v %v", err, payload)
			}

			id, docs, rem, ok = wiremessage.ReadMsgSectionDocumentSequence(rem)
			if !ok {
				return nil, trace.BadParameter("malformed OP_MSG: missing document sequence section %v", payload)
			}
			documentSequenceSections = append(documentSequenceSections, SectionDocumentSequence{
				Identifier: id,
				Documents:  docs,
			})
		}
	}
	if bodySection == nil {
		return nil, trace.BadParameter("malformed OP_MSG: missing body section %v", payload)
	}
	return &MessageOpMsg{
		Header:                   header,
		Flags:                    flags,
		BodySection:              *bodySection,
		DocumentSequenceSections: documentSequenceSections,
		Checksum:                 checksum,
		bytes:                    append(header.bytes[:], payload...),
	}, nil
}

// validateDocumentSize validates document length encoded in the message.
func validateDocumentSize(src []byte) error {
	const headerLen = 4
	if len(src) < headerLen {
		return trace.BadParameter("document is too short")
	}

	// document length is encoded in the first 4 bytes
	documentLength := int(int32(src[0]) | int32(src[1])<<8 | int32(src[2])<<16 | int32(src[3])<<24)
	// Ensure that idx is not negative.
	if documentLength-4 < 0 {
		return trace.BadParameter("invalid document length")
	}
	return nil
}

// ToWire converts this message to wire protocol message bytes.
//
// https://docs.mongodb.com/manual/reference/mongodb-wire-protocol/#op_msg
func (m *MessageOpMsg) ToWire(responseTo int32) (dst []byte) {
	var idx int32
	idx, dst = wiremessage.AppendHeaderStart(dst, m.Header.RequestID, responseTo, wiremessage.OpMsg)
	dst = wiremessage.AppendMsgFlags(dst, m.Flags)
	dst = append(dst, m.BodySection.ToWire()...)
	for _, section := range m.DocumentSequenceSections {
		dst = append(dst, section.ToWire()...)
	}
	if m.Flags&wiremessage.ChecksumPresent == wiremessage.ChecksumPresent {
		dst = bsoncore.AppendInt32(dst, int32(m.Checksum))
	}
	return bsoncore.UpdateLength(dst, idx, int32(len(dst[idx:])))
}
