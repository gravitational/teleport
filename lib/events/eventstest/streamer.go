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

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
)

// NewFakeStreamer returns a session streamer that streams the provided events, sending one
// event per interval. An interval of 0 sends the events immediately, throttled only by the
// ability of the receiver to keep up.
func NewFakeStreamer(events []apievents.AuditEvent, interval time.Duration) events.SessionStreamer {
	return fakeStreamer{
		events:   events,
		interval: interval,
	}
}

type fakeStreamer struct {
	events   []apievents.AuditEvent
	interval time.Duration
}

func (f fakeStreamer) StreamSessionEvents(ctx context.Context, sessionID session.ID, startIndex int64) (chan apievents.AuditEvent, chan error) {
	errors := make(chan error, 1)
	events := make(chan apievents.AuditEvent)

	go func() {
		defer close(events)

		for _, event := range f.events {
			if f.interval != 0 {
				select {
				case <-ctx.Done():
					return
				case <-time.After(f.interval):
				}
			}

			select {
			case <-ctx.Done():
				return
			case events <- event:
			}
		}
	}()

	return events, errors
}
