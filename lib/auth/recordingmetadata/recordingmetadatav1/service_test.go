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

package recordingmetadatav1

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/encoding/protodelim"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	recordingmetadatav1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingmetadata/v1"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/session"
)

func TestService_GetThumbnail(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	uploader := eventstest.NewMemoryUploader()

	thumbnail := newThumbnail()

	thumbnail.Svg = []byte("<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"50\" height=\"50\"><rect width=\"100%\" height=\"100%\" fill=\"red\"/></svg>")

	err := uploadThumbnail(ctx, uploader, session.ID("test-session"), thumbnail)
	require.NoError(t, err, "failed to upload thumbnail")

	service := &Service{
		authorizer:      &fakeAuthorizer{authorizedSessions: map[string]bool{"test-session": true}},
		downloadHandler: uploader,
	}

	resp, err := service.GetThumbnail(ctx, &recordingmetadatav1pb.GetThumbnailRequest{
		SessionId: "test-session",
	})
	require.NoError(t, err, "failed to get thumbnail")
	require.NotNil(t, resp, "expected non-nil response")

	require.Equal(t, thumbnail.Svg, resp.Thumbnail.Svg, "thumbnail SVG does not match expected value")
}

func TestService_GetThumbnailNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	uploader := eventstest.NewMemoryUploader()

	service := &Service{
		authorizer:      &fakeAuthorizer{authorizedSessions: map[string]bool{"test-session": true}},
		downloadHandler: uploader,
	}

	_, err := service.GetThumbnail(ctx, &recordingmetadatav1pb.GetThumbnailRequest{
		SessionId: "test-session",
	})
	require.Error(t, err, "expected error for test session")
	require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)
}

func TestService_GetThumbnailAuthorized(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	authorizer := &fakeAuthorizer{
		authorizedSessions: map[string]bool{
			"allowed-session": true,
			"test-session":    true,
		},
	}

	uploader := eventstest.NewMemoryUploader()

	allowedSessionThumbnail := newThumbnail()

	err := uploadThumbnail(ctx, uploader, session.ID("allowed-session"), allowedSessionThumbnail)
	require.NoError(t, err, "failed to upload thumbnail for allowed session")

	deniedSessionThumbnail := newThumbnail()
	err = uploadThumbnail(ctx, uploader, session.ID("denied-session"), deniedSessionThumbnail)
	require.NoError(t, err, "failed to upload thumbnail for denied session")

	testSessionThumbnail := newThumbnail()
	err = uploadThumbnail(ctx, uploader, session.ID("test-session"), testSessionThumbnail)
	require.NoError(t, err, "failed to upload thumbnail for test session")

	service := &Service{
		authorizer:      authorizer,
		downloadHandler: uploader,
	}

	testCases := []struct {
		name      string
		sessionID string
		wantError bool
		errorType func(error) bool
	}{
		{
			name:      "authorized session",
			sessionID: "allowed-session",
			wantError: false,
		},
		{
			name:      "unauthorized session",
			sessionID: "denied-session",
			wantError: true,
			errorType: trace.IsAccessDenied,
		},
		{
			name:      "empty session ID",
			sessionID: "",
			wantError: true,
			errorType: trace.IsAccessDenied,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := service.GetThumbnail(ctx, &recordingmetadatav1pb.GetThumbnailRequest{
				SessionId: tc.sessionID,
			})

			if tc.wantError {
				require.Error(t, err)
				if tc.errorType != nil {
					require.True(t, tc.errorType(err), "expected error type check to pass, got %v", err)
				}
			} else {
				require.NoError(t, err, "expected no error for session %s", tc.sessionID)
			}
		})
	}
}

