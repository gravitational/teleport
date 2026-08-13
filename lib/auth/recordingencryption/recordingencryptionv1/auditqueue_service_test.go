// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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
	"bytes"
	"context"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"runtime"
	"sync"
	"testing"

	"filippo.io/age"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protodelim"
	"google.golang.org/protobuf/protoadapt"

	recordingencryptionv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/recordingencryption"
	"github.com/gravitational/teleport/lib/auth/recordingencryption/recordingencryptionv1"
	"github.com/gravitational/teleport/lib/auth/recordingmetadata"
	"github.com/gravitational/teleport/lib/auth/summarizer"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/events/eventstest"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

type rsaKeyUnwrapper struct {
	key *rsa.PrivateKey
}

func (u rsaKeyUnwrapper) UnwrapKey(ctx context.Context, in recordingencryption.UnwrapInput) ([]byte, error) {
	if u.key == nil {
		return nil, trace.NotFound("no decryption key available")
	}
	fileKey, err := u.key.Decrypt(in.Rand, in.WrappedKey, in.Opts)
	return fileKey, trace.Wrap(err)
}

var (
	auditQueueKeyOnce sync.Once
	auditQueueKey     *rsa.PrivateKey
	auditQueueKeyErr  error
)

func auditQueueTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	auditQueueKeyOnce.Do(func() {
		signer, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.RSA4096)
		if err != nil {
			auditQueueKeyErr = err
			return
		}
		key, ok := signer.(*rsa.PrivateKey)
		if !ok {
			auditQueueKeyErr = trace.Errorf("expected *rsa.PrivateKey, got %T", signer)
			return
		}
		auditQueueKey = key
	})
	require.NoError(t, auditQueueKeyErr)
	return auditQueueKey
}

func sealAuditBatch(t *testing.T, key *rsa.PrivateKey, batchEvents []apievents.AuditEvent) []byte {
	t.Helper()
	var payloadBuf bytes.Buffer
	for _, event := range batchEvents {
		oneOf, err := apievents.ToOneOf(event)
		require.NoError(t, err)
		_, err = protodelim.MarshalTo(&payloadBuf, protoadapt.MessageV2Of(oneOf))
		require.NoError(t, err)
	}
	payload := payloadBuf.Bytes()

	pubDER, err := x509.MarshalPKIXPublicKey(key.Public())
	require.NoError(t, err)
	recipient, err := recordingencryption.ParseAuditQueueRecipient(pubDER)
	require.NoError(t, err)

	var sealed bytes.Buffer
	w, err := age.Encrypt(&sealed, recipient)
	require.NoError(t, err)
	_, err = w.Write(payload)
	require.NoError(t, err)
	require.NoError(t, w.Close())
	return sealed.Bytes()
}

func newAuditQueueService(t *testing.T, key *rsa.PrivateKey, emitter apievents.Emitter) *recordingencryptionv1.Service {
	t.Helper()
	service, err := recordingencryptionv1.NewService(recordingencryptionv1.ServiceConfig{
		Authorizer:                &fakeAuthorizer{},
		Logger:                    logtest.NewLogger(),
		Uploader:                  fakeUploader{},
		KeyRotater:                newFakeKeyRotater(),
		RecordingMetadataProvider: recordingmetadata.NewProvider(),
		SessionSummarizerProvider: summarizer.NewSessionSummarizerProvider(),
		OnUploadComplete:          func(ctx context.Context, sessionID session.ID) (apievents.AuditEvent, error) { return nil, nil },
		KeyUnwrapper:              rsaKeyUnwrapper{key: key},
		Emitter:                   emitter,
	})
	require.NoError(t, err)
	return service
}

func newServerAuthCtx(t *testing.T, role types.SystemRole, serverID string) authz.Context {
	t.Helper()
	const clusterName = "test-cluster"
	builtin := authz.BuiltinRole{
		Role:        role,
		Username:    serverID + "." + clusterName,
		ClusterName: clusterName,
		Identity:    tlsca.Identity{Username: serverID + "." + clusterName},
	}
	authCtx, err := authz.ContextForBuiltinRole(builtin, nil)
	require.NoError(t, err)
	return *authCtx
}

