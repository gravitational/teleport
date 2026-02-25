/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package events

import (
	"sync"

	apievents "github.com/gravitational/teleport/api/types/events"
)

// GeneratedDynamicEventFactory creates a new zero-value instance of an audit event.
type GeneratedDynamicEventFactory func() apievents.AuditEvent

var generatedDynamicEvents struct {
	mu sync.RWMutex
	m  map[string]GeneratedDynamicEventFactory
}

// RegisterGeneratedDynamicEvent registers a generated dynamic event factory.
//
// This function is intended to be called from generated files using init().
func RegisterGeneratedDynamicEvent(eventType string, factory GeneratedDynamicEventFactory) {
	if eventType == "" {
		panic("events: event type is required")
	}
	if factory == nil {
		panic("events: dynamic event factory is nil")
	}

	generatedDynamicEvents.mu.Lock()
	defer generatedDynamicEvents.mu.Unlock()

	if generatedDynamicEvents.m == nil {
		generatedDynamicEvents.m = make(map[string]GeneratedDynamicEventFactory)
	}
	if _, exists := generatedDynamicEvents.m[eventType]; exists {
		panic("events: duplicate generated dynamic event for type " + eventType)
	}
	generatedDynamicEvents.m[eventType] = factory
}

// testEvent holds test event metadata for generated event types.
type testEvent struct {
	eventType string
	eventCode string
	event     apievents.AuditEvent
}

var generatedTestEvents struct {
	mu sync.RWMutex
	es []testEvent
}

// RegisterGeneratedTestEvent registers a test event entry for generated event types.
//
// This function is intended to be called from generated files using init().
func RegisterGeneratedTestEvent(te testEvent) {
	generatedTestEvents.mu.Lock()
	defer generatedTestEvents.mu.Unlock()

	generatedTestEvents.es = append(generatedTestEvents.es, te)
}
