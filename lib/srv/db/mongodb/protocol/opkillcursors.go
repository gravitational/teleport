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

	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
	"go.mongodb.org/mongo-driver/x/mongo/driver/wiremessage"

	"github.com/gravitational/trace"
)

// MessageOpKillCursors represents parsed OP_KILL_CURSORS wire message.
//
// https://docs.mongodb.com/manual/reference/mongodb-wire-protocol/#op_kill_cursors
//
// struct {
//     MsgHeader header;            // standard message header
//     int32     ZERO;              // 0 - reserved for future use
//     int32     numberOfCursorIDs; // number of cursorIDs in message
//     int64*    cursorIDs;         // sequence of cursorIDs to close
// }
//
// OP_KILL_CURSORS is deprecated starting MongoDB 5.0 in favor of OP_MSG.
type MessageOpKillCursors struct {
	Header            MessageHeader
	Zero              int32
	NumberOfCursorIDs int32
	CursorIDs         []int64
	// bytes is the full wire message bytes (incl. header) read from the connection.
	bytes []byte
}

// GetHeader returns the wire message header.
func (m *MessageOpKillCursors) GetHeader() MessageHeader {
	return m.Header
}

// GetBytes returns the message raw bytes read from the connection.
func (m *MessageOpKillCursors) GetBytes() []byte {
	return m.bytes
}

// GetDatabase returns the command's database.
func (m *MessageOpKillCursors) GetDatabase() (string, error) {
	return "", nil
}

// GetCommand returns the message's command.
func (m *MessageOpKillCursors) GetCommand() (string, error) {
	return "killCursors", nil
}

// String returns the message string representation.
func (m *MessageOpKillCursors) String() string {
	return fmt.Sprintf("OpKillCursors(NumberOfCursorIDs=%v, CursorIDs=%v)",
		m.NumberOfCursorIDs, m.CursorIDs)
}

// MoreToCome is whether sender will send another message right after this one.
func (m *MessageOpKillCursors) MoreToCome(_ Message) bool {
	return true
}

// readOpKillCursors converts OP_KILL_CURSORS wire message bytes to a structured form.
func readOpKillCursors(header MessageHeader, payload []byte) (*MessageOpKillCursors, error) {
	zero, rem, ok := readInt32(payload)
	if !ok {
		return nil, trace.BadParameter("malformed OP_KILL_CURSORS: missing zero %v", payload)
	}
	numberOfCursorIDs, rem, ok := readInt32(rem)
	if !ok {
		return nil, trace.BadParameter("malformed OP_KILL_CURSORS: missing number of cursor IDs %v", payload)
	}
	var cursorIDs []int64
	for n := 0; n < int(numberOfCursorIDs); n++ {
		var cursorID int64
		cursorID, rem, ok = readInt64(rem)
		if !ok {
			return nil, trace.BadParameter("malformed OP_KILL_CURSORS: missing cursor ID %v", payload)
		}
		cursorIDs = append(cursorIDs, cursorID)
	}
	return &MessageOpKillCursors{
		Header:            header,
		Zero:              zero,
		NumberOfCursorIDs: numberOfCursorIDs,
		CursorIDs:         cursorIDs,
		bytes:             append(header.bytes[:], payload...),
	}, nil
}

// ToWire converts this message to wire protocol message bytes.
func (m *MessageOpKillCursors) ToWire(responseTo int32) (dst []byte) {
	var idx int32
	idx, dst = wiremessage.AppendHeaderStart(dst, m.Header.RequestID, responseTo, wiremessage.OpKillCursors)
	dst = appendInt32(dst, m.Zero)
	dst = appendInt32(dst, m.NumberOfCursorIDs)
	for _, cursorID := range m.CursorIDs {
		dst = appendInt64(dst, cursorID)
	}
	return bsoncore.UpdateLength(dst, idx, int32(len(dst[idx:])))
}
