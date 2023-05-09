/*

 Copyright 2023 Gravitational, Inc.

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

package web

import (
	"encoding/json"
	"io"
	"net"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gravitational/trace"
)

// WSConn is a gorilla/websocket minimal interface used by our web implementation.
type WSConn interface {
	Close() error

	LocalAddr() net.Addr
	RemoteAddr() net.Addr

	WriteControl(messageType int, data []byte, deadline time.Time) error
	WriteMessage(messageType int, data []byte) error
	NextReader() (messageType int, r io.Reader, err error)
	ReadMessage() (messageType int, p []byte, err error)

	SetReadDeadline(t time.Time) error
	PingHandler() func(appData string) error
	SetPingHandler(h func(appData string) error)
	PongHandler() func(appData string) error
	SetPongHandler(h func(appData string) error)
}

// outEnvelope is an envelope used to wrap messages send back to the client connected over WS.
type outEnvelope struct {
	NodeID  string `json:"node_id"`
	Type    string `json:"type"`
	Payload []byte `json:"payload"`
}

type payloadWriter struct {
	nodeID string
	// output name, can be stdout, stderr or teleport-error.
	outputName string
	// stream is the underlying stream.
	stream io.Writer
}

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
	return len(b), err
}

func newPayloadWriter(nodeID, outputName string, stream io.Writer) *payloadWriter {
	return &payloadWriter{
		nodeID:     nodeID,
		outputName: outputName,
		stream:     stream,
	}
}

// noopCloserWS is a wrapper around websocket.Conn which does nothing on Close().
// This struct is used to prevent WS being closed by wrapping stream.
type noopCloserWS struct {
	*websocket.Conn
}

// Close does nothing.
func (ws *noopCloserWS) Close() error {
	return nil
}

func removeDuplicates(hosts []hostInfo) []hostInfo {
	if len(hosts) <= 1 {
		return hosts
	}

	unique := make(map[hostInfo]struct{}, len(hosts))
	uniqueHosts := make([]hostInfo, 0, len(hosts))

	for _, h := range hosts {
		if _, ok := unique[h]; ok {
			continue
		}
		unique[h] = struct{}{}
		uniqueHosts = append(uniqueHosts, h)
	}

	return uniqueHosts
}
