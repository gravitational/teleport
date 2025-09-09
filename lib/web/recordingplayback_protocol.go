/**
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package web

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/gravitational/trace"
	"github.com/hinshun/vt10x"

	"github.com/gravitational/teleport/lib/terminal"
)

const (
	// requestHeaderSize is the size of the request header (event type, start time, end time, request ID, and current screen flag)
	requestHeaderSize = 22
	// responseHeaderSize is the size of the response header (event type, timestamp, data size, and request ID)
	responseHeaderSize = 17
)

type requestType byte

// Identifies requests coming from the client (web UI)
const (
	// requestTypeFetch requests event data
	requestTypeFetch requestType = 1
)

type responseType byte

// Response types sent back to the client
const (
	// eventTypeStart indicates the start of a response of events
	eventTypeStart responseType = 1
	// eventTypeStop indicates the stop of a response of events
	eventTypeStop responseType = 2
	// eventTypeError indicates an error
	eventTypeError responseType = 3
	// eventTypeSessionStart indicates session started
	eventTypeSessionStart responseType = 4
	// eventTypeSessionPrint contains terminal output
	eventTypeSessionPrint responseType = 5
	// eventTypeSessionEnd indicates session ended
	eventTypeSessionEnd responseType = 6
	// eventTypeResize indicates terminal resize
	eventTypeResize responseType = 7
	// eventTypeScreen contains terminal screen state
	eventTypeScreen responseType = 8
	// eventTypeBatch indicates a batch of events
	eventTypeBatch responseType = 9
)

// encodeScreenEvent encodes the current terminal screen state into a byte slice.
func encodeScreenEvent(state vt10x.TerminalState, cols, rows int, cursor vt10x.Cursor) []byte {
	var buf bytes.Buffer
	buf.Write(make([]byte, responseHeaderSize))

	terminal.VtStateToANSI(&buf, state)

	eventData := buf.Bytes()
	eventData[0] = byte(eventTypeScreen)
	binary.BigEndian.PutUint32(eventData[1:5], uint32(cols))
	binary.BigEndian.PutUint32(eventData[5:9], uint32(rows))
	binary.BigEndian.PutUint32(eventData[9:13], uint32(cursor.X))
	binary.BigEndian.PutUint32(eventData[13:17], uint32(cursor.Y))
	binary.BigEndian.PutUint32(eventData[17:21], uint32(len(eventData)-responseHeaderSize))

	return eventData
}

// encodeEvent encodes a session event into a byte slice.
func encodeEvent(buf []byte, offset int, eventType responseType, timeOffset time.Duration, data []byte, requestID int) {
	buf[offset] = byte(eventType)

	binary.BigEndian.PutUint64(buf[offset+1:offset+9], uint64(timeOffset/time.Millisecond))
	binary.BigEndian.PutUint32(buf[offset+9:offset+13], uint32(len(data)))
	binary.BigEndian.PutUint32(buf[offset+13:offset+17], uint32(requestID))

	copy(buf[offset+responseHeaderSize:], data)
}

// encodeTime encodes the start and end times into a byte slice.
func encodeTime(startTime, endTime time.Duration) []byte {
	buf := make([]byte, 16)

	binary.BigEndian.PutUint64(buf, uint64(startTime/time.Millisecond))
	binary.BigEndian.PutUint64(buf[8:], uint64(endTime/time.Millisecond))

	return buf
}

// decodeBinaryRequest decodes a binary request from the client.
func decodeBinaryRequest(data []byte) (*fetchRequest, error) {
	if len(data) != requestHeaderSize {
		return nil, trace.BadParameter("invalid request size: expected %d bytes, got %d bytes", requestHeaderSize, len(data))
	}

	req := &fetchRequest{
		requestType:          requestType(data[0]),
		startOffset:          time.Duration(binary.BigEndian.Uint64(data[1:9])) * time.Millisecond,
		endOffset:            time.Duration(binary.BigEndian.Uint64(data[9:17])) * time.Millisecond,
		requestID:            int(binary.BigEndian.Uint32(data[17:21])),
		requestCurrentScreen: data[21] == 1,
	}

	return req, nil
}

// validateRequest validates the fetch request parameters.
func validateRequest(req *fetchRequest) error {
	if req.startOffset < 0 || req.endOffset < 0 || req.endOffset < req.startOffset {
		return fmt.Errorf("invalid time range (%v, %v)", req.startOffset, req.endOffset)
	}

	if req.endOffset-req.startOffset > maxRequestRange {
		return trace.LimitExceeded("time range too large, max is %s", maxRequestRange)
	}

	return nil
}
