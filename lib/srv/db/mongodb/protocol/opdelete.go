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

// MessageOpDelete represents parsed OP_DELETE wire message.
//
// https://docs.mongodb.com/manual/reference/mongodb-wire-protocol/#op_delete
//
//	struct {
//	    MsgHeader header;             // standard message header
//	    int32     ZERO;               // 0 - reserved for future use
//	    cstring   fullCollectionName; // "dbname.collectionname"
//	    int32     flags;              // bit vector - see below for details.
//	    document  selector;           // query object.  See below for details.
//	}
//
// OP_DELETE is deprecated starting MongoDB 5.0 in favor of OP_MSG.
type MessageOpDelete struct {
	Header             MessageHeader
	Zero               int32
	FullCollectionName string
	Flags              int32
	Selector           bsoncore.Document
	// bytes is the full wire message bytes (incl. header) read from the connection.
	bytes []byte
}

// GetHeader returns the wire message header.
func (m *MessageOpDelete) GetHeader() MessageHeader {
	return m.Header
}

// GetBytes returns the message raw bytes read from the connection.
func (m *MessageOpDelete) GetBytes() []byte {
	return m.bytes
}

// GetDatabase returns the command's database.
func (m *MessageOpDelete) GetDatabase() (string, error) {
	// Full collection name has "<db>.<collection>" format.
	return strings.Split(m.FullCollectionName, ".")[0], nil
}

// GetCommand returns the message's command.
func (m *MessageOpDelete) GetCommand() (string, error) {
	return "delete", nil
}

// String returns the message string representation.
func (m *MessageOpDelete) String() string {
	return fmt.Sprintf("OpDelete(FullCollectionName=%v, Selector=%v, Flags=%v)",
		m.FullCollectionName, m.Selector.String(), m.Flags)
}

// MoreToCome is whether sender will send another message right after this one.
func (m *MessageOpDelete) MoreToCome(_ Message) bool {
	return true
}

// readOpDelete converts OP_DELETE wire message bytes to a structured form.
func readOpDelete(header MessageHeader, payload []byte) (*MessageOpDelete, error) {
	zero, rem, ok := readInt32(payload)
	if !ok {
		return nil, trace.BadParameter("malformed OP_DELETE: missing zero %v", payload)
	}
	fullCollectionName, rem, ok := readCString(rem)
	if !ok {
		return nil, trace.BadParameter("malformed OP_DELETE: missing full collection name %v", payload)
	}
	flags, rem, ok := readInt32(rem)
	if !ok {
		return nil, trace.BadParameter("malformed OP_DELETE: missing flags %v", payload)
	}
	selector, _, ok := bsoncore.ReadDocument(rem)
	if !ok {
		return nil, trace.BadParameter("malformed OP_DELETE: missing selector %v", payload)
	}
	return &MessageOpDelete{
		Header:             header,
		Zero:               zero,
		FullCollectionName: fullCollectionName,
		Flags:              flags,
		Selector:           selector,
		bytes:              append(header.bytes[:], payload...),
	}, nil
}

// ToWire converts this message to wire protocol message bytes.
func (m *MessageOpDelete) ToWire(responseTo int32) (dst []byte) {
	var idx int32
	idx, dst = wiremessage.AppendHeaderStart(dst, m.Header.RequestID, responseTo, wiremessage.OpDelete)
	dst = appendInt32(dst, m.Zero)
	dst = appendCString(dst, m.FullCollectionName)
	dst = appendInt32(dst, m.Flags)
	dst = bsoncore.AppendDocument(dst, m.Selector)
	return bsoncore.UpdateLength(dst, idx, int32(len(dst[idx:])))
}
