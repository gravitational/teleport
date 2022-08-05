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

// MessageOpInsert represents parsed OP_INSERT wire message.
//
// https://docs.mongodb.com/manual/reference/mongodb-wire-protocol/#op_insert
//
// struct {
//     MsgHeader header;             // standard message header
//     int32     flags;              // bit vector - see below
//     cstring   fullCollectionName; // "dbname.collectionname"
//     document* documents;          // one or more documents to insert into the collection
// }
//
// OP_INSERT is deprecated starting MongoDB 5.0 in favor of OP_MSG.
type MessageOpInsert struct {
	Header             MessageHeader
	Flags              int32
	FullCollectionName string
	Documents          []bsoncore.Document
	// bytes is the full wire message bytes (incl. header) read from the connection.
	bytes []byte
}

// GetHeader returns the wire message header.
func (m *MessageOpInsert) GetHeader() MessageHeader {
	return m.Header
}

// GetBytes returns the message raw bytes read from the connection.
func (m *MessageOpInsert) GetBytes() []byte {
	return m.bytes
}

// GetDatabase returns the command's database.
func (m *MessageOpInsert) GetDatabase() (string, error) {
	// Full collection name has "<db>.<collection>" format.
	return strings.Split(m.FullCollectionName, ".")[0], nil
}

// GetCommand returns the message's command.
func (m *MessageOpInsert) GetCommand() (string, error) {
	return "insert", nil
}

// String returns the message string representation.
func (m *MessageOpInsert) String() string {
	documents := make([]string, 0, len(m.Documents))
	for _, document := range m.Documents {
		documents = append(documents, document.String())
	}
	return fmt.Sprintf("OpInsert(FullCollectionName=%v, Documents=%s, Flags=%v)",
		m.FullCollectionName, documents, m.Flags)
}

// MoreToCome is whether sender will send another message right after this one.
func (m *MessageOpInsert) MoreToCome(_ Message) bool {
	return true
}

// readOpInsert converts OP_INSERT wire message bytes to a structured form.
func readOpInsert(header MessageHeader, payload []byte) (*MessageOpInsert, error) {
	flags, rem, ok := readInt32(payload)
	if !ok {
		return nil, trace.BadParameter("malformed OP_INSERT: missing flags %v", payload)
	}
	fullCollectionName, rem, ok := readCString(rem)
	if !ok {
		return nil, trace.BadParameter("malformed OP_INSERT: missing full collection name %v", payload)
	}
	var documents []bsoncore.Document
	for len(rem) > 0 {
		var document bsoncore.Document
		document, rem, ok = bsoncore.ReadDocument(rem)
		if !ok {
			return nil, trace.BadParameter("malformed OP_INSERT: missing document %v", payload)
		}
		documents = append(documents, document)
	}
	return &MessageOpInsert{
		Header:             header,
		Flags:              flags,
		FullCollectionName: fullCollectionName,
		Documents:          documents,
		bytes:              append(header.bytes[:], payload...),
	}, nil
}

// ToWire converts this message to wire protocol message bytes.
func (m *MessageOpInsert) ToWire(responseTo int32) (dst []byte) {
	var idx int32
	idx, dst = wiremessage.AppendHeaderStart(dst, m.Header.RequestID, responseTo, wiremessage.OpInsert)
	dst = appendInt32(dst, m.Flags)
	dst = appendCString(dst, m.FullCollectionName)
	for _, document := range m.Documents {
		dst = bsoncore.AppendDocument(dst, document)
	}
	return bsoncore.UpdateLength(dst, idx, int32(len(dst[idx:])))
}
