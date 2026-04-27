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
	"io"
	"net"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	recordingencryptionv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/recordingencryption/recordingencryptionv1"
	"github.com/gravitational/teleport/lib/auth/recordingmetadata"
	"github.com/gravitational/teleport/lib/auth/summarizer"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestMain(m *testing.M) {
	modules.SetInsecureTestMode(true)
	os.Exit(m.Run())
}

type authKey struct{}

func withAuthCtx(ctx context.Context, authCtx authz.Context) context.Context {
	return context.WithValue(ctx, authKey{}, authCtx)
}

func newFakeService(t *testing.T, rotater *fakeKeyRotater) *recordingencryptionv1.Service {
	t.Helper()
	service, err := recordingencryptionv1.NewService(recordingencryptionv1.ServiceConfig{
		Authorizer:                &fakeAuthorizer{},
		Logger:                    logtest.NewLogger(),
		Uploader:                  fakeUploader{},
		KeyRotater:                rotater,
		RecordingMetadataProvider: recordingmetadata.NewProvider(),
		SessionSummarizerProvider: summarizer.NewSessionSummarizerProvider(),
		OnUploadComplete:          func(ctx context.Context, sessionID session.ID) (apievents.AuditEvent, error) { return nil, nil },
	})
	require.NoError(t, err)
	return service
}

func rotationCases(t *testing.T) []struct {
	name      string
	ctx       authz.Context
	assertErr require.ErrorAssertionFunc
} {
	t.Helper()
	accessDenied := func(t require.TestingT, err error, _ ...any) {
		require.True(t, trace.IsAccessDenied(err), "expected access denied, got %v", err)
	}
	return []struct {
		name      string
		ctx       authz.Context
		assertErr require.ErrorAssertionFunc
	}{
		{
			name:      "MFA verified is authorized",
			ctx:       newAuthCtx(t, authz.AdminActionAuthMFAVerified),
			assertErr: require.NoError,
		},
		{
			name:      "MFA unauthorized is denied",
			ctx:       newAuthCtx(t, authz.AdminActionAuthUnauthorized),
			assertErr: accessDenied,
		},
		{
			name:      "no KindRecordingEncryption access is denied",
			ctx:       newAuthCtxWithoutRecordingEncryption(t),
			assertErr: accessDenied,
		},
	}
}

func TestRotateKey(t *testing.T) {
	for _, c := range rotationCases(t) {
		t.Run(c.name, func(t *testing.T) {
			rotater := newFakeKeyRotater()
			service := newFakeService(t, rotater)
			require.Len(t, rotater.keys, 1)

			_, err := service.RotateKey(withAuthCtx(t.Context(), c.ctx), nil)
			c.assertErr(t, err)
			if err == nil {
				require.Len(t, rotater.keys, 2)
			} else {
				require.Len(t, rotater.keys, 1)
			}
		})
	}
}

func TestCompleteRotation(t *testing.T) {
	for _, c := range rotationCases(t) {
		t.Run(c.name, func(t *testing.T) {
			rotater := newFakeKeyRotater()
			service := newFakeService(t, rotater)

			authCtx := withAuthCtx(t.Context(), newAuthCtx(t, authz.AdminActionAuthMFAVerified))
			_, err := service.RotateKey(authCtx, nil)
			require.NoError(t, err)
			require.Len(t, rotater.keys, 2)

			_, err = service.CompleteRotation(withAuthCtx(t.Context(), c.ctx), nil)
			c.assertErr(t, err)
			if err == nil {
				require.Len(t, rotater.keys, 1)
			} else {
				require.Len(t, rotater.keys, 2)
			}
		})
	}
}

