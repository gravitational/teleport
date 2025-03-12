/*
Copyright 2015-2021 Gravitational, Inc.

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

package main

import (
	"context"
	"log/slog"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/export"
)

// mockTeleportEventWatcher is Teleport client mock
type mockTeleportEventWatcher struct {
	export.Client
	mu sync.Mutex
	// events is the mock list of events
	events []events.AuditEvent
	// mockSearchErr is an error to return
	mockSearchErr error
}

func (c *mockTeleportEventWatcher) setEvents(events []events.AuditEvent) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.events = events
}

func (c *mockTeleportEventWatcher) setSearchEventsError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.mockSearchErr = err
}

func (c *mockTeleportEventWatcher) SearchEvents(ctx context.Context, fromUTC, toUTC time.Time, namespace string, eventTypes []string, limit int, order types.EventOrder, startKey string) ([]events.AuditEvent, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.mockSearchErr != nil {
		return nil, "", c.mockSearchErr
	}

	var startIndex int
	if startKey != "" {
		startIndex, _ = strconv.Atoi(startKey)
	}

	endIndex := startIndex + limit
	if endIndex >= len(c.events) {
		endIndex = len(c.events)
	}

	// validate time
	var e []events.AuditEvent
	for i, event := range c.events {
		if i < startIndex {
			continue
		}
		if i >= endIndex {
			break
		}
		if event.GetTime().After(fromUTC) && event.GetTime().Before(toUTC) {
			e = append(e, event)
		}
	}

	// Check if we finished the page
	var lastKey string
	if len(e) == limit {
		lastKey = strconv.Itoa(startIndex + (len(e) - 1))
	}

	return e, lastKey, nil
}

func (c *mockTeleportEventWatcher) StreamSessionEvents(ctx context.Context, sessionID string, startIndex int64) (chan events.AuditEvent, chan error) {
	return nil, nil
}

func (c *mockTeleportEventWatcher) SearchUnstructuredEvents(ctx context.Context, fromUTC, toUTC time.Time, namespace string, eventTypes []string, limit int, order types.EventOrder, startKey string) ([]*auditlogpb.EventUnstructured, string, error) {
	events, lastKey, err := c.SearchEvents(ctx, fromUTC, toUTC, namespace, eventTypes, limit, order, startKey)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	protoEvents, err := eventsToProto(events)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return protoEvents, lastKey, nil
}

func (c *mockTeleportEventWatcher) StreamUnstructuredSessionEvents(ctx context.Context, sessionID string, startIndex int64) (chan *auditlogpb.EventUnstructured, chan error) {
	return nil, nil
}

func (c *mockTeleportEventWatcher) UpsertLock(ctx context.Context, lock types.Lock) error {
	return nil
}

func (c *mockTeleportEventWatcher) Ping(ctx context.Context) (proto.PingResponse, error) {
	return proto.PingResponse{
		ServerVersion: Version,
	}, nil
}

func (c *mockTeleportEventWatcher) Close() error {
	return nil
}

func newTeleportEventWatcher(t *testing.T, eventsClient TeleportSearchEventsClient, startTime time.Time, skipEventTypesRaw []string, exportFn func(context.Context, *TeleportEvent) error) *LegacyEventsWatcher {
	skipEventTypes := map[string]struct{}{}
	for _, eventType := range skipEventTypesRaw {
		skipEventTypes[eventType] = struct{}{}
	}

	cursor := LegacyCursorValues{
		WindowStartTime: startTime,
	}

	return NewLegacyEventsWatcher(&StartCmdConfig{
		IngestConfig: IngestConfig{
			BatchSize:           5,
			ExitOnLastEvent:     true,
			SkipEventTypes:      skipEventTypes,
			SkipSessionTypesRaw: skipEventTypesRaw,
			WindowSize:          24 * time.Hour,
		},
	}, eventsClient, cursor, exportFn, slog.Default())
}

func TestEvents(t *testing.T) {
	ctx := context.Background()

	// create fake audit events with ids 0-19
	testAuditEvents := make([]events.AuditEvent, 20)
	for i := 0; i < 20; i++ {
		testAuditEvents[i] = &events.UserCreate{
			Metadata: events.Metadata{
				ID:   strconv.Itoa(i),
				Time: time.Now(),
				Type: libevents.UserUpdatedEvent,
			},
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Add the 20 events to a mock event watcher.
	mockEventWatcher := &mockTeleportEventWatcher{events: testAuditEvents}

	chEvt, chErr := make(chan *TeleportEvent, 128), make(chan error, 1)
	client := newTeleportEventWatcher(t, mockEventWatcher, time.Now().Add(-48*time.Hour), nil, func(ctx context.Context, evt *TeleportEvent) error {
		select {
		case chEvt <- evt:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	go func() {
		chErr <- client.ExportEvents(ctx)
	}()

	// Collect all 20 events
	for i := 0; i < 20; i++ {
		select {
		case event, ok := <-chEvt:
			require.NotNil(t, event, "Expected an event but got nil. i: %v", i)
			require.Equal(t, strconv.Itoa(i), event.ID)
			if !ok {
				return
			}
		case err := <-chErr:
			require.NoError(t, err)
			return
		case <-time.After(2 * time.Second):
			t.Fatalf("No events received within deadline")
		}
	}

	// watcher should exit automatically
	select {
	case evt := <-chEvt:
		t.Fatalf("received unexpected event while waiting for watcher exit: %v", evt)
	case err := <-chErr:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for watcher to exit")
	}
}

func TestEventsError(t *testing.T) {
	ctx := context.Background()

	// create fake audit events with ids 0-19
	testAuditEvents := make([]events.AuditEvent, 20)
	for i := 0; i < 20; i++ {
		testAuditEvents[i] = &events.UserCreate{
			Metadata: events.Metadata{
				ID:   strconv.Itoa(i),
				Time: time.Now(),
				Type: libevents.UserUpdatedEvent,
			},
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Add the 20 events to a mock event watcher.
	mockErr := trace.Errorf("error")
	mockEventWatcher := &mockTeleportEventWatcher{events: testAuditEvents, mockSearchErr: mockErr}

	chEvt, chErr := make(chan *TeleportEvent, 128), make(chan error, 1)
	client := newTeleportEventWatcher(t, mockEventWatcher, time.Now().Add(-48*time.Hour), nil, func(ctx context.Context, evt *TeleportEvent) error {
		select {
		case chEvt <- evt:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	go func() {
		chErr <- client.ExportEvents(ctx)
	}()

	select {
	case evt := <-chEvt:
		t.Fatalf("received unexpected event while waiting for watcher exit: %v", evt)
	case err := <-chErr:
		require.ErrorIs(t, err, mockErr)
	case <-time.After(2 * time.Second):
		t.Fatalf("No events received within deadline")
	}
}

func TestUpdatePage(t *testing.T) {
	ctx := context.Background()

	// create fake audit events with ids 0-9
	testAuditEvents := make([]events.AuditEvent, 10)
	for i := 0; i < 10; i++ {
		testAuditEvents[i] = &events.UserCreate{
			Metadata: events.Metadata{
				ID:   strconv.Itoa(i),
				Time: time.Now(),
				Type: libevents.UserUpdatedEvent,
			},
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mockEventWatcher := &mockTeleportEventWatcher{}

	chEvt, chErr := make(chan *TeleportEvent, 128), make(chan error, 1)
	client := newTeleportEventWatcher(t, mockEventWatcher, time.Now().Add(-1*time.Hour), nil, func(ctx context.Context, evt *TeleportEvent) error {
		select {
		case chEvt <- evt:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	client.config.ExitOnLastEvent = false

	go func() {
		chErr <- client.ExportEvents(ctx)
	}()

	// Add an incomplete page of 3 events and collect them.
	mockEventWatcher.setEvents(testAuditEvents[:3])
	var i int
	for ; i < 3; i++ {
		select {
		case event, ok := <-chEvt:
			require.NotNil(t, event, "Expected an event but got nil")
			require.Equal(t, strconv.Itoa(i), event.ID)
			if !ok {
				return
			}
		case err := <-chErr:
			require.NoError(t, err)
			return
		case <-time.After(2 * time.Second):
			t.Fatalf("No events received within deadline")
		}
	}

	// Both channels should still be open and empty.
	select {
	case <-chEvt:
		t.Fatalf("Events channel should be open")
	case <-chErr:
		t.Fatalf("Events channel should be open")
	case <-time.After(2 * time.Second):
	}

	// Update the event watcher with the full page of events an collect.
	mockEventWatcher.setEvents(testAuditEvents[:5])
	for ; i < 5; i++ {
		select {
		case event, ok := <-chEvt:
			require.NotNil(t, event, "Expected an event but got nil")
			require.Equal(t, strconv.Itoa(i), event.ID)
			if !ok {
				return
			}
		case err := <-chErr:
			require.NoError(t, err)
			return
		case <-time.After(2 * time.Second):
			t.Fatalf("No events received within deadline")
		}
	}

	// Both channels should still be open and empty.
	select {
	case <-chEvt:
		t.Fatalf("Events channel should be open")
	case <-chErr:
		t.Fatalf("Events channel should be open")
	case <-time.After(2 * time.Second):
	}

	// Add another partial page and collect the events
	mockEventWatcher.setEvents(testAuditEvents[:7])
	for ; i < 7; i++ {
		select {
		case event, ok := <-chEvt:
			require.NotNil(t, event, "Expected an event but got nil")
			require.Equal(t, strconv.Itoa(i), event.ID)
			if !ok {
				return
			}
		case err := <-chErr:
			require.NoError(t, err)
			return
		case <-time.After(2 * time.Second):
			t.Fatalf("No events received within deadline")
		}
	}

	// Events goroutine should return update page errors
	mockErr := trace.Errorf("error")
	mockEventWatcher.setSearchEventsError(mockErr)

	select {
	case evt := <-chEvt:
		t.Fatalf("received unexpected event while waiting for watcher exit: %v", evt)
	case err := <-chErr:
		require.ErrorIs(t, err, mockErr)
	case <-time.After(2 * time.Second):
		t.Fatalf("No events received within deadline")
	}
}

func TestValidateConfig(t *testing.T) {
	for _, tc := range []struct {
		name      string
		cfg       StartCmdConfig
		wantError bool
	}{
		{
			name: "Identity file configured",
			cfg: StartCmdConfig{
				FluentdConfig{},
				TeleportConfig{
					TeleportIdentityFile: "not_empty_string",
				},
				IngestConfig{},
				LockConfig{},
			},
			wantError: false,
		}, {
			name: "Cert, key, ca files configured",
			cfg: StartCmdConfig{
				FluentdConfig{},
				TeleportConfig{
					TeleportCA:   "not_empty_string",
					TeleportCert: "not_empty_string",
					TeleportKey:  "not_empty_string",
				},
				IngestConfig{},
				LockConfig{},
			},
			wantError: false,
		}, {
			name: "Identity and teleport cert/ca/key files configured",
			cfg: StartCmdConfig{
				FluentdConfig{},
				TeleportConfig{
					TeleportIdentityFile: "not_empty_string",
					TeleportCA:           "not_empty_string",
					TeleportCert:         "not_empty_string",
					TeleportKey:          "not_empty_string",
				},
				IngestConfig{},
				LockConfig{},
			},
			wantError: true,
		}, {
			name: "None set",
			cfg: StartCmdConfig{
				FluentdConfig{},
				TeleportConfig{},
				IngestConfig{},
				LockConfig{},
			},
			wantError: true,
		}, {
			name: "Some of teleport cert/key/ca unset",
			cfg: StartCmdConfig{
				FluentdConfig{},
				TeleportConfig{
					TeleportCA: "not_empty_string",
				},
				IngestConfig{},
				LockConfig{},
			},
			wantError: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantError {
				require.True(t, trace.IsBadParameter(err))
				return
			}
			require.NoError(t, err)
		})
	}
}

func Test_splitRangeByDay(t *testing.T) {
	type args struct {
		from time.Time
		to   time.Time
	}
	tests := []struct {
		name string
		args args
		want []time.Time
	}{
		{
			name: "Same day",
			args: args{
				from: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2021, 1, 1, 23, 59, 59, 0, time.UTC),
			},
			want: []time.Time{
				time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2021, 1, 1, 23, 59, 59, 0, time.UTC),
			},
		},
		{
			name: "Two days",
			args: args{
				from: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2021, 1, 2, 23, 59, 59, 0, time.UTC),
			},
			want: []time.Time{
				time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC),
				time.Date(2021, 1, 2, 23, 59, 59, 0, time.UTC),
			},
		},
		{
			name: "week",
			args: args{
				from: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
				to:   time.Date(2021, 1, 7, 23, 59, 59, 0, time.UTC),
			},
			want: []time.Time{
				time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
				time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC),
				time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC),
				time.Date(2021, 1, 4, 0, 0, 0, 0, time.UTC),
				time.Date(2021, 1, 5, 0, 0, 0, 0, time.UTC),
				time.Date(2021, 1, 6, 0, 0, 0, 0, time.UTC),
				time.Date(2021, 1, 7, 0, 0, 0, 0, time.UTC),
				time.Date(2021, 1, 7, 23, 59, 59, 0, time.UTC),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitRangeByDay(tt.args.from, tt.args.to, 24*time.Hour)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestEventsWithWindowSkip(t *testing.T) {
	ctx := context.Background()

	// create fake audit events with ids 0-29
	testAuditEvents := make([]events.AuditEvent, 30)
	for i := 0; i < 10; i++ {
		testAuditEvents[i] = &events.UserCreate{
			Metadata: events.Metadata{
				ID:   strconv.Itoa(i),
				Time: time.Now(),
				Type: libevents.UserUpdatedEvent,
			},
		}
	}
	for i := 10; i < 20; i++ {
		testAuditEvents[i] = &events.UserCreate{
			Metadata: events.Metadata{
				ID:   strconv.Itoa(i),
				Time: time.Now(),
				Type: libevents.UserCreateEvent,
			},
		}
	}

	for i := 20; i < 30; i++ {
		testAuditEvents[i] = &events.UserCreate{
			Metadata: events.Metadata{
				ID:   strconv.Itoa(i),
				Time: time.Now(),
				Type: libevents.UserUpdatedEvent,
			},
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Add the 20 events to a mock event watcher.
	mockEventWatcher := &mockTeleportEventWatcher{events: testAuditEvents}

	chEvt, chErr := make(chan *TeleportEvent, 128), make(chan error, 1)
	client := newTeleportEventWatcher(t, mockEventWatcher, time.Now().Add(-48*time.Hour), []string{libevents.UserCreateEvent}, func(ctx context.Context, evt *TeleportEvent) error {
		select {
		case chEvt <- evt:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	go func() {
		chErr <- client.ExportEvents(ctx)
	}()

	// Collect all 10 first events
	for i := 0; i < 10; i++ {
		select {
		case event, ok := <-chEvt:
			require.NotNil(t, event, "Expected an event but got nil. i: %v", i)
			require.Equal(t, strconv.Itoa(i), event.ID)
			if !ok {
				return
			}
		case err := <-chErr:
			require.NoError(t, err)
			return
		case <-time.After(2 * time.Second):
			t.Fatalf("No events received within deadline")
		}
	}

	for i := 20; i < 30; i++ {
		select {
		case event, ok := <-chEvt:
			require.NotNil(t, event, "Expected an event but got nil. i: %v", i)
			require.Equal(t, strconv.Itoa(i), event.ID)
			if !ok {
				return
			}
		case err := <-chErr:
			require.NoError(t, err)
			return
		case <-time.After(2 * time.Second):
			t.Fatalf("No events received within deadline")
		}
	}

	// watcher should exit automatically
	select {
	case evt := <-chEvt:
		t.Fatalf("received unexpected event while waiting for watcher exit: %v", evt)
	case err := <-chErr:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatalf("timeout waiting for watcher to exit")
	}
}
