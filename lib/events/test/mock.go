// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

import (
	"context"

	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
)

type MockAuditLogSessionStreamer struct {
	events.DiscardAuditLog
	events        []apievents.AuditEvent
	verifyRequest func(events.SearchEventsRequest) error
}

func NewMockAuditLogSessionStreamer(events []apievents.AuditEvent, verifyRequest func(events.SearchEventsRequest) error) *MockAuditLogSessionStreamer {
	return &MockAuditLogSessionStreamer{
		events:        events,
		verifyRequest: verifyRequest,
	}
}

// SearchEvents implements events.AuditLogSessionStreamer
func (m *MockAuditLogSessionStreamer) SearchEvents(ctx context.Context, req events.SearchEventsRequest) ([]apievents.AuditEvent, string, error) {
	if m.verifyRequest != nil {
		if err := m.verifyRequest(req); err != nil {
			return nil, "", trace.Wrap(err)
		}
	}
	var results []apievents.AuditEvent
	for _, ev := range m.events {
		if !req.From.IsZero() && ev.GetTime().Before(req.From) {
			continue
		}
		if !req.To.IsZero() && ev.GetTime().After(req.To) {
			continue
		}
		results = append(results, ev)
	}
	return results, "", nil
}
