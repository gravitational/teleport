/*
Copyright 2018-2020 Gravitational, Inc.

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
