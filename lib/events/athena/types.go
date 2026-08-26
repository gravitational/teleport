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

package athena

import (
	"time"

	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/utils"
)

type eventParquet struct {
	EventType string    `parquet:"event_type"`
	EventTime time.Time `parquet:"event_time,timestamp(millisecond)"`
	UID       string    `parquet:"uid"`
	SessionID string    `parquet:"session_id"`
	User      string    `parquet:"user"`
	EventData string    `parquet:"event_data"`
}

func auditEventToParquet(event apievents.AuditEvent) (*eventParquet, error) {
	jsonBlob, err := utils.FastMarshal(event)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &eventParquet{
		EventType: event.GetType(),
		EventTime: event.GetTime().UTC(),
		UID:       event.GetID(),
		SessionID: events.GetSessionID(event),
		User:      events.GetTeleportUser(event),
		EventData: string(jsonBlob),
	}, nil
}

func auditEventFromParquet(event eventParquet) (apievents.AuditEvent, error) {
	var fields events.EventFields
	if err := utils.FastUnmarshal([]byte(event.EventData), &fields); err != nil {
		return nil, trace.Wrap(err, "failed to unmarshal event, %s", event.EventData)
	}
	e, err := events.FromEventFields(fields)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return e, nil
}
