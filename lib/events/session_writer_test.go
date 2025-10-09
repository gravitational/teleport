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
	"bytes"
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/session"
)

// TestSessionWriter tests session writer - a component used for
// session recording
func TestSessionWriter(t *testing.T) {
	// Session tests emission of multiple session events
	t.Run("Session", func(t *testing.T) {
		test := newSessionWriterTest(t, nil)
		defer test.cancel()

		inEvents := eventstest.GenerateTestSession(eventstest.SessionParams{
			PrintEvents: 1024,
			SessionID:   string(test.sid),
		})

		for _, event := range inEvents {
			event, err := test.preparer.PrepareSessionEvent(event)
			require.NoError(t, err)
			err = test.writer.RecordEvent(test.ctx, event)
			require.NoError(t, err)
		}
		err := test.writer.Complete(test.ctx)
		require.NoError(t, err)

		select {
		case event := <-test.eventsCh:
			require.Equal(t, string(test.sid), event.SessionID)
			require.NoError(t, event.Error)
		case <-test.ctx.Done():
			t.Fatalf("Timeout waiting for async upload, try `go test -v` to get more logs for details")
		}

		var outEvents []apievents.AuditEvent
		uploads, err := test.uploader.ListUploads(test.ctx)
		require.NoError(t, err)
		parts, err := test.uploader.GetParts(uploads[0].ID)
		require.NoError(t, err)

		for _, part := range parts {
			reader := events.NewProtoReader(bytes.NewReader(part))
			out, err := reader.ReadAll(test.ctx)
			require.NoError(t, err, "part crash %#v", part)
			outEvents = append(outEvents, out...)
		}

		require.Equal(t, inEvents, outEvents)
	})

	// ResumeStart resumes stream after it was broken at the start of transmission
	t.Run("ResumeStart", func(t *testing.T) {
		var streamCreated, terminateConnection, streamResumed atomic.Uint64
		terminateConnection.Store(1)

		test := newSessionWriterTest(t, func(streamer events.Streamer) (*events.CallbackStreamer, error) {
			return events.NewCallbackStreamer(events.CallbackStreamerConfig{
				Inner: streamer,
				OnRecordEvent: func(ctx context.Context, sid session.ID, pe apievents.PreparedSessionEvent) error {
					event := pe.GetAuditEvent()
					if event.GetIndex() > 1 && terminateConnection.CompareAndSwap(1, 0) == true {
						t.Logf("Terminating connection at event %v", event.GetIndex())
						return trace.ConnectionProblem(nil, "connection terminated")
					}
					return nil
				},
				OnCreateAuditStream: func(ctx context.Context, sid session.ID, streamer events.Streamer) (apievents.Stream, error) {
					stream, err := streamer.CreateAuditStream(ctx, sid)
					require.NoError(t, err)
					if streamCreated.Add(1) == 1 {
						// simulate status update loss
						select {
						case <-stream.Status():
							t.Log("Stealing status update.")
						case <-time.After(time.Second):
							return nil, trace.BadParameter("timeout")
						}
					}
					return stream, nil
				},
				OnResumeAuditStream: func(ctx context.Context, sid session.ID, uploadID string, streamer events.Streamer) (apievents.Stream, error) {
					stream, err := streamer.ResumeAuditStream(ctx, sid, uploadID)
					require.NoError(t, err)
					streamResumed.Add(1)
					return stream, nil
				},
			})
		})

		defer test.cancel()

		inEvents := eventstest.GenerateTestSession(eventstest.SessionParams{
			PrintEvents: 1024,
			SessionID:   string(test.sid),
		})

		start := time.Now()
		for _, event := range inEvents {
			event, err := test.preparer.PrepareSessionEvent(event)
			require.NoError(t, err)
			err = test.writer.RecordEvent(test.ctx, event)
			require.NoError(t, err)
		}
		t.Logf("Emitted %v events in %v.", len(inEvents), time.Since(start))
		err := test.writer.Complete(test.ctx)
		require.NoError(t, err)

		outEvents := test.collectEvents(t)

		require.Len(t, inEvents, len(outEvents))
		require.Equal(t, inEvents, outEvents)
		require.Equal(t, 0, int(streamResumed.Load()), "Stream not resumed.")
		require.Equal(t, 2, int(streamCreated.Load()), "Stream created twice.")
	})

	// ResumeMiddle resumes stream after it was broken in the middle of transmission
	t.Run("ResumeMiddle", func(t *testing.T) {
		var streamCreated, terminateConnection, streamResumed atomic.Uint64
		terminateConnection.Store(1)

		test := newSessionWriterTest(t, func(streamer events.Streamer) (*events.CallbackStreamer, error) {
			return events.NewCallbackStreamer(events.CallbackStreamerConfig{
				Inner: streamer,
				OnRecordEvent: func(ctx context.Context, sid session.ID, pe apievents.PreparedSessionEvent) error {
					event := pe.GetAuditEvent()
					if event.GetIndex() > 600 && terminateConnection.CompareAndSwap(1, 0) == true {
						t.Logf("Terminating connection at event %v", event.GetIndex())
						return trace.ConnectionProblem(nil, "connection terminated")
					}
					return nil
				},
				OnCreateAuditStream: func(ctx context.Context, sid session.ID, streamer events.Streamer) (apievents.Stream, error) {
					stream, err := streamer.CreateAuditStream(ctx, sid)
					require.NoError(t, err)
					streamCreated.Add(1)
					return stream, nil
				},
				OnResumeAuditStream: func(ctx context.Context, sid session.ID, uploadID string, streamer events.Streamer) (apievents.Stream, error) {
					stream, err := streamer.ResumeAuditStream(ctx, sid, uploadID)
					require.NoError(t, err)
					streamResumed.Add(1)
					return stream, nil
				},
			})
		})

		defer test.cancel()

		inEvents := eventstest.GenerateTestSession(eventstest.SessionParams{
			PrintEvents: 1024,
			SessionID:   string(test.sid),
		})

		start := time.Now()
		for _, event := range inEvents {
			event, err := test.preparer.PrepareSessionEvent(event)
			require.NoError(t, err)
			err = test.writer.RecordEvent(test.ctx, event)
			require.NoError(t, err)
		}
		t.Logf("Emitted all events in %v.", time.Since(start))
		err := test.writer.Complete(test.ctx)
		require.NoError(t, err)

		outEvents := test.collectEvents(t)

		require.Len(t, inEvents, len(outEvents))
		require.Equal(t, inEvents, outEvents)
		require.Equal(t, 1, int(streamResumed.Load()), "Stream resumed once.")
		require.Equal(t, 1, int(streamCreated.Load()), "Stream created once.")
	})

	// Backoff loses the events on emitter hang, but does not lock
	t.Run("Backoff", func(t *testing.T) {
		var streamCreated, terminateConnection, streamResumed atomic.Uint64
		terminateConnection.Store(1)

		submitEvents := 600
		hangCtx, hangCancel := context.WithCancel(context.TODO())
		defer hangCancel()

		test := newSessionWriterTest(t, func(streamer events.Streamer) (*events.CallbackStreamer, error) {
			return events.NewCallbackStreamer(events.CallbackStreamerConfig{
				Inner: streamer,
				OnRecordEvent: func(ctx context.Context, sid session.ID, pe apievents.PreparedSessionEvent) error {
					event := pe.GetAuditEvent()
					if event.GetIndex() >= int64(submitEvents-1) && terminateConnection.CompareAndSwap(1, 0) == true {
						t.Logf("Locking connection at event %v", event.GetIndex())
						<-hangCtx.Done()
						return trace.ConnectionProblem(hangCtx.Err(), "stream hangs")
					}
					return nil
				},
				OnCreateAuditStream: func(ctx context.Context, sid session.ID, streamer events.Streamer) (apievents.Stream, error) {
					stream, err := streamer.CreateAuditStream(ctx, sid)
					require.NoError(t, err)
					streamCreated.Add(1)
					return stream, nil
				},
				OnResumeAuditStream: func(ctx context.Context, sid session.ID, uploadID string, streamer events.Streamer) (apievents.Stream, error) {
					stream, err := streamer.ResumeAuditStream(ctx, sid, uploadID)
					require.NoError(t, err)
					streamResumed.Add(1)
					return stream, nil
				},
			})
		}, withBackoff(100*time.Millisecond, time.Second))

		defer test.cancel()

		inEvents := eventstest.GenerateTestSession(eventstest.SessionParams{
			PrintEvents: 1024,
			SessionID:   string(test.sid),
		})

		start := time.Now()
		for _, event := range inEvents {
			event, err := test.preparer.PrepareSessionEvent(event)
			require.NoError(t, err)
			err = test.writer.RecordEvent(test.ctx, event)
			require.NoError(t, err)
		}
		elapsedTime := time.Since(start)
		t.Logf("Emitted all events in %v.", elapsedTime)
		require.Less(t, elapsedTime, time.Second)
		hangCancel()
		err := test.writer.Complete(test.ctx)
		require.NoError(t, err)
		outEvents := test.collectEvents(t)

		submittedEvents := inEvents[:submitEvents]
		require.Equal(t, submittedEvents, outEvents)
		require.Equal(t, 1, int(streamResumed.Load()), "Stream resumed.")
	})

	t.Run("SetsEventDefaults", func(t *testing.T) {
		var emittedEvents []apievents.AuditEvent
		test := newSessionWriterTest(t, func(streamer events.Streamer) (*events.CallbackStreamer, error) {
			return events.NewCallbackStreamer(events.CallbackStreamerConfig{
				Inner: streamer,
				OnRecordEvent: func(ctx context.Context, sid session.ID, event apievents.PreparedSessionEvent) error {
					emittedEvents = append(emittedEvents, event.GetAuditEvent())
					return nil
				},
			})
		})
		defer test.Close(context.Background())
		inEvents := eventstest.GenerateTestSession(eventstest.SessionParams{
			SessionID: string(test.sid),
			// ClusterName explicitly empty in parameters
		})
		for _, event := range inEvents {
			require.Empty(t, event.GetClusterName())
			event, err := test.preparer.PrepareSessionEvent(event)
			require.NoError(t, err)
			require.NoError(t, test.writer.RecordEvent(test.ctx, event))
		}
		test.Close(context.Background())
		require.Len(t, inEvents, len(emittedEvents))
		for _, event := range emittedEvents {
			require.Equal(t, "cluster", event.GetClusterName())
		}
	})

	t.Run("NonRecoverable", func(t *testing.T) {
		test := newSessionWriterTest(t, func(streamer events.Streamer) (*events.CallbackStreamer, error) {
			return events.NewCallbackStreamer(events.CallbackStreamerConfig{
				Inner: streamer,
				OnRecordEvent: func(_ context.Context, _ session.ID, _ apievents.PreparedSessionEvent) error {
					// Returns an unrecoverable error.
					return errors.New("uploader failed to reserve upload part")
				},
			})
		})

		// First event will not fail since it is processed in the goroutine.
		events := eventstest.GenerateTestSession(eventstest.SessionParams{SessionID: string(test.sid)})
		event := events[1]
		preparedEvent, err := test.preparer.PrepareSessionEvent(event)
		require.NoError(t, err)
		require.NoError(t, test.writer.RecordEvent(test.ctx, preparedEvent))

		// Subsequent events will fail.
		require.Error(t, test.writer.RecordEvent(test.ctx, preparedEvent))

		require.Eventually(t, func() bool {
			select {
			case <-test.writer.Done():
				return true
			default:
				return false
			}
		}, 300*time.Millisecond, 100*time.Millisecond)
	})
}