func TestRollbackRotation(t *testing.T) {
	for _, c := range rotationCases(t) {
		t.Run(c.name, func(t *testing.T) {
			rotater := newFakeKeyRotater()
			service := newFakeService(t, rotater)

			authCtx := withAuthCtx(t.Context(), newAuthCtx(t, authz.AdminActionAuthMFAVerified))
			_, err := service.RotateKey(authCtx, nil)
			require.NoError(t, err)
			require.Len(t, rotater.keys, 2)

			_, err = service.RollbackRotation(withAuthCtx(t.Context(), c.ctx), nil)
			c.assertErr(t, err)
			if err == nil {
				require.Len(t, rotater.keys, 1)
			} else {
				require.Len(t, rotater.keys, 2)
			}
		})
	}
}

func TestGetRotationState(t *testing.T) {
	for _, c := range rotationCases(t) {
		t.Run(c.name, func(t *testing.T) {
			rotater := newFakeKeyRotater()
			service := newFakeService(t, rotater)

			res, err := service.GetRotationState(withAuthCtx(t.Context(), c.ctx), nil)
			c.assertErr(t, err)
			if err == nil {
				require.Len(t, res.KeyPairStates, 1)
			} else {
				require.Nil(t, res)
			}
		})
	}
}

func newAuthCtx(t *testing.T, action authz.AdminActionAuthState) authz.Context {
	t.Helper()
	// Build a full admin context so the RBAC checker is populated.
	ctx, err := authz.NewBuiltinRoleContext(types.RoleAdmin)
	require.NoError(t, err)
	ctx.AdminActionAuthState = action
	return *ctx
}

// newAuthCtxWithoutRecordingEncryption returns an MFA-verified context whose
// role set does not include KindRecordingEncryption, so RBAC is the failing
// condition rather than admin-action auth.
func newAuthCtxWithoutRecordingEncryption(t *testing.T) authz.Context {
	t.Helper()
	// RoleNode has KindEvent RW (passes the event check) but no KindRecordingEncryption.
	ctx, err := authz.NewBuiltinRoleContext(types.RoleNode)
	require.NoError(t, err)
	ctx.AdminActionAuthState = authz.AdminActionAuthMFAVerified
	return *ctx
}

type fakeUploader struct {
	events.MultipartUploader
}

func (f fakeUploader) CreateUpload(ctx context.Context, sessionID session.ID) (*events.StreamUpload, error) {
	return &events.StreamUpload{ID: uuid.NewString(), SessionID: sessionID}, nil
}

func (f fakeUploader) ReserveUploadPart(ctx context.Context, upload events.StreamUpload, partNumber int64) error {
	return nil
}

