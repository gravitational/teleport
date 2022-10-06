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

import "sync"

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

type sharedDirectoryAuditCacheEntry struct {
	nameCache         map[directoryID]directoryName
	readRequestCache  map[completionID]readRequestInfo
	writeRequestCache map[completionID]writeRequestInfo
}

func (e *sharedDirectoryAuditCacheEntry) init() {
	e.nameCache = make(map[directoryID]directoryName)
	e.readRequestCache = make(map[completionID]readRequestInfo)
	e.writeRequestCache = make(map[completionID]writeRequestInfo)

}

// sharedDirectoryAuditCache is a data structure for caching information
// from shared directory TDP messages so that it can be used later for
// creating shared directory audit events.
type sharedDirectoryAuditCache struct {
	m map[sessionID]sharedDirectoryAuditCacheEntry
	sync.RWMutex
}

func NewSharedDirectoryAuditCache() sharedDirectoryAuditCache {
	return sharedDirectoryAuditCache{
		m: make(map[sessionID]sharedDirectoryAuditCacheEntry),
	}
}

func (c *sharedDirectoryAuditCache) get(sid sessionID) (entry sharedDirectoryAuditCacheEntry, ok bool) {
	c.RLock()
	defer c.RUnlock()

	entry, ok = c.m[sid]
	return
}

// getInitialized gets an initialized sharedDirectoryAuditCacheEntry, mapped to sid.
// If an entry at sid already exists, it returns that, otherwise it returns an empty, initialized entry.
//
// This should be called at the start of any SetX method to ensure that we never get a
// "panic: assignment to entry in nil map".
func (c *sharedDirectoryAuditCache) getInitialized(sid sessionID) (entry sharedDirectoryAuditCacheEntry) {
	entry, ok := c.get(sid)

	if !ok {
		c.Lock()
		defer c.Unlock()
		entry.init()
		c.m[sid] = entry
	}

	return entry
}

func (c *sharedDirectoryAuditCache) SetName(sid sessionID, did directoryID, name directoryName) {
	entry := c.getInitialized(sid)

	c.Lock()
	defer c.Unlock()
	entry.nameCache[did] = name
}

func (c *sharedDirectoryAuditCache) SetReadRequestInfo(sid sessionID, cid completionID, info readRequestInfo) {
	entry := c.getInitialized(sid)

	c.Lock()
	defer c.Unlock()
	entry.readRequestCache[cid] = info
}

func (c *sharedDirectoryAuditCache) SetWriteRequestInfo(sid sessionID, cid completionID, info writeRequestInfo) {
	entry := c.getInitialized(sid)

	c.Lock()
	defer c.Unlock()
	entry.writeRequestCache[cid] = info
}

func (c *sharedDirectoryAuditCache) GetName(sid sessionID, did directoryID) (name directoryName, ok bool) {
	c.RLock()
	defer c.RUnlock()

	entry, ok := c.get(sid)
	if !ok {
		return
	}

	name, ok = entry.nameCache[did]
	return
}

func (c *sharedDirectoryAuditCache) GetReadRequestInfo(sid sessionID, cid completionID) (info readRequestInfo, ok bool) {
	c.RLock()
	defer c.RUnlock()

	entry, ok := c.get(sid)
	if !ok {
		return
	}

	info, ok = entry.readRequestCache[cid]
	return
}

func (c *sharedDirectoryAuditCache) GetWriteRequestInfo(sid sessionID, cid completionID) (info writeRequestInfo, ok bool) {
	c.RLock()
	defer c.RUnlock()

	entry, ok := c.get(sid)
	if !ok {
		return
	}

	info, ok = entry.writeRequestCache[cid]
	return
}

func (c *sharedDirectoryAuditCache) Delete(sid sessionID) {
	delete(c.m, sid)
}
