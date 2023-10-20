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

package eventstest

import (
	"context"
	"time"

	"github.com/gravitational/trace"

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

func (f fakeStreamer) GetSessionChunk(namespace string, sid session.ID, offsetBytes, maxBytes int) ([]byte, error) {
	return nil, trace.NotImplemented("GetSessionChunk")
}

func (f fakeStreamer) GetSessionEvents(namespace string, sid session.ID, after int) ([]events.EventFields, error) {
	return nil, trace.NotImplemented("GetSessionEvents")
}