func submitBatchRequest(payload []byte, eventCount int64) *recordingencryptionv1pb.SubmitAuditQueueBatchRequest {
	return recordingencryptionv1pb.SubmitAuditQueueBatchRequest_builder{
		Batches: []*recordingencryptionv1pb.AuditQueueSealedBatch{
			recordingencryptionv1pb.AuditQueueSealedBatch_builder{
				Payload:    payload,
				Format:     recordingencryptionv1pb.AuditQueueBatchFormat_AUDIT_QUEUE_BATCH_FORMAT_AGE_V1,
				EventCount: eventCount,
			}.Build(),
		},
	}.Build()
}

func auditQueueTestEvent(id string) apievents.AuditEvent {
	return &apievents.UserLogin{
		Metadata: apievents.Metadata{
			ID:   id,
			Type: "user.login",
			Code: "T1000I",
		},
	}
}

type failingEmitter struct{}

func (failingEmitter) EmitAuditEvent(context.Context, apievents.AuditEvent) error {
	return trace.Errorf("audit backend unavailable")
}

func TestSubmitAuditQueueBatch_RoundTrip(t *testing.T) {
	key := auditQueueTestKey(t)
	emitter := &eventstest.MockRecorderEmitter{}
	service := newAuditQueueService(t, key, emitter)

	batch := []apievents.AuditEvent{
		auditQueueTestEvent("a"),
		auditQueueTestEvent("b"),
		auditQueueTestEvent("c"),
	}
	sealed := sealAuditBatch(t, key, batch)

	ctx := withAuthCtx(t.Context(), newServerAuthCtx(t, types.RoleNode, "node-1"))
	resp, err := service.SubmitAuditQueueBatch(ctx, submitBatchRequest(sealed, int64(len(batch))))
	require.NoError(t, err)
	require.NotNil(t, resp)
	require.Equal(t, batch, emitter.Events())
}

func TestSubmitAuditQueueBatch_MultipleBatchesPreserveOrder(t *testing.T) {
	const minProcs = 4
	if prev := runtime.GOMAXPROCS(0); prev < minProcs {
		runtime.GOMAXPROCS(minProcs)
		t.Cleanup(func() { runtime.GOMAXPROCS(prev) })
	}

	key := auditQueueTestKey(t)
	emitter := &eventstest.MockRecorderEmitter{}
	service := newAuditQueueService(t, key, emitter)

	const batchCount = 24
	const eventsPerBatch = 3
	batches := make([]*recordingencryptionv1pb.AuditQueueSealedBatch, batchCount)
	var want []apievents.AuditEvent
	for i := range batches {
		batchEvents := make([]apievents.AuditEvent, eventsPerBatch)
		for j := range batchEvents {
			batchEvents[j] = auditQueueTestEvent(fmt.Sprintf("b%02d-e%d", i, j))
		}
		want = append(want, batchEvents...)
		batches[i] = recordingencryptionv1pb.AuditQueueSealedBatch_builder{
			Payload:    sealAuditBatch(t, key, batchEvents),
			Format:     recordingencryptionv1pb.AuditQueueBatchFormat_AUDIT_QUEUE_BATCH_FORMAT_AGE_V1,
			EventCount: eventsPerBatch,
		}.Build()
	}

	ctx := withAuthCtx(t.Context(), newServerAuthCtx(t, types.RoleNode, "node-1"))
	_, err := service.SubmitAuditQueueBatch(ctx, recordingencryptionv1pb.SubmitAuditQueueBatchRequest_builder{
		Batches: batches,
	}.Build())
	require.NoError(t, err)
	require.Equal(t, want, emitter.Events())
}

func TestSubmitAuditQueueBatch_ServerMetadata(t *testing.T) {
	key := auditQueueTestKey(t)

	sessionStart := func(serverID, forwardedBy string) apievents.AuditEvent {
		return &apievents.SessionStart{
			Metadata: apievents.Metadata{ID: "s1", Type: "session.start"},
			ServerMetadata: apievents.ServerMetadata{
				ServerID:    serverID,
				ForwardedBy: forwardedBy,
			},
		}
	}

	t.Run("matching server id is accepted", func(t *testing.T) {
		emitter := &eventstest.MockRecorderEmitter{}
		service := newAuditQueueService(t, key, emitter)
		sealed := sealAuditBatch(t, key, []apievents.AuditEvent{sessionStart("node-1", "")})

		ctx := withAuthCtx(t.Context(), newServerAuthCtx(t, types.RoleNode, "node-1"))
		_, err := service.SubmitAuditQueueBatch(ctx, submitBatchRequest(sealed, 1))
		require.NoError(t, err)
		require.Len(t, emitter.Events(), 1)
	})

	t.Run("matching forwarded proxy is accepted", func(t *testing.T) {
		emitter := &eventstest.MockRecorderEmitter{}
		service := newAuditQueueService(t, key, emitter)
		sealed := sealAuditBatch(t, key, []apievents.AuditEvent{sessionStart("node-9", "proxy-1")})

		ctx := withAuthCtx(t.Context(), newServerAuthCtx(t, types.RoleProxy, "proxy-1"))
		_, err := service.SubmitAuditQueueBatch(ctx, submitBatchRequest(sealed, 1))
		require.NoError(t, err)
		require.Len(t, emitter.Events(), 1)
	})
}

