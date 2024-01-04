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
