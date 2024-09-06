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

package events_test

import (
	"context"
	"errors"
	"fmt"
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
		GracePeriod:    24 * time.Hour,
	})
	require.NoError(t, err)

	upload, err := mu.CreateUpload(context.Background(), sessionID)
	require.NoError(t, err)

	err = uc.CheckUploads(context.Background())
	require.NoError(t, err)
	require.False(t, mu.IsCompleted(upload.ID))

	// enough to expire the session tracker, not enough to pass the grace period
	clock.Advance(2 * time.Hour)

	err = uc.CheckUploads(context.Background())
	require.NoError(t, err)
	require.False(t, mu.IsCompleted(upload.ID))

	trackers, err := sessionTrackerService.GetActiveSessionTrackers(context.Background())
	require.NoError(t, err)
	require.Empty(t, trackers)

	clock.Advance(22*time.Hour + time.Nanosecond)

	err = uc.CheckUploads(context.Background())
	require.NoError(t, err)
	require.True(t, mu.IsCompleted(upload.ID))
}

// TestUploadCompleterNeedsSemaphore verifies that the upload completer
// does not complete uploads if it cannot acquire a semaphore.
func TestUploadCompleterNeedsSemaphore(t *testing.T) {
	clock := clockwork.NewFakeClock()
	mu := eventstest.NewMemoryUploader()
	mu.Clock = clock

	log := &eventstest.MockAuditLog{}
	sessionID := session.NewID()
	sessionTrackerService := &mockSessionTrackerService{clock: clock}

	uc, err := events.NewUploadCompleter(events.UploadCompleterConfig{
		Uploader:       mu,
		AuditLog:       log,
		SessionTracker: sessionTrackerService,
		Clock:          clock,
		ClusterName:    "teleport-cluster",
		CheckPeriod:    3 * time.Minute,
		ServerID:       "abc123",
		Semaphores: mockSemaphores{
			acquireErr: errors.New("semaphore already taken"),
		},
	})
	require.NoError(t, err)

	upload, err := mu.CreateUpload(context.Background(), sessionID)
	require.NoError(t, err)

	uc.PerformPeriodicCheck(context.Background())

	// upload should not have completed as the semaphore could not be acquired
	require.False(t, mu.IsCompleted(upload.ID), "upload %v should not have completed", upload.ID)
}

