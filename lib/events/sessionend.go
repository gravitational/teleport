/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

import (
	"context"

	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
)

// FindSessionEndEvent streams session events to find the session end event for the given session ID.
// It returns:
// - SessionEnd
// - DatabaseSessionEnd
// - WindowsDesktopSessionEnd
// - AppSessionEnd
// - MCPSessionEnd,
// or nil if none is found.
// TODO(tigrato): Revisit this approach for large sessions, as it's highly inefficient.
// Instead, consider downloading the last few parts of the recording to find the session end event
// instead of streaming all events.
func FindSessionEndEvent(ctx context.Context, streamer SessionStreamer, sessionID session.ID) (apievents.AuditEvent, error) {
	switch {
	case streamer == nil:
		return nil, trace.BadParameter("session streamer is required")
	case sessionID == "":
		return nil, trace.BadParameter("session ID is required")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	eventsCh, errCh := streamer.StreamSessionEvents(ctx, sessionID, 0)
	for {
		select {
		case event, ok := <-eventsCh:
			if !ok {
				return nil, trace.NotFound("session end event not found")
			}
			switch e := event.(type) {
			case *apievents.WindowsDesktopSessionEnd:
				return e, nil
			case *apievents.SessionEnd:
				return e, nil
			case *apievents.DatabaseSessionEnd:
				return e, nil
			case *apievents.AppSessionEnd:
				return e, nil
			case *apievents.MCPSessionEnd:
				return e, nil
			}
		case err := <-errCh:
			return nil, trace.Wrap(err)
		case <-ctx.Done():
			return nil, trace.Wrap(ctx.Err())
		}
	}
}
