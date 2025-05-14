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
		err            error
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
				if tc.err != nil {
					require.IsType(t, tc.err, err)
					return
				}
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
				reader, err := events.NewProtoReader(bytes.NewReader(part), nil)
				require.NoError(t, err)
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
			Inner: eventstest.NewSlowEmitter(time.Hour),
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
			Inner: chanEmitter,
		})

		require.NoError(t, err)
		defer emitter.Close()
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		for _, event := range evts {
			err := emitter.EmitAuditEvent(ctx, event)
			require.NoError(t, err)
		}

		for i := 0; i < len(evts); i++ {
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
	reader, err := events.NewProtoReader(io.MultiReader(readers...), nil)
	require.NoError(t, err)
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
