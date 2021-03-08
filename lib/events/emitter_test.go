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
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/google/go-cmp/cmp"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

// TestProtoStreamer tests edge cases of proto streamer implementation
func TestProtoStreamer(t *testing.T) {
	type testCase struct {
		name           string
		minUploadBytes int64
		events         []AuditEvent
		err            error
	}
	testCases := []testCase{
		{
			name:           "5MB similar to S3 min size in bytes",
			minUploadBytes: 1024 * 1024 * 5,
			events:         GenerateTestSession(SessionParams{PrintEvents: 1}),
		},
		{
			name:           "get a part per message",
			minUploadBytes: 1,
			events:         GenerateTestSession(SessionParams{PrintEvents: 1}),
		},
		{
			name:           "small load test with some uneven numbers",
			minUploadBytes: 1024,
			events:         GenerateTestSession(SessionParams{PrintEvents: 1000}),
		},
		{
			name:           "no events",
			minUploadBytes: 1024*1024*5 + 64*1024,
		},
		{
			name:           "one event using the whole part",
			minUploadBytes: 1,
			events:         GenerateTestSession(SessionParams{PrintEvents: 0})[:1],
		},
	}

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	for i, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			uploader := NewMemoryUploader()
			streamer, err := NewProtoStreamer(ProtoStreamerConfig{
				Uploader:       uploader,
				MinUploadBytes: tc.minUploadBytes,
			})
			require.Nil(t, err)

			sid := session.ID(fmt.Sprintf("test-%v", i))
			stream, err := streamer.CreateAuditStream(ctx, sid)
			require.Nil(t, err)

			events := tc.events
			for _, event := range events {
				err := stream.EmitAuditEvent(ctx, event)
				if tc.err != nil {
					require.IsType(t, tc.err, err)
					return
				}
				require.Nil(t, err)
			}
			err = stream.Complete(ctx)
			require.Nil(t, err)

			var outEvents []AuditEvent
			uploads, err := uploader.ListUploads(ctx)
			require.Nil(t, err)
			parts, err := uploader.GetParts(uploads[0].ID)
			require.Nil(t, err)

			for _, part := range parts {
				reader := NewProtoReader(bytes.NewReader(part))
				out, err := reader.ReadAll(ctx)
				require.Nil(t, err, "part crash %#v", part)
				outEvents = append(outEvents, out...)
			}

			require.Equal(t, events, outEvents)
		})
	}
}

func TestWriterEmitter(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Second)
	defer cancel()

	events := GenerateTestSession(SessionParams{PrintEvents: 0})
	buf := &bytes.Buffer{}
	emitter := NewWriterEmitter(utils.NopWriteCloser(buf))

	for _, event := range events {
		err := emitter.EmitAuditEvent(ctx, event)
		require.NoError(t, err)
	}

	scanner := bufio.NewScanner(buf)
	for i := 0; scanner.Scan(); i++ {
		require.Contains(t, scanner.Text(), events[i].GetCode())
	}
}

func TestAsyncEmitter(t *testing.T) {
	clock := clockwork.NewRealClock()
	events := GenerateTestSession(SessionParams{PrintEvents: 20})

	// Slow tests that async emitter does not block
	// on slow emitters
	t.Run("Slow", func(t *testing.T) {
		emitter, err := NewAsyncEmitter(AsyncEmitterConfig{
			Inner: &slowEmitter{clock: clock, timeout: time.Hour},
		})
		require.NoError(t, err)
		defer emitter.Close()
		ctx, cancel := context.WithTimeout(context.TODO(), time.Second)
		defer cancel()
		for _, event := range events {
			err := emitter.EmitAuditEvent(ctx, event)
			require.NoError(t, err)
		}
		require.NoError(t, ctx.Err())
	})

	// Receive makes sure all events are recevied in the same order as they are sent
	t.Run("Receive", func(t *testing.T) {
		chanEmitter := &channelEmitter{eventsCh: make(chan AuditEvent, len(events))}
		emitter, err := NewAsyncEmitter(AsyncEmitterConfig{
			Inner: chanEmitter,
		})

		require.NoError(t, err)
		defer emitter.Close()
		ctx, cancel := context.WithTimeout(context.TODO(), time.Second)
		defer cancel()
		for _, event := range events {
			err := emitter.EmitAuditEvent(ctx, event)
			require.NoError(t, err)
		}

		for i := 0; i < len(events); i++ {
			select {
			case event := <-chanEmitter.eventsCh:
				require.Equal(t, events[i], event)
			case <-time.After(time.Second):
				t.Fatalf("timeout at event %v", i)
			}
		}
	})

	// Close makes sure that close cancels operations and context
	t.Run("Close", func(t *testing.T) {
		counter := &counterEmitter{}
		emitter, err := NewAsyncEmitter(AsyncEmitterConfig{
			Inner:      counter,
			BufferSize: len(events),
		})
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.TODO(), time.Second)
		defer cancel()

		emitsDoneC := make(chan struct{}, len(events))
		for _, e := range events {
			go func(event AuditEvent) {
				emitter.EmitAuditEvent(ctx, event)
				emitsDoneC <- struct{}{}
			}(e)
		}

		// context will not wait until all events have been submitted
		emitter.Close()
		require.True(t, int(counter.count.Load()) <= len(events))

		// make sure context is done to prevent context leaks
		select {
		case <-emitter.ctx.Done():
		default:
			t.Fatal("Context leak, should be closed")
		}

		// make sure all emit calls returned after context is done
		for range events {
			select {
			case <-time.After(time.Second):
				t.Fatal("Timed out waiting for emit events.")
			case <-emitsDoneC:
			}
		}
	})
}

