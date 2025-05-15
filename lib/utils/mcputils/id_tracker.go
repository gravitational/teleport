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
	"container/list"
	"sync"

	"github.com/mark3labs/mcp-go/mcp"
)

// IDTracker tracks message information like method based on ID. IDTracker
// internally uses a linked list to keep track the last X messages to avoid
// growing infinitely.
type IDTracker struct {
	list   *list.List
	mu     sync.Mutex
	maxLen int
}

type idTrackerItem struct {
	id     mcp.RequestId
	method mcp.MCPMethod
}

// NewIDTracker creates a new IDTracker with provided maximum size.
func NewIDTracker(maxLen int) *IDTracker {
	return &IDTracker{
		list:   list.New(),
		maxLen: maxLen,
	}
}

// Push tracks a request.
func (t *IDTracker) Push(msg *JSONRPCRequest) {
	if msg == nil || msg.ID == nil || msg.Method == "" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.list.PushBack(idTrackerItem{
		id:     msg.ID,
		method: msg.Method,
	})
	for t.list.Len() > t.maxLen {
		t.list.Remove(t.list.Front())
	}
}

// Pop retrieves the tracked information and remove it from the tracker.
func (t *IDTracker) Pop(msg *JSONRPCResponse) (mcp.MCPMethod, bool) {
	if msg == nil || msg.ID == nil {
		return "", false
	}

	t.mu.Lock()
	defer t.mu.Unlock()
	for e := t.list.Front(); e != nil; e = e.Next() {
		if item, ok := e.Value.(idTrackerItem); ok && item.id == msg.ID {
			t.list.Remove(e)
			return item.method, true
		}
	}
	return "", false
}

// Len returns the size of the tracker list.
func (t *IDTracker) Len() int {
	return t.list.Len()
}
