/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package expiry

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/clientutils"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/interval"
)

const (
	semaphoreName       = "auth.expiry"
	semaphoreExpiration = time.Minute * 5

	// scanInterval is the interval at which the expiry checker scans for access requests.
	scanInterval = time.Minute * 5

	// pendingRequestGracePeriod is the grace period used when checking a pending request's expiry
	// as the expiry time may be extended on approval.
	pendingRequestGracePeriod = time.Second * 40

	// maxExpiresPerCycle is an arbitrary limit on the number of requests to expire per cycle
	// to prevent any one auth server holding the lease for more than a couple of minutes.
	maxExpiresPerCycle = 120
)

// AccessPoint is the API used by the expiry service.
type AccessPoint interface {
	// Semaphores provides semaphore operations
	types.Semaphores

	// ListExpiredAccessRequests lists all access requests that are expired. This is used by
	// the expiry service. Access requests expiration handling is done outside the backend
	// because we need to emit audit events on the access requests expiry.
	ListExpiredAccessRequests(ctx context.Context, limit int, pageToken string) ([]*types.AccessRequestV3, string, error)

	// DeleteAccessRequest deletes an access request.
	DeleteAccessRequest(ctx context.Context, reqID string) error
}

// Config provides configuration for the expiry server.
type Config struct {
	// Log is the logger.
	Log *slog.Logger
	// Emitter is an events emitter, used to submit discrete events.
	Emitter apievents.Emitter
	// AccessPoint provides backend operations.
	AccessPoint AccessPoint
	// HostID is a unique ID of this host.
	HostID string
}

// CheckAndSetDefaults checks required fields and sets default values.
func (c *Config) CheckAndSetDefaults() error {
	if c.Emitter == nil {
		return trace.BadParameter("no Emitter configured for expiry")
	}
	if c.AccessPoint == nil {
		return trace.BadParameter("no AccessPoint configured for expiry")
	}
	return nil
}

// Service is a expiry service.
type Service struct {
	*Config
}

// New initializes a expiry service
func New(cfg *Config) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	s := &Service{
		Config: cfg,
	}
	return s, nil
}

// Run starts the expiry service.
func (s *Service) Run(ctx context.Context) error {
	return s.run(ctx, interval.Config{
		Duration:      scanInterval,
		FirstDuration: retryutils.FullJitter(scanInterval),
		Jitter:        retryutils.SeventhJitter,
	})
}

// run is there for testing, so a testing interval can be set.
func (s *Service) run(ctx context.Context, intervalCfg interval.Config) error {
	for {
		if err := s.runWithLock(ctx, intervalCfg); err != nil && !errors.Is(err, context.Canceled) {
			s.Log.ErrorContext(ctx, "Expiry service failed", "error", err)
		}

		select {
		case <-ctx.Done():
			// If context was canceled, we should stop.
			return nil
		case <-time.After(semaphoreExpiration):
			// Otherwise, semaphore is likely not available. Wait and retry.
		}
	}
}

func (s *Service) runWithLock(ctx context.Context, intervalCfg interval.Config) error {
	lease, err := services.AcquireSemaphoreLockWithRetry(
		ctx,
		services.SemaphoreLockConfigWithRetry{
			SemaphoreLockConfig: services.SemaphoreLockConfig{
				Service: s.AccessPoint,
				Params: types.AcquireSemaphoreRequest{
					SemaphoreKind: types.KindAccessRequest,
					SemaphoreName: semaphoreName,
					MaxLeases:     1,
					Holder:        s.HostID,
				},
				Expiry: semaphoreExpiration,
			},
			Retry: retryutils.LinearConfig{
				First:  time.Second,
				Step:   semaphoreExpiration / 2,
				Max:    semaphoreExpiration,
				Jitter: retryutils.DefaultJitter,
			},
		},
	)
	if err != nil {
		return trace.Wrap(err)
	}
	defer func() {
		lease.Stop()
		if err := lease.Wait(); err != nil {
			s.Log.WarnContext(ctx, "Error cleaning up semaphore.", "error", err)
		}
	}()

	err = s.loop(lease, intervalCfg)
	return trace.Wrap(err)
}

// run is for testing so a duration without jitter can be specified.
func (s *Service) loop(ctx context.Context, intervalCfg interval.Config) error {
	interval := interval.New(intervalCfg)
	defer interval.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-interval.Next():
			s.processRequests(ctx)
		}
	}
}

func (s *Service) processRequests(ctx context.Context) {
	s.Log.DebugContext(ctx, "Cleaning up expired access requests.")

	requestsExpired := 0
	readTime := time.Now()
	for expiredAccessRequest, err := range clientutils.Resources(ctx, s.AccessPoint.ListExpiredAccessRequests) {
		if err != nil {
			s.Log.ErrorContext(ctx, "Error listing expired access requests.", "error", err)
			return
		}

		// Add grace period for pending access requests as expiry time may be extended on approval.
		if expiredAccessRequest.GetState() == types.RequestState_PENDING {
			expiry := expiredAccessRequest.Expiry().Add(pendingRequestGracePeriod)
			if !readTime.After(expiry) {
				continue
			}
		}

		requestsExpired++
		s.Log.DebugContext(ctx, "Expiring access request.", "request", expiredAccessRequest.GetName())
		if err := s.expireRequest(ctx, expiredAccessRequest); err != nil {
			s.Log.ErrorContext(ctx, "Error expiring access request.", "error", err)
			continue
		}
		if requestsExpired >= maxExpiresPerCycle {
			s.Log.DebugContext(ctx, "Cleaned up maximum amount of expired access requests. Will continue in the next run.", "max", maxExpiresPerCycle)
			return
		}
	}

	s.Log.DebugContext(ctx, "Successfully cleaned up expired access requests.", "count", requestsExpired)
}

func (s *Service) expireRequest(ctx context.Context, req types.AccessRequest) error {
	expiry := req.Expiry()
	event := &apievents.AccessRequestExpire{
		Metadata: apievents.Metadata{
			Type: events.AccessRequestExpireEvent,
			Code: events.AccessRequestExpireCode,
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Expires: req.GetAccessExpiry(),
		},
		RequestID:      req.GetName(),
		ResourceExpiry: &expiry,
	}
	// Emit expiry event before deletion as event we know event is expired here
	// but the deletion may fail.
	if err := s.Emitter.EmitAuditEvent(ctx, event); err != nil {
		return trace.Wrap(err)
	}

	if err := s.AccessPoint.DeleteAccessRequest(ctx, req.GetName()); err != nil {
		if trace.IsNotFound(err) {
			s.Log.InfoContext(ctx, "access request was already deleted", "request", req.GetName())
			return nil
		}
		return trace.Wrap(err)
	}
	return nil
}
