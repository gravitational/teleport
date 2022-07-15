/*
Copyright 2020 Gravitational, Inc.

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
	"bytes"
	"context"
	"errors"
	"io"
	"math/rand"
	"testing"
	"time"

	"github.com/gravitational/trace"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

// TestAuditWriter tests audit writer - a component used for
// session recording
func TestAuditWriter(t *testing.T) {
	// Session tests emission of multiple session events
	t.Run("Session", func(t *testing.T) {
		test := newAuditWriterTest(t, nil)
		defer test.cancel()

		inEvents := GenerateTestSession(SessionParams{
			PrintEvents: 1024,
			SessionID:   string(test.sid),
		})

		for _, event := range inEvents {
			err := test.writer.EmitAuditEvent(test.ctx, event)
			require.NoError(t, err)
		}
		err := test.writer.Complete(test.ctx)
		require.NoError(t, err)

		select {
		case event := <-test.eventsCh:
			require.Equal(t, string(test.sid), event.SessionID)
			require.Nil(t, event.Error)
		case <-test.ctx.Done():
			t.Fatalf("Timeout waiting for async upload, try `go test -v` to get more logs for details")
		}

		var outEvents []apievents.AuditEvent
		uploads, err := test.uploader.ListUploads(test.ctx)
		require.NoError(t, err)
		parts, err := test.uploader.GetParts(uploads[0].ID)
		require.NoError(t, err)

		for _, part := range parts {
			reader := NewProtoReader(bytes.NewReader(part))
			out, err := reader.ReadAll(test.ctx)
			require.Nil(t, err, "part crash %#v", part)
			outEvents = append(outEvents, out...)
		}

		require.Equal(t, inEvents, outEvents)
	})

	// ResumeStart resumes stream after it was broken at the start of transmission
	t.Run("ResumeStart", func(t *testing.T) {
		streamCreated := atomic.NewUint64(0)
		terminateConnection := atomic.NewUint64(1)
		streamResumed := atomic.NewUint64(0)

		test := newAuditWriterTest(t, func(streamer Streamer) (*CallbackStreamer, error) {
			return NewCallbackStreamer(CallbackStreamerConfig{
				Inner: streamer,
				OnEmitAuditEvent: func(ctx context.Context, sid session.ID, event apievents.AuditEvent) error {
					if event.GetIndex() > 1 && terminateConnection.CAS(1, 0) == true {
						log.Debugf("Terminating connection at event %v", event.GetIndex())
						return trace.ConnectionProblem(nil, "connection terminated")
					}
					return nil
				},
				OnCreateAuditStream: func(ctx context.Context, sid session.ID, streamer Streamer) (apievents.Stream, error) {
					stream, err := streamer.CreateAuditStream(ctx, sid)
					require.NoError(t, err)
					if streamCreated.Inc() == 1 {
						// simulate status update loss
						select {
						case <-stream.Status():
							log.Debugf("Stealing status update.")
						case <-time.After(time.Second):
							return nil, trace.BadParameter("timeout")
						}
					}
					return stream, nil
				},
				OnResumeAuditStream: func(ctx context.Context, sid session.ID, uploadID string, streamer Streamer) (apievents.Stream, error) {
					stream, err := streamer.ResumeAuditStream(ctx, sid, uploadID)
					require.NoError(t, err)
					streamResumed.Inc()
					return stream, nil
				},
			})
		})

		defer test.cancel()

		inEvents := GenerateTestSession(SessionParams{
			PrintEvents: 1024,
			SessionID:   string(test.sid),
		})

		start := time.Now()
		for _, event := range inEvents {
			err := test.writer.EmitAuditEvent(test.ctx, event)
			require.NoError(t, err)
		}
		log.Debugf("Emitted %v events in %v.", len(inEvents), time.Since(start))
		err := test.writer.Complete(test.ctx)
		require.NoError(t, err)

		outEvents := test.collectEvents(t)

		require.Equal(t, len(inEvents), len(outEvents))
		require.Equal(t, inEvents, outEvents)
		require.Equal(t, 0, int(streamResumed.Load()), "Stream not resumed.")
		require.Equal(t, 2, int(streamCreated.Load()), "Stream created twice.")
	})

	// ResumeMiddle resumes stream after it was broken in the middle of transmission
	t.Run("ResumeMiddle", func(t *testing.T) {
		streamCreated := atomic.NewUint64(0)
		terminateConnection := atomic.NewUint64(1)
		streamResumed := atomic.NewUint64(0)

		test := newAuditWriterTest(t, func(streamer Streamer) (*CallbackStreamer, error) {
			return NewCallbackStreamer(CallbackStreamerConfig{
				Inner: streamer,
				OnEmitAuditEvent: func(ctx context.Context, sid session.ID, event apievents.AuditEvent) error {
					if event.GetIndex() > 600 && terminateConnection.CAS(1, 0) == true {
						log.Debugf("Terminating connection at event %v", event.GetIndex())
						return trace.ConnectionProblem(nil, "connection terminated")
					}
					return nil
				},
				OnCreateAuditStream: func(ctx context.Context, sid session.ID, streamer Streamer) (apievents.Stream, error) {
					stream, err := streamer.CreateAuditStream(ctx, sid)
					require.NoError(t, err)
					streamCreated.Inc()
					return stream, nil
				},
				OnResumeAuditStream: func(ctx context.Context, sid session.ID, uploadID string, streamer Streamer) (apievents.Stream, error) {
					stream, err := streamer.ResumeAuditStream(ctx, sid, uploadID)
					require.NoError(t, err)
					streamResumed.Inc()
					return stream, nil
				},
			})
		})

		defer test.cancel()

		inEvents := GenerateTestSession(SessionParams{
			PrintEvents: 1024,
			SessionID:   string(test.sid),
		})

		start := time.Now()
		for _, event := range inEvents {
			err := test.writer.EmitAuditEvent(test.ctx, event)
			require.NoError(t, err)
		}
		log.Debugf("Emitted all events in %v.", time.Since(start))
		err := test.writer.Complete(test.ctx)
		require.NoError(t, err)

		outEvents := test.collectEvents(t)

		require.Equal(t, len(inEvents), len(outEvents))
		require.Equal(t, inEvents, outEvents)
		require.Equal(t, 1, int(streamResumed.Load()), "Stream resumed once.")
		require.Equal(t, 1, int(streamCreated.Load()), "Stream created once.")
	})

	// Backoff loses the events on emitter hang, but does not lock
	t.Run("Backoff", func(t *testing.T) {
		streamCreated := atomic.NewUint64(0)
		terminateConnection := atomic.NewUint64(1)
		streamResumed := atomic.NewUint64(0)

		submitEvents := 600
		hangCtx, hangCancel := context.WithCancel(context.TODO())
		defer hangCancel()

		test := newAuditWriterTest(t, func(streamer Streamer) (*CallbackStreamer, error) {
			return NewCallbackStreamer(CallbackStreamerConfig{
				Inner: streamer,
				OnEmitAuditEvent: func(ctx context.Context, sid session.ID, event apievents.AuditEvent) error {
					if event.GetIndex() >= int64(submitEvents-1) && terminateConnection.CAS(1, 0) == true {
						log.Debugf("Locking connection at event %v", event.GetIndex())
						<-hangCtx.Done()
						return trace.ConnectionProblem(hangCtx.Err(), "stream hangs")
					}
					return nil
				},
				OnCreateAuditStream: func(ctx context.Context, sid session.ID, streamer Streamer) (apievents.Stream, error) {
					stream, err := streamer.CreateAuditStream(ctx, sid)
					require.NoError(t, err)
					streamCreated.Inc()
					return stream, nil
				},
				OnResumeAuditStream: func(ctx context.Context, sid session.ID, uploadID string, streamer Streamer) (apievents.Stream, error) {
					stream, err := streamer.ResumeAuditStream(ctx, sid, uploadID)
					require.NoError(t, err)
					streamResumed.Inc()
					return stream, nil
				},
			})
		})

		defer test.cancel()

		test.writer.cfg.BackoffTimeout = 100 * time.Millisecond
		test.writer.cfg.BackoffDuration = time.Second

		inEvents := GenerateTestSession(SessionParams{
			PrintEvents: 1024,
			SessionID:   string(test.sid),
		})

		start := time.Now()
		for _, event := range inEvents {
			err := test.writer.EmitAuditEvent(test.ctx, event)
			require.NoError(t, err)
		}
		elapsedTime := time.Since(start)
		log.Debugf("Emitted all events in %v.", elapsedTime)
		require.True(t, elapsedTime < time.Second)
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
		test := newAuditWriterTest(t, func(streamer Streamer) (*CallbackStreamer, error) {
			return NewCallbackStreamer(CallbackStreamerConfig{
				Inner: streamer,
				OnEmitAuditEvent: func(ctx context.Context, sid session.ID, event apievents.AuditEvent) error {
					emittedEvents = append(emittedEvents, event)
					return nil
				},
			})
		})
		defer test.Close(context.Background())
		inEvents := GenerateTestSession(SessionParams{
			SessionID: string(test.sid),
			// ClusterName explicitly empty in parameters
		})
		for _, event := range inEvents {
			require.Empty(t, event.GetClusterName())
			require.NoError(t, test.writer.EmitAuditEvent(test.ctx, event))
		}
		test.Close(context.Background())
		require.Equal(t, len(inEvents), len(emittedEvents))
		for _, event := range emittedEvents {
			require.Equal(t, event.GetClusterName(), "cluster")
		}
	})

	t.Run("NonRecoverable", func(t *testing.T) {
		test := newAuditWriterTest(t, func(streamer Streamer) (*CallbackStreamer, error) {
			return NewCallbackStreamer(CallbackStreamerConfig{
				Inner: streamer,
				OnEmitAuditEvent: func(_ context.Context, _ session.ID, _ apievents.AuditEvent) error {
					// Returns an unrecoverable error.
					return errors.New(uploaderReservePartErrorMessage)
				},
			})
		})

		// First event will not fail since it is processed in the goroutine.
		events := GenerateTestSession(SessionParams{SessionID: string(test.sid)})
		require.NoError(t, test.writer.EmitAuditEvent(test.ctx, events[1]))

		// Subsequent events will fail.
		err := test.writer.EmitAuditEvent(test.ctx, events[1])
		require.Error(t, err)

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

func TestBytesToSessionPrintEvents(t *testing.T) {
	b := make([]byte, MaxProtoMessageSizeBytes+1)
	_, err := rand.Read(b)
	require.NoError(t, err)

	events := bytesToSessionPrintEvents(b)
	require.Len(t, events, 2)

	event0, ok := events[0].(*apievents.SessionPrint)
	require.True(t, ok)

	event1, ok := events[1].(*apievents.SessionPrint)
	require.True(t, ok)

	allBytes := append(event0.Data, event1.Data...)
	require.Equal(t, b, allBytes)
}

type auditWriterTest struct {
	eventsCh chan UploadEvent
	uploader *MemoryUploader
	ctx      context.Context
	cancel   context.CancelFunc
	writer   *AuditWriter
	sid      session.ID
}

type newStreamerFn func(streamer Streamer) (*CallbackStreamer, error)

func newAuditWriterTest(t *testing.T, newStreamer newStreamerFn) *auditWriterTest {
	eventsCh := make(chan UploadEvent, 1)
	uploader := NewMemoryUploader(eventsCh)
	protoStreamer, err := NewProtoStreamer(ProtoStreamerConfig{
		Uploader: uploader,
	})
	require.NoError(t, err)

	var streamer Streamer
	if newStreamer != nil {
		callbackStreamer, err := newStreamer(protoStreamer)
		require.NoError(t, err)
		streamer = callbackStreamer
	} else {
		streamer = protoStreamer
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)

	sid := session.NewID()
	writer, err := NewAuditWriter(AuditWriterConfig{
		SessionID:    sid,
		Namespace:    apidefaults.Namespace,
		RecordOutput: true,
		Streamer:     streamer,
		Context:      ctx,
		ClusterName:  "cluster",
	})
	require.NoError(t, err)

	return &auditWriterTest{
		ctx:      ctx,
		cancel:   cancel,
		writer:   writer,
		uploader: uploader,
		eventsCh: eventsCh,
		sid:      sid,
	}
}

func (a *auditWriterTest) collectEvents(t *testing.T) []apievents.AuditEvent {
	start := time.Now()
	var uploadID string
	select {
	case event := <-a.eventsCh:
		log.Debugf("Got status update, upload %v in %v.", event.UploadID, time.Since(start))
		require.Equal(t, string(a.sid), event.SessionID)
		require.Nil(t, event.Error)
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
	reader := NewProtoReader(io.MultiReader(readers...))
	outEvents, err := reader.ReadAll(a.ctx)
	require.Nil(t, err, "failed to read")
	log.WithFields(reader.GetStats().ToFields()).Debugf("Reader stats.")

	return outEvents
}

func (a *auditWriterTest) Close(ctx context.Context) error {
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
			got := IsPermanentEmitError(tt.err)
			require.Equal(t, tt.want, got)
		})
	}
}
