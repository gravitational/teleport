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

package externalauditstorage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
)

func TestErrorCounter(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	testError := errors.New("test error")
	badError := errors.New(strings.Repeat("bad test error\r\n", 1000))
	sanitizedBadError := "bad test error  bad test error  bad test error  bad test error  bad test error  bad test error  bad test error  bad test error  bad test error  bad test error  bad test error  bad test error  bad test error  bad test error  bad test error  bad test error  "

	for _, tc := range []struct {
		desc         string
		steps        []testStep
		err          error
		expectAlerts []alert
	}{
		{
			desc: "recording upload errors alert",
			steps: []testStep{
				{
					action: func(pack *testPack) {
						pack.errHandler.Upload(ctx, "", nil)
						pack.successHandler.Upload(ctx, "", nil)
					},
					repeat: 10,
				},
			},
			err: testError,
			expectAlerts: []alert{{
				name:    sessionUploadFailureClusterAlert,
				message: fmt.Sprintf(sessionUploadFailureClusterAlertMsgTemplate, testError),
			}},
		},
		{
			desc: "recording download errors alert",
			steps: []testStep{
				{
					action: func(pack *testPack) {
						pack.errHandler.StreamSessionRecording(ctx, "", "")
						pack.successHandler.StreamSessionRecording(ctx, "", "")
					},
					repeat: 10,
				},
			},
			err: testError,
			expectAlerts: []alert{{
				name:    sessionDownloadFailureClusterAlert,
				message: fmt.Sprintf(sessionDownloadFailureClusterAlertMsgTemplate, testError),
			}},
		},
		{
			desc: "summary upload errors alert",
			steps: []testStep{
				{
					action: func(pack *testPack) {
						pack.errHandler.UploadSummary(ctx, "", nil)
						pack.successHandler.UploadSummary(ctx, "", nil)
					},
					repeat: 10,
				},
			},
			err: testError,
			expectAlerts: []alert{{
				name:    sessionUploadFailureClusterAlert,
				message: fmt.Sprintf(sessionUploadFailureClusterAlertMsgTemplate, testError),
			}},
		},
		{
			desc: "summary download errors alert",
			steps: []testStep{
				{
					action: func(pack *testPack) {
						pack.errHandler.StreamSessionSummary(ctx, "")
						pack.successHandler.StreamSessionSummary(ctx, "")
					},
					repeat: 10,
				},
			},
			err: testError,
			expectAlerts: []alert{{
				name:    sessionDownloadFailureClusterAlert,
				message: fmt.Sprintf(sessionDownloadFailureClusterAlertMsgTemplate, testError),
			}},
		},
		{
			desc: "metadata upload errors alert",
			steps: []testStep{
				{
					action: func(pack *testPack) {
						pack.errHandler.UploadMetadata(ctx, "", nil)
						pack.successHandler.UploadMetadata(ctx, "", nil)
					},
					repeat: 10,
				},
			},
			err: testError,
			expectAlerts: []alert{{
				name:    sessionUploadFailureClusterAlert,
				message: fmt.Sprintf(sessionUploadFailureClusterAlertMsgTemplate, testError),
			}},
		},
		{
			desc: "metadata download errors alert",
			steps: []testStep{
				{
					action: func(pack *testPack) {
						pack.errHandler.StreamSessionMetadata(ctx, "")
						pack.successHandler.StreamSessionMetadata(ctx, "")
					},
					repeat: 10,
				},
			},
			err: testError,
			expectAlerts: []alert{{
				name:    sessionDownloadFailureClusterAlert,
				message: fmt.Sprintf(sessionDownloadFailureClusterAlertMsgTemplate, testError),
			}},
		},
		{
			desc: "thumbnail upload errors alert",
			steps: []testStep{
				{
					action: func(pack *testPack) {
						pack.errHandler.UploadThumbnail(ctx, "", nil)
						pack.successHandler.UploadThumbnail(ctx, "", nil)
					},
					repeat: 10,
				},
			},
			err: testError,
			expectAlerts: []alert{{
				name:    sessionUploadFailureClusterAlert,
				message: fmt.Sprintf(sessionUploadFailureClusterAlertMsgTemplate, testError),
			}},
		},
		{
			desc: "thumbnail download errors alert",
			steps: []testStep{
				{
					action: func(pack *testPack) {
						pack.errHandler.StreamSessionThumbnail(ctx, "")
						pack.successHandler.StreamSessionThumbnail(ctx, "")
					},
					repeat: 10,
				},
			},
			err: testError,
			expectAlerts: []alert{{
				name:    sessionDownloadFailureClusterAlert,
				message: fmt.Sprintf(sessionDownloadFailureClusterAlertMsgTemplate, testError),
			}},
		},
		{
			desc: "emit errors alert",
			steps: []testStep{
				{
					action: func(pack *testPack) {
						pack.observeEmitError(testError)
						pack.observeEmitError(nil)
					},
					repeat: 10,
				},
			},
			err: testError,
			expectAlerts: []alert{{
				name:    eventEmitFailureClusterAlert,
				message: fmt.Sprintf(eventEmitFailureClusterAlertMsgTemplate, testError),
			}},
		},
		{
			desc: "search errors alert",
			steps: []testStep{
				{
					action: func(pack *testPack) {
						pack.successLogger.SearchEvents(ctx, events.SearchEventsRequest{})
						pack.errLogger.SearchEvents(ctx, events.SearchEventsRequest{})
					},
					repeat: 10,
				},
			},
			err: testError,
			expectAlerts: []alert{{
				name:    eventSearchFailureClusterAlert,
				message: fmt.Sprintf(eventSearchFailureClusterAlertMsgTemplate, testError),
			}},
		},
		{
			desc: "alert recovery",
			steps: []testStep{
				{
					action: func(pack *testPack) { pack.errLogger.SearchEvents(ctx, events.SearchEventsRequest{}) },
					repeat: 10,
				},
				{
					action: func(pack *testPack) { pack.successLogger.SearchEvents(ctx, events.SearchEventsRequest{}) },
					repeat: 10,
				},
			},
			err:          testError,
			expectAlerts: []alert{},
		},
		{
			desc: "quick alert after many successes",
			steps: []testStep{
				{
					action: func(pack *testPack) { pack.successLogger.SearchEvents(ctx, events.SearchEventsRequest{}) },
					repeat: 1000,
				},
				{
					action: func(pack *testPack) { pack.errLogger.SearchEvents(ctx, events.SearchEventsRequest{}) },
					repeat: 10,
				},
			},
			err: testError,
			expectAlerts: []alert{{
				name:    eventSearchFailureClusterAlert,
				message: fmt.Sprintf(eventSearchFailureClusterAlertMsgTemplate, testError),
			}},
		},
		{
			desc: "quick recovery after many errors",
			steps: []testStep{
				{
					action: func(pack *testPack) { pack.errLogger.SearchEvents(ctx, events.SearchEventsRequest{}) },
					repeat: 1000,
				},
				{
					action: func(pack *testPack) { pack.successLogger.SearchEvents(ctx, events.SearchEventsRequest{}) },
					repeat: 10,
				},
			},
			err:          testError,
			expectAlerts: []alert{},
		},
		{
			desc: "bad error message",
			steps: []testStep{
				{
					action: func(pack *testPack) {
						pack.errHandler.Upload(ctx, "", nil)
					},
					repeat: 10,
				},
			},
			err: badError,
			expectAlerts: []alert{{
				name:    sessionUploadFailureClusterAlert,
				message: fmt.Sprintf(sessionUploadFailureClusterAlertMsgTemplate, sanitizedBadError),
			}},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			alertService := newFakeAlertService()
			counter := NewErrorCounter(alertService)
			pack := &testPack{
				errLogger:        counter.WrapAuditLogger(&errorLogger{err: tc.err}),
				successLogger:    counter.WrapAuditLogger(&errorLogger{err: nil}),
				errHandler:       counter.WrapSessionHandler(&errorHandler{err: tc.err}),
				successHandler:   counter.WrapSessionHandler(&errorHandler{err: nil}),
				observeEmitError: counter.ObserveEmitError,
			}

			for _, step := range tc.steps {
				for i := 0; i < max(step.repeat, 1); i++ {
					step.action(pack)
				}
				// Reliably advancing the clock and waiting for the run loop to
				// finish syncing would require instrumenting the non-test code,
				// so this test just manually calls sync.
				counter.sync(ctx)
			}
			assert.Len(t, alertService.alerts, len(tc.expectAlerts))
			for _, expected := range tc.expectAlerts {
				assert.Equal(t, expected.message, alertService.alerts[expected.name])
			}
		})
	}
}

