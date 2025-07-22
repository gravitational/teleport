// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package summarizer

import (
	"context"

	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
)

// SessionSummarizer summarizes session recordings using language model
// inference.
type SessionSummarizer interface {
	// Summarize summarizes a session recording with a given ID. The
	// sessionEndEvent is optional, but should be specified if possible, as an
	// optimization to skip reading the session stream in order to find the end
	// event.
	Summarize(ctx context.Context, sessionID session.ID, sessionEndEvent AnySessionEndEvent) error
}

// AnySessionEndEvent holds a value of one of the event types supported by the
// [SessionSummarizer] as a session-ending event. By default, it holds an empty
// value (see [AnySessionEndEvent.IsEmpty]).
type AnySessionEndEvent struct {
	event *events.OneOf
}

// AnySessionEndEventFromOneOf converts an [events.OneOf] to an
// [AnySessionEndEvent] if it contains a session end event. Returns false if
// the event is not a session end event.
func AnySessionEndEventFromOneOf(e *events.OneOf) (AnySessionEndEvent, bool) {
	switch e.Event.(type) {
	case *events.OneOf_SessionEnd, *events.OneOf_DatabaseSessionEnd:
		return AnySessionEndEvent{event: e}, true
	}
	return AnySessionEndEvent{}, false
}

// IsEmpty checks if the AnySessionEndEvent holds a valid session end event.
func (e AnySessionEndEvent) IsEmpty() bool {
	return e.event == nil
}

// GetSessionEnd returns the underlying [events.SessionEnd] event if it's been
// set.
func (e AnySessionEndEvent) GetSessionEnd() *events.SessionEnd {
	return e.event.GetSessionEnd()
}

// GetDatabaseSessionEnd returns the underlying [events.DatabaseSessionEnd]
// event if it's been set.
func (e AnySessionEndEvent) GetDatabaseSessionEnd() *events.DatabaseSessionEnd {
	return e.event.GetDatabaseSessionEnd()
}