type sessionWriterTest struct {
	eventsCh chan events.UploadEvent
	uploader *eventstest.MemoryUploader
	ctx      context.Context
	cancel   context.CancelFunc
	preparer events.SessionEventPreparer
	writer   *events.SessionWriter
	sid      session.ID
}

type newStreamerFn func(streamer events.Streamer) (*events.CallbackStreamer, error)

type sessionWriterOption func(c *events.SessionWriterConfig)

func withBackoff(timeout, dur time.Duration) sessionWriterOption {
	return func(c *events.SessionWriterConfig) {
		c.BackoffTimeout = timeout
		c.BackoffDuration = dur
	}
}

func newSessionWriterTest(t *testing.T, newStreamer newStreamerFn, opts ...sessionWriterOption) *sessionWriterTest {
	eventsCh := make(chan events.UploadEvent, 1)
	uploader := eventstest.NewMemoryUploader(eventsCh)
	protoStreamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader: uploader,
	})
	require.NoError(t, err)

	var streamer events.Streamer
	if newStreamer != nil {
		callbackStreamer, err := newStreamer(protoStreamer)
		require.NoError(t, err)
		streamer = callbackStreamer
	} else {
		streamer = protoStreamer
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)

	sid := session.NewID()
	preparer, err := events.NewPreparer(events.PreparerConfig{
		SessionID:   sid,
		Namespace:   apidefaults.Namespace,
		ClusterName: "cluster",
	})
	require.NoError(t, err)

	cfg := events.SessionWriterConfig{
		SessionID: sid,
		Preparer:  preparer,
		Streamer:  streamer,
		Context:   ctx,
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	writer, err := events.NewSessionWriter(cfg)
	require.NoError(t, err)

	return &sessionWriterTest{
		ctx:      ctx,
		cancel:   cancel,
		preparer: preparer,
		writer:   writer,
		uploader: uploader,
		eventsCh: eventsCh,
		sid:      sid,
	}
}