type testPack struct {
	errLogger        events.AuditLogger
	successLogger    events.AuditLogger
	errHandler       events.MultipartHandler
	successHandler   events.MultipartHandler
	observeEmitError func(error)
}

type testStep struct {
	repeat int
	action func(*testPack)
}

type fakeAlertService struct {
	alerts map[string]string
}

func newFakeAlertService() *fakeAlertService {
	return &fakeAlertService{
		alerts: make(map[string]string),
	}
}

func (f *fakeAlertService) UpsertClusterAlert(ctx context.Context, alert types.ClusterAlert) error {
	f.alerts[alert.GetName()] = alert.Spec.Message
	return nil
}

func (f *fakeAlertService) DeleteClusterAlert(ctx context.Context, alertID string) error {
	if _, found := f.alerts[alertID]; !found {
		return trace.NotFound("cluster alert %s not found", alertID)
	}
	delete(f.alerts, alertID)
	return nil
}

type errorLogger struct {
	err error
	events.AuditLogger
}

func (l *errorLogger) EmitAuditEvent(ctx context.Context, e apievents.AuditEvent) error {
	return l.err
}

func (l *errorLogger) SearchEvents(ctx context.Context, req events.SearchEventsRequest) ([]apievents.AuditEvent, string, error) {
	return nil, "", l.err
}

