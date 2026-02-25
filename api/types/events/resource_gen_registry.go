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

import "sync"

// GeneratedOneOfConverter converts an AuditEvent to its isOneOf_Event wrapper.
// Returns nil if the event type is not handled by this converter.
type GeneratedOneOfConverter func(e AuditEvent) isOneOf_Event

var generatedOneOfConverters struct {
	mu sync.RWMutex
	cs []GeneratedOneOfConverter
}

// RegisterGeneratedOneOf registers a generated OneOf converter.
//
// This function is intended to be called from generated files using init().
func RegisterGeneratedOneOf(fn GeneratedOneOfConverter) {
	if fn == nil {
		panic("events: OneOf converter is nil")
	}

	generatedOneOfConverters.mu.Lock()
	defer generatedOneOfConverters.mu.Unlock()

	generatedOneOfConverters.cs = append(generatedOneOfConverters.cs, fn)
}

func tryGeneratedOneOfConverters(e AuditEvent) isOneOf_Event {
	generatedOneOfConverters.mu.RLock()
	defer generatedOneOfConverters.mu.RUnlock()

	for _, fn := range generatedOneOfConverters.cs {
		if wrapper := fn(e); wrapper != nil {
			return wrapper
		}
	}
	return nil
}
