// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package recordingencryptionv1_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	recordingencryptionv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/recordingencryption/recordingencryptionv1"
	"github.com/gravitational/teleport/lib/auth/recordingmetadata"
	"github.com/gravitational/teleport/lib/auth/summarizer"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

type authKey struct{}

func withAuthCtx(ctx context.Context, authCtx authz.Context) context.Context {
	return context.WithValue(ctx, authKey{}, authCtx)
}

func TestRotateKey(t *testing.T) {
	cases := []struct {
		name      string
		ctx       authz.Context
		expectErr bool
	}{
		{
			name: "authorized RotateKey",
			ctx:  newAuthCtx(authz.AdminActionAuthMFAVerified),
		}, {
			name:      "unauthorized RotateKey",
			ctx:       newAuthCtx(authz.AdminActionAuthUnauthorized),
			expectErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := withAuthCtx(t.Context(), c.ctx)
			rotater := newFakeKeyRotater()
			cfg := recordingencryptionv1.ServiceConfig{
				Authorizer:                &fakeAuthorizer{},
				Logger:                    logtest.NewLogger(),
				Uploader:                  fakeUploader{},
				KeyRotater:                rotater,
				SessionStreamer:           &fakeSessionStreamer{},
				RecordingMetadataProvider: recordingmetadata.NewProvider(),
				SessionSummarizerProvider: summarizer.NewSessionSummarizerProvider(),
			}

			service, err := recordingencryptionv1.NewService(cfg)
			require.NoError(t, err)
			require.Len(t, rotater.keys, 1)

			_, err = service.RotateKey(ctx, nil)
			if c.expectErr {
				require.True(t, trace.IsAccessDenied(err))
				require.Len(t, rotater.keys, 1)
			} else {
				require.NoError(t, err)
				require.Len(t, rotater.keys, 2)
			}
		})
	}
}

func TestCompleteRotation(t *testing.T) {
	cases := []struct {
		name      string
		ctx       authz.Context
		expectErr bool
	}{
		{
			name: "authorized CompleteRotation",
			ctx:  newAuthCtx(authz.AdminActionAuthMFAVerified),
		}, {
			name:      "unauthorized CompleteRotation",
			ctx:       newAuthCtx(authz.AdminActionAuthUnauthorized),
			expectErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			authCtx := withAuthCtx(t.Context(), newAuthCtx(authz.AdminActionAuthMFAVerified))
			ctx := withAuthCtx(t.Context(), c.ctx)
			rotater := newFakeKeyRotater()
			cfg := recordingencryptionv1.ServiceConfig{
				Authorizer:                &fakeAuthorizer{},
				Logger:                    logtest.NewLogger(),
				Uploader:                  fakeUploader{},
				KeyRotater:                rotater,
				SessionStreamer:           &fakeSessionStreamer{},
				RecordingMetadataProvider: recordingmetadata.NewProvider(),
				SessionSummarizerProvider: summarizer.NewSessionSummarizerProvider(),
			}

			service, err := recordingencryptionv1.NewService(cfg)
			require.NoError(t, err)

			_, err = service.RotateKey(authCtx, nil)
			require.NoError(t, err)
			require.Len(t, rotater.keys, 2)

			_, err = service.CompleteRotation(ctx, nil)
			if c.expectErr {
				require.True(t, trace.IsAccessDenied(err))
				require.Len(t, rotater.keys, 2)
			} else {
				require.NoError(t, err)
				require.Len(t, rotater.keys, 1)
			}
		})
	}
}

