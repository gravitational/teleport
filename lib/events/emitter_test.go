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
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"
)

// TestProtoStreamer tests edge cases of proto streamer implementation
func TestProtoStreamer(t *testing.T) {
	type testCase struct {
		name           string
		minUploadBytes int64
		events         []apievents.AuditEvent
	}
	testCases := []testCase{
		{
			name:           "5MB similar to S3 min size in bytes",
			minUploadBytes: 1024 * 1024 * 5,
			events:         eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: 1}),
		},
		{
			name:           "get a part per message",
			minUploadBytes: 1,
			events:         eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: 1}),
		},
		{
			name:           "small load test with some uneven numbers",
			minUploadBytes: 1024,
			events:         eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: 1000}),
		},
		{
			name:           "no events",
			minUploadBytes: 1024*1024*5 + 64*1024,
		},
		{
			name:           "one event using the whole part",
			minUploadBytes: 1,
			events:         eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: 0})[:1],
		},
	}

	ctx := t.Context()

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uploader := eventstest.NewMemoryUploader()
			streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
				Uploader:       uploader,
				MinUploadBytes: tc.minUploadBytes,
			})
			require.NoError(t, err)

			sid := session.ID(fmt.Sprintf("test-%v", i))
			stream, err := streamer.CreateAuditStream(ctx, sid)
			require.NoError(t, err)

			evts := tc.events
			for _, event := range evts {
				err := stream.RecordEvent(ctx, eventstest.PrepareEvent(event))
				require.NoError(t, err)
			}
			err = stream.Complete(ctx)
			require.NoError(t, err)

			var outEvents []apievents.AuditEvent
			uploads, err := uploader.ListUploads(ctx)
			require.NoError(t, err)
			parts, err := uploader.GetParts(uploads[0].ID)
			require.NoError(t, err)

			for _, part := range parts {
				reader := events.NewProtoReader(bytes.NewReader(part), nil)
				out, err := reader.ReadAll(ctx)
				require.NoError(t, err, "part crash %#v", part)
				outEvents = append(outEvents, out...)
			}

			require.Equal(t, evts, outEvents)
		})
	}
}

func TestWriterEmitter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	evts := eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: 0})
	buf := &bytes.Buffer{}
	emitter := events.NewWriterEmitter(utils.NopWriteCloser(buf))

	for _, event := range evts {
		err := emitter.EmitAuditEvent(ctx, event)
		require.NoError(t, err)
	}

	scanner := bufio.NewScanner(buf)
	for i := 0; scanner.Scan(); i++ {
		require.Contains(t, scanner.Text(), evts[i].GetCode())
	}
}

func TestAsyncEmitter(t *testing.T) {
	ctx := context.Background()
	evts := eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: 20})

	// Slow tests that async emitter does not block
	// on slow emitters
	t.Run("Slow", func(t *testing.T) {
		emitter, err := events.NewAsyncEmitter(events.AsyncEmitterConfig{
			Inner:   eventstest.NewSlowEmitter(time.Hour),
			DataDir: t.TempDir(),
		})
		require.NoError(t, err)
		defer emitter.Close()
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		for _, event := range evts {
			err := emitter.EmitAuditEvent(ctx, event)
			require.NoError(t, err)
		}
		require.NoError(t, ctx.Err())
	})

	// Receive makes sure all events are recevied in the same order as they are sent
	t.Run("Receive", func(t *testing.T) {
		chanEmitter := eventstest.NewChannelEmitter(len(evts))
		emitter, err := events.NewAsyncEmitter(events.AsyncEmitterConfig{
			Inner:   chanEmitter,
			DataDir: t.TempDir(),
		})

		require.NoError(t, err)
		defer emitter.Close()
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		for _, event := range evts {
			err := emitter.EmitAuditEvent(ctx, event)
			require.NoError(t, err)
		}

		for i := range evts {
			select {
			case event := <-chanEmitter.C():
				require.Equal(t, evts[i], event)
			case <-time.After(time.Second):
				t.Fatalf("timeout at event %v", i)
			}
		}
	})

	// Close makes sure that close cancels operations and context
	t.Run("Close", func(t *testing.T) {
		counter := eventstest.NewCountingEmitter()
		emitter, err := events.NewAsyncEmitter(events.AsyncEmitterConfig{
			Inner:      counter,
			BufferSize: len(evts),
			DataDir:    t.TempDir(),
		})
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()

		emitsDoneC := make(chan struct{}, len(evts))
		for _, e := range evts {
			go func(event apievents.AuditEvent) {
				emitter.EmitAuditEvent(ctx, event)
				emitsDoneC <- struct{}{}
			}(e)
		}

		// context will not wait until all events have been submitted
		emitter.Close()
		require.LessOrEqual(t, int(counter.Count()), len(evts))

		// make sure all emit calls returned after context is done
		for range evts {
			select {
			case <-time.After(time.Second):
				t.Fatal("Timed out waiting for emit events.")
			case <-emitsDoneC:
			}
		}
	})
}

