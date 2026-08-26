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

package main

import (
	"bytes"
	"errors"
	"log/slog"
	"regexp"
	"testing"

	"github.com/gravitational/trace"
	"github.com/peterbourgon/diskv/v3"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/breaker"
	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
)

func TestBreakerTripErrorDetection(t *testing.T) {
	require.True(t, errors.Is(trace.Wrap(breaker.ErrStateTripped), breaker.ErrStateTripped), "wrapped ErrStateTripped should be detected")

	require.False(t, errors.Is(trace.ConnectionProblem(nil, "connection refused"), breaker.ErrStateTripped),
		"unrelated connection error should not be detected as tripped breaker")

	require.False(t, errors.Is(trace.NotFound("not found"), breaker.ErrStateTripped),
		"non-connection error should not be detected as tripped breaker")
}

func TestEventHandlerFilters(t *testing.T) {
	tests := []struct {
		name         string
		ingestConfig IngestConfig
		events       []apievents.AuditEvent
	}{
		{
			name: "types filter out role.created",
			ingestConfig: IngestConfig{
				Types:            map[string]struct{}{"join_token.create": {}},
				SkipSessionTypes: map[string]struct{}{"print": {}, "desktop.recording": {}},
				DryRun:           true,
			},
			events: []apievents.AuditEvent{
				&apievents.RoleCreate{
					Metadata: apievents.Metadata{
						Type: events.RoleCreatedEvent,
						Code: events.RoleCreatedCode,
					},
				},
				&apievents.ProvisionTokenCreate{
					Metadata: apievents.Metadata{
						Type: events.ProvisionTokenCreateEvent,
						Code: events.ProvisionTokenCreateCode,
					},
				},
			},
		},
		{
			name: "skip-event-types filter out role.created",
			ingestConfig: IngestConfig{
				SkipEventTypes:   map[string]struct{}{"role.created": {}},
				SkipSessionTypes: map[string]struct{}{"print": {}, "desktop.recording": {}},
				DryRun:           true,
			},
			events: []apievents.AuditEvent{
				&apievents.RoleCreate{
					Metadata: apievents.Metadata{
						Type: events.RoleCreatedEvent,
						Code: events.RoleCreatedCode,
					},
				},
				&apievents.ProvisionTokenCreate{
					Metadata: apievents.Metadata{
						Type: events.ProvisionTokenCreateEvent,
						Code: events.ProvisionTokenCreateCode,
					},
				},
			},
		},
	}

	skipEvent := regexp.MustCompile("\"Event sent\".*type=role.created")
	checkEvent := regexp.MustCompile("\"Event sent\".*type=join_token.create")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &bytes.Buffer{}
			log := slog.New(slog.NewTextHandler(out, &slog.HandlerOptions{Level: slog.LevelDebug}))

			job := NewEventsJob(&App{
				Config: &StartCmdConfig{
					IngestConfig: tt.ingestConfig},
				State: &State{
					dv: diskv.New(diskv.Options{
						BasePath: t.TempDir(),
					}),
				},
				client: &mockClient{},
				log:    log,
			})

			generateEvents, err := eventsToProto(tt.events)
			require.NoError(t, err)

			for _, event := range generateEvents {
				exportEvent := &auditlogpb.ExportEventUnstructured{Event: event}

				err := job.handleEventV2(t.Context(), exportEvent)
				require.NoError(t, err)

			}

			require.NotRegexp(t, skipEvent, out.String())
			require.Regexp(t, checkEvent, out.String())
		})
	}
}
