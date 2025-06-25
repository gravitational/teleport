/*
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

package mcputils

import (
	"context"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
)

// MessageWriter defines an interface for writing JSON RPC messages.
type MessageWriter interface {
	// WriteMessage writes an JSON RPC message.
	WriteMessage(context.Context, mcp.JSONRPCMessage) error
}

// SyncMessageWriter process goroutine safety for MessageWriter
type SyncMessageWriter struct {
	w  MessageWriter
	mu sync.Mutex
}

// NewSyncMessageWriter returns a SyncMessageWriter.
func NewSyncMessageWriter(w MessageWriter) *SyncMessageWriter {
	return &SyncMessageWriter{
		w: w,
	}
}

// WriteMessage acquires a lock then writes the message.
func (s *SyncMessageWriter) WriteMessage(ctx context.Context, msg mcp.JSONRPCMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.WriteMessage(ctx, msg)
}
