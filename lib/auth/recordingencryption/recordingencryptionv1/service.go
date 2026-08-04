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

package recordingencryptionv1

import (
	"bytes"
	"context"
	"log/slog"
	"runtime"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	recordingencryptionv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/recordingencryption/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/recordingencryption"
	"github.com/gravitational/teleport/lib/auth/recordingmetadata"
	"github.com/gravitational/teleport/lib/auth/summarizer"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
	sessionpostprocessing "github.com/gravitational/teleport/lib/events/sessionpostprocessing"
	"github.com/gravitational/teleport/lib/session"
)

const maxSealedBatchesPerRequest = 32

// A KeyRotater facilitates rotation of encryption keys.
type KeyRotater interface {
	RotateKey(context.Context) error
	CompleteRotation(context.Context) error
	RollbackRotation(context.Context) error
	GetRotationState(context.Context) ([]*recordingencryptionv1.FingerprintWithState, error)
}

// ServiceConfig captures everything a [Service] requires to fulfill requests.
type ServiceConfig struct {
	Authorizer authz.Authorizer
	Logger     *slog.Logger
	Uploader   events.MultipartUploader
	KeyRotater KeyRotater
	// SessionSummarizerProvider is a provider of the session summarizer service.
	// It can be nil or provide a nil summarizer if summarization is not needed.
	// The summarizer itself summarizes session recordings.
	SessionSummarizerProvider *summarizer.SessionSummarizerProvider
	// RecordingMetadataProvider is a provider of the recording metadata service.
	RecordingMetadataProvider *recordingmetadata.Provider
	// OnUploadComplete is called after an upload completes to find or recover the
	// session end event.
	OnUploadComplete func(ctx context.Context, sessionID session.ID) (apievents.AuditEvent, error)
	// KeyUnwrapper decrypts wrapped file keys through the keystore. It is used
	// to decrypt sealed audit event batches submitted by agents.
	KeyUnwrapper recordingencryption.KeyUnwrapper
	// Emitter is the audit event emitter that decrypted audit event batches
	// are forwarded to.
	Emitter apievents.Emitter
}

// NewService returns a new [Service] based on the given [ServiceConfig].
func NewService(cfg ServiceConfig) (*Service, error) {
	switch {
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.Uploader == nil:
		return nil, trace.BadParameter("uploader is required")
	case cfg.KeyRotater == nil:
		return nil, trace.BadParameter("key rotater is required")
	case cfg.RecordingMetadataProvider == nil:
		return nil, trace.BadParameter("recording metadata provider is required")
	case cfg.SessionSummarizerProvider == nil:
		return nil, trace.BadParameter("session summarizer provider is required")
	case cfg.OnUploadComplete == nil:
		return nil, trace.BadParameter("on upload complete callback is required")
	case cfg.KeyUnwrapper == nil:
		return nil, trace.BadParameter("key unwrapper is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("emitter is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.With(teleport.ComponentKey, teleport.ComponentRecordingEncryption)
	}

	opener, err := recordingencryption.NewAuditQueueOpener(cfg.KeyUnwrapper)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		logger:                    cfg.Logger,
		uploader:                  cfg.Uploader,
		auth:                      cfg.Authorizer,
		rotater:                   cfg.KeyRotater,
		sessionSummarizerProvider: cfg.SessionSummarizerProvider,
		recordingMetadataProvider: cfg.RecordingMetadataProvider,
		onUploadComplete:          cfg.OnUploadComplete,
		opener:                    opener,
		emitter:                   cfg.Emitter,
		decryptSem:                semaphore.NewWeighted(int64(runtime.GOMAXPROCS(0))),
	}, nil
}

// Service implements a gRPC server for interacting with encrypted recordings.
type Service struct {
	recordingencryptionv1.UnimplementedRecordingEncryptionServiceServer

	auth     authz.Authorizer
	logger   *slog.Logger
	uploader events.MultipartUploader
	rotater  KeyRotater
	// sessionSummarizerProvider is a provider of the session summarizer service.
	// It can be nil or provide a nil summarizer if summarization is not needed.
	// The summarizer itself summarizes session recordings.
	sessionSummarizerProvider *summarizer.SessionSummarizerProvider
	// recordingMetadataProvider is a provider of the recording metadata service.
	recordingMetadataProvider *recordingmetadata.Provider
	// onUploadComplete is called after an upload completes to find or recover the
	// session end event for post-processing.
	onUploadComplete func(ctx context.Context, sessionID session.ID) (apievents.AuditEvent, error)
	// opener decrypts sealed audit event batches submitted by agents.
	opener *recordingencryption.AuditQueueOpener
	// emitter is the audit event emitter that decrypted audit event batches
	// are forwarded to.
	emitter    apievents.Emitter
	decryptSem *semaphore.Weighted
}

