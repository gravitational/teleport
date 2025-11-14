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

package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// SyncGatherers wraps a [prometheus.Gatherers] so it can be accessed
// in a thread-safe fashion. Gatherer can be added with AddGatherer.
type SyncGatherers struct {
	mutex     sync.Mutex
	gatherers prometheus.Gatherers
}

// NewSyncGatherers returns a thread-safe [prometheus.Gatherers].
func NewSyncGatherers(gatherers ...prometheus.Gatherer) *SyncGatherers {
	return &SyncGatherers{
		gatherers: gatherers,
	}
}

// AddGatherer adds a gatherer to the SyncGatherers slice.
func (s *SyncGatherers) AddGatherer(g prometheus.Gatherer) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.gatherers = append(s.gatherers, g)
}

// Gather implements [prometheus.Gatherer].
func (s *SyncGatherers) Gather() ([]*dto.MetricFamily, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.gatherers == nil {
		return nil, nil
	}
	return s.gatherers.Gather()
}
