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

// MessageOpKillCursors represents parsed OP_KILL_CURSORS wire message.
//
// https://docs.mongodb.com/manual/reference/mongodb-wire-protocol/#op_kill_cursors
//
//	struct {
//	    MsgHeader header;            // standard message header
//	    int32     ZERO;              // 0 - reserved for future use
//	    int32     numberOfCursorIDs; // number of cursorIDs in message
//	    int64*    cursorIDs;         // sequence of cursorIDs to close
//	}
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
	for range int(numberOfCursorIDs) {
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
