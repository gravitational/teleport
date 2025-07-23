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

	"github.com/gravitational/trace"
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

// MessageWriterFunc defines a message writer function that implements
// MessageWriter.
type MessageWriterFunc func(context.Context, mcp.JSONRPCMessage) error

// WriteMessage writes an JSON RPC message.
func (f MessageWriterFunc) WriteMessage(ctx context.Context, msg mcp.JSONRPCMessage) error {
	return f(ctx, msg)
}

// MultiMessageWriter creates a writer that duplicates its writes to all the
// provided writers.
//
// Each write is written to each listed writer, one at a time. If a listed
// writer returns an error, that overall writes operation stops and returns the
// error; it does not continue down the list.
type MultiMessageWriter struct {
	writers []MessageWriter
}

// NewMultiMessageWriter creates a new MultiMessageWriter.
func NewMultiMessageWriter(writers ...MessageWriter) *MultiMessageWriter {
	return &MultiMessageWriter{writers: writers}
}

// WriteMessage writes the message to each listed writer, one at a time.
func (w *MultiMessageWriter) WriteMessage(ctx context.Context, msg mcp.JSONRPCMessage) error {
	for _, writer := range w.writers {
		if err := writer.WriteMessage(ctx, msg); err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
