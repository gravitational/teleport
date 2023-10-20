/*
Copyright 2022 Gravitational, Inc.

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

package events_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/session"
)

// TestUploadCompleterCompletesAbandonedUploads verifies that the upload completer
// completes uploads that don't have an associated session tracker.
func TestUploadCompleterCompletesAbandonedUploads(t *testing.T) {
	clock := clockwork.NewFakeClock()
	mu := eventstest.NewMemoryUploader()
	mu.Clock = clock

	log := &eventstest.MockAuditLog{}

	sessionID := session.NewID()
	expires := clock.Now().Add(time.Hour * 1)
	sessionTracker := &types.SessionTrackerV1{
		Spec: types.SessionTrackerSpecV1{
			SessionID: string(sessionID),
		},
		ResourceHeader: types.ResourceHeader{
			Metadata: types.Metadata{
				Expires: &expires,
			},
		},
	}

	sessionTrackerService := &mockSessionTrackerService{
		clock:    clock,
		trackers: []types.SessionTracker{sessionTracker},
	}

	uc, err := events.NewUploadCompleter(events.UploadCompleterConfig{
		Uploader:       mu,
		AuditLog:       log,
		SessionTracker: sessionTrackerService,
		Clock:          clock,
		ClusterName:    "teleport-cluster",
	})
	require.NoError(t, err)

	upload, err := mu.CreateUpload(context.Background(), sessionID)
	require.NoError(t, err)

	err = uc.CheckUploads(context.Background())
	require.NoError(t, err)
	require.False(t, mu.IsCompleted(upload.ID))

	clock.Advance(1 * time.Hour)

	err = uc.CheckUploads(context.Background())
	require.NoError(t, err)
	require.True(t, mu.IsCompleted(upload.ID))
}

// TestUploadCompleterEmitsSessionEnd verifies that the upload completer
// emits session.end or windows.desktop.session.end events for sessions
// that are completed.
func TestUploadCompleterEmitsSessionEnd(t *testing.T) {
	for _, test := range []struct {
		startEvent   apievents.AuditEvent
		endEventType string
	}{
		{&apievents.SessionStart{}, events.SessionEndEvent},
		{&apievents.WindowsDesktopSessionStart{}, events.WindowsDesktopSessionEndEvent},
	} {
		t.Run(test.endEventType, func(t *testing.T) {
			clock := clockwork.NewFakeClock()
			mu := eventstest.NewMemoryUploader()
			mu.Clock = clock
			startTime := clock.Now().UTC()
			endTime := startTime.Add(2 * time.Minute)

			test.startEvent.SetTime(startTime)

			log := &eventstest.MockAuditLog{
				Emitter: &eventstest.MockRecorderEmitter{},
				SessionEvents: []apievents.AuditEvent{
					test.startEvent,
					&apievents.SessionPrint{Metadata: apievents.Metadata{Time: endTime}},
				},
			}

			uc, err := events.NewUploadCompleter(events.UploadCompleterConfig{
				Uploader:       mu,
				AuditLog:       log,
				Clock:          clock,
				SessionTracker: &mockSessionTrackerService{},
				ClusterName:    "teleport-cluster",
			})
			require.NoError(t, err)

			upload, err := mu.CreateUpload(context.Background(), session.NewID())
			require.NoError(t, err)

			// session end events are only emitted if there's at least one
			// part to be uploaded, so create that here
			_, err = mu.UploadPart(context.Background(), *upload, 0, strings.NewReader("part"))
			require.NoError(t, err)

			err = uc.CheckUploads(context.Background())
			require.NoError(t, err)

			// advance the clock to force the asynchronous session end event emission
			clock.BlockUntil(1)
			clock.Advance(3 * time.Minute)

			// expect two events - a session end and a session upload
			// the session end is done asynchronously, so wait for that
			require.Eventually(t, func() bool { return len(log.Emitter.Events()) == 2 }, 5*time.Second, 1*time.Second,
				"should have emitted 2 events, but only got %d", len(log.Emitter.Events()))

			require.IsType(t, &apievents.SessionUpload{}, log.Emitter.Events()[0])
			require.Equal(t, startTime, log.Emitter.Events()[0].GetTime())
			require.Equal(t, test.endEventType, log.Emitter.Events()[1].GetType())
			require.Equal(t, endTime, log.Emitter.Events()[1].GetTime())
		})
	}
}

type mockSessionTrackerService struct {
	clock    clockwork.Clock
	trackers []types.SessionTracker
}

func (m *mockSessionTrackerService) GetActiveSessionTrackers(ctx context.Context) ([]types.SessionTracker, error) {
	var trackers []types.SessionTracker
	for _, tracker := range m.trackers {
		// mock session tracker expiration
		if tracker.Expiry().After(m.clock.Now()) {
			trackers = append(trackers, tracker)
		}
	}
	return trackers, nil
}

func (m *mockSessionTrackerService) GetActiveSessionTrackersWithFilter(ctx context.Context, filter *types.SessionTrackerFilter) ([]types.SessionTracker, error) {
	return nil, trace.NotImplemented("GetActiveSessionTrackersWithFilter is not implemented")
}

func (m *mockSessionTrackerService) GetSessionTracker(ctx context.Context, sessionID string) (types.SessionTracker, error) {
	for _, tracker := range m.trackers {
		// mock session tracker expiration
		if tracker.GetSessionID() == sessionID && tracker.Expiry().After(m.clock.Now()) {
			return tracker, nil
		}
	}
	return nil, trace.NotFound("tracker not found")
}

func (m *mockSessionTrackerService) CreateSessionTracker(ctx context.Context, st types.SessionTracker) (types.SessionTracker, error) {
	return nil, trace.NotImplemented("CreateSessionTracker is not implemented")
}

func (m *mockSessionTrackerService) UpdateSessionTracker(ctx context.Context, req *proto.UpdateSessionTrackerRequest) error {
	return trace.NotImplemented("UpdateSessionTracker is not implemented")
}

func (m *mockSessionTrackerService) RemoveSessionTracker(ctx context.Context, sessionID string) error {
	return trace.NotImplemented("RemoveSessionTracker is not implemented")
}

func (m *mockSessionTrackerService) UpdatePresence(ctx context.Context, sessionID, user string) error {
	return trace.NotImplemented("UpdatePresence is not implemented")
}