func TestService_GetMetadata(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	uploader := eventstest.NewMemoryUploader()

	metadata := newMetadata()

	frames := []*recordingmetadatav1pb.SessionRecordingThumbnail{
		newThumbnail(),
		newThumbnail(),
	}

	metadata.ClusterName = "test-cluster"

	err := uploadMetadata(ctx, uploader, session.ID("test-session"), metadata, frames)
	require.NoError(t, err, "failed to upload metadata")

	service := &Service{
		authorizer:      &fakeAuthorizer{authorizedSessions: map[string]bool{"test-session": true}},
		downloadHandler: uploader,
	}

	stream := &fakeServerStream{ctx: ctx}
	err = service.GetMetadata(&recordingmetadatav1pb.GetMetadataRequest{
		SessionId: "test-session",
	}, stream)

	require.NoError(t, err, "failed to get metadata")
	require.NotNil(t, stream, "expected non-nil stream")

	{
		chunk := &recordingmetadatav1pb.GetMetadataResponseChunk{}
		err = stream.RecvMsg(chunk)

		require.NoError(t, err, "failed to receive metadata chunk")
		require.NotNil(t, chunk, "expected non-nil chunk")

		require.NotNil(t, chunk.GetMetadata(), "expected metadata in chunk")
		require.Equal(t, metadata.ClusterName, chunk.GetMetadata().ClusterName, "metadata cluster name does not match expected value")
	}

	{
		chunk := &recordingmetadatav1pb.GetMetadataResponseChunk{}
		err = stream.RecvMsg(chunk)

		require.NoError(t, err, "failed to receive first frame chunk")
		require.NotNil(t, chunk, "expected non-nil chunk")

		require.NotNil(t, chunk.GetFrame(), "expected frame in chunk")
		require.Equal(t, frames[0].Svg, chunk.GetFrame().Svg,
			"first frame SVG does not match expected value")
	}

	{
		chunk := &recordingmetadatav1pb.GetMetadataResponseChunk{}
		err = stream.RecvMsg(chunk)

		require.NoError(t, err, "failed to receive second frame chunk")
		require.NotNil(t, chunk, "expected non-nil chunk")

		require.NotNil(t, chunk.GetFrame(), "expected frame in chunk")
		require.Equal(t, frames[1].Svg, chunk.GetFrame().Svg,
			"second frame SVG does not match expected value")
	}
}

func TestService_GetMetadataNotFound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	uploader := eventstest.NewMemoryUploader()

	service := &Service{
		authorizer:      &fakeAuthorizer{authorizedSessions: map[string]bool{"test-session": true}},
		downloadHandler: uploader,
	}

	stream := &fakeServerStream{ctx: ctx}
	err := service.GetMetadata(&recordingmetadatav1pb.GetMetadataRequest{
		SessionId: "test-session",
	}, stream)

	require.Error(t, err, "expected error for test session")
	require.True(t, trace.IsNotFound(err), "expected NotFound error, got %v", err)
}

func TestService_GetMetadataAuthorized(t *testing.T) {
	t.Parallel()

	authorizer := &fakeAuthorizer{
		authorizedSessions: map[string]bool{
			"allowed-session": true,
			"test-session":    true,
		},
	}

	uploader := eventstest.NewMemoryUploader()

	allowedSessionMetadata := newMetadata()
	err := uploadMetadata(context.Background(), uploader, session.ID("allowed-session"), allowedSessionMetadata, nil)
	require.NoError(t, err, "failed to upload metadata for allowed session")

	deniedSessionMetadata := newMetadata()
	err = uploadMetadata(context.Background(), uploader, session.ID("denied-session"), deniedSessionMetadata, nil)
	require.NoError(t, err, "failed to upload metadata for denied session")

	testSessionMetadata := newMetadata()
	err = uploadMetadata(context.Background(), uploader, session.ID("test-session"), testSessionMetadata, nil)
	require.NoError(t, err, "failed to upload metadata for test session")

	service := &Service{
		authorizer:      authorizer,
		downloadHandler: uploader,
	}

	testCases := []struct {
		name      string
		sessionID string
		wantError bool
		errorType func(error) bool
	}{
		{
			name:      "authorized session",
			sessionID: "allowed-session",
			wantError: false,
		},
		{
			name:      "unauthorized session",
			sessionID: "denied-session",
			wantError: true,
			errorType: trace.IsAccessDenied,
		},
		{
			name:      "empty session ID",
			sessionID: "",
			wantError: true,
			errorType: trace.IsAccessDenied,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stream := &fakeServerStream{ctx: context.Background()}

			err := service.GetMetadata(&recordingmetadatav1pb.GetMetadataRequest{
				SessionId: tc.sessionID,
			}, stream)

			if tc.wantError {
				require.Error(t, err)
				if tc.errorType != nil {
					require.True(t, tc.errorType(err), "expected error type check to pass, got %v", err)
				}
			} else {
				require.NoError(t, err, "expected no error for session %s", tc.sessionID)
			}
		})
	}
}