func streamUploadAsProto(upload events.StreamUpload) *recordingencryptionv1.Upload {
	return recordingencryptionv1.Upload_builder{
		UploadId:    upload.ID,
		SessionId:   upload.SessionID.String(),
		InitiatedAt: timestamppb.New(upload.Initiated),
	}.Build()
}

func protoAsStreamUpload(upload *recordingencryptionv1.Upload) (events.StreamUpload, error) {
	sessionID, err := session.ParseID(upload.GetSessionId())
	if err != nil {
		return events.StreamUpload{}, trace.Wrap(err)
	}

	return events.StreamUpload{
		ID:        upload.GetUploadId(),
		SessionID: *sessionID,
		Initiated: upload.GetInitiatedAt().AsTime(),
	}, nil
}

func protoAsStreamPart(part *recordingencryptionv1.Part) events.StreamPart {
	return events.StreamPart{
		Number:       part.GetPartNumber(),
		ETag:         part.GetEtag(),
		LastModified: time.Now(),
	}
}

// CreateUpload begins a multipart upload for an encrypted session recording.
func (s *Service) CreateUpload(ctx context.Context, req *recordingencryptionv1.CreateUploadRequest) (*recordingencryptionv1.CreateUploadResponse, error) {
	if err := s.authorizeUpload(ctx); err != nil {
		return nil, trace.AccessDenied("access denied")
	}

	s.logger.DebugContext(ctx, "creating encrypted session upload", "session_id", req.GetSessionId())
	sessionID, err := session.ParseID(req.GetSessionId())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	upload, err := s.uploader.CreateUpload(ctx, *sessionID)
	if err != nil {
		return nil, trace.Wrap(err, "creating encrypted recording upload")
	}

	return recordingencryptionv1.CreateUploadResponse_builder{
		Upload: streamUploadAsProto(*upload),
	}.Build(), nil
}

// UploadPart uploads an encrypted session recording part to the given upload ID.
func (s *Service) UploadPart(ctx context.Context, req *recordingencryptionv1.UploadPartRequest) (*recordingencryptionv1.UploadPartResponse, error) {
	if err := validateUpload(req.GetUpload()); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.authorizeUpload(ctx); err != nil {
		return nil, trace.AccessDenied("access denied")
	}

	s.logger.DebugContext(ctx, "uploading encrypted session part", "upload_id", req.GetUpload().GetUploadId(), "session_id", req.GetUpload().GetSessionId(), "part_number", req.GetPartNumber())
	upload, err := protoAsStreamUpload(req.GetUpload())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.uploader.ReserveUploadPart(ctx, upload, req.GetPartNumber()); err != nil {
		return nil, trace.Wrap(err)
	}

	// If upload part is not at least the minimum upload part size, append an empty part
	// to pad up to the minimum upload size.
	part := req.GetPart()
	if !req.GetIsLast() && len(part) < events.MinUploadPartSizeBytes {
		part = events.PadUploadPart(part, events.MinUploadPartSizeBytes)
	}

	streamPart, err := s.uploader.UploadPart(ctx, upload, req.GetPartNumber(), bytes.NewReader(part))
	if err != nil {
		return nil, trace.Wrap(err, "uploading encrypted recording part")
	}

	return recordingencryptionv1.UploadPartResponse_builder{
		Part: recordingencryptionv1.Part_builder{
			PartNumber: streamPart.Number,
			Etag:       streamPart.ETag,
		}.Build(),
	}.Build(), nil
}

