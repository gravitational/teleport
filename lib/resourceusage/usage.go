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

package resourceusage

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
)

// GetAccessRequestMonthlyUsage returns the number of access requests that have been created this month.
func GetAccessRequestMonthlyUsage(ctx context.Context, alog events.AuditLogger, now time.Time) (int, error) {
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)

	created := make(map[string]struct{})

	var results []apievents.AuditEvent
	var startKey string
	var err error
	for {
		results, startKey, err = alog.SearchEvents(ctx, events.SearchEventsRequest{
			From:       monthStart,
			To:         now,
			Order:      types.EventOrderAscending,
			EventTypes: []string{events.AccessRequestCreateEvent},
			StartKey:   startKey,
		})
		if err != nil {
			return 0, trace.Wrap(err)
		}
		for _, ev := range results {
			ev, ok := ev.(*apievents.AccessRequestCreate)
			if !ok {
				return 0, trace.BadParameter("expected *AccessRequestCreate, but got %T", ev)
			}
			id := ev.RequestID
			switch ev.GetType() {
			case events.AccessRequestCreateEvent:
				created[id] = struct{}{}
			default:
				slog.WarnContext(ctx, "Got unexpected event type", "expected_event", events.AccessRequestCreateEvent, "received_event", ev.GetType())
			}
		}
		if startKey == "" {
			break
		}
	}

	return len(created), nil
}
