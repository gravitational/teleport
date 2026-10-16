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

package auditlogv1

import (
	"cmp"
	"context"
	"log/slog"
	"runtime"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sync/semaphore"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	auditlogpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/auditlog/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth/recordingencryption"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/events"
)

const defaultDecryptWaitTimeout = 5 * time.Second

// AuditQueueServiceConfig configures the audit queue service.
type AuditQueueServiceConfig struct {
	Authorizer authz.Authorizer
	Logger     *slog.Logger
	// KeyUnwrapper decrypts wrapped file keys through the keystore. It is used
	// to decrypt sealed audit event batches submitted by agents.
	KeyUnwrapper recordingencryption.KeyUnwrapper
	// Emitter is the audit event emitter that decrypted audit event batches
	// are forwarded to.
	Emitter apievents.Emitter
	// MaxConcurrentDecrypts bounds how many sealed batches are decrypted at
	// once. Zero means one per available CPU.
	MaxConcurrentDecrypts int
	// DecryptWaitTimeout bounds how long a submission waits for a decrypt
	// slot before it is rejected. Zero means the default.
	DecryptWaitTimeout time.Duration
}

// NewAuditQueueService constructs a new AuditQueueService.
func NewAuditQueueService(cfg AuditQueueServiceConfig) (*AuditQueueService, error) {
	switch {
	case cfg.Authorizer == nil:
		return nil, trace.BadParameter("authorizer is required")
	case cfg.KeyUnwrapper == nil:
		return nil, trace.BadParameter("key unwrapper is required")
	case cfg.Emitter == nil:
		return nil, trace.BadParameter("emitter is required")
	}

	cfg.Logger = cmp.Or(cfg.Logger, slog.With(teleport.ComponentKey, teleport.ComponentAuditLog))

	opener, err := recordingencryption.NewAuditQueueOpener(cfg.KeyUnwrapper, nil)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	maxConcurrentDecrypts := cfg.MaxConcurrentDecrypts
	if maxConcurrentDecrypts <= 0 {
		maxConcurrentDecrypts = runtime.GOMAXPROCS(0)
	}
	decryptWaitTimeout := cfg.DecryptWaitTimeout
	if decryptWaitTimeout <= 0 {
		decryptWaitTimeout = defaultDecryptWaitTimeout
	}

	return &AuditQueueService{
		auth:               cfg.Authorizer,
		logger:             cfg.Logger,
		opener:             opener,
		emitter:            cfg.Emitter,
		decryptSem:         semaphore.NewWeighted(int64(maxConcurrentDecrypts)),
		decryptWaitTimeout: decryptWaitTimeout,
	}, nil
}

// AuditQueueService accepts sealed audit event batches from agents' local
// audit queues, decrypts them, and forwards the events to the audit log.
type AuditQueueService struct {
	auth               authz.Authorizer
	logger             *slog.Logger
	opener             *recordingencryption.AuditQueueOpener
	emitter            apievents.Emitter
	decryptSem         *semaphore.Weighted
	decryptWaitTimeout time.Duration
}

// SubmitAuditQueueBatch delivers a sealed batch of audit events from an
// agent's local audit queue.
func (s *AuditQueueService) SubmitAuditQueueBatch(ctx context.Context, req *auditlogpb.SubmitAuditQueueBatchRequest) (*auditlogpb.SubmitAuditQueueBatchResponse, error) {
	serverID, isProxy, err := s.authorizeAuditQueueBatch(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sealed := req.GetBatch()
	if sealed.GetFormat() != auditlogpb.AuditQueueBatchFormat_AUDIT_QUEUE_BATCH_FORMAT_AGE_V1 {
		return nil, trace.BadParameter("unsupported audit queue batch format %v", sealed.GetFormat())
	}

	ctx, cancel := context.WithTimeout(ctx, apidefaults.DefaultIOTimeout)
	defer cancel()

	batch, err := s.decryptBatch(ctx, sealed.GetPayload(), serverID)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if sealed.GetEventCount() != int64(len(batch)) {
		s.logger.WarnContext(ctx, "sealed audit event batch reported an event count that does not match its payload",
			"reported_count", sealed.GetEventCount(),
			"decrypted_count", len(batch),
			"server_id", serverID,
		)
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
	return &auditlogpb.SubmitAuditQueueBatchResponse{}, nil
}

func (s *AuditQueueService) decryptBatch(ctx context.Context, payload []byte, serverID string) ([]apievents.AuditEvent, error) {
	waitStart := time.Now()
	acquireCtx, cancelAcquire := context.WithTimeout(ctx, s.decryptWaitTimeout)
	err := s.decryptSem.Acquire(acquireCtx, 1)
	cancelAcquire()
	if err != nil {
		if ctx.Err() != nil {
			return nil, trace.Wrap(ctx.Err())
		}
		BatchSubmissionsRejectedMetric.Inc()
		return nil, trace.LimitExceeded("too many concurrent audit queue batch submissions, try again later")
	}
	defer s.decryptSem.Release(1)
	BatchSubmissionsAcceptedMetric.Inc()
	DecryptSlotWaitMetric.Observe(time.Since(waitStart).Seconds())

	batch, err := s.opener.DecryptBatch(ctx, payload)
	if err != nil {
		s.logger.WarnContext(ctx, "failed to decrypt sealed audit event batch",
			"server_id", serverID,
			"error", err,
		)
		// this message is sparse on purpose to avoid conveying key state to the caller
		return nil, trace.BadParameter("failed to decrypt audit event batch")
	}
	return batch, nil
}

func (s *AuditQueueService) authorizeAuditQueueBatch(ctx context.Context) (serverID string, isProxy bool, err error) {
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