func (a *sessionWriterTest) collectEvents(t *testing.T) []apievents.AuditEvent {
	start := time.Now()
	var uploadID string
	select {
	case event := <-a.eventsCh:
		t.Logf("Got status update, upload %v in %v.", event.UploadID, time.Since(start))
		require.Equal(t, string(a.sid), event.SessionID)
		require.NoError(t, event.Error)
		uploadID = event.UploadID
	case <-a.ctx.Done():
		t.Fatalf("Timeout waiting for async upload, try `go test -v` to get more logs for details")
	}

	parts, err := a.uploader.GetParts(uploadID)
	require.NoError(t, err)

	var readers []io.Reader
	for _, part := range parts {
		readers = append(readers, bytes.NewReader(part))
	}
	reader := events.NewProtoReader(io.MultiReader(readers...))
	outEvents, err := reader.ReadAll(a.ctx)
	require.NoError(t, err, "failed to read")
	t.Logf("Reader stats :%v", reader.GetStats().ToFields())

	return outEvents
}

func (a *sessionWriterTest) Close(ctx context.Context) error {
	return a.writer.Close(ctx)
}

func TestIsPermanentEmitError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "badParameter err",
			err:  trace.BadParameter(""),
			want: true,
		},
		{
			name: "agg badParameter and nil",
			err:  trace.NewAggregate(trace.BadParameter(""), nil),
			want: true,
		},
		{
			name: "agg badParameter and badParameter",
			err: trace.NewAggregate(
				trace.BadParameter(""),
				trace.BadParameter(""),
			),
			want: true,
		},
		{
			name: "agg badParameter and accessDenied",
			err: trace.NewAggregate(
				trace.BadParameter(""),
				trace.AccessDenied(""),
			),
			want: false,
		},
		{
			name: "add accessDenied and badParameter",
			err: trace.NewAggregate(
				trace.AccessDenied(""),
				trace.BadParameter(""),
			),
			want: false,
		},
		{
			name: "agg badParameter with wrap",
			err: trace.Wrap(
				trace.NewAggregate(
					trace.Wrap(trace.BadParameter("")),
					trace.Wrap(trace.BadParameter(""))),
			),
			want: true,
		},
		{
			name: "agg badParameter and accessDenied with wrap",
			err: trace.Wrap(
				trace.NewAggregate(
					trace.Wrap(trace.BadParameter("")),
					trace.Wrap(trace.AccessDenied(""))),
			),
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := events.IsPermanentEmitError(tt.err)
			require.Equal(t, tt.want, got)
		})
	}
}
