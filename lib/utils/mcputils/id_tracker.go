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
	"sync"

	"github.com/gravitational/trace"
	"github.com/hashicorp/golang-lru/v2/simplelru"
	"github.com/mark3labs/mcp-go/mcp"
)

// IDTracker tracks message information like method based on ID. IDTracker
// internally uses an LRU cache to keep track the last X messages to avoid
// growing infinitely. IDTracker is safe for concurrent use.
type IDTracker struct {
	mu       sync.Mutex
	lruCache *simplelru.LRU[mcp.RequestId, mcp.MCPMethod]
}

// NewIDTracker creates a new IDTracker with provided maximum size.
func NewIDTracker(size int) (*IDTracker, error) {
	lruCache, err := simplelru.NewLRU[mcp.RequestId, mcp.MCPMethod](size, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &IDTracker{
		lruCache: lruCache,
	}, nil
}

// PushRequest tracks a request. Returns true if the request has been added to
// cache.
func (t *IDTracker) PushRequest(msg *JSONRPCRequest) bool {
	if msg == nil || msg.ID.IsNil() || msg.Method == "" {
		return false
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lruCache.Add(msg.ID, msg.Method)
	return true
}

// PopByID retrieves the tracked information and remove it from the tracker.
func (t *IDTracker) PopByID(id mcp.RequestId) (mcp.MCPMethod, bool) {
	if id.IsNil() {
		return "", false
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	retrieved, ok := t.lruCache.Get(id)
	if !ok {
		return "", false
	}
	t.lruCache.Remove(id)
	return retrieved, true
}

// Len returns the size of the tracker cache.
func (t *IDTracker) Len() int {
	return t.lruCache.Len()
}
