package test

import (
	"context"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/trace"
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
