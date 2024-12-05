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

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/interval"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

var (
	// scanInterval is the interval at which the expiry checker scans for access requests
	scanInterval              = time.Minute * 5
	pendingRequestGracePeriod = time.Second * 40
)

const (
	semaphoreName       = "expiry"
	semaphoreExpiration = time.Minute
	semaphoreJitter     = time.Minute

	// minPageDelay is the minimum delay between processing each page of access requests
	minPageDelay           = time.Millisecond * 200
	accessRequestPageLimit = 100
	maxExpiresPerCycle     = 120
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
		return trace.BadParameter("no Clock configured for expiry")
	}
	return nil
}

// Service is a expiry service.
type Service struct {
	*Config

	ctx      context.Context
	cancelfn context.CancelFunc
}

// New initializes a expiry service
func New(ctx context.Context, cfg *Config) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	s := &Service{
		Config: cfg,
		ctx:    ctx,
	}
	return s, nil
}

// Run starts the expiry service.
func (s *Service) Run() error {
	semCfg := services.SemaphoreLockConfigWithRetry{
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

	poll := interval.New(interval.Config{
		Duration:      scanInterval,
		FirstDuration: retryutils.FullJitter(scanInterval),
		Jitter:        retryutils.SeventhJitter,
		Clock:         s.Clock,
	})
	defer poll.Stop()

	for {
		lease, err := services.AcquireSemaphoreLockWithRetry(
			s.ctx,
			semCfg,
		)
		if err != nil {
			s.Log.WarnContext(s.ctx, "error aquiring semaphore", "error", err)
			continue
		}

		if err := s.processRequests(); err != nil {
			s.Log.WarnContext(s.ctx, "error processing access requests", "error", err)
		}
		ctx, cancel := context.WithCancel(lease)
		defer cancel()
		lease.Stop()
		if err := lease.Wait(); err != nil {
			s.Log.WarnContext(ctx, "error cleaning up semaphore", "error", err)
		}
		select {
		case <-s.ctx.Done():
			return nil
		case <-poll.Next():
		}
	}
}
func (s *Service) processRequests() error {
	requestsExpired := 0
	nextPageStart := ""
	for {
		var page []*types.AccessRequestV3
		var err error
		readTime := s.Clock.Now()
		page, nextPageStart, err = s.getNextPageOfAccessRequests(nextPageStart)
		if err != nil {
			return trace.Wrap(err)
		}
		if len(page) == 0 || requestsExpired >= maxExpiresPerCycle {
			return nil
		}
		minPageDelay := time.After(retryutils.SeventhJitter(minPageDelay))
		for _, req := range page {
			if !s.shouldExpire(req, readTime) {
				continue
			}
			if err := s.expireRequest(s.ctx, req); err != nil {
				return trace.Wrap(err)
			}
			requestsExpired++
			if requestsExpired >= maxExpiresPerCycle {
				return nil
			}
		}
		<-minPageDelay
	}
}

func (s *Service) getNextPageOfAccessRequests(startKey string) ([]*types.AccessRequestV3, string, error) {
	req := &proto.ListAccessRequestsRequest{
		Sort:       proto.AccessRequestSort_CREATED,
		Descending: true,
		Limit:      accessRequestPageLimit,
		StartKey:   startKey,
	}
	resp, err := s.AccessPoint.ListAccessRequests(s.ctx, req)
	if err != nil {
		return nil, "", trace.Wrap(err)
	}

	return resp.AccessRequests, resp.NextKey, nil
}

func (s *Service) shouldExpire(req types.AccessRequest, readTime time.Time) bool {
	expires := req.Expiry()
	if req.GetState() == types.RequestState_PENDING {
		expires = expires.Add(pendingRequestGracePeriod)
	}
	return readTime.After(expires)
}

func (s *Service) expireRequest(ctx context.Context, req types.AccessRequest) error {
	var annotations *apievents.Struct
	if sa := req.GetSystemAnnotations(); len(sa) > 0 {
		var err error
		annotations, err = apievents.EncodeMapStrings(sa)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	expiry := req.Expiry()
	event := &apievents.AccessRequestExpire{
		Metadata: apievents.Metadata{
			Type: events.AccessRequestExpireEvent,
			Code: events.AccessRequestExpireCode,
		},
		ResourceMetadata: apievents.ResourceMetadata{
			Expires: req.GetAccessExpiry(),
		},
		Roles:                req.GetRoles(),
		RequestedResourceIDs: apievents.ResourceIDs(req.GetRequestedResourceIDs()),
		RequestID:            req.GetName(),
		RequestState:         req.GetState().String(),
		Reason:               req.GetRequestReason(),
		MaxDuration:          req.GetMaxDuration(),
		Annotations:          annotations,
		ResourceExpiry:       &expiry,
	}
	if err := s.Emitter.EmitAuditEvent(ctx, event); err != nil {
		return trace.Wrap(err)
	}

	if err := s.AccessPoint.DeleteAccessRequest(ctx, req.GetName()); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
