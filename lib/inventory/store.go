/*
Copyright 2022 Gravitational, Inc.

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

package inventory

import (
	"hash/maphash"
	"sync"

	"go.uber.org/atomic"
)

const SHARDS = 128

// Store is a sharded key-value store that manages inventory control handles.
//
// note: the sharding here may not really be necessary. sharding does improve perf under high
// combined read/write load, but perf isn't terrible without sharding (~2.5s vs ~0.5s
// in the basic benchmark). we've previously seen outages due to contention on a similar
// structure in the event fanout system, and I opted to shard here as well since the expected
// load on startup is similar to that system (though the fanout system performs more memory
// allocation under lock, which I suspect is why it has worse single-lock perf despite
// being otherwise quite similar).
type Store struct {
	// shards is an array of individually locked mappings of the form serverID -> handle(s).
	// keys are assigned to shards via a maphash.
	shards [SHARDS]*shard
	// seed is used to deterministically calculate key hashes in order to select
	// the correct shard for a given key.
	seed maphash.Seed
}

// NewStore creates a new inventory control handle store.
func NewStore() *Store {
	var shards [SHARDS]*shard
	for i := range shards {
		shards[i] = newShard()
	}
	return &Store{
		shards: shards,
		seed:   maphash.MakeSeed(),
	}
}

// Get attempts to load a handle for the given server ID.
// note: if multiple handles exist for a given server, the returned handle
// is selected pseudorandomly from the available set.
func (s *Store) Get(serverID string) (handle UpstreamHandle, ok bool) {
	return s.getShard(serverID).get(serverID)
}

// Insert adds a new handle to the store.
func (s *Store) Insert(handle UpstreamHandle) {
	s.getShard(handle.Hello().ServerID).insert(handle)
}

// Remove removes the handle from the store.
func (s *Store) Remove(handle UpstreamHandle) {
	s.getShard(handle.Hello().ServerID).remove(handle)
}

// Iter iterates across all handles registered with this store.
// note: if multiple handles are registered for a given server, only
// one handle is selected pseudorandomly to be observed.
func (s *Store) Iter(fn func(UpstreamHandle)) {
	for _, shard := range s.shards {
		shard.iter(fn)
	}
}

// Len returns the count of currently registered servers (servers with
// multiple handles registered still only count as one).
func (s *Store) Len() int {
	var total int
	for _, shard := range s.shards {
		total += shard.Len()
	}
	return total
}

// getShard loads the shard for the given serverID.
func (s *Store) getShard(serverID string) *shard {
	var h maphash.Hash
	// all hashes must use the same seed in order for subsequent calls
	// to land at the same shard for a given serverID.
	h.SetSeed(s.seed)
	h.WriteString(serverID)
	idx := h.Sum64() % uint64(SHARDS)
	return s.shards[int(idx)]
}

type shard struct {
	// rw protects inner mapping
	rw sync.RWMutex
	// mapping of server id => handle(s).
	m map[string]*entry
}

func newShard() *shard {
	return &shard{
		m: make(map[string]*entry),
	}
}

type entry struct {
	// ct is atomically incremented as a means to pseudorandomly distribute
	// load for instances that have multiple handles registered.
	ct      *atomic.Uint64
	handles []UpstreamHandle
}

func (s *shard) get(serverID string) (handle UpstreamHandle, ok bool) {
	s.rw.RLock()
	defer s.rw.RUnlock()
	entry, ok := s.m[serverID]
	if !ok {
		return nil, false
	}
	idx := entry.ct.Inc() % uint64(len(entry.handles))
	handle = entry.handles[int(idx)]
	return handle, true
}

func (s *shard) iter(fn func(UpstreamHandle)) {
	s.rw.RLock()
	defer s.rw.RUnlock()
	for _, entry := range s.m {
		idx := entry.ct.Inc() % uint64(len(entry.handles))
		handle := entry.handles[int(idx)]
		fn(handle)
	}
}

func (s *shard) insert(handle UpstreamHandle) {
	s.rw.Lock()
	defer s.rw.Unlock()
	e, ok := s.m[handle.Hello().ServerID]
	if !ok {
		e = &entry{
			ct: atomic.NewUint64(0),
		}
		s.m[handle.Hello().ServerID] = e
	}
	e.handles = append(e.handles, handle)
}

func (s *shard) remove(handle UpstreamHandle) {
	s.rw.Lock()
	defer s.rw.Unlock()
	e, ok := s.m[handle.Hello().ServerID]
	if !ok {
		return
	}
	for i, h := range e.handles {
		if handle == h {
			e.handles = swapRemove(e.handles, i)
			return
		}
	}
}

func swapRemove(s []UpstreamHandle, i int) []UpstreamHandle {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

func (s *shard) Len() int {
	s.rw.RLock()
	defer s.rw.RUnlock()
	return len(s.m)
}
