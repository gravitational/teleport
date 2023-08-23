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

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
)

type MockAuditLog struct {
	*events.DiscardAuditLog

	Emitter       *MockRecorderEmitter
	SessionEvents []apievents.AuditEvent
}

func (m *MockAuditLog) StreamSessionEvents(ctx context.Context, sid session.ID, startIndex int64) (chan apievents.AuditEvent, chan error) {
	errors := make(chan error, 1)
	events := make(chan apievents.AuditEvent)

	go func() {
		defer close(events)

		for _, event := range m.SessionEvents {
			select {
			case <-ctx.Done():
				return
			case events <- event:
			}
		}
	}()

	return events, errors
}

func (m *MockAuditLog) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	return m.Emitter.EmitAuditEvent(ctx, event)
}