func (l *errorLogger) SearchSessionEvents(ctx context.Context, req events.SearchSessionEventsRequest) ([]apievents.AuditEvent, string, error) {
	return nil, "", l.err
}

type errorHandler struct {
	err error
	events.MultipartHandler
}

func (h *errorHandler) Upload(ctx context.Context, sessionID session.ID, reader io.Reader) (string, error) {
	return "", h.err
}

func (h *errorHandler) UploadSummary(ctx context.Context, sessionID session.ID, reader io.Reader) (string, error) {
	return "", h.err
}

func (h *errorHandler) UploadMetadata(ctx context.Context, sessionID session.ID, reader io.Reader) (string, error) {
	return "", h.err
}

func (h *errorHandler) UploadThumbnail(ctx context.Context, sessionID session.ID, reader io.Reader) (string, error) {
	return "", h.err
}

func (h *errorHandler) StreamSessionRecording(ctx context.Context, sessionID session.ID, uploadID string) (io.ReadCloser, error) {
	return nil, h.err
}

func (h *errorHandler) StreamSessionSummary(ctx context.Context, sessionID session.ID) (io.ReadCloser, error) {
	return nil, h.err
}

func (h *errorHandler) StreamSessionMetadata(ctx context.Context, sessionID session.ID) (io.ReadCloser, error) {
	return nil, h.err
}

func (h *errorHandler) StreamSessionThumbnail(ctx context.Context, sessionID session.ID) (io.ReadCloser, error) {
	return nil, h.err
}

// bodyErrorHandler is a handler whose Download succeeds (returns a ReadCloser)
// but whose body returns readErr when Read is called.
type bodyErrorHandler struct {
	readErr error
	events.MultipartHandler
}

func (h *bodyErrorHandler) StreamSessionRecording(_ context.Context, _ session.ID, _ string) (io.ReadCloser, error) {
	return io.NopCloser(errorReader{h.readErr}), nil
}

// errorReader is an io.Reader that always returns a fixed error.
type errorReader struct{ err error }

func (e errorReader) Read(_ []byte) (int, error) { return 0, e.err }

// TestDownloadBodyReadError verifies that an error returned while reading the
// body of a successful Download call is still counted as a download failure
// and eventually raises a cluster alert.
func TestDownloadBodyReadError(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	readErr := errors.New("read body error")
	alertService := newFakeAlertService()
	counter := NewErrorCounter(alertService)
	handler := counter.WrapSessionHandler(&bodyErrorHandler{readErr: readErr})

	for range 4 {
		rc, err := handler.StreamSessionRecording(ctx, "", "")
		assert.NoError(t, err, "Download call itself must succeed")
		_, readErr := io.ReadAll(rc)
		assert.Error(t, readErr, "reading the body must fail")
		rc.Close()
	}

	counter.sync(ctx)

	assert.Equal(t, map[string]string{
		sessionDownloadFailureClusterAlert: fmt.Sprintf(sessionDownloadFailureClusterAlertMsgTemplate, readErr),
	}, alertService.alerts)
}
