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

package discovery

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	discoveryservicev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryservice/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/slices"
)

const (
	// heartbeatCheckPeriod bounds how late the announcer observes a retry or
	// renewal deadline. Snapshot construction happens only when one is due.
	heartbeatCheckPeriod = 5 * time.Second
	// heartbeatTTL is how long an Auth-stored heartbeat remains present without
	// renewal. Absence does not identify why announcements stopped.
	heartbeatTTL = apidefaults.ServerAnnounceTTL
	// heartbeatRetryPeriod is how long the announcer waits before retrying
	// after a transient announce failure.
	heartbeatRetryPeriod = 30 * time.Second
	// heartbeatRetryJitter is the maximum positive jitter added to each
	// retry wait so a fleet failing together does not retry together.
	heartbeatRetryJitter = 5 * time.Second
	// heartbeatRenewalJitter is the maximum positive jitter added to each
	// renewal interval so instance renewals spread out across the fleet.
	heartbeatRenewalJitter = time.Minute
	// heartbeatRPCTimeout bounds a single announce RPC so a hung call
	// cannot block the loop past the next check deadline indefinitely.
	heartbeatRPCTimeout = 20 * time.Second

	// maxStaticMatchersSize caps the serialized size of the static matcher
	// detail carried on the heartbeat. When the assembled payload exceeds the
	// cap, detail is replaced by per-cloud counts
	// with matchers_truncated set; never silently shortened, because a
	// partial matcher list produces confidently wrong diagnostics.
	maxStaticMatchersSize = 64 * 1024
)

// heartbeatAnnouncePeriod returns half the TTL plus positive jitter, bounded
// so renewal is due no later than six minutes after the attempt starts.
func (s *Server) heartbeatAnnouncePeriod() time.Duration {
	jitter := s.heartbeatJitter(heartbeatRenewalJitter)
	if jitter == 0 {
		jitter = time.Nanosecond
	}
	return heartbeatTTL/2 + jitter
}

// Announcer upserts this service's configuration heartbeat resource. It is
// satisfied by the auth client; a nil announcer disables heartbeating.
type Announcer interface {
	UpsertDiscoveryService(ctx context.Context, svc *discoveryservicev1.DiscoveryService) (*discoveryservicev1.DiscoveryService, error)
}

type staticMatcherProjection struct {
	matchers  *discoveryservicev1.StaticMatchers
	truncated bool
	counts    map[string]int32
}

func projectMatchers[T any](matchers []T, project func(*T)) []*T {
	return slices.Map(matchers, func(matcher T) *T {
		if project != nil {
			project(&matcher)
		}
		return &matcher
	})
}

// buildStaticMatchers converts the effective static matcher set into its
// heartbeat representation. Installer parameters are omitted because they
// control post-discovery enrollment rather than resource selection. When the
// remaining detail exceeds the size budget, it falls back to per-cloud counts
// with matchers_truncated set.
func (s *Server) buildStaticMatchers() (staticMatcherProjection, error) {
	// Copies are shallow, which is enough: only top-level fields are ever
	// written (the Params = nil projections below), never nested contents.
	// Nested slices and label maps still alias live config, which static
	// discovery treats as immutable after startup.
	var accessGraph *types.AccessGraphSync
	if s.Matchers.AccessGraph != nil {
		accessGraphCopy := *s.Matchers.AccessGraph
		accessGraph = &accessGraphCopy
	}
	staticMatchers := discoveryservicev1.StaticMatchers_builder{
		Aws: projectMatchers(s.Matchers.AWS, func(matcher *types.AWSMatcher) {
			matcher.Params = nil
		}),
		Azure: projectMatchers(s.Matchers.Azure, func(matcher *types.AzureMatcher) {
			matcher.Params = nil
		}),
		Gcp: projectMatchers(s.Matchers.GCP, func(matcher *types.GCPMatcher) {
			matcher.Params = nil
		}),
		Kube:        projectMatchers(s.Matchers.Kubernetes, nil),
		AccessGraph: accessGraph,
	}.Build()

	// encoding/json measures the same open-struct representation the storage
	// codec persists; like [services.MarshalDiscoveryService], this size check
	// requires the generated open struct and must be revisited together with
	// that codec before any protoopaque migration.
	encoded, err := json.Marshal(staticMatchers)
	if err != nil {
		return staticMatcherProjection{}, trace.Wrap(err)
	}
	if len(encoded) <= maxStaticMatchersSize {
		return staticMatcherProjection{matchers: staticMatchers}, nil
	}

	counts := make(map[string]int32)
	setCount := func(name string, count int) {
		if count > 0 {
			counts[name] = int32(count)
		}
	}
	setCount(services.StaticMatcherCountKeyAWS, len(s.Matchers.AWS))
	setCount(services.StaticMatcherCountKeyAzure, len(s.Matchers.Azure))
	setCount(services.StaticMatcherCountKeyGCP, len(s.Matchers.GCP))
	setCount(services.StaticMatcherCountKeyKube, len(s.Matchers.Kubernetes))
	if m := s.Matchers.AccessGraph; m != nil {
		setCount(services.StaticMatcherCountKeyAccessGraph, len(m.AWS)+len(m.Azure))
	}
	return staticMatcherProjection{truncated: true, counts: counts}, nil
}