func TestRollbackRotation(t *testing.T) {
	cases := []struct {
		name      string
		ctx       authz.Context
		expectErr bool
	}{
		{
			name: "authorized Rollback",
			ctx:  newAuthCtx(authz.AdminActionAuthMFAVerified),
		}, {
			name:      "unauthorized Rollback",
			ctx:       newAuthCtx(authz.AdminActionAuthUnauthorized),
			expectErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			authCtx := withAuthCtx(t.Context(), newAuthCtx(authz.AdminActionAuthMFAVerified))
			ctx := withAuthCtx(t.Context(), c.ctx)
			rotater := newFakeKeyRotater()
			cfg := recordingencryptionv1.ServiceConfig{
				Authorizer:                &fakeAuthorizer{},
				Logger:                    logtest.NewLogger(),
				Uploader:                  fakeUploader{},
				KeyRotater:                rotater,
				SessionStreamer:           &fakeSessionStreamer{},
				RecordingMetadataProvider: recordingmetadata.NewProvider(),
				SessionSummarizerProvider: summarizer.NewSessionSummarizerProvider(),
			}

			service, err := recordingencryptionv1.NewService(cfg)
			require.NoError(t, err)

			_, err = service.RotateKey(authCtx, nil)
			require.NoError(t, err)
			require.Len(t, rotater.keys, 2)

			_, err = service.RollbackRotation(ctx, nil)
			if c.expectErr {
				require.True(t, trace.IsAccessDenied(err))
				require.Len(t, rotater.keys, 2)
			} else {
				require.NoError(t, err)
				require.Len(t, rotater.keys, 1)
			}
		})
	}
}

func TestGetRotationState(t *testing.T) {
	cases := []struct {
		name      string
		ctx       authz.Context
		expectErr bool
	}{
		{
			name: "authorized",
			ctx:  newAuthCtx(authz.AdminActionAuthMFAVerified),
		}, {
			name:      "unauthorized",
			ctx:       newAuthCtx(authz.AdminActionAuthUnauthorized),
			expectErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := withAuthCtx(t.Context(), c.ctx)
			rotater := newFakeKeyRotater()
			cfg := recordingencryptionv1.ServiceConfig{
				Authorizer:                &fakeAuthorizer{},
				Logger:                    logtest.NewLogger(),
				Uploader:                  fakeUploader{},
				KeyRotater:                rotater,
				SessionStreamer:           &fakeSessionStreamer{},
				RecordingMetadataProvider: recordingmetadata.NewProvider(),
				SessionSummarizerProvider: summarizer.NewSessionSummarizerProvider(),
			}

			service, err := recordingencryptionv1.NewService(cfg)
			require.NoError(t, err)

			res, err := service.GetRotationState(ctx, nil)
			if c.expectErr {
				require.Error(t, err)
				require.Nil(t, res)
			} else {
				require.NoError(t, err)
				require.Len(t, res.KeyPairStates, 1)
			}
		})
	}
}

func newAuthCtx(action authz.AdminActionAuthState) authz.Context {
	return authz.Context{
		AdminActionAuthState: action,
	}
}

type fakeUploader struct {
	events.MultipartUploader
}

func (f fakeUploader) CompleteUpload(ctx context.Context, upload events.StreamUpload, parts []events.StreamPart) error {
	return nil
}

type fakeAuthorizer struct{}

func (f *fakeAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	authCtx, ok := ctx.Value(authKey{}).(authz.Context)
	if !ok {
		return nil, errors.New("no auth")
	}

	return &authCtx, nil
}

type fakeKeyRotater struct {
	keys []*recordingencryptionv1pb.FingerprintWithState
}

func newFakeKeyRotater() *fakeKeyRotater {
	return &fakeKeyRotater{
		keys: []*recordingencryptionv1pb.FingerprintWithState{
			{
				Fingerprint: uuid.New().String(),
				State:       recordingencryptionv1pb.KeyPairState_KEY_PAIR_STATE_ACTIVE,
			},
		},
	}
}

func (f *fakeKeyRotater) RotateKey(ctx context.Context) error {
	if len(f.keys) != 1 {
		return errors.New("rotation in progress")
	}

	if f.keys[0].State != recordingencryptionv1pb.KeyPairState_KEY_PAIR_STATE_ACTIVE {
		return fmt.Errorf("keys in unexpected state: %v", f.keys[0].State)
	}

	f.keys[0].State = recordingencryptionv1pb.KeyPairState_KEY_PAIR_STATE_ROTATING
	f.keys = append(f.keys, &recordingencryptionv1pb.FingerprintWithState{
		Fingerprint: uuid.New().String(),
		State:       recordingencryptionv1pb.KeyPairState_KEY_PAIR_STATE_ACTIVE,
	})

	return nil
}

func (f *fakeKeyRotater) CompleteRotation(ctx context.Context) error {
	var keys []*recordingencryptionv1pb.FingerprintWithState
	for _, key := range f.keys {
		if key.State == recordingencryptionv1pb.KeyPairState_KEY_PAIR_STATE_ACTIVE {
			keys = append(keys, key)
		}
	}

	f.keys = keys
	return nil
}