func (f fakeUploader) UploadPart(ctx context.Context, upload events.StreamUpload, partNumber int64, partBody io.ReadSeeker) (*events.StreamPart, error) {
	return &events.StreamPart{Number: partNumber}, nil
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

func TestSessionCompleter(t *testing.T) {
	sessionID := session.ID(uuid.NewString())

	metadataProvider := recordingmetadata.NewProvider()
	recorderMetadata := &fakeRecordingMetadata{}
	recorderMetadata.On("ProcessSessionRecording", mock.Anything, sessionID, mock.Anything, mock.Anything).
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
		RecordingMetadataProvider: metadataProvider,
		SessionSummarizerProvider: summarizerProvider,
		OnUploadComplete: func(_ context.Context, sid session.ID) (apievents.AuditEvent, error) {
			now := time.Now()
			return &apievents.SessionEnd{
				SessionMetadata: apievents.SessionMetadata{SessionID: string(sid)},
				StartTime:       now.Add(-time.Minute),
				EndTime:         now,
			}, nil
		},
	}

	service, err := recordingencryptionv1.NewService(cfg)
	require.NoError(t, err)

	ctx := withAuthCtx(t.Context(), newServiceAuthCtx(t))
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

func (f *fakeRecordingMetadata) ProcessSessionRecording(ctx context.Context, sessionID session.ID, sessionType recordingmetadata.SessionType, duration time.Duration) error {
	args := f.Called(ctx, sessionID, sessionType, duration)
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

// TestCompleteUploadRecoversMissingSessionEnd verifies that when an encrypted
// session recording has no session end event (e.g. due to a connection drop),
// CompleteUpload uses the OnUploadComplete callback to recover and emit one.
func TestCompleteUploadRecoversMissingSessionEnd(t *testing.T) {
	sessionID := session.ID(uuid.NewString())
	const clusterName = "test-cluster"

	userMeta := apievents.UserMetadata{User: "alice", Login: "root"}
	sessionMeta := apievents.SessionMetadata{SessionID: string(sessionID)}

	// Session events without a session end — simulates a dropped connection.
	sessionEvents := []apievents.AuditEvent{
		&apievents.SessionStart{
			Metadata:        apievents.Metadata{Type: events.SessionStartEvent, ClusterName: clusterName},
			UserMetadata:    userMeta,
			SessionMetadata: sessionMeta,
			TerminalSize:    "80:25",
		},
		&apievents.SessionPrint{
			Metadata: apievents.Metadata{Type: events.SessionPrintEvent},
		},
	}

	auditLog := &eventstest.MockRecorderEmitter{}
	streamer := eventstest.NewFakeStreamer(sessionEvents, 0)

	slog := logtest.NewLogger()

	cfg := recordingencryptionv1.ServiceConfig{
		Authorizer:                &fakeAuthorizer{},
		Logger:                    slog,
		Uploader:                  fakeUploader{},
		KeyRotater:                newFakeKeyRotater(),
		RecordingMetadataProvider: recordingmetadata.NewProvider(),
		SessionSummarizerProvider: summarizer.NewSessionSummarizerProvider(),
		OnUploadComplete: func(ctx context.Context, sid session.ID) (apievents.AuditEvent, error) {
			return events.FindOrRecoverSessionEnd(ctx, events.FindOrRecoverSessionEndConfig{
				ClusterName: clusterName,
				Streamer:    streamer,
				SessionID:   sid,
				AuditLog:    auditLog,
				Log:         slog,
				Clock:       clockwork.NewRealClock(),
			})
		},
	}

	service, err := recordingencryptionv1.NewService(cfg)
	require.NoError(t, err)

	ctx := withAuthCtx(t.Context(), newServiceAuthCtx(t))
	_, err = service.CompleteUpload(ctx, &recordingencryptionv1pb.CompleteUploadRequest{
		Upload: &recordingencryptionv1pb.Upload{
			SessionId:   string(sessionID),
			InitiatedAt: timestamppb.Now(),
			UploadId:    uuid.NewString(),
		},
	})
	require.NoError(t, err)

	// The recovered session end event must have been emitted to the audit log.
	emitted := auditLog.Events()
	require.Len(t, emitted, 1)
	sessionEnd, ok := emitted[0].(*apievents.SessionEnd)
	require.True(t, ok, "expected *apievents.SessionEnd, got %T", emitted[0])
	require.Equal(t, string(sessionID), sessionEnd.GetSessionID())
	require.Equal(t, userMeta, sessionEnd.UserMetadata)
	require.True(t, sessionEnd.Interactive)
}

func newServiceAuthCtx(t *testing.T) authz.Context {
	t.Helper()
	role := authz.BuiltinRole{Role: types.RoleProxy, Username: string(types.RoleProxy)}
	ctx, err := authz.ContextForBuiltinRole(role, nil)
	require.NoError(t, err)
	return *ctx
}

func TestUploadValidation(t *testing.T) {
	newService := func(t *testing.T) *recordingencryptionv1.Service {
		t.Helper()
		svc, err := recordingencryptionv1.NewService(recordingencryptionv1.ServiceConfig{
			Authorizer:                &fakeAuthorizer{},
			Logger:                    logtest.NewLogger(),
			Uploader:                  fakeUploader{},
			KeyRotater:                newFakeKeyRotater(),
			RecordingMetadataProvider: recordingmetadata.NewProvider(),
			SessionSummarizerProvider: summarizer.NewSessionSummarizerProvider(),
			OnUploadComplete:          func(ctx context.Context, sessionID session.ID) (apievents.AuditEvent, error) { return nil, nil },
		})
		require.NoError(t, err)
		return svc
	}

	assertBadParameter := func(t require.TestingT, err error, _ ...any) {
		require.True(t, trace.IsBadParameter(err), "expected bad parameter error, got %v", err)
	}

	cases := []struct {
		name          string
		uploadPartReq *recordingencryptionv1pb.UploadPartRequest
		completeReq   *recordingencryptionv1pb.CompleteUploadRequest
	}{
		{
			name:          "nil upload field",
			uploadPartReq: &recordingencryptionv1pb.UploadPartRequest{},
			completeReq:   &recordingencryptionv1pb.CompleteUploadRequest{},
		},
		{
			name: "missing upload_id",
			uploadPartReq: &recordingencryptionv1pb.UploadPartRequest{
				Upload: &recordingencryptionv1pb.Upload{SessionId: uuid.NewString()},
			},
			completeReq: &recordingencryptionv1pb.CompleteUploadRequest{
				Upload: &recordingencryptionv1pb.Upload{SessionId: uuid.NewString()},
			},
		},
		{
			name: "missing session_id",
			uploadPartReq: &recordingencryptionv1pb.UploadPartRequest{
				Upload: &recordingencryptionv1pb.Upload{UploadId: uuid.NewString()},
			},
			completeReq: &recordingencryptionv1pb.CompleteUploadRequest{
				Upload: &recordingencryptionv1pb.Upload{UploadId: uuid.NewString()},
			},
		},
	}

	ctx := withAuthCtx(t.Context(), newServiceAuthCtx(t))

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc := newService(t)

			_, err := svc.UploadPart(ctx, c.uploadPartReq)
			assertBadParameter(t, err)

			_, err = svc.CompleteUpload(ctx, c.completeReq)
			assertBadParameter(t, err)
		})
	}
}

