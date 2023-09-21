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

package inventory

import (
	"sync"
	"sync/atomic"

	"github.com/gravitational/teleport/api/types"
)

// serviceCounter will count services seen in the inventory.
type serviceCounter struct {
	countMap sync.Map
}

// counts returns the count of each service seen in the counter.
func (s *serviceCounter) counts() map[types.SystemRole]uint64 {
	counts := map[types.SystemRole]uint64{}
	s.countMap.Range(func(key, value any) bool {
		counts[key.(types.SystemRole)] = value.(*atomic.Uint64).Load()
		return true
	})

	return counts
}

// increment will increment the counter for a service.
func (s *serviceCounter) increment(service types.SystemRole) {
	s.load(service).Add(1)
}

// decrement will decrement the counter for a service.
func (s *serviceCounter) decrement(service types.SystemRole) {
	// refer to the docs for atomic.AddUint64 for why this works as a decrement.
	s.load(service).Add(^uint64(0))
}

// get will return the value of a counter for a service.
func (s *serviceCounter) get(service types.SystemRole) uint64 {
	if result, ok := s.countMap.Load(service); ok {
		return result.(*atomic.Uint64).Load()
	}
	return 0
}

// load will load the underlying atomic value in the sync map. This should
// only be used within the service counter.
func (s *serviceCounter) load(service types.SystemRole) *atomic.Uint64 {
	if result, ok := s.countMap.Load(service); ok {
		return result.(*atomic.Uint64)
	}
	result, _ := s.countMap.LoadOrStore(service, &atomic.Uint64{})
	return result.(*atomic.Uint64)
}