// TestUploadCompleterAcquiresSemaphore verifies that the upload completer
// successfully completes uploads after acquiring the required semaphore.
func TestUploadCompleterAcquiresSemaphore(t *testing.T) {
	clock := clockwork.NewFakeClock()
	mu := eventstest.NewMemoryUploader()
	mu.Clock = clock

	log := &eventstest.MockAuditLog{}
	sessionID := session.NewID()
	sessionTrackerService := &mockSessionTrackerService{clock: clock}

	uc, err := events.NewUploadCompleter(events.UploadCompleterConfig{
		Uploader:       mu,
		AuditLog:       log,
		SessionTracker: sessionTrackerService,
		Clock:          clock,
		ClusterName:    "teleport-cluster",
		CheckPeriod:    3 * time.Minute,
		ServerID:       "abc123",
		Semaphores: mockSemaphores{
			lease: &types.SemaphoreLease{
				Expires: clock.Now().Add(10 * time.Minute),
			},
			acquireErr: nil,
		},
		GracePeriod: -1,
	})
	require.NoError(t, err)

	upload, err := mu.CreateUpload(context.Background(), sessionID)
	require.NoError(t, err)

	uc.PerformPeriodicCheck(context.Background())

	// upload should have completed as semaphore acquisition was successful
	require.True(t, mu.IsCompleted(upload.ID), "upload %v should have completed", upload.ID)
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
				GracePeriod:    -1,
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

func TestCheckUploadsSkipsUploadsInProgress(t *testing.T) {
	clock := clockwork.NewFakeClock()
	sessionTrackers := []types.SessionTracker{}

	sessionTrackerService := &mockSessionTrackerService{
		clock:    clock,
		trackers: sessionTrackers,
	}

	// simulate an upload that started well before the grace period,
	// but the most recently uploaded part is still within the grace period
	gracePeriod := 10 * time.Minute
	uploadInitiated := clock.Now().Add(-3 * gracePeriod)
	lastPartUploaded := clock.Now().Add(-2 * gracePeriod / 3)

	var completedUploads []events.StreamUpload

	uploader := &eventstest.MockUploader{
		MockListUploads: func(ctx context.Context) ([]events.StreamUpload, error) {
			return []events.StreamUpload{
				{
					ID:        "upload-1234",
					SessionID: session.NewID(),
					Initiated: uploadInitiated,
				},
			}, nil
		},
		MockListParts: func(ctx context.Context, upload events.StreamUpload) ([]events.StreamPart, error) {
			return []events.StreamPart{
				{
					Number:       int64(1),
					ETag:         "foo",
					LastModified: lastPartUploaded,
				},
			}, nil
		},
		MockCompleteUpload: func(ctx context.Context, upload events.StreamUpload, parts []events.StreamPart) error {
			completedUploads = append(completedUploads, upload)
			return nil
		},
	}

	uc, err := events.NewUploadCompleter(events.UploadCompleterConfig{
		Uploader:       uploader,
		AuditLog:       &eventstest.MockAuditLog{},
		SessionTracker: sessionTrackerService,
		Clock:          clock,
		ClusterName:    "teleport-cluster",
		GracePeriod:    gracePeriod,
	})
	require.NoError(t, err)

	uc.CheckUploads(context.Background())
	require.Empty(t, completedUploads)

}

func TestCheckUploadsContinuesOnError(t *testing.T) {
	clock := clockwork.NewFakeClock()
	expires := clock.Now().Add(time.Hour * 1)

	sessionTrackers := []types.SessionTracker{
		&types.SessionTrackerV1{
			Spec: types.SessionTrackerSpecV1{
				SessionID: string(session.NewID()),
			},
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Expires: &expires,
				},
			},
		},
		&types.SessionTrackerV1{
			Spec: types.SessionTrackerSpecV1{
				SessionID: string(session.NewID()),
			},
			ResourceHeader: types.ResourceHeader{
				Metadata: types.Metadata{
					Expires: &expires,
				},
			},
		},
	}

	sessionTrackerService := &mockSessionTrackerService{
		clock:    clock,
		trackers: sessionTrackers,
	}

	var completedUploads []session.ID
	uploader := &eventstest.MockUploader{
		MockCompleteUpload: func(ctx context.Context, upload events.StreamUpload, parts []events.StreamPart) error {
			// simulate a not found error on the first complete upload
			if upload.SessionID == session.ID(sessionTrackers[0].GetSessionID()) {
				return trace.NotFound("no such upload %v", sessionTrackers[0].GetSessionID())
			}

			completedUploads = append(completedUploads, upload.SessionID)
			return nil
		},
		MockListUploads: func(ctx context.Context) ([]events.StreamUpload, error) {
			var result []events.StreamUpload
			for i, sess := range sessionTrackers {
				result = append(result, events.StreamUpload{
					ID:        fmt.Sprintf("upload-%v", i),
					SessionID: session.ID(sess.GetSessionID()),
					Initiated: clock.Now(),
				})
			}
			return result, nil
		},
	}

	uc, err := events.NewUploadCompleter(events.UploadCompleterConfig{
		Uploader:       uploader,
		AuditLog:       &eventstest.MockAuditLog{},
		SessionTracker: sessionTrackerService,
		Clock:          clock,
		ClusterName:    "teleport-cluster",
		GracePeriod:    -1,
	})
	require.NoError(t, err)

	// verify that the 2nd upload completed even though the first one failed
	clock.Advance(1 * time.Hour)
	uc.CheckUploads(context.Background())
	require.ElementsMatch(t, completedUploads, []session.ID{session.ID(sessionTrackers[1].GetSessionID())})
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

type mockSemaphores struct {
	types.Semaphores

	lease      *types.SemaphoreLease
	acquireErr error
}

func (m mockSemaphores) AcquireSemaphore(ctx context.Context, params types.AcquireSemaphoreRequest) (*types.SemaphoreLease, error) {
	return m.lease, m.acquireErr
}

func (m mockSemaphores) CancelSemaphoreLease(ctx context.Context, lease types.SemaphoreLease) error {
	return nil
}