func (f *fakeKeyRotater) RollbackRotation(ctx context.Context) error {
	var keys []*recordingencryptionv1pb.FingerprintWithState
	for _, key := range f.keys {
		if key.State == recordingencryptionv1pb.KeyPairState_KEY_PAIR_STATE_ROTATING {
			keys = append(keys, key)
		}
	}

	f.keys = keys
	return nil
}

func (f *fakeKeyRotater) GetRotationState(ctx context.Context) ([]*recordingencryptionv1pb.FingerprintWithState, error) {
	return f.keys, nil
}

type fakeSessionStreamer struct{}

func (f *fakeSessionStreamer) StreamSessionEvents(ctx context.Context, sessionID session.ID, startIndex int64) (chan apievents.AuditEvent, chan error) {
	returnChan := make(chan apievents.AuditEvent, 1)
	errChan := make(chan error, 1)
	close(errChan)
	events := eventstest.GenerateTestSession(eventstest.SessionParams{
		UserName:  "alice",
		SessionID: string(sessionID),
		ServerID:  "testcluster",
		PrintData: []string{"net", "stat"},
	})
	returnChan <- events[len(events)-1]
	return returnChan, nil
}

func TestSessionCompleter(t *testing.T) {
	sessionID := session.ID(uuid.NewString())

	metadataProvider := recordingmetadata.NewProvider()
	recorderMetadata := &fakeRecordingMetadata{}
	recorderMetadata.On("ProcessSessionRecording", mock.Anything, sessionID, mock.Anything).
		Return(nil).Once()
	metadataProvider.SetService(recorderMetadata)

	summarizerProvider := summarizer.NewSessionSummarizerProvider()
	sessionSummarizer := &fakeSummarizer{}
	sessionSummarizer.On("SummarizeSSH", mock.Anything, mock.Anything).
		Return(nil).Once()

	summarizerProvider.SetSummarizer(sessionSummarizer)
	cfg := recordingencryptionv1.ServiceConfig{
		Authorizer:                &fakeAuthorizer{},
		Logger:                    logtest.NewLogger(),
		Uploader:                  fakeUploader{},
		KeyRotater:                newFakeKeyRotater(),
		SessionStreamer:           &fakeSessionStreamer{},
		RecordingMetadataProvider: metadataProvider,
		SessionSummarizerProvider: summarizerProvider,
	}

	service, err := recordingencryptionv1.NewService(cfg)
	require.NoError(t, err)

	ctx := withAuthCtx(t.Context(), newServiceAuthCtx())
	_, err = service.CompleteUpload(ctx, &recordingencryptionv1pb.CompleteUploadRequest{
		Upload: &recordingencryptionv1pb.Upload{
			SessionId:   string(sessionID),
			InitiatedAt: timestamppb.Now(),
			UploadId:    uuid.NewString(),
		},
	})
	require.NoError(t, err)

	recorderMetadata.AssertExpectations(t)
	sessionSummarizer.AssertExpectations(t)
}

type fakeRecordingMetadata struct {
	mock.Mock
}

func (f *fakeRecordingMetadata) ProcessSessionRecording(ctx context.Context, sessionID session.ID, duration time.Duration) error {
	args := f.Called(ctx, sessionID, duration)
	return args.Error(0)
}

type fakeSummarizer struct {
	mock.Mock
}

func (f *fakeSummarizer) SummarizeSSH(ctx context.Context, sessionEndEvent *apievents.SessionEnd) error {
	args := f.Called(ctx, sessionEndEvent)
	return args.Error(0)
}

func (f *fakeSummarizer) SummarizeDatabase(ctx context.Context, sessionEndEvent *apievents.DatabaseSessionEnd) error {
	args := f.Called(ctx, sessionEndEvent)
	return args.Error(0)
}

func (f *fakeSummarizer) SummarizeWithoutEndEvent(ctx context.Context, sessionID session.ID) error {
	args := f.Called(ctx, sessionID)
	return args.Error(0)
}

func newServiceAuthCtx() authz.Context {
	return authz.Context{
		Identity: authz.BuiltinRole{
			Role: types.RoleProxy,
		},
		UnmappedIdentity: authz.BuiltinRole{
			Role: types.RoleProxy,
		},
	}
}
