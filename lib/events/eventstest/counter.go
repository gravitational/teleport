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

package eventstest

import (
	"context"
	"sync/atomic"

	"github.com/gravitational/teleport/api/types/events"
)

// NewCountingEmitter returns an emitter that counts the number
// of events that are emitted. It is safe for concurrent use.
func NewCountingEmitter() *CounterEmitter {
	return &CounterEmitter{}
}

type CounterEmitter struct {
	count int64
}

func (c *CounterEmitter) EmitAuditEvent(ctx context.Context, event events.AuditEvent) error {
	atomic.AddInt64(&c.count, 1)
	return nil
}

// Count returns the number of events that have been emitted.
// It is safe for concurrent use.
func (c *CounterEmitter) Count() int64 {
	return atomic.LoadInt64(&c.count)
}