// CompleteUpload marks a given encrypted session upload as complete.
func (s *Service) CompleteUpload(ctx context.Context, req *recordingencryptionv1.CompleteUploadRequest) (*recordingencryptionv1.CompleteUploadResponse, error) {
	if err := validateUpload(req.GetUpload()); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.authorizeUpload(ctx); err != nil {
		return nil, trace.AccessDenied("access denied")
	}

	s.logger.DebugContext(ctx, "completing encrypted session upload", "upload_id", req.GetUpload().GetUploadId(), "session_id", req.GetUpload().GetSessionId(), "parts", len(req.GetParts()))
	upload, err := protoAsStreamUpload(req.GetUpload())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	parts := make([]events.StreamPart, len(req.GetParts()))
	for idx, part := range req.GetParts() {
		parts[idx] = protoAsStreamPart(part)
	}

	if err := s.uploader.CompleteUpload(ctx, upload, parts); err != nil {
		return nil, trace.Wrap(err)
	}

	if s.onUploadComplete == nil {
		return &recordingencryptionv1.CompleteUploadResponse{}, nil
	}

	sessionEnd, err := s.onUploadComplete(ctx, upload.SessionID)
	if err != nil || sessionEnd == nil {
		return &recordingencryptionv1.CompleteUploadResponse{}, nil
	}

	if err := sessionpostprocessing.Process(
		ctx,
		sessionpostprocessing.Config{
			SessionEnd:                sessionEnd,
			SessionID:                 upload.SessionID,
			SessionSummarizerProvider: s.sessionSummarizerProvider,
			RecordingMetadataProvider: s.recordingMetadataProvider,
		},
	); err != nil {
		s.logger.WarnContext(ctx, "session post-processing failed", "error", err)
	}

	return &recordingencryptionv1.CompleteUploadResponse{}, nil
}