func TestAuthorizeUpload(t *testing.T) {
	newUpload := func() *recordingencryptionv1pb.Upload {
		return &recordingencryptionv1pb.Upload{
			SessionId:   uuid.NewString(),
			UploadId:    uuid.NewString(),
			InitiatedAt: timestamppb.Now(),
		}
	}
	newUploadReq := func() *recordingencryptionv1pb.CreateUploadRequest {
		return &recordingencryptionv1pb.CreateUploadRequest{SessionId: uuid.NewString()}
	}

	accessDeniedAssert := func(t require.TestingT, err error, i ...any) {
		require.Error(t, err)
		require.True(t, trace.IsAccessDenied(err))
		require.ErrorContains(t, err, "access denied")
	}

	cases := []struct {
		name          string
		authCtx       authz.Context
		errAssertFunc require.ErrorAssertionFunc
	}{
		{
			name:          "server builtin role is authorized",
			authCtx:       newServiceAuthCtx(t),
			errAssertFunc: require.NoError,
		},
		{
			name: "non-server builtin role is denied",
			authCtx: authz.Context{
				Identity: authz.BuiltinRole{Role: types.RoleAdmin},
			},

			errAssertFunc: accessDeniedAssert,
		},
		{
			name: "local user identity is denied",
			authCtx: authz.Context{
				Identity: authz.LocalUser{},
			},
			errAssertFunc: accessDeniedAssert,
		},
		{
			name: "unauthenticated role is denied",
			authCtx: authz.Context{
				Identity: authz.UnauthenticatedRole{Role: types.RoleNop},
			},
			errAssertFunc: accessDeniedAssert,
		},
		{
			name: "remote builtin role is denied",
			authCtx: authz.Context{
				Identity: authz.RemoteBuiltinRole{Role: types.RoleProxy},
			},
			errAssertFunc: accessDeniedAssert,
		},
	}

	newService := func(t *testing.T) *recordingencryptionv1.Service {
		t.Helper()
		svc := newFakeService(t, newFakeKeyRotater())
		return svc
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ctx := withAuthCtx(t.Context(), c.authCtx)
			svc := newService(t)

			// Test all three upload endpoints
			_, err := svc.CreateUpload(ctx, newUploadReq())
			c.errAssertFunc(t, err)

			_, err = svc.UploadPart(ctx, &recordingencryptionv1pb.UploadPartRequest{Upload: newUpload(), IsLast: true})
			c.errAssertFunc(t, err)

			_, err = svc.CompleteUpload(ctx, &recordingencryptionv1pb.CompleteUploadRequest{Upload: newUpload()})
			c.errAssertFunc(t, err)
		})
	}
}

