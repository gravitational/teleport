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

package web

import (
	"encoding/json"
	"io"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/web/terminal"
)

const (
	EnvelopeTypeStdout  = "stdout"
	envelopeTypeStderr  = "stderr"
	envelopeTypeError   = "teleport-error"
	envelopeTypeSummary = "summary"
)

// outEnvelope is an envelope used to wrap messages send back to the client connected over WS.
type outEnvelope struct {
	NodeID  string `json:"node_id"`
	Type    string `json:"type"`
	Payload []byte `json:"payload"`
}

// payloadWriter is a wrapper around io.Writer, which wraps the given bytes into
// outEnvelope and writes it to the underlying stream.
type payloadWriter struct {
	nodeID string
	// output name, can be stdout, stderr, teleport-error or summary.
	outputName string
	// stream is the underlying stream.
	stream io.Writer
}

// Write writes the given bytes to the underlying stream.
func (p *payloadWriter) Write(b []byte) (n int, err error) {
	out := &outEnvelope{
		NodeID:  p.nodeID,
		Type:    p.outputName,
		Payload: b,
	}
	data, err := json.Marshal(out)
	if err != nil {
		return 0, trace.Wrap(err)
	}

	_, err = p.stream.Write(data)
	// return the size of the original message as a message send over stream
	// is larger due to json marshaling and envelope.
	return len(b), trace.Wrap(err)
}

func newPayloadWriter(nodeID, outputName string, stream io.Writer) *payloadWriter {
	return &payloadWriter{
		nodeID:     nodeID,
		outputName: outputName,
		stream:     stream,
	}
}

// noopCloserWS is a wrapper around websocket.Conn, which does nothing on Close().
// This struct is used to prevent WS being closed by wrapping stream.
// Currently, it is being used in Command web handler to prevent WS being closed
// by any underlying code as we want to keep the connection open until the command
// is executed on all nodes and a single failure should not close the connection.
type noopCloserWS struct {
	terminal.WSConn
}

// Close does nothing.
func (ws *noopCloserWS) Close() error {
	return nil
}

// syncRWWSConn is a wrapper around websocket.Conn, which serializes
// read and write to a web socket connection. This is needed to prevent
// a race conditions and panics in gorilla/websocket.
// Details https://pkg.go.dev/github.com/gorilla/websocket#hdr-Concurrency
// This struct does not lock SetReadDeadline() as the SetReadDeadline()
// is called from the pong handler, which is interanlly called on ReadMessage()
// according to https://pkg.go.dev/github.com/gorilla/websocket#hdr-Control_Messages
// This would prevent the pong handler from being called.
type syncRWWSConn struct {
	// WSConn the underlying websocket connection.
	terminal.WSConn
	// rmtx is a mutex used to serialize reads.
	rmtx sync.Mutex
	// wmtx is a mutex used to serialize writes.
	wmtx sync.Mutex
}

func (s *syncRWWSConn) WriteMessage(messageType int, data []byte) error {
	s.wmtx.Lock()
	defer s.wmtx.Unlock()
	return s.WSConn.WriteMessage(messageType, data)
}

func (s *syncRWWSConn) ReadMessage() (messageType int, p []byte, err error) {
	s.rmtx.Lock()
	defer s.rmtx.Unlock()
	return s.WSConn.ReadMessage()
}

func newBufferedPayloadWriter(pw *payloadWriter, buffer *summaryBuffer) *bufferedPayloadWriter {
	return &bufferedPayloadWriter{
		payloadWriter: pw,
		buffer:        buffer,
	}
}

type bufferedPayloadWriter struct {
	*payloadWriter
	buffer *summaryBuffer
}

func (bp *bufferedPayloadWriter) Write(data []byte) (int, error) {
	bp.buffer.Write(bp.nodeID, data)
	return bp.payloadWriter.Write(data)
}

func newSummaryBuffer(capacity int) *summaryBuffer {
	return &summaryBuffer{
		buffer:            make(map[string][]byte),
		remainingCapacity: capacity,
		invalid:           false,
		mutex:             sync.Mutex{},
	}
}

type summaryBuffer struct {
	buffer            map[string][]byte
	remainingCapacity int
	invalid           bool
	// mutex protects all members of the struct and must be acquired before
	// performing any read or write operation
	mutex sync.Mutex
}

func (b *summaryBuffer) Write(node string, data []byte) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if b.invalid {
		return
	}
	if len(data) > b.remainingCapacity {
		// We're out of capacity, not all content will be written to the buffer
		// it should not be used anymore
		b.invalid = true
		return
	}
	b.buffer[node] = append(b.buffer[node], data...)
	b.remainingCapacity -= len(data)
}

// Export returns the buffer content and a whether the Export is valid.
// Exporting the buffer can only happen once.
func (b *summaryBuffer) Export() (map[string][]byte, bool) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	if b.invalid {
		return nil, false
	}
	b.invalid = true
	return b.buffer, len(b.buffer) != 0
}