// SubmitAuditQueueBatch delivers one sealed batch of audit events from an
// agent's local audit queue.
func (s *Service) SubmitAuditQueueBatch(ctx context.Context, req *recordingencryptionv1.SubmitAuditQueueBatchRequest) (*recordingencryptionv1.SubmitAuditQueueBatchResponse, error) {
	serverID, isProxy, err := s.authorizeAuditQueueBatch(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(req.GetBatches()) > maxSealedBatchesPerRequest {
		return nil, trace.BadParameter("too many sealed audit event batches in a single request")
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), apidefaults.DefaultIOTimeout)
	defer cancel()
	for _, sealed := range req.GetBatches() {
		if sealed.GetFormat() != recordingencryptionv1.AuditQueueBatchFormat_AUDIT_QUEUE_BATCH_FORMAT_AGE_V1 {
			return nil, trace.BadParameter("unsupported audit queue batch format %v", sealed.GetFormat())
		}
	}

	decrypted := make([][]apievents.AuditEvent, len(req.GetBatches()))
	var g errgroup.Group
	for i, sealed := range req.GetBatches() {
		if err := s.decryptSem.Acquire(ctx, 1); err != nil {
			_ = g.Wait()
			return nil, trace.Wrap(err)
		}
		g.Go(func() error {
			defer s.decryptSem.Release(1)
			d, err := s.opener.DecryptBatch(ctx, sealed.GetPayload())
			if err != nil {
				s.logger.WarnContext(ctx, "failed to decrypt sealed audit event batch",
					"server_id", serverID,
					"error", err,
				)
				// this message is sparse on purpose to avoid conveying key state to the caller
				return trace.BadParameter("failed to decrypt audit event batch")
			}

			if sealed.GetEventCount() != int64(len(d)) {
				s.logger.WarnContext(ctx, "sealed audit event batch reported an event count that does not match its payload",
					"reported_count", sealed.GetEventCount(),
					"decrypted_count", len(d),
					"server_id", serverID,
				)
			}
			decrypted[i] = d
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, trace.Wrap(err)
	}

	var batch []apievents.AuditEvent
	for _, d := range decrypted {
		batch = append(batch, d...)
	}

	for _, event := range batch {
		if err := events.ValidateServerMetadata(event, serverID, isProxy); err != nil {
			const msg = "Rejecting audit event, the client is attempting to " +
				"submit events for an identity other than the one on its x509 certificate."
			s.logger.WarnContext(ctx, msg,
				"event_type", event.GetType(),
				"event_id", event.GetID(),
				"server_id", serverID,
				"error", err,
			)
			// this message is sparse on purpose to avoid conveying extra data to an attacker
			return nil, trace.AccessDenied("failed to validate event metadata")
		}
	}

	ctx = events.WithForwardedEmit(ctx)
	if err := events.EmitAuditEvents(ctx, s.emitter, batch); err != nil {
		return nil, trace.Wrap(err)
	}
	return &recordingencryptionv1.SubmitAuditQueueBatchResponse{}, nil
}

// authorizeAuditQueueBatch verifies that the caller is allowed to submit
// sealed audit event batches.
func (s *Service) authorizeAuditQueueBatch(ctx context.Context) (serverID string, isProxy bool, err error) {
	authCtx, err := s.auth.Authorize(ctx)
	if err != nil {
		return "", false, trace.Wrap(err)
	}

	role, ok := authCtx.Identity.(authz.BuiltinRole)
	if !ok || !role.IsServer() {
		return "", false, trace.AccessDenied("this request can be only executed by a Teleport server")
	}

	if err := authCtx.CheckAccessToKind(types.KindEvent, types.VerbCreate); err != nil {
		return "", false, trace.Wrap(err)
	}

	return role.GetServerID(), authz.HasBuiltinRole(*authCtx, string(types.RoleProxy)), nil
}

func (s *Service) authorizeKeyRotation(ctx context.Context) error {
	authCtx, err := s.auth.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := authCtx.CheckAccessToKind(types.KindRecordingEncryption, types.VerbCreate, types.VerbUpdate); err != nil {
		return trace.Wrap(err)
	}

	if err := authCtx.AuthorizeAdminAction(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// RotateKey starts the rotation process for the active key pair used while encrypting session recording data.
func (s *Service) RotateKey(ctx context.Context, req *recordingencryptionv1.RotateKeyRequest) (*recordingencryptionv1.RotateKeyResponse, error) {
	if err := s.authorizeKeyRotation(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.rotater.RotateKey(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return &recordingencryptionv1.RotateKeyResponse{}, nil
}

// CompleteRotation moves rotated keys out of the active set into new RotatedKey resources.
func (s *Service) CompleteRotation(ctx context.Context, req *recordingencryptionv1.CompleteRotationRequest) (*recordingencryptionv1.CompleteRotationResponse, error) {
	if err := s.authorizeKeyRotation(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.rotater.CompleteRotation(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return &recordingencryptionv1.CompleteRotationResponse{}, nil
}

// RollbackRotation removes active keys and reverts rotating keys back to being active.
func (s *Service) RollbackRotation(ctx context.Context, req *recordingencryptionv1.RollbackRotationRequest) (*recordingencryptionv1.RollbackRotationResponse, error) {
	if err := s.authorizeKeyRotation(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	if err := s.rotater.RollbackRotation(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	return &recordingencryptionv1.RollbackRotationResponse{}, nil
}

// GetRotationState the state and fingerprint of all currently active keys.
func (s *Service) GetRotationState(ctx context.Context, req *recordingencryptionv1.GetRotationStateRequest) (*recordingencryptionv1.GetRotationStateResponse, error) {
	if err := s.authorizeKeyRotation(ctx); err != nil {
		return nil, trace.Wrap(err)
	}

	states, err := s.rotater.GetRotationState(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return recordingencryptionv1.GetRotationStateResponse_builder{
		KeyPairStates: states,
	}.Build(), nil
}

func validateUpload(upload *recordingencryptionv1.Upload) error {
	switch {
	case upload == nil:
		return trace.BadParameter("upload is required")
	case upload.GetUploadId() == "":
		return trace.BadParameter("upload.upload_id is required")
	case upload.GetSessionId() == "":
		return trace.BadParameter("upload.session_id is required")
	}
	return nil
}

// authorizeUpload verifies that the caller is allowed to write encrypted
// session recording data.  Two conditions must both be satisfied:
//
//  1. Identity: the request must come from a local Teleport server (a
//     [authz.BuiltinRole] whose [authz.BuiltinRole.IsServer] returns true).
//     This excludes unauthenticated clients, regular users, remote (leaf
//     cluster) services, and non-server system roles such as RoleAdmin.
//
//  2. RBAC: the server's role set must hold create and update permission on
//     [types.KindEvent] in the default namespace.  Server roles that manage
//     session recordings (e.g. RoleProxy) satisfy this requirement through
//     their built-in rule definitions.
func (s *Service) authorizeUpload(ctx context.Context) error {
	authCtx, err := s.auth.Authorize(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	role, ok := authCtx.Identity.(authz.BuiltinRole)
	if !ok || !role.IsServer() {
		return trace.AccessDenied("this request can be only executed by a Teleport server")
	}

	var errs []error
	for _, verb := range []string{types.VerbCreate, types.VerbUpdate} {
		if err := authCtx.CheckAccessToKind(types.KindEvent, verb); err != nil {
			errs = append(errs, err)
		}
	}

	return trace.NewAggregate(errs...)
}
