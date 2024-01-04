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

package events

import (
	"context"

	"github.com/gravitational/trace"

	apievents "github.com/gravitational/teleport/api/types/events"
)

// NewMultiLog returns a new instance of a multi logger
func NewMultiLog(loggers ...AuditLogger) (*MultiLog, error) {
	emitters := make([]apievents.Emitter, 0, len(loggers))
	for _, logger := range loggers {
		emitter, ok := logger.(apievents.Emitter)
		if !ok {
			return nil, trace.BadParameter("expected emitter, got %T", logger)
		}
		emitters = append(emitters, emitter)
	}
	return &MultiLog{
		MultiEmitter: NewMultiEmitter(emitters...),
		loggers:      loggers,
	}, nil
}

// MultiLog is a logger that fan outs write operations
// to all loggers, and performs all read and search operations
// on the first logger that implements the operation
type MultiLog struct {
	loggers []AuditLogger
	*MultiEmitter
}

// Close releases connections and resources associated with logs if any
func (m *MultiLog) Close() error {
	var errors []error
	for _, log := range m.loggers {
		errors = append(errors, log.Close())
	}
	return trace.NewAggregate(errors...)
}

// SearchEvents is a flexible way to find events.
//
// Event types to filter can be specified and pagination is handled by an iterator key that allows
// a query to be resumed.
//
// The only mandatory requirement is a date range (UTC).
//
// This function may never return more than 1 MiB of event data.
func (m *MultiLog) SearchEvents(ctx context.Context, req SearchEventsRequest) (events []apievents.AuditEvent, lastKey string, err error) {
	for _, log := range m.loggers {
		events, lastKey, err := log.SearchEvents(ctx, req)
		if !trace.IsNotImplemented(err) {
			return events, lastKey, err
		}
	}
	return events, lastKey, err
}

// SearchSessionEvents is a flexible way to find session events.
// Only session.end and windows.desktop.session.end events are returned by this function.
// This is used to find completed sessions.
//
// Event types to filter can be specified and pagination is handled by an iterator key that allows
// a query to be resumed.
func (m *MultiLog) SearchSessionEvents(ctx context.Context, req SearchSessionEventsRequest) (events []apievents.AuditEvent, lastKey string, err error) {
	for _, log := range m.loggers {
		events, lastKey, err = log.SearchSessionEvents(ctx, req)
		if !trace.IsNotImplemented(err) {
			return events, lastKey, err
		}
	}
	return events, lastKey, err
}
