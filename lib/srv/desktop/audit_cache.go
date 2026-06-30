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

package desktop

import (
	"sync"

	"github.com/gravitational/trace"
)

type directoryID uint32
type completionID uint32
type directoryName string

type readRequestInfo struct {
	path   string
	offset uint64
}

type writeRequestInfo readRequestInfo

// maxAuditCacheItems is the maximum number of items we want
// to allow in a single sharedDirectoryAuditCacheEntry.
//
// It's not a precise value, just one that should prevent the
// cache from growing too large due to a misbehaving client.
const maxAuditCacheItems = 2000

// totalItems returns the total number of items held in the cache.
// The caller should hold a lock on the cache prior to calling this method.
func (e *sharedDirectoryAuditCache) totalItems() int {
	sum := 0
	for _, cache := range e.directories {
		// +1 because a directoryAuditCache with no read/write info
		// items should still count as at least one entry.
		sum += cache.totalItems() + 1
	}
	return sum
}

func newSharedDirectoryAuditCache() sharedDirectoryAuditCache {
	return sharedDirectoryAuditCache{
		directories: make(map[directoryID]directoryAuditCache),
	}
}

type sharedDirectoryAuditCache struct {
	sync.Mutex
	directories map[directoryID]directoryAuditCache
}

type directoryAuditCache struct {
	name              directoryName
	readRequestCache  map[completionID]readRequestInfo
	writeRequestCache map[completionID]writeRequestInfo
}

func (d directoryAuditCache) totalItems() int {
	return len(d.readRequestCache) + len(d.writeRequestCache)
}

func newdirectoryAuditCache(name directoryName) directoryAuditCache {
	return directoryAuditCache{
		name:              name,
		readRequestCache:  make(map[completionID]readRequestInfo),
		writeRequestCache: make(map[completionID]writeRequestInfo),
	}
}

func (c *sharedDirectoryAuditCache) NewDirectory(did directoryID, name directoryName) error {
	c.Lock()
	defer c.Unlock()

	if c.totalItems() >= maxAuditCacheItems {
		return trace.LimitExceeded("audit cache exceeded maximum size")
	}

	c.directories[did] = newdirectoryAuditCache(name)
	return nil
}

func (c *sharedDirectoryAuditCache) RemoveDirectory(did directoryID) bool {
	c.Lock()
	defer c.Unlock()

	_, ok := c.directories[did]
	delete(c.directories, did)
	return ok
}

// SetReadRequestInfo returns a non-nil error if the audit cache exceeds its maximum size.
// It is the responsibility of the caller to terminate the session if a non-nil error is returned.
func (c *sharedDirectoryAuditCache) SetReadRequestInfo(did directoryID, cid completionID, info readRequestInfo) error {
	c.Lock()
	defer c.Unlock()

	if c.totalItems() >= maxAuditCacheItems {
		return trace.LimitExceeded("audit cache exceeded maximum size")
	}

	if directory, exists := c.directories[did]; exists {
		directory.readRequestCache[cid] = info
		return nil
	}

	return trace.NotFound("no such directory with id %d ", did)
}

// SetWriteRequestInfo returns a non-nil error if the audit cache exceeds its maximum size.
// It is the responsibility of the caller to terminate the session if a non-nil error is returned.
func (c *sharedDirectoryAuditCache) SetWriteRequestInfo(did directoryID, cid completionID, info writeRequestInfo) error {
	c.Lock()
	defer c.Unlock()

	if c.totalItems() >= maxAuditCacheItems {
		return trace.LimitExceeded("audit cache exceeded maximum size")
	}

	if directory, exists := c.directories[did]; exists {
		directory.writeRequestCache[cid] = info
		return nil
	}

	return trace.NotFound("no such directory with id %d ", did)
}

func (c *sharedDirectoryAuditCache) GetName(did directoryID) (directoryName, bool) {
	c.Lock()
	defer c.Unlock()

	directory, ok := c.directories[did]
	return directory.name, ok
}

// TakeReadRequestInfo gets the readRequestInfo for completion ID cid,
// removing the readRequestInfo from the cache in the process.
func (c *sharedDirectoryAuditCache) TakeReadRequestInfo(did directoryID, cid completionID) (info readRequestInfo, name directoryName, ok bool) {
	c.Lock()
	defer c.Unlock()

	if directory, exists := c.directories[did]; exists {
		info, ok = directory.readRequestCache[cid]
		name = directory.name
		if ok {
			delete(directory.readRequestCache, cid)
		}
	}
	return
}

// TakeWriteRequestInfo gets the writeRequestInfo for completion ID cid,
// removing the writeRequestInfo from the cache in the process.
func (c *sharedDirectoryAuditCache) TakeWriteRequestInfo(did directoryID, cid completionID) (info writeRequestInfo, name directoryName, ok bool) {
	c.Lock()
	defer c.Unlock()

	if directory, exists := c.directories[did]; exists {
		info, ok = directory.writeRequestCache[cid]
		name = directory.name
		if ok {
			delete(directory.writeRequestCache, cid)
		}
	}
	return
}
