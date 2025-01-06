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
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/interval"
)

var (
	// scanInterval is the interval at which the expiry checker scans for access requests.
	scanInterval = time.Minute * 5

	// pendingRequestGracePeriod is the grace period used when checking a pending request's expiry
	// as the expiry time may be extended on approval.
	pendingRequestGracePeriod = time.Second * 40
)

const (
	semaphoreName       = "auth.expiry"
	semaphoreExpiration = time.Minute * 5
	semaphoreJitter     = time.Minute

	// minPageDelay is the minimum delay between processing each page of access requests.
	minPageDelay           = time.Millisecond * 200
	accessRequestPageLimit = 100
	// maxExpiresPerCycle is an arbitrary limit on the number of requests to expire per cycle
	// to prevent any one auth server holding the lease for more than a couple of minutes.
	maxExpiresPerCycle = 120
)

// Config provides configuration for the expiry server.
type Config struct {
	// Log is the logger.
	Log *slog.Logger
	// Emitter is an events emitter, used to submit discrete events.
	Emitter apievents.Emitter
	// AccessPoint is a expiry access point.
	AccessPoint authclient.ExpiryAccessPoint
	// Clock is the clock used to calculate expiry.
	Clock clockwork.Clock
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
	if c.Clock == nil {
		c.Clock = clockwork.NewRealClock()
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

func (s *Service) getSemaphoreConfig() services.SemaphoreLockConfigWithRetry {
	return services.SemaphoreLockConfigWithRetry{
		SemaphoreLockConfig: services.SemaphoreLockConfig{
			Service: s.AccessPoint,
			Params: types.AcquireSemaphoreRequest{
				SemaphoreKind: types.KindAccessRequest,
				SemaphoreName: semaphoreName,
				MaxLeases:     1,
				Holder:        s.HostID,
			},
			Expiry: semaphoreExpiration,
			Clock:  s.Clock,
		},
		Retry: retryutils.LinearConfig{
			Clock:  s.Clock,
			First:  time.Second,
			Step:   semaphoreExpiration / 2,
			Max:    semaphoreExpiration,
			Jitter: retryutils.DefaultJitter,
		},
	}
}

// Run starts the expiry service.
func (s *Service) Run(ctx context.Context) error {
	semCfg := s.getSemaphoreConfig()

	poll := interval.New(interval.Config{
		Duration:      scanInterval,
		FirstDuration: retryutils.FullJitter(scanInterval),
		Jitter:        retryutils.SeventhJitter,
		Clock:         s.Clock,
	})
	defer poll.Stop()

	for {
		lease, err := services.AcquireSemaphoreLockWithRetry(ctx, semCfg)
		if err != nil {
			s.Log.WarnContext(ctx, "error acquiring semaphore", "error", err)
		} else {
			if err := s.processRequests(ctx); err != nil {
				s.Log.WarnContext(ctx, "error processing access requests", "error", err)
			}
			lease.Stop()
			if err := lease.Wait(); err != nil {
				s.Log.WarnContext(ctx, "error cleaning up semaphore", "error", err)
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-poll.Next():
		}
	}
}
func (s *Service) processRequests(ctx context.Context) error {
	requestsExpired := 0
	nextPageStart := ""
	for {
		var page []*types.AccessRequestV3
		var err error
		// Use time at read when calculating expiry of requests.
		readTime := s.Clock.Now()
		page, nextPageStart, err = s.getNextPageOfAccessRequests(ctx, nextPageStart)
		if err != nil {
			return trace.Wrap(err)
		}
		if len(page) == 0 {
			return nil
		}
		for _, req := range page {
			if !s.shouldExpire(req, readTime) {
				continue
			}
			requestsExpired++
			s.Log.InfoContext(ctx, "expiring access request", "request", req.GetName())
			if err := s.expireRequest(ctx, req); err != nil {
				s.Log.WarnContext(ctx, "error expiring access request", "error", err)
				continue
			}
			if requestsExpired >= maxExpiresPerCycle {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-s.Clock.After(retryutils.SeventhJitter(minPageDelay)):
		}
	}
}

func (s *Service) getNextPageOfAccessRequests(ctx context.Context, startKey string) ([]*types.AccessRequestV3, string, error) {
	req := &proto.ListAccessRequestsRequest{
		Sort:     proto.AccessRequestSort_DEFAULT,
		Limit:    accessRequestPageLimit,
		StartKey: startKey,
	}
	resp, err := s.AccessPoint.ListAccessRequests(ctx, req)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return resp.AccessRequests, resp.NextKey, nil
}

func (s *Service) shouldExpire(req types.AccessRequest, readTime time.Time) bool {
	expires := req.Expiry()
	// Add grace period for pending access requests as expiry time may be extended on approval.
	if req.GetState() == types.RequestState_PENDING {
		expires = expires.Add(pendingRequestGracePeriod)
	}
	return readTime.After(expires)
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
