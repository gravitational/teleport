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

package desktop

import (
	"sync"

	"github.com/gravitational/trace"
)

type sessionID string
type directoryID uint32
type completionID uint32
type directoryName string

type readRequestInfo struct {
	directoryID directoryID
	path        string
	offset      uint64
}

type writeRequestInfo readRequestInfo

const (
	// entryMaxItems is the maximum number of items we want
	// to allow in a single sharedDirectoryAuditCacheEntry.
	//
	// It's not a precise value, just one that should give us
	// prevent the cache from growing too large due to a
	// misbehaving client.
	entryMaxItems = 2000
)

type sharedDirectoryAuditCacheEntry struct {
	nameCache         map[directoryID]directoryName
	readRequestCache  map[completionID]readRequestInfo
	writeRequestCache map[completionID]writeRequestInfo
}

func newSharedDirectoryAuditCacheEntry() *sharedDirectoryAuditCacheEntry {
	return &sharedDirectoryAuditCacheEntry{
		nameCache:         make(map[directoryID]directoryName),
		readRequestCache:  make(map[completionID]readRequestInfo),
		writeRequestCache: make(map[completionID]writeRequestInfo),
	}
}

// totalItems returns the total numbewr of items held in the entry.
func (e *sharedDirectoryAuditCacheEntry) totalItems() int {
	return len(e.nameCache) + len(e.readRequestCache) + len(e.writeRequestCache)
}

// sharedDirectoryAuditCache is a data structure for caching information
// from shared directory TDP messages so that it can be used later for
// creating shared directory audit events.
type sharedDirectoryAuditCache struct {
	m map[sessionID]*sharedDirectoryAuditCacheEntry
	sync.Mutex
}

func newSharedDirectoryAuditCache() sharedDirectoryAuditCache {
	return sharedDirectoryAuditCache{
		m: make(map[sessionID]*sharedDirectoryAuditCacheEntry),
	}
}

// getInitialized gets an initialized sharedDirectoryAuditCacheEntry, mapped to sid.
// If an entry at sid already exists, it returns that, otherwise it returns an empty, initialized entry.
//
// This should be called at the start of any SetX method to ensure that we never get a
// "panic: assignment to entry in nil map".
//
// It is the responsibility of the caller to ensure that it has obtained the Lock before calling
// getInitialized, and that it calls Unlock once the entry returned by getInitialized is no longer going to
// be modified or otherwise used.
func (c *sharedDirectoryAuditCache) getInitialized(sid sessionID) (entry *sharedDirectoryAuditCacheEntry) {
	entry, ok := c.m[sid]

	if !ok {
		entry = newSharedDirectoryAuditCacheEntry()
		c.m[sid] = entry
	}

	return entry
}

// SetName returns a non-nil error if the audit cache entry for sid exceeds its maximum size.
// It is the responsibility of the caller to terminate the session if a non-nil error is returned.
func (c *sharedDirectoryAuditCache) SetName(sid sessionID, did directoryID, name directoryName) error {
	c.Lock()
	defer c.Unlock()

	entry := c.getInitialized(sid)
	if entry.totalItems() >= entryMaxItems {
		return trace.LimitExceeded("audit cache for sessionID(%v) exceeded maximum size", sid)
	}

	entry.nameCache[did] = name

	return nil
}

// SetReadRequestInfo returns a non-nil error if the audit cache entry for sid exceeds its maximum size.
// It is the responsibility of the caller to terminate the session if a non-nil error is returned.
func (c *sharedDirectoryAuditCache) SetReadRequestInfo(sid sessionID, cid completionID, info readRequestInfo) error {
	c.Lock()
	defer c.Unlock()

	entry := c.getInitialized(sid)
	if entry.totalItems() >= entryMaxItems {
		return trace.LimitExceeded("audit cache for sessionID(%v) exceeded maximum size", sid)
	}

	entry.readRequestCache[cid] = info

	return nil
}

// SetWriteRequestInfo returns a non-nil error if the audit cache entry for sid exceeds its maximum size.
// It is the responsibility of the caller to terminate the session if a non-nil error is returned.
func (c *sharedDirectoryAuditCache) SetWriteRequestInfo(sid sessionID, cid completionID, info writeRequestInfo) error {
	c.Lock()
	defer c.Unlock()

	entry := c.getInitialized(sid)
	if entry.totalItems() >= entryMaxItems {
		return trace.LimitExceeded("audit cache for sessionID(%v) exceeded maximum size", sid)
	}

	entry.writeRequestCache[cid] = info

	return nil
}

func (c *sharedDirectoryAuditCache) GetName(sid sessionID, did directoryID) (name directoryName, ok bool) {
	c.Lock()
	defer c.Unlock()

	entry, ok := c.m[sid]
	if !ok {
		return
	}

	name, ok = entry.nameCache[did]
	return
}

// TakeReadRequestInfo gets the readRequestInfo for completion id cid of session id sid,
// removing the readRequestInfo from the cache in the process.
func (c *sharedDirectoryAuditCache) TakeReadRequestInfo(sid sessionID, cid completionID) (info readRequestInfo, ok bool) {
	c.Lock()
	defer c.Unlock()

	entry, ok := c.m[sid]
	if !ok {
		return
	}

	info, ok = entry.readRequestCache[cid]
	if ok {
		delete(entry.readRequestCache, cid)
	}
	return
}

// TakeWriteRequestInfo gets the writeRequestInfo for completion id cid of session id sid,
// removing the writeRequestInfo from the cache in the process.
func (c *sharedDirectoryAuditCache) TakeWriteRequestInfo(sid sessionID, cid completionID) (info writeRequestInfo, ok bool) {
	c.Lock()
	defer c.Unlock()

	entry, ok := c.m[sid]
	if !ok {
		return
	}

	info, ok = entry.writeRequestCache[cid]
	if ok {
		delete(entry.writeRequestCache, cid)
	}
	return
}

func (c *sharedDirectoryAuditCache) Delete(sid sessionID) {
	c.Lock()
	defer c.Unlock()

	delete(c.m, sid)
}