type slowEmitter struct {
	clock   clockwork.Clock
	timeout time.Duration
}

func (s *slowEmitter) EmitAuditEvent(ctx context.Context, event AuditEvent) error {
	<-s.clock.After(s.timeout)
	return nil
}

type counterEmitter struct {
	count atomic.Int64
}

func (c *counterEmitter) EmitAuditEvent(ctx context.Context, event AuditEvent) error {
	c.count.Inc()
	return nil
}

type channelEmitter struct {
	eventsCh chan AuditEvent
}

func (c *channelEmitter) EmitAuditEvent(ctx context.Context, event AuditEvent) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case c.eventsCh <- event:
		return nil
	}
}

// TestExport tests export to JSON format.
func TestExport(t *testing.T) {
	sid := session.NewID()
	events := GenerateTestSession(SessionParams{PrintEvents: 1, SessionID: sid.String()})
	uploader := NewMemoryUploader()
	streamer, err := NewProtoStreamer(ProtoStreamerConfig{
		Uploader: uploader,
	})
	require.NoError(t, err)

	ctx := context.TODO()
	stream, err := streamer.CreateAuditStream(ctx, sid)
	require.NoError(t, err)

	for _, event := range events {
		err := stream.EmitAuditEvent(ctx, event)
		require.NoError(t, err)
	}
	err = stream.Complete(ctx)
	require.NoError(t, err)

	uploads, err := uploader.ListUploads(ctx)
	require.NoError(t, err)
	parts, err := uploader.GetParts(uploads[0].ID)
	require.NoError(t, err)

	f, err := ioutil.TempFile("", "")
	require.NoError(t, err)
	defer os.Remove(f.Name())

	var readers []io.Reader
	for _, part := range parts {
		readers = append(readers, bytes.NewReader(part))
		_, err := f.Write(part)
		require.NoError(t, err)
	}
	reader := NewProtoReader(io.MultiReader(readers...))
	outEvents, err := reader.ReadAll(ctx)
	require.NoError(t, err)

	_, err = f.Seek(0, 0)
	require.NoError(t, err)

	buf := &bytes.Buffer{}
	err = Export(ctx, f, buf, teleport.JSON)
	require.NoError(t, err)

	count := 0
	snl := bufio.NewScanner(buf)
	for snl.Scan() {
		require.Contains(t, snl.Text(), outEvents[count].GetCode())
		count++
	}
	require.NoError(t, snl.Err())
	require.Equal(t, len(outEvents), count)
}

// TestEnforcesClusterNameDefault validates that different emitter/stream/streamer
// implementations enforce cluster name defaults
func TestEnforcesClusterNameDefault(t *testing.T) {
	clock := clockwork.NewFakeClock()

	t.Run("CheckingStream", func(t *testing.T) {
		emitter := NewCheckingStream(&DiscardStream{}, clock, "cluster")
		event := &SessionStart{
			Metadata: Metadata{
				ID:   "event.id",
				Code: "event.code",
				Type: "event.type",
			},
		}
		require.NoError(t, emitter.EmitAuditEvent(context.Background(), event))
		require.Empty(t, cmp.Diff(event, &SessionStart{
			Metadata: Metadata{
				ID:          "event.id",
				Code:        "event.code",
				Type:        "event.type",
				Time:        clock.Now(),
				ClusterName: "cluster",
			},
		}))
	})
	t.Run("CheckingStreamer.CreateAuditStream", func(t *testing.T) {
		streamer := &CheckingStreamer{
			CheckingStreamerConfig: CheckingStreamerConfig{
				Inner:        &DiscardEmitter{},
				Clock:        clock,
				ClusterName:  "cluster",
				UIDGenerator: utils.NewFakeUID(),
			},
		}
		emitter, err := streamer.CreateAuditStream(context.Background(), "session.id")
		require.NoError(t, err)
		event := &SessionStart{
			Metadata: Metadata{
				ID:   "event.id",
				Code: "event.code",
				Type: "event.type",
			},
		}
		require.NoError(t, emitter.EmitAuditEvent(context.Background(), event))
		require.Empty(t, cmp.Diff(event, &SessionStart{
			Metadata: Metadata{
				ID:          "event.id",
				Code:        "event.code",
				Type:        "event.type",
				Time:        clock.Now(),
				ClusterName: "cluster",
			},
		}))
	})
	t.Run("CheckingStreamer.ResumeAuditStream", func(t *testing.T) {
		streamer := &CheckingStreamer{
			CheckingStreamerConfig: CheckingStreamerConfig{
				Inner:        &DiscardEmitter{},
				Clock:        clock,
				ClusterName:  "cluster",
				UIDGenerator: utils.NewFakeUID(),
			},
		}
		emitter, err := streamer.ResumeAuditStream(context.Background(), "session.id", "upload.id")
		require.NoError(t, err)
		event := &SessionStart{
			Metadata: Metadata{
				ID:   "event.id",
				Code: "event.code",
				Type: "event.type",
			},
		}
		require.NoError(t, emitter.EmitAuditEvent(context.Background(), event))
		require.Empty(t, cmp.Diff(event, &SessionStart{
			Metadata: Metadata{
				ID:          "event.id",
				Code:        "event.code",
				Type:        "event.type",
				Time:        clock.Now(),
				ClusterName: "cluster",
			},
		}))
	})
}
