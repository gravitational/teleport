/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

// Package player includes an API to play back recorded sessions.
package player

import (
	"fmt"
	"time"

	"github.com/gravitational/teleport/api/types/events"
)

type translater interface {
	Translate(evt events.AuditEvent) (events.AuditEvent, bool)
}

type noopTranslater struct {
}

func (n *noopTranslater) Translate(evt events.AuditEvent) (events.AuditEvent, bool) {
	return evt, true
}

type postgresTranslater struct {
	startTime time.Time
}

func (p *postgresTranslater) getDelayMiliseconds(eventTime time.Time) int64 {
	if p.startTime.IsZero() {
		return 0
	}
	return eventTime.Sub(p.startTime).Milliseconds()
}

func (p *postgresTranslater) Translate(event events.AuditEvent) (events.AuditEvent, bool) {
	switch evt := event.(type) {
	case *events.DatabaseSessionStart:
		p.startTime = evt.Time
		return &events.SessionStart{
			Metadata: evt.Metadata,
		}, true
	case *events.DatabaseSessionEnd:
		return &events.SessionEnd{
			Metadata: evt.Metadata,
		}, true

	case *events.DatabaseSessionQuery:
		var data []byte
		// append prompt =>
		// TODO add color for fun?
		data = append(data, []byte(
			fmt.Sprintf("%s=> ", evt.DatabaseName),
		)...)

		// append query
		data = append(data, []byte(evt.DatabaseQuery)...)

		// append CR LF
		data = append(data, byte(13), byte(10))

		return &events.SessionPrint{
			Metadata:          evt.Metadata,
			Data:              data,
			DelayMilliseconds: p.getDelayMiliseconds(evt.Time),
		}, true

		// TODO print properly
	case *events.PostgresRowDescription:
		var data []byte
		data = append(data, []byte("(TODO) received row description\r\n")...)
		return &events.SessionPrint{
			Metadata:          evt.Metadata,
			Data:              data,
			DelayMilliseconds: p.getDelayMiliseconds(evt.Time),
		}, true

		// TODO print properly
	case *events.PostgresDataRow:
		var data []byte
		data = append(data, []byte("(TODO) received row data:")...)
		for _, value := range evt.Values {
			data = append(data, value...)
		}
		return &events.SessionPrint{
			Metadata:          evt.Metadata,
			Data:              data,
			DelayMilliseconds: p.getDelayMiliseconds(evt.Time),
		}, true
		// TODO print properly
	case *events.PostgresCommandComplete:
		return &events.SessionPrint{
			Metadata:          evt.Metadata,
			Data:              []byte{13, 10},
			DelayMilliseconds: p.getDelayMiliseconds(evt.Time),
		}, true
	}
	return nil, false
}