func newTestTLSServer(t testing.TB) *authtest.TLSServer {
	t.Helper()
	as, err := authtest.NewAuthServer(authtest.AuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClockAt(time.Now().Round(time.Second).UTC()),
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, as.Close()) })

	srv, err := as.NewTestTLSServer(authtest.WithBufconnListener())
	require.NoError(t, err)

	t.Cleanup(func() {
		err := srv.Close()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		require.NoError(t, err)
	})

	return srv
}

func TestRecordingEncryptionService(t *testing.T) {
	t.Parallel()

	accessDenied := func(t require.TestingT, err error, _ ...any) {
		require.True(t, trace.IsAccessDenied(err), "expected access denied, got %v", err)
	}

	cases := []struct {
		name      string
		identity  authtest.TestIdentity
		assertErr require.ErrorAssertionFunc
	}{
		// Local user is never a server.
		{
			name:      "local user is denied",
			identity:  authtest.TestUser("alice"),
			assertErr: accessDenied,
		},
		// Server roles: IsServer() == true.
		{
			name:      "proxy builtin role is authorized",
			identity:  authtest.TestBuiltin(types.RoleProxy),
			assertErr: require.NoError,
		},
		{
			name:      "node builtin role is authorized",
			identity:  authtest.TestBuiltin(types.RoleNode),
			assertErr: require.NoError,
		},
		{
			name:      "kube builtin role is authorized",
			identity:  authtest.TestBuiltin(types.RoleKube),
			assertErr: require.NoError,
		},
		// Non-server system roles: IsServer() == false.
		{
			name:      "admin builtin role is denied",
			identity:  authtest.TestBuiltin(types.RoleAdmin),
			assertErr: accessDenied,
		},
		{
			name:      "nop (unauthenticated) is denied",
			identity:  authtest.TestNop(),
			assertErr: accessDenied,
		},
		{
			name:      "signup role is denied",
			identity:  authtest.TestBuiltin(types.RoleSignup),
			assertErr: accessDenied,
		},
	}

	testSrv := newTestTLSServer(t)

	_, _, err := authtest.CreateUserAndRole(testSrv.Auth(), "alice", nil, []types.Rule{
		types.NewRule(types.KindEvent, services.RW()),
	})
	require.NoError(t, err)

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			authClient, err := testSrv.NewClient(c.identity)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, authClient.Close()) })

			cli := authClient.RecordingEncryptionServiceClient()

			upload := &recordingencryptionv1pb.Upload{
				UploadId:    uuid.NewString(),
				SessionId:   uuid.NewString(),
				InitiatedAt: timestamppb.Now(),
			}

			createResp, err := cli.CreateUpload(t.Context(), &recordingencryptionv1pb.CreateUploadRequest{
				SessionId: uuid.NewString(),
			})
			c.assertErr(t, err)
			if err == nil {
				upload = createResp.Upload
			}

			_, err = cli.UploadPart(t.Context(), &recordingencryptionv1pb.UploadPartRequest{
				Upload: upload,
				IsLast: true,
			})
			c.assertErr(t, err)

			_, err = cli.CompleteUpload(t.Context(), &recordingencryptionv1pb.CompleteUploadRequest{
				Upload: upload,
			})
			c.assertErr(t, err)
		})
	}
}