// TestCheckingAsyncEmitter_FieldsSetBeforePersist verifies that fields
// are written to the event before it is enqueued.
func TestCheckingAsyncEmitter_FieldsSetBeforePersist(t *testing.T) {
	emitter, err := events.NewCheckingAsyncEmitter(
		events.CheckingEmitterConfig{
			ClusterName: "test-cluster",
		},
		events.AsyncEmitterConfig{
			Inner:      eventstest.NewCountingEmitter(),
			BufferSize: 8,
			DataDir:    t.TempDir(),
		},
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = emitter.Close()
	})

	event := &apievents.UserLogin{
		Metadata: apievents.Metadata{
			Type: events.UserLoginEvent,
			Code: events.UserLocalLoginCode,
		},
	}
	require.Empty(t, event.GetID(), "event must start with empty UID")
	require.True(t, event.GetTime().IsZero(), "event must start with zero Time")

	before := time.Now().Truncate(time.Millisecond)
	require.NoError(t, emitter.EmitAuditEvent(context.Background(), event))

	require.NotEmpty(t, event.GetID(),
		"UID should be set on the input event after EmitAuditEvent",
	)
	require.False(t, event.GetTime().Before(before),
		"Time on event should be >= time captured before EmitAuditEvent")
	require.Equal(t, "test-cluster", event.GetClusterName())
}

// TestExport tests export to JSON format.
func TestExport(t *testing.T) {
	sid := session.NewID()
	evts := eventstest.GenerateTestSession(eventstest.SessionParams{PrintEvents: 1, SessionID: sid.String()})
	uploader := eventstest.NewMemoryUploader()
	streamer, err := events.NewProtoStreamer(events.ProtoStreamerConfig{
		Uploader: uploader,
	})
	require.NoError(t, err)

	ctx := context.Background()
	stream, err := streamer.CreateAuditStream(ctx, sid)
	require.NoError(t, err)

	for _, event := range evts {
		err := stream.RecordEvent(ctx, eventstest.PrepareEvent(event))
		require.NoError(t, err)
	}
	err = stream.Complete(ctx)
	require.NoError(t, err)

	uploads, err := uploader.ListUploads(ctx)
	require.NoError(t, err)
	parts, err := uploader.GetParts(uploads[0].ID)
	require.NoError(t, err)

	f, err := os.CreateTemp("", "")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	var readers []io.Reader
	for _, part := range parts {
		readers = append(readers, bytes.NewReader(part))
		_, err := f.Write(part)
		require.NoError(t, err)
	}
	reader := events.NewProtoReader(io.MultiReader(readers...), nil)
	outEvents, err := reader.ReadAll(ctx)
	require.NoError(t, err)

	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	buf := &bytes.Buffer{}
	err = events.Export(ctx, f, buf, teleport.JSON)
	require.NoError(t, err)

	count := 0
	snl := bufio.NewScanner(buf)
	for snl.Scan() {
		require.Contains(t, snl.Text(), outEvents[count].GetCode())
		count++
	}
	require.NoError(t, snl.Err())
	require.Len(t, outEvents, count)
}
