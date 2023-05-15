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

package reversetunnel

import (
	"bytes"
	"context"
	"net"
	"sync"

	"github.com/gravitational/teleport/api/constants"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// emitConn is a wrapper for a net.Conn that emits an audit event for non-Teleport connections.
type emitConn struct {
	net.Conn
	mu       sync.RWMutex
	buffer   bytes.Buffer
	emitter  apievents.Emitter
	ctx      context.Context
	serverID string
	emitted  bool
}

func newEmitConn(ctx context.Context, conn net.Conn, emitter apievents.Emitter, serverID string) *emitConn {
	return &emitConn{
		Conn:     conn,
		emitter:  emitter,
		ctx:      ctx,
		serverID: serverID,
	}
}

func (conn *emitConn) Read(p []byte) (int, error) {
	conn.mu.RLock()
	n, err := conn.Conn.Read(p)

	// Skip buffering if already could have emitted or will never emit.
	if err != nil || conn.buffer.Len() == len(constants.ProxyHelloSignature) || conn.serverID == "" {
		conn.mu.RUnlock()
		return n, err
	}
	conn.mu.RUnlock()

	conn.mu.Lock()
	defer conn.mu.Unlock()

	remaining := len(constants.ProxyHelloSignature) - conn.buffer.Len()
	_, err = conn.buffer.Write(p[:min(n, remaining)])
	if err != nil {
		return n, err
	}

	// Only emit when we don't see the proxy hello signature in the first few bytes.
	if conn.buffer.Len() == len(constants.ProxyHelloSignature) &&
		!bytes.HasPrefix(conn.buffer.Bytes(), []byte(constants.ProxyHelloSignature)) {
		event := &apievents.SessionConnect{
			Metadata: apievents.Metadata{
				Type: events.SessionConnectEvent,
				Code: events.SessionConnectCode,
			},
			ServerMetadata: apievents.ServerMetadata{
				ServerID:   conn.serverID,
				ServerAddr: conn.LocalAddr().String(),
			},
			ConnectionMetadata: apievents.ConnectionMetadata{
				LocalAddr:  conn.LocalAddr().String(),
				RemoteAddr: conn.RemoteAddr().String(),
			},
		}
		conn.emitted = true
		go conn.emitter.EmitAuditEvent(conn.ctx, event)
	}

	return n, err
}