func TestService_AuthorizerError(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	authorizer := &fakeAuthorizer{
		shouldError: true,
	}

	service := &Service{
		authorizer: authorizer,
	}

	_, err := service.GetThumbnail(ctx, &recordingmetadatav1pb.GetThumbnailRequest{
		SessionId: "any-session",
	})
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))

	stream := &fakeServerStream{ctx: ctx}
	err = service.GetMetadata(&recordingmetadatav1pb.GetMetadataRequest{
		SessionId: "any-session",
	}, stream)
	require.Error(t, err)
	require.True(t, trace.IsAccessDenied(err))
}

func newThumbnail() *recordingmetadatav1pb.SessionRecordingThumbnail {
	return &recordingmetadatav1pb.SessionRecordingThumbnail{
		Svg:         []byte("<svg xmlns=\"http://www.w3.org/2000/svg\" width=\"100\" height=\"100\"><rect width=\"100%\" height=\"100%\" fill=\"red\"/></svg>"),
		Cols:        80,
		Rows:        24,
		CursorX:     0,
		CursorY:     0,
		StartOffset: durationpb.New(0),
		EndOffset:   durationpb.New(60),
	}
}

func newMetadata() *recordingmetadatav1pb.SessionRecordingMetadata {
	return &recordingmetadatav1pb.SessionRecordingMetadata{
		Duration:    durationpb.New(60),
		StartCols:   80,
		StartRows:   24,
		StartTime:   timestamppb.New(time.Now().Add(-time.Minute)),
		EndTime:     timestamppb.New(time.Now()),
		ClusterName: "test-cluster",
	}
}

func uploadThumbnail(ctx context.Context, uploader events.UploadHandler, sessionID session.ID, thumbnail *recordingmetadatav1pb.SessionRecordingThumbnail) error {
	data, err := proto.Marshal(thumbnail)
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := uploader.UploadThumbnail(ctx, sessionID, bytes.NewReader(data)); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func uploadMetadata(ctx context.Context, uploader events.UploadHandler, sessionID session.ID, metadata *recordingmetadatav1pb.SessionRecordingMetadata, frames []*recordingmetadatav1pb.SessionRecordingThumbnail) error {
	buf := &bytes.Buffer{}
	writer := bufio.NewWriter(buf)

	if _, err := protodelim.MarshalTo(writer, metadata); err != nil {
		return trace.Wrap(err)
	}

	for _, frame := range frames {
		if _, err := protodelim.MarshalTo(writer, frame); err != nil {
			return trace.Wrap(err)
		}
	}

	if err := writer.Flush(); err != nil {
		return trace.Wrap(err)
	}

	if _, err := uploader.UploadMetadata(ctx, sessionID, buf); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// fakeAuthorizer is a test implementation of the Authorizer interface
type fakeAuthorizer struct {
	// authorizedSessions contains the session IDs that should be authorized
	authorizedSessions map[string]bool
	// shouldError indicates if the authorizer should return an error
	shouldError bool
}

func (f *fakeAuthorizer) Authorize(ctx context.Context, sessionID string) error {
	if f.shouldError {
		return trace.AccessDenied("test error")
	}
	if f.authorizedSessions[sessionID] {
		return nil
	}
	return trace.AccessDenied("access denied to session %s", sessionID)
}

// fakeServerStream is a test implementation of grpc.ServerStreamingServer
type fakeServerStream struct {
	grpc.ServerStream

	ctx       context.Context
	sent      []*recordingmetadatav1pb.GetMetadataResponseChunk
	recvIndex int
}

func (f *fakeServerStream) Context() context.Context {
	return f.ctx
}

func (f *fakeServerStream) Send(resp *recordingmetadatav1pb.GetMetadataResponseChunk) error {
	f.sent = append(f.sent, resp)
	return nil
}

func (f *fakeServerStream) SendHeader(m metadata.MD) error {
	return nil
}

func (f *fakeServerStream) SetHeader(m metadata.MD) error {
	return nil
}

func (f *fakeServerStream) SetTrailer(m metadata.MD) {
}

func (f *fakeServerStream) SendMsg(m interface{}) error {
	return nil
}

func (f *fakeServerStream) RecvMsg(m interface{}) error {
	if chunk, ok := m.(*recordingmetadatav1pb.GetMetadataResponseChunk); ok {
		if f.recvIndex >= len(f.sent) {
			return io.EOF
		}
		proto.Merge(chunk, f.sent[f.recvIndex])
		f.recvIndex++
		return nil
	}
	return fmt.Errorf("unexpected message type %T", m)
}