func TestSubmitAuditQueueBatch_Authorization(t *testing.T) {
	key := auditQueueTestKey(t)
	sealed := sealAuditBatch(t, key, []apievents.AuditEvent{auditQueueTestEvent("a")})

	t.Run("unauthenticated is rejected", func(t *testing.T) {
		emitter := &eventstest.MockRecorderEmitter{}
		service := newAuditQueueService(t, key, emitter)

		_, err := service.SubmitAuditQueueBatch(t.Context(), submitBatchRequest(sealed, 1))
		require.Error(t, err)
		require.Empty(t, emitter.Events())
	})

	t.Run("builtin non-server is rejected", func(t *testing.T) {
		emitter := &eventstest.MockRecorderEmitter{}
		service := newAuditQueueService(t, key, emitter)

		ctx := withAuthCtx(t.Context(), newAuthCtx(t, authz.AdminActionAuthMFAVerified))
		_, err := service.SubmitAuditQueueBatch(ctx, submitBatchRequest(sealed, 1))
		require.True(t, trace.IsAccessDenied(err), "expected access denied, got %v", err)
		require.Empty(t, emitter.Events())
	})
}

func TestSubmitAuditQueueBatch_RejectsTooManyBatches(t *testing.T) {
	key := auditQueueTestKey(t)
	sealed := sealAuditBatch(t, key, []apievents.AuditEvent{auditQueueTestEvent("a")})
	ctx := withAuthCtx(t.Context(), newServerAuthCtx(t, types.RoleNode, "node-1"))

	batches := make([]*recordingencryptionv1pb.AuditQueueSealedBatch, 33)
	for i := range batches {
		batches[i] = recordingencryptionv1pb.AuditQueueSealedBatch_builder{
			Payload:    sealed,
			Format:     recordingencryptionv1pb.AuditQueueBatchFormat_AUDIT_QUEUE_BATCH_FORMAT_AGE_V1,
			EventCount: 1,
		}.Build()
	}

	emitter := &eventstest.MockRecorderEmitter{}
	service := newAuditQueueService(t, key, emitter)
	_, err := service.SubmitAuditQueueBatch(ctx, recordingencryptionv1pb.SubmitAuditQueueBatchRequest_builder{
		Batches: batches,
	}.Build())
	require.True(t, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	require.Empty(t, emitter.Events())

	_, err = service.SubmitAuditQueueBatch(ctx, recordingencryptionv1pb.SubmitAuditQueueBatchRequest_builder{
		Batches: batches[:32],
	}.Build())
	require.NoError(t, err)
	require.Len(t, emitter.Events(), 32)
}

func TestSubmitAuditQueueBatch_RejectsUnknownFormat(t *testing.T) {
	key := auditQueueTestKey(t)
	sealed := sealAuditBatch(t, key, []apievents.AuditEvent{auditQueueTestEvent("a")})
	ctx := withAuthCtx(t.Context(), newServerAuthCtx(t, types.RoleNode, "node-1"))

	for name, format := range map[string]recordingencryptionv1pb.AuditQueueBatchFormat{
		"unspecified": recordingencryptionv1pb.AuditQueueBatchFormat_AUDIT_QUEUE_BATCH_FORMAT_UNSPECIFIED,
		"unknown":     recordingencryptionv1pb.AuditQueueBatchFormat(99),
	} {
		t.Run(name, func(t *testing.T) {
			emitter := &eventstest.MockRecorderEmitter{}
			service := newAuditQueueService(t, key, emitter)

			req := recordingencryptionv1pb.SubmitAuditQueueBatchRequest_builder{
				Batches: []*recordingencryptionv1pb.AuditQueueSealedBatch{
					recordingencryptionv1pb.AuditQueueSealedBatch_builder{
						Payload:    sealed,
						Format:     format,
						EventCount: 1,
					}.Build(),
				},
			}.Build()
			_, err := service.SubmitAuditQueueBatch(ctx, req)
			require.True(t, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
			require.Empty(t, emitter.Events())
		})
	}
}

func TestSubmitAuditQueueBatch_DecryptFailure(t *testing.T) {
	key := auditQueueTestKey(t)
	ctx := withAuthCtx(t.Context(), newServerAuthCtx(t, types.RoleNode, "node-1"))

	t.Run("garbage payload", func(t *testing.T) {
		emitter := &eventstest.MockRecorderEmitter{}
		service := newAuditQueueService(t, key, emitter)

		_, err := service.SubmitAuditQueueBatch(ctx, submitBatchRequest([]byte("not an age payload"), 1))
		require.ErrorContains(t, err, "failed to decrypt audit event batch")
		require.Empty(t, emitter.Events())
	})

	t.Run("wrong key", func(t *testing.T) {
		otherSigner, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.RSA4096)
		require.NoError(t, err)
		otherKey, ok := otherSigner.(*rsa.PrivateKey)
		require.True(t, ok)

		emitter := &eventstest.MockRecorderEmitter{}
		service := newAuditQueueService(t, key, emitter)
		sealed := sealAuditBatch(t, otherKey, []apievents.AuditEvent{auditQueueTestEvent("a")})

		_, err = service.SubmitAuditQueueBatch(ctx, submitBatchRequest(sealed, 1))
		require.ErrorContains(t, err, "failed to decrypt audit event batch")
		require.Empty(t, emitter.Events())
	})
}

func TestSubmitAuditQueueBatch_EmitFailureIsNacked(t *testing.T) {
	key := auditQueueTestKey(t)
	service := newAuditQueueService(t, key, failingEmitter{})
	sealed := sealAuditBatch(t, key, []apievents.AuditEvent{auditQueueTestEvent("a")})

	ctx := withAuthCtx(t.Context(), newServerAuthCtx(t, types.RoleNode, "node-1"))
	resp, err := service.SubmitAuditQueueBatch(ctx, submitBatchRequest(sealed, 1))
	require.ErrorContains(t, err, "audit backend unavailable")
	require.Nil(t, resp)
}

func TestSubmitAuditQueueBatch_EventCountMismatchStillDelivers(t *testing.T) {
	key := auditQueueTestKey(t)
	emitter := &eventstest.MockRecorderEmitter{}
	service := newAuditQueueService(t, key, emitter)
	sealed := sealAuditBatch(t, key, []apievents.AuditEvent{auditQueueTestEvent("a")})

	ctx := withAuthCtx(t.Context(), newServerAuthCtx(t, types.RoleNode, "node-1"))
	_, err := service.SubmitAuditQueueBatch(ctx, submitBatchRequest(sealed, 99))
	require.NoError(t, err)
	require.Len(t, emitter.Events(), 1)
}

func TestNewService_AuditQueueConfigValidation(t *testing.T) {
	baseConfig := func() recordingencryptionv1.ServiceConfig {
		return recordingencryptionv1.ServiceConfig{
			Authorizer:                &fakeAuthorizer{},
			Logger:                    logtest.NewLogger(),
			Uploader:                  fakeUploader{},
			KeyRotater:                newFakeKeyRotater(),
			RecordingMetadataProvider: recordingmetadata.NewProvider(),
			SessionSummarizerProvider: summarizer.NewSessionSummarizerProvider(),
			OnUploadComplete:          func(ctx context.Context, sessionID session.ID) (apievents.AuditEvent, error) { return nil, nil },
			KeyUnwrapper:              rsaKeyUnwrapper{},
			Emitter:                   &eventstest.MockRecorderEmitter{},
		}
	}

	t.Run("missing key unwrapper", func(t *testing.T) {
		cfg := baseConfig()
		cfg.KeyUnwrapper = nil
		_, err := recordingencryptionv1.NewService(cfg)
		require.ErrorContains(t, err, "key unwrapper is required")
	})

	t.Run("missing emitter", func(t *testing.T) {
		cfg := baseConfig()
		cfg.Emitter = nil
		_, err := recordingencryptionv1.NewService(cfg)
		require.ErrorContains(t, err, "emitter is required")
	})
}