// buildSelfHeartbeat assembles this service's discovery_service resource from
// current producer state immediately before a due announcement attempt. v1
// fields originate in configuration initialized at Server startup.
func (s *Server) buildSelfHeartbeat() (*discoveryservicev1.DiscoveryService, error) {
	staticMatchers, err := s.buildStaticMatchers()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return discoveryservicev1.DiscoveryService_builder{
		Kind:    types.KindDiscoveryService,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name: s.ServerID,
		}.Build(),
		Spec: discoveryservicev1.DiscoveryServiceSpec_builder{
			Hostname:            s.Hostname,
			TeleportVersion:     teleport.Version,
			DiscoveryGroup:      s.DiscoveryGroup,
			PollInterval:        durationpb.New(s.PollInterval),
			StaticMatchers:      staticMatchers.matchers,
			MatchersTruncated:   staticMatchers.truncated,
			StaticMatcherCounts: staticMatchers.counts,
		}.Build(),
		Status: &discoveryservicev1.DiscoveryServiceStatus{},
	}.Build(), nil
}

type heartbeatAnnounceState struct {
	retryNotBefore       time.Time
	renewalDeadline      time.Time
	loggedNotImplemented bool
	matchersTruncated    bool
}

func (s heartbeatAnnounceState) attemptDue(now time.Time) bool {
	if !s.retryNotBefore.IsZero() {
		return !now.Before(s.retryNotBefore)
	}
	return s.renewalDeadline.IsZero() || !now.Before(s.renewalDeadline)
}

func (s *heartbeatAnnounceState) matcherTruncationStarted(truncated bool) bool {
	started := truncated && !s.matchersTruncated
	s.matchersTruncated = truncated
	return started
}

func (s *Server) announceHeartbeatOnce(attemptStart time.Time, state *heartbeatAnnounceState) {
	hb, err := s.buildSelfHeartbeat()
	if err != nil {
		s.Log.WarnContext(s.ctx, "Failed to build discovery service heartbeat", "error", err)
	} else {
		if state.matcherTruncationStarted(hb.GetSpec().GetMatchersTruncated()) {
			s.Log.WarnContext(s.ctx, "Static matcher details exceed the discovery service heartbeat size limit and will be omitted", "limit", maxStaticMatchersSize)
		}

		ctx, cancel := clockwork.WithTimeout(s.ctx, s.clock, heartbeatRPCTimeout)
		_, err = s.DiscoveryServiceAnnouncer.UpsertDiscoveryService(ctx, hb)
		cancel()
		switch {
		case err == nil:
			state.retryNotBefore = time.Time{}
			state.renewalDeadline = attemptStart.Add(s.heartbeatAnnouncePeriod())
			s.Log.DebugContext(s.ctx, "Announced discovery service heartbeat", "renewal_deadline", state.renewalDeadline)
			return
		case trace.IsNotImplemented(err):
			if !state.loggedNotImplemented {
				s.Log.WarnContext(s.ctx, "Auth server does not support discovery service heartbeats; will retry periodically until it is upgraded")
				state.loggedNotImplemented = true
			}
		default:
			// A canceled server context means shutdown interrupted the
			// RPC; that is not an announce failure and logging it as one
			// makes a routine shutdown look like a heartbeat problem.
			if s.ctx.Err() != nil {
				return
			}
			s.Log.WarnContext(s.ctx, "Failed to announce discovery service heartbeat", "error", err)
		}
	}
	state.retryNotBefore = s.clock.Now().Add(heartbeatRetryPeriod + s.heartbeatJitter(heartbeatRetryJitter))
}

// startHeartbeatAnnouncer runs the announce loop until the server context is
// done: a full upsert on start and at every jittered renewal interval. The
// snapshot is rebuilt only when an attempt is due. All failures, including
// snapshot construction and NotImplemented, use the ordinary bounded retry.
// Publication is diagnostic and does not affect service readiness.
//
// The loop is deliberately not built on srv.Heartbeat or srv.HeartbeatV2:
// the former announces legacy types.Resource values through a shared per-mode
// state machine that every new resource kind must be registered into, and the
// latter rides the inventory control stream, whose fleet-global schema this
// service-specific payload should not extend. A protov2 resource announced
// through its own self-scoped RPC needs neither; the cost is this small loop.
func (s *Server) startHeartbeatAnnouncer() {
	go func() {
		var state heartbeatAnnounceState

		ticker := s.clock.NewTicker(heartbeatCheckPeriod)
		defer ticker.Stop()
		for {
			// A due attempt against an already-canceled context would fail
			// instantly and log a spurious announce failure during shutdown.
			if s.ctx.Err() != nil {
				return
			}
			now := s.clock.Now()
			if state.attemptDue(now) {
				s.announceHeartbeatOnce(now, &state)
			}
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.Chan():
			}
		}
	}()
}
