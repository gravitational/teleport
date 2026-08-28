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

// MessageOpGetMore represents parsed OP_GET_MORE wire message.
//
// https://docs.mongodb.com/manual/reference/mongodb-wire-protocol/#op_get_more
//
//	struct {
//	    MsgHeader header;             // standard message header
//	    int32     ZERO;               // 0 - reserved for future use
//	    cstring   fullCollectionName; // "dbname.collectionname"
//	    int32     numberToReturn;     // number of documents to return
//	    int64     cursorID;           // cursorID from the OP_REPLY
//	}
//
// OP_GET_MORE is deprecated starting MongoDB 5.0 in favor of OP_MSG.
type MessageOpGetMore struct {
	Header             MessageHeader
	Zero               int32
	FullCollectionName string
	NumberToReturn     int32
	CursorID           int64
	// bytes is the full wire message bytes (incl. header) read from the connection.
	bytes []byte
}

// GetHeader returns the wire message header.
func (m *MessageOpGetMore) GetHeader() MessageHeader {
	return m.Header
}

// GetBytes returns the message raw bytes read from the connection.
func (m *MessageOpGetMore) GetBytes() []byte {
	return m.bytes
}

// GetDatabase returns the command's database.
func (m *MessageOpGetMore) GetDatabase() (string, error) {
	// Full collection name has "<db>.<collection>" format.
	return strings.Split(m.FullCollectionName, ".")[0], nil
}

// GetCommand returns the message's command.
func (m *MessageOpGetMore) GetCommand() (string, error) {
	return "getMore", nil
}

// String returns the message string representation.
func (m *MessageOpGetMore) String() string {
	return fmt.Sprintf("OpGetMore(FullCollectionName=%v, NumberToReturn=%v, CursorID=%v)",
		m.FullCollectionName, m.NumberToReturn, m.CursorID)
}

// MoreToCome is whether sender will send another message right after this one.
func (m *MessageOpGetMore) MoreToCome(_ Message) bool {
	return false
}

// readOpGetMore converts OP_GET_MORE wire message bytes to a structured form.
func readOpGetMore(header MessageHeader, payload []byte) (*MessageOpGetMore, error) {
	zero, rem, ok := readInt32(payload)
	if !ok {
		return nil, trace.BadParameter("malformed OP_GET_MORE: missing zero %v", payload)
	}
	fullCollectionName, rem, ok := readCString(rem)
	if !ok {
		return nil, trace.BadParameter("malformed OP_GET_MORE: missing full collection name %v", payload)
	}
	numberToReturn, rem, ok := readInt32(rem)
	if !ok {
		return nil, trace.BadParameter("malformed OP_GET_MORE: missing number to return %v", payload)
	}
	cursorID, _, ok := readInt64(rem)
	if !ok {
		return nil, trace.BadParameter("malformed OP_GET_MORE: missing cursor ID %v", payload)
	}
	return &MessageOpGetMore{
		Header:             header,
		Zero:               zero,
		FullCollectionName: fullCollectionName,
		NumberToReturn:     numberToReturn,
		CursorID:           cursorID,
		bytes:              append(header.bytes[:], payload...),
	}, nil
}

// ToWire converts this message to wire protocol message bytes.
func (m *MessageOpGetMore) ToWire(responseTo int32) (dst []byte) {
	var idx int32
	idx, dst = wiremessage.AppendHeaderStart(dst, m.Header.RequestID, responseTo, wiremessage.OpGetMore)
	dst = appendInt32(dst, m.Zero)
	dst = appendCString(dst, m.FullCollectionName)
	dst = appendInt32(dst, m.NumberToReturn)
	dst = appendInt64(dst, m.CursorID)
	return bsoncore.UpdateLength(dst, idx, int32(len(dst[idx:])))
}
