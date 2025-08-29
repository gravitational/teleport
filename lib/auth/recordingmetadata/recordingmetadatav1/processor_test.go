/**
 * Teleport
 * Copyright (C) 2025 Gravitational, Inc.
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

package recordingmetadatav1

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protodelim"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/session"
)

func TestProcessSessionRecording_StreamError(t *testing.T) {
	sessionID := session.NewID()

	streamer := &mockStreamerErrorOnly{
		err: errors.New("stream error"),
	}

	processor := newSessionProcessor(sessionID)

	events, errs := streamer.StreamSessionEvents(t.Context(), sessionID, 0)

	err := processor.processEventStream(t.Context(), events, errs)

	require.Error(t, err)
	require.Contains(t, err.Error(), "stream error")
}

func TestProcessSessionRecording_ContextCancellation(t *testing.T) {
	sessionID := session.NewID()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	streamer := newMockStreamerNeverSends()

	processor := newSessionProcessor(sessionID)

	events, errs := streamer.StreamSessionEvents(ctx, sessionID, 0)

	processDone := make(chan error, 1)
	go func() {
		processDone <- processor.processEventStream(ctx, events, errs)
	}()

	streamer.WaitUntilBlocking()

	cancel()

	select {
	case err := <-processDone:
		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("test timed out - processEventStream did not exit after context cancellation")
	}
}

func TestProcessSessionRecording_Metadata(t *testing.T) {
	startTime := time.Now()

	tests := []struct {
		name             string
		events           []apievents.AuditEvent
		expectError      bool
		expectedMetadata func(t *testing.T, metadata *pb.SessionRecordingMetadata)
		expectFrames     bool
	}{
		{
			name:   "ssh session with print events",
			events: generateBasicSession(startTime),
			expectedMetadata: func(t *testing.T, metadata *pb.SessionRecordingMetadata) {
				require.Equal(t, timestamppb.New(startTime), metadata.StartTime)
				require.Equal(t, timestamppb.New(startTime.Add(10*time.Second)), metadata.EndTime)
				require.Equal(t, durationpb.New(10*time.Second), metadata.Duration)

				require.Equal(t, int32(80), metadata.StartCols)
				require.Equal(t, int32(24), metadata.StartRows)

				require.Equal(t, "test-cluster", metadata.ClusterName)
				require.Equal(t, "test-server", metadata.ResourceName)

				require.Equal(t, pb.SessionRecordingType_SESSION_RECORDING_TYPE_SSH, metadata.Type)
			},
			expectFrames: true,
		},
		{
			name:   "kube session with print events",
			events: generateKubernetesSession(startTime),
			expectedMetadata: func(t *testing.T, metadata *pb.SessionRecordingMetadata) {
				require.Equal(t, timestamppb.New(startTime), metadata.StartTime)
				require.Equal(t, timestamppb.New(startTime.Add(10*time.Second)), metadata.EndTime)
				require.Equal(t, durationpb.New(10*time.Second), metadata.Duration)

				require.Equal(t, int32(80), metadata.StartCols)
				require.Equal(t, int32(24), metadata.StartRows)

				require.Equal(t, "test-cluster", metadata.ClusterName)
				require.Equal(t, "test-k8s-cluster", metadata.ResourceName)

				require.Equal(t, pb.SessionRecordingType_SESSION_RECORDING_TYPE_KUBERNETES, metadata.Type)
			},
			expectFrames: true,
		},
		{
			name:        "desktop session",
			events:      generateDesktopSession(startTime),
			expectError: false,
			expectedMetadata: func(t *testing.T, metadata *pb.SessionRecordingMetadata) {
				require.Equal(t, timestamppb.New(startTime), metadata.StartTime)
				require.Equal(t, timestamppb.New(startTime.Add(10*time.Second)), metadata.EndTime)
				require.Equal(t, durationpb.New(10*time.Second), metadata.Duration)

				require.Empty(t, metadata.Events)
				require.Equal(t, "test-desktop", metadata.ResourceName)

				require.Equal(t, pb.SessionRecordingType_SESSION_RECORDING_TYPE_DESKTOP, metadata.Type)
			},
			expectFrames: false,
		},
		{
			name:        "database session",
			events:      generateDatabaseSession(startTime),
			expectError: false,
			expectedMetadata: func(t *testing.T, metadata *pb.SessionRecordingMetadata) {
				require.Equal(t, timestamppb.New(startTime), metadata.StartTime)
				require.Equal(t, timestamppb.New(startTime.Add(10*time.Second)), metadata.EndTime)
				require.Equal(t, durationpb.New(10*time.Second), metadata.Duration)

				require.Empty(t, metadata.Events)
				require.Equal(t, "test-database", metadata.ResourceName)

				require.Equal(t, pb.SessionRecordingType_SESSION_RECORDING_TYPE_DATABASE, metadata.Type)
			},
			expectFrames: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessionID := session.NewID()

			streamer := &mockStreamer{
				events:       tt.events,
				errorOnEvent: -1,
			}

			processor := newSessionProcessor(sessionID)

			events, errs := streamer.StreamSessionEvents(t.Context(), sessionID, 0)
			err := processor.processEventStream(t.Context(), events, errs)

			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)

				metadata, frames := processor.collect()

				tt.expectedMetadata(t, metadata)

				if tt.expectFrames {
					require.NotEmpty(t, frames, "expected thumbnail frames")
				} else {
					require.Empty(t, frames, "did not expect any thumbnail frames")
				}
			}
		})
	}
}

func generateDesktopSession(startTime time.Time) []apievents.AuditEvent {
	return []apievents.AuditEvent{
		&apievents.WindowsDesktopSessionStart{
			Metadata: apievents.Metadata{
				Time: startTime,
			},
			DesktopName: "test-desktop",
		},
		&apievents.WindowsDesktopSessionEnd{
			Metadata: apievents.Metadata{
				Time: startTime.Add(10 * time.Second),
			},
			StartTime: startTime,
			EndTime:   startTime.Add(10 * time.Second),
		},
	}
}

func generateDatabaseSession(startTime time.Time) []apievents.AuditEvent {
	return []apievents.AuditEvent{
		&apievents.DatabaseSessionStart{
			Metadata: apievents.Metadata{
				Time: startTime,
			},
			DatabaseMetadata: apievents.DatabaseMetadata{
				DatabaseService: "test-database",
			},
		},
		&apievents.DatabaseSessionEnd{
			Metadata: apievents.Metadata{
				Time: startTime.Add(10 * time.Second),
			},
			StartTime: startTime,
			EndTime:   startTime.Add(10 * time.Second),
		},
	}
}

func generateBasicSession(startTime time.Time) []apievents.AuditEvent {
	events := []apievents.AuditEvent{
		&apievents.SessionStart{
			Metadata: apievents.Metadata{
				ClusterName: "test-cluster",
				Type:        "session.start",
				Time:        startTime,
			},
			ConnectionMetadata: apievents.ConnectionMetadata{
				Protocol: "ssh",
			},
			ServerMetadata: apievents.ServerMetadata{
				ServerHostname: "test-server",
			},
			TerminalSize: "80:24",
		},
		&apievents.SessionPrint{
			Metadata: apievents.Metadata{
				Type: "print",
				Time: startTime.Add(1 * time.Second),
			},
			Data: []byte("Hello World\n"),
		},
		&apievents.SessionPrint{
			Metadata: apievents.Metadata{
				Type: "print",
				Time: startTime.Add(2 * time.Second),
			},
			Data: []byte("$ ls -la\n"),
		},
		&apievents.SessionEnd{
			Metadata: apievents.Metadata{
				Type: "session.end",
				Time: startTime.Add(10 * time.Second),
			},
			StartTime: startTime,
			EndTime:   startTime.Add(10 * time.Second),
		},
	}
	return events
}

func generateKubernetesSession(startTime time.Time) []apievents.AuditEvent {
	events := []apievents.AuditEvent{
		&apievents.SessionStart{
			Metadata: apievents.Metadata{
				ClusterName: "test-cluster",
				Type:        "session.start",
				Time:        startTime,
			},
			ConnectionMetadata: apievents.ConnectionMetadata{
				Protocol: "kube",
			},
			KubernetesClusterMetadata: apievents.KubernetesClusterMetadata{
				KubernetesCluster: "test-k8s-cluster",
			},
			TerminalSize: "80:24",
		},
		&apievents.SessionPrint{
			Metadata: apievents.Metadata{
				Type: "print",
				Time: startTime.Add(1 * time.Second),
			},
			Data: []byte("Hello World\n"),
		},
		&apievents.SessionPrint{
			Metadata: apievents.Metadata{
				Type: "print",
				Time: startTime.Add(2 * time.Second),
			},
			Data: []byte("$ ls -la\n"),
		},
		&apievents.SessionEnd{
			Metadata: apievents.Metadata{
				Type: "session.end",
				Time: startTime.Add(10 * time.Second),
			},
			StartTime: startTime,
			EndTime:   startTime.Add(10 * time.Second),
		},
	}
	return events
}

func generateSessionWithResize(startTime time.Time) []apievents.AuditEvent {
	events := []apievents.AuditEvent{
		&apievents.SessionStart{
			Metadata: apievents.Metadata{
				Type: "session.start",
				Time: startTime,
			},
			TerminalSize: "80:24",
		},
		&apievents.SessionPrint{
			Metadata: apievents.Metadata{
				Type: "print",
				Time: startTime.Add(1 * time.Second),
			},
			Data: []byte("Initial output\n"),
		},
		&apievents.Resize{
			Metadata: apievents.Metadata{
				Type: "resize",
				Time: startTime.Add(2 * time.Second),
			},
			TerminalSize: "120:40",
		},
		&apievents.SessionPrint{
			Metadata: apievents.Metadata{
				Type: "print",
				Time: startTime.Add(3 * time.Second),
			},
			Data: []byte("After resize\n"),
		},
		&apievents.SessionEnd{
			Metadata: apievents.Metadata{
				Type: "session.end",
				Time: startTime.Add(10 * time.Second),
			},
			StartTime: startTime,
			EndTime:   startTime.Add(10 * time.Second),
		},
	}
	return events
}

func generateSessionWithInactivity(startTime time.Time) []apievents.AuditEvent {
	events := []apievents.AuditEvent{
		&apievents.SessionStart{
			Metadata: apievents.Metadata{
				Type: "session.start",
				Time: startTime,
			},
			TerminalSize: "80:24",
		},
		&apievents.SessionPrint{
			Metadata: apievents.Metadata{
				Type: "print",
				Time: startTime.Add(1 * time.Second),
			},
			Data: []byte("Active\n"),
		},
		// 20 second gap
		&apievents.SessionPrint{
			Metadata: apievents.Metadata{
				Type: "print",
				Time: startTime.Add(21 * time.Second),
			},
			Data: []byte("After inactivity\n"),
		},
		&apievents.SessionEnd{
			Metadata: apievents.Metadata{
				Type: "session.end",
				Time: startTime.Add(20 * time.Second),
			},
			StartTime: startTime,
			EndTime:   startTime.Add(20 * time.Second),
		},
	}
	return events
}

func generateSessionWithJoinLeave(startTime time.Time) []apievents.AuditEvent {
	events := []apievents.AuditEvent{
		&apievents.SessionStart{
			Metadata: apievents.Metadata{
				Type: "session.start",
				Time: startTime,
			},
			TerminalSize: "80:24",
		},
		&apievents.SessionJoin{
			Metadata: apievents.Metadata{
				Type: "session.join",
				Time: startTime.Add(2 * time.Second),
			},
			UserMetadata: apievents.UserMetadata{
				User: "alice",
			},
		},
		&apievents.SessionPrint{
			Metadata: apievents.Metadata{
				Type: "print",
				Time: startTime.Add(3 * time.Second),
			},
			Data: []byte("Alice joined\n"),
		},
		&apievents.SessionJoin{
			Metadata: apievents.Metadata{
				Type: "session.join",
				Time: startTime.Add(4 * time.Second),
			},
			UserMetadata: apievents.UserMetadata{
				User: "bob",
			},
		},
		&apievents.SessionLeave{
			Metadata: apievents.Metadata{
				Type: "session.leave",
				Time: startTime.Add(6 * time.Second),
			},
			UserMetadata: apievents.UserMetadata{
				User: "alice",
			},
		},
		&apievents.SessionEnd{
			Metadata: apievents.Metadata{
				Type: "session.end",
				Time: startTime.Add(10 * time.Second),
			},
			StartTime: startTime,
			EndTime:   startTime.Add(10 * time.Second),
		},
	}
	return events
}

func unmarshalMetadata(data []byte) (*pb.SessionRecordingMetadata, []*pb.SessionRecordingThumbnail, error) {
	reader := bytes.NewReader(data)

	metadata := &pb.SessionRecordingMetadata{}
	err := protodelim.UnmarshalOptions{MaxSize: -1}.UnmarshalFrom(reader, metadata)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	var frames []*pb.SessionRecordingThumbnail

	for {
		frame := &pb.SessionRecordingThumbnail{}
		err := protodelim.UnmarshalOptions{MaxSize: -1}.UnmarshalFrom(reader, frame)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		frames = append(frames, frame)
	}

	return metadata, frames, nil
}

// mockStreamer implements player.Streamer for testing
type mockStreamer struct {
	events       []apievents.AuditEvent
	errorOnEvent int
	err          error
}

func (m *mockStreamer) StreamSessionEvents(ctx context.Context, sessionID session.ID, startIndex int64) (chan apievents.AuditEvent, chan error) {
	errors := make(chan error, 1)
	events := make(chan apievents.AuditEvent)

	go func() {
		defer close(events)
		defer close(errors)

		for i, evt := range m.events {
			if m.errorOnEvent == i {
				errors <- m.err
				return
			}
			select {
			case <-ctx.Done():
				return
			case events <- evt:
			}
		}
	}()

	return events, errors
}

// mockStreamerErrorOnly immediately sends an error without any events
type mockStreamerErrorOnly struct {
	err error
}

func (m *mockStreamerErrorOnly) StreamSessionEvents(ctx context.Context, sessionID session.ID, startIndex int64) (chan apievents.AuditEvent, chan error) {
	errors := make(chan error, 1)
	events := make(chan apievents.AuditEvent)

	go func() {
		errors <- m.err
	}()

	return events, errors
}

// mockStreamerNeverSends never sends any events - just blocks immediately
type mockStreamerNeverSends struct {
	called chan struct{}
}

func newMockStreamerNeverSends() *mockStreamerNeverSends {
	return &mockStreamerNeverSends{called: make(chan struct{})}
}

func (m *mockStreamerNeverSends) StreamSessionEvents(ctx context.Context, _ session.ID, _ int64) (chan apievents.AuditEvent, chan error) {
	evts := make(chan apievents.AuditEvent)
	errs := make(chan error, 1)

	close(m.called)

	go func() {
		<-ctx.Done()

		errs <- ctx.Err()

		close(errs)
	}()

	return evts, errs
}

func (m *mockStreamerNeverSends) WaitUntilBlocking() {
	<-m.called
}
