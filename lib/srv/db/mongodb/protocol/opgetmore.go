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

// MessageOpGetMore represents parsed OP_GET_MORE wire message.
//
// https://docs.mongodb.com/manual/reference/mongodb-wire-protocol/#op_get_more
//
// struct {
//     MsgHeader header;             // standard message header
//     int32     ZERO;               // 0 - reserved for future use
//     cstring   fullCollectionName; // "dbname.collectionname"
//     int32     numberToReturn;     // number of documents to return
//     int64     cursorID;           // cursorID from the OP_REPLY
// }
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
