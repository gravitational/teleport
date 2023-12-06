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

	"github.com/gravitational/trace"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"go.mongodb.org/mongo-driver/x/mongo/driver/wiremessage"
)

// MessageOpReply represents parsed OP_REPLY wire message.
//
// https://docs.mongodb.com/manual/reference/mongodb-wire-protocol/#op_reply
type MessageOpReply struct {
	Header         MessageHeader
	Flags          wiremessage.ReplyFlag
	CursorID       int64
	StartingFrom   int32
	NumberReturned int32
	Documents      []bsoncore.Document
	// bytes is the full wire message bytes (incl. header) read from the connection.
	bytes []byte
}

// MakeOpReply is a shorthand to create OP_REPLY message from a single document.
func MakeOpReply(document bsoncore.Document) *MessageOpReply {
	return &MessageOpReply{
		NumberReturned: 1,
		Documents: []bsoncore.Document{
			document,
		},
	}
}

// MakeOpReplyWithFlags is a shorthand to create OP_REPLY message from a single document
// with provided flags.
func MakeOpReplyWithFlags(document bsoncore.Document, flags wiremessage.ReplyFlag) *MessageOpReply {
	return &MessageOpReply{
		Flags:          flags,
		NumberReturned: 1,
		Documents: []bsoncore.Document{
			document,
		},
	}
}

// GetHeader returns the wire message header.
func (m *MessageOpReply) GetHeader() MessageHeader {
	return m.Header
}

// GetBytes returns the message raw bytes read from the connection.
func (m *MessageOpReply) GetBytes() []byte {
	return m.bytes
}

// GetDocumentsAsStrings is a convenience method to return all message bson
// documents converted to their string representations.
func (m *MessageOpReply) GetDocumentsAsStrings() (documents []string) {
	for _, document := range m.Documents {
		documents = append(documents, document.String())
	}
	return documents
}

// String returns the message string representation.
func (m *MessageOpReply) String() string {
	return fmt.Sprintf("OpReply(Documents=%v, StartingFrom=%v, NumberReturned=%v, CursorID=%v, Flags=%v)",
		m.GetDocumentsAsStrings(), m.StartingFrom, m.NumberReturned, m.CursorID, m.Flags.String())
}

// MoreToCome is whether sender will send another message right after this one.
func (m *MessageOpReply) MoreToCome(msg Message) bool {
	// Check if this is an exhaust cursor.
	opQuery, ok := msg.(*MessageOpQuery)
	return ok && opQuery.Flags&wiremessage.Exhaust == wiremessage.Exhaust && m.CursorID != 0
}

// GetDatabase is a no-op for OpReply since this is a server message.
func (m *MessageOpReply) GetDatabase() (string, error) {
	return "", nil
}

// GetCommand is a no-op for OpReply since this is a server message.
func (m *MessageOpReply) GetCommand() (string, error) {
	return "", nil
}

// readOpReply converts OP_REPLY wire message bytes into a structured form.
//
// https://docs.mongodb.com/manual/reference/mongodb-wire-protocol/#op_reply
func readOpReply(header MessageHeader, payload []byte) (*MessageOpReply, error) {
	flags, rem, ok := wiremessage.ReadReplyFlags(payload)
	if !ok {
		return nil, trace.BadParameter("malformed OP_REPLY: missing response flags %v", payload)
	}
	cursorID, rem, ok := wiremessage.ReadReplyCursorID(rem)
	if !ok {
		return nil, trace.BadParameter("malformed OP_REPLY: missing cursor ID %v", payload)
	}
	startingFrom, rem, ok := wiremessage.ReadReplyStartingFrom(rem)
	if !ok {
		return nil, trace.BadParameter("malformed OP_REPLY: missing starting from %v", payload)
	}
	numberReturned, rem, ok := wiremessage.ReadReplyNumberReturned(rem)
	if !ok {
		return nil, trace.BadParameter("malformed OP_REPLY: missing number returned %v", payload)
	}
	documents, _, ok := ReadReplyDocuments(rem)
	if !ok {
		return nil, trace.BadParameter("malformed OP_REPLY: missing documents %v", payload)
	}
	return &MessageOpReply{
		Header:         header,
		Flags:          flags,
		CursorID:       cursorID,
		StartingFrom:   startingFrom,
		NumberReturned: numberReturned,
		Documents:      documents,
		bytes:          append(header.bytes[:], payload...),
	}, nil
}

// ReadReplyDocuments reads multiple documents from the source.
//
// This function works in the same way as wiremessage.ReadReplyDocuments except, it can handle document of size 0.
// When a document of size 0 is passed to wiremessage.ReadReplyDocuments, it will keep creating empty documents until
// it uses all system memory/application crash.
func ReadReplyDocuments(src []byte) (docs []bsoncore.Document, rem []byte, ok bool) {
	rem = src
	for {
		var doc bsoncore.Document
		doc, rem, ok = bsoncore.ReadDocument(rem)
		if !ok || len(doc) == 0 {
			break
		}

		docs = append(docs, doc)
	}

	return docs, rem, true
}

// ToWire converts this message to wire protocol message bytes.
//
// https://docs.mongodb.com/manual/reference/mongodb-wire-protocol/#op_reply
func (m *MessageOpReply) ToWire(responseTo int32) (dst []byte) {
	var idx int32
	idx, dst = wiremessage.AppendHeaderStart(dst, m.Header.RequestID, responseTo, wiremessage.OpReply)
	dst = wiremessage.AppendReplyFlags(dst, m.Flags)
	dst = wiremessage.AppendReplyCursorID(dst, m.CursorID)
	dst = wiremessage.AppendReplyStartingFrom(dst, m.StartingFrom)
	dst = wiremessage.AppendReplyNumberReturned(dst, m.NumberReturned)
	for _, document := range m.Documents {
		dst = bsoncore.AppendDocument(dst, document)
	}
	return bsoncore.UpdateLength(dst, idx, int32(len(dst[idx:])))
}
