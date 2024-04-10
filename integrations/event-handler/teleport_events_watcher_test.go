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
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"

	"github.com/gravitational/teleport/api/client/proto"
	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
)

// mockTeleportEventWatcher is Teleport client mock
type mockTeleportEventWatcher struct {
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

	// Get the next page
	e := c.events[startIndex:endIndex]

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

func newTeleportEventWatcher(t *testing.T, eventsClient TeleportSearchEventsClient) *TeleportEventsWatcher {
	client := &TeleportEventsWatcher{
		client: eventsClient,
		pos:    -1,
		config: &StartCmdConfig{
			IngestConfig: IngestConfig{
				BatchSize:       5,
				ExitOnLastEvent: true,
			},
		},
	}

	return client
}

func TestEvents(t *testing.T) {
	ctx := context.Background()

	// create fake audit events with ids 0-19
	testAuditEvents := make([]events.AuditEvent, 20)
	for i := 0; i < 20; i++ {
		testAuditEvents[i] = &events.UserCreate{
			Metadata: events.Metadata{
				ID: strconv.Itoa(i),
			},
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Add the 20 events to a mock event watcher.
	mockEventWatcher := &mockTeleportEventWatcher{events: testAuditEvents}
	client := newTeleportEventWatcher(t, mockEventWatcher)

	// Start the events goroutine
	chEvt, chErr := client.Events(ctx)

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
			t.Fatalf("Received unexpected error from error channel: %v", err)
			return
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("No events received within deadline")
		}
	}

	// Both channels should be closed once the last event is reached.
	select {
	case _, ok := <-chEvt:
		require.False(t, ok, "Events channel should be closed")
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("No events received within deadline")
	}
}

func TestEventsNoExit(t *testing.T) {
	ctx := context.Background()

	// create fake audit events with ids 0-19
	testAuditEvents := make([]events.AuditEvent, 20)
	for i := 0; i < 20; i++ {
		testAuditEvents[i] = &events.UserCreate{
			Metadata: events.Metadata{
				ID: strconv.Itoa(i),
			},
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Add the 20 events to a mock event watcher.
	mockEventWatcher := &mockTeleportEventWatcher{events: testAuditEvents}
	client := newTeleportEventWatcher(t, mockEventWatcher)
	client.config.ExitOnLastEvent = false

	// Start the events goroutine
	chEvt, chErr := client.Events(ctx)

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
			t.Fatalf("Received unexpected error from error channel: %v", err)
			return
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("No events received within deadline")
		}
	}

	// Events goroutine should return next page errors
	mockErr := trace.Errorf("error")
	mockEventWatcher.setSearchEventsError(mockErr)

	select {
	case err, ok := <-chErr:
		require.True(t, ok, "Channel unexpectedly close")
		require.ErrorIs(t, err, mockErr)
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("No events received within deadline")
	}

	// Both channels should be closed
	select {
	case _, ok := <-chEvt:
		require.False(t, ok, "Events channel should be closed")
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("No events received within deadline")
	}

	select {
	case _, ok := <-chErr:
		require.False(t, ok, "Error channel should be closed")
	case <-time.After(100 * time.Millisecond):
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
				ID: strconv.Itoa(i),
			},
		}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mockEventWatcher := &mockTeleportEventWatcher{}
	client := newTeleportEventWatcher(t, mockEventWatcher)
	client.config.ExitOnLastEvent = false

	// Start the events goroutine
	chEvt, chErr := client.Events(ctx)

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
			t.Fatalf("Received unexpected error from error channel: %v", err)
			return
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("No events received within deadline")
		}
	}

	// Both channels should still be open and empty.
	select {
	case <-chEvt:
		t.Fatalf("Events channel should be open")
	case <-chErr:
		t.Fatalf("Events channel should be open")
	case <-time.After(100 * time.Millisecond):
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
			t.Fatalf("Received unexpected error from error channel: %v", err)
			return
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("No events received within deadline")
		}
	}

	// Both channels should still be open and empty.
	select {
	case <-chEvt:
		t.Fatalf("Events channel should be open")
	case <-chErr:
		t.Fatalf("Events channel should be open")
	case <-time.After(100 * time.Millisecond):
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
			t.Fatalf("Received unexpected error from error channel: %v", err)
			return
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("No events received within deadline")
		}
	}

	// Events goroutine should return update page errors
	mockErr := trace.Errorf("error")
	mockEventWatcher.setSearchEventsError(mockErr)

	select {
	case err, ok := <-chErr:
		require.True(t, ok, "Channel unexpectedly close")
		require.ErrorIs(t, err, mockErr)
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("No events received within deadline")
	}

	// Both channels should be closed
	select {
	case _, ok := <-chEvt:
		require.False(t, ok, "Events channel should be closed")
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("No events received within deadline")
	}

	select {
	case _, ok := <-chErr:
		require.False(t, ok, "Error channel should be closed")
	case <-time.After(100 * time.Millisecond):
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
