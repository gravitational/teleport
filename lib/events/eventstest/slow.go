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
	"time"

	"github.com/gravitational/teleport/api/types/events"
)

// NewSlowEmitter creates an emitter that introduces an artificial
// delay before "emitting" (discarding) an audit event.
func NewSlowEmitter(delay time.Duration) events.Emitter {
	return &slowEmitter{
		delay: delay,
	}
}

// slowEmitter is an events.Emitter that introduces an artificial
// delay before emitting an event.
type slowEmitter struct {
	delay time.Duration
}

func (s *slowEmitter) EmitAuditEvent(ctx context.Context, event events.AuditEvent) error {
	select {
	case <-time.After(s.delay):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
