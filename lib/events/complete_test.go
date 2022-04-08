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

package events

import (
	"context"
	"strings"
	"testing"
	"time"

	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/session"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

// TestUploadCompleterEmitsSessionEnd verifies that the upload completer
// emits session.end or windows.desktop.session.end events for sessions
// that are completed.
func TestUploadCompleterEmitsSessionEnd(t *testing.T) {
	for _, test := range []struct {
		startEvent   apievents.AuditEvent
		endEventType string
	}{
		{&apievents.SessionStart{}, SessionEndEvent},
		{&apievents.WindowsDesktopSessionStart{}, WindowsDesktopSessionEndEvent},
	} {
		t.Run(test.endEventType, func(t *testing.T) {
			clock := clockwork.NewFakeClock()
			mu := NewMemoryUploader()
			mu.Clock = clock

			log := &MockAuditLog{
				sessionEvents: []apievents.AuditEvent{test.startEvent},
			}

			uc, err := NewUploadCompleter(UploadCompleterConfig{
				Unstarted:   true,
				Uploader:    mu,
				AuditLog:    log,
				GracePeriod: 2 * time.Hour,
				Clock:       clock,
			})
			require.NoError(t, err)

			upload, err := mu.CreateUpload(context.Background(), session.NewID())
			require.NoError(t, err)

			// advance the clock to force the grace period check to succeed
			clock.Advance(3 * time.Hour)

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
			require.Eventually(t, func() bool { return len(log.emitter.Events()) == 2 }, 5*time.Second, 1*time.Second,
				"should have emitted 2 events, but only got %d", len(log.emitter.Events()))

			require.IsType(t, &apievents.SessionUpload{}, log.emitter.Events()[0])
			require.Equal(t, test.endEventType, log.emitter.Events()[1].GetType())
		})
	}
}

// TestUploadCompleterCompletesEmptyUploads verifies that the upload completer
// completes uploads that have no parts. This ensures that we don't leave empty
// directories behind.
func TestUploadCompleterCompletesEmptyUploads(t *testing.T) {
	clock := clockwork.NewFakeClock()
	mu := NewMemoryUploader()
	mu.Clock = clock

	log := &MockAuditLog{}

	uc, err := NewUploadCompleter(UploadCompleterConfig{
		Unstarted:   true,
		Uploader:    mu,
		AuditLog:    log,
		GracePeriod: 2 * time.Hour,
	})
	require.NoError(t, err)

	upload, err := mu.CreateUpload(context.Background(), session.NewID())
	require.NoError(t, err)
	clock.Advance(3 * time.Hour)

	err = uc.CheckUploads(context.Background())
	require.NoError(t, err)

	require.True(t, mu.uploads[upload.ID].completed)
}

type MockAuditLog struct {
	DiscardAuditLog

	emitter       eventstest.MockEmitter
	sessionEvents []apievents.AuditEvent
}

func (m *MockAuditLog) StreamSessionEvents(ctx context.Context, sid session.ID, startIndex int64) (chan apievents.AuditEvent, chan error) {
	errors := make(chan error, 1)
	events := make(chan apievents.AuditEvent)

	go func() {
		defer close(events)

		for _, event := range m.sessionEvents {
			select {
			case <-ctx.Done():
				return
			case events <- event:
			}
		}
	}()

	return events, errors
}

func (m *MockAuditLog) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	return m.emitter.EmitAuditEvent(ctx, event)
}
