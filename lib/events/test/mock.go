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
