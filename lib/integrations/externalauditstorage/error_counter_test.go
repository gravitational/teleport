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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
			desc: "upload errors alert",
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
			desc: "download errors alert",
			steps: []testStep{
				{
					action: func(pack *testPack) {
						pack.errHandler.Download(ctx, "", nil)
						pack.successHandler.Download(ctx, "", nil)
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
		tc := tc
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

func (h *errorHandler) Download(ctx context.Context, sessionID session.ID, writer io.WriterAt) error {
	return h.err
}
