// Copyright 2021 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package events

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/teleport/lib/session"

	"github.com/stretchr/testify/require"
)

// TestStreamerCompleteEmpty makes sure that streamer Complete function
// does not hang if streamer got closed a without getting a single event
func TestStreamerCompleteEmpty(t *testing.T) {
	uploader := NewMemoryUploader()

	streamer, err := NewProtoStreamer(ProtoStreamerConfig{
		Uploader: uploader,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.TODO(), time.Second)
	defer cancel()

	events := GenerateTestSession(SessionParams{PrintEvents: 1})
	sid := session.ID(events[0].(SessionMetadataGetter).GetSessionID())

	stream, err := streamer.CreateAuditStream(ctx, sid)
	require.NoError(t, err)

	err = stream.Complete(ctx)
	require.NoError(t, err)

	doneC := make(chan struct{})
	go func() {
		defer close(doneC)
		stream.Complete(ctx)
		stream.Close(ctx)
	}()

	select {
	case <-ctx.Done():
		t.Fatal("Timeout waiting for emitter to complete")
	case <-doneC:
	}
}

// TestNewSliceErrors guarantees that if an error on the `newSlice` process
// happens, the streamer will be canceled and the error will be returned in
// future `EmitAuditEvent` calls.
func TestNewSliceErrors(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("test upload error")
	streamer, err := NewProtoStreamer(ProtoStreamerConfig{
		Uploader: &mockUploader{reserveUploadPartError: expectedErr},
	})
	require.NoError(t, err)

	events := GenerateTestSession(SessionParams{PrintEvents: 1})
	sid := session.ID(events[0].(SessionMetadataGetter).GetSessionID())

	_, err = streamer.CreateAuditStream(ctx, sid)
	require.Error(t, err)
	require.ErrorIs(t, err, expectedErr)
}

// TestNewStreamErrors when creating a new stream, it will also initialize
// the current sliceWriter. If there is any error on this, it should be
// returned.
func TestNewStreamErrors(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("test upload error")

	t.Run("CreateAuditStream", func(t *testing.T) {
		for _, tt := range []struct {
			desc        string
			uploader    *mockUploader
			expectedErr error
		}{
			{
				desc:     "CreateUploadError",
				uploader: &mockUploader{createUploadError: expectedErr},
			},
			{
				desc:     "ReserveUploadPartError",
				uploader: &mockUploader{reserveUploadPartError: expectedErr},
			},
		} {
			t.Run(tt.desc, func(t *testing.T) {
				streamer, err := NewProtoStreamer(ProtoStreamerConfig{
					Uploader: tt.uploader,
				})
				require.NoError(t, err)

				events := GenerateTestSession(SessionParams{PrintEvents: 1})
				sid := session.ID(events[0].(SessionMetadataGetter).GetSessionID())

				_, err = streamer.CreateAuditStream(ctx, sid)
				require.Error(t, err)
				require.ErrorIs(t, err, expectedErr)
			})
		}
	})

	t.Run("ResumeAuditStream", func(t *testing.T) {
		for _, tt := range []struct {
			desc        string
			uploader    *mockUploader
			expectedErr error
		}{
			{
				desc:     "ListPartsError",
				uploader: &mockUploader{listPartsError: expectedErr},
			},
			{
				desc:     "ReserveUploadPartError",
				uploader: &mockUploader{reserveUploadPartError: expectedErr},
			},
		} {
			t.Run(tt.desc, func(t *testing.T) {
				streamer, err := NewProtoStreamer(ProtoStreamerConfig{
					Uploader: tt.uploader,
				})
				require.NoError(t, err)

				events := GenerateTestSession(SessionParams{PrintEvents: 1})
				sid := session.ID(events[0].(SessionMetadataGetter).GetSessionID())

				_, err = streamer.ResumeAuditStream(ctx, sid, uuid.New().String())
				require.Error(t, err)
				require.ErrorIs(t, err, expectedErr)
			})
		}
	})
}

type mockUploader struct {
	MultipartUploader
	createUploadError      error
	reserveUploadPartError error
	listPartsError         error
}

func (m *mockUploader) CreateUpload(ctx context.Context, sessionID session.ID) (*StreamUpload, error) {
	if m.createUploadError != nil {
		return nil, m.createUploadError
	}

	return &StreamUpload{
		ID:        uuid.New().String(),
		SessionID: sessionID,
	}, nil
}

func (m *mockUploader) ReserveUploadPart(_ context.Context, _ StreamUpload, _ int64) error {
	return m.reserveUploadPartError
}

func (m *mockUploader) ListParts(_ context.Context, _ StreamUpload) ([]StreamPart, error) {
	if m.listPartsError != nil {
		return nil, m.listPartsError
	}

	return []StreamPart{}, nil
}
