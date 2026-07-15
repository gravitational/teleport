/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package discovery

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types/discoveryconfig"
)

const (
	// syntheticDiscoveryConfigTTL is how long a synthetic discovery config
	// stays in the backend without being refreshed by its publisher. Once the
	// publishing Discovery Service stops, the resource expires within this
	// window.
	syntheticDiscoveryConfigTTL = time.Hour
	// syntheticDiscoveryConfigKeepAliveInterval is the cadence at which the
	// synthetic discovery config is re-upserted to refresh its TTL. Following
	// the heartbeat convention, it is 2/3 of the TTL, leaving one full retry
	// budget before the resource expires.
	syntheticDiscoveryConfigKeepAliveInterval = syntheticDiscoveryConfigTTL * 2 / 3
	// syntheticDiscoveryConfigRetryInterval is the retry cadence after a
	// transient publish failure.
	syntheticDiscoveryConfigRetryInterval = time.Minute
)

// startSyntheticDiscoveryConfigPublisher publishes the service's static
// (file-based) matchers to the backend as a synthetic DiscoveryConfig and
// keeps the resource alive by refreshing its TTL periodically. The published
// matchers are the effective ones, i.e. after unsupported matchers were
// discarded.
//
// The static configuration cannot change without a service restart, so the
// resource content is built once; only the expiry moves on each refresh.
//
// Does not block; runs until the server context is done. No-op if the service
// has no static matchers.
func (s *Server) startSyntheticDiscoveryConfigPublisher() {
	if s.Matchers.IsEmpty() {
		return
	}

	dc, err := discoveryconfig.NewSyntheticDiscoveryConfig(s.ServerID, discoveryconfig.Spec{
		DiscoveryGroup: s.DiscoveryGroup,
		AWS:            s.Matchers.AWS,
		Azure:          s.Matchers.Azure,
		GCP:            s.Matchers.GCP,
		Kube:           s.Matchers.Kubernetes,
		AccessGraph:    s.Matchers.AccessGraph,
	})
	if err != nil {
		// The synthetic config is informational only; failing to build it must
		// not prevent the service from starting.
		s.Log.WarnContext(s.ctx, "Not publishing the static discovery configuration as a synthetic discovery config", "error", err)
		return
	}

	s.Log.DebugContext(s.ctx, "Starting synthetic discovery config publisher", "discovery_config", dc.GetName())
	go s.runSyntheticDiscoveryConfigPublisher(s.ctx, dc)
}

// runSyntheticDiscoveryConfigPublisher runs the keep-alive loop for the
// synthetic discovery config until ctx is done.
func (s *Server) runSyntheticDiscoveryConfigPublisher(ctx context.Context, dc *discoveryconfig.DiscoveryConfig) {
	// rejected tracks whether the last failure was a persistent rejection
	// (access denied or a name conflict), so those (expected in some cluster
	// states) errors are logged prominently only once per streak.
	var rejected bool

	for {
		wait := s.jitter(syntheticDiscoveryConfigKeepAliveInterval)

		dc.SetExpiry(s.clock.Now().UTC().Add(syntheticDiscoveryConfigTTL))
		switch _, err := s.AccessPoint.UpsertDiscoveryConfig(ctx, dc); {
		case err == nil:
			rejected = false
			s.syntheticDiscoveryConfigPublished.Store(true)
			s.Log.DebugContext(ctx, "Published synthetic discovery config", "discovery_config", dc.GetName())
		case trace.IsAccessDenied(err):
			// Older Auth servers do not allow Discovery Services to publish
			// synthetic discovery configs. This only affects visibility of the
			// static configuration in the backend; discovery itself is
			// unaffected. Keep trying at the keep-alive cadence in case Auth
			// gets upgraded.
			s.syntheticDiscoveryConfigPublished.Store(false)
			if !rejected {
				s.Log.WarnContext(ctx, "Auth server rejected the synthetic discovery config; it likely predates this feature. Static discovery configuration will not be visible in the cluster until Auth is upgraded.",
					"discovery_config", dc.GetName(),
					"error", err,
				)
				rejected = true
			} else {
				s.Log.DebugContext(ctx, "Auth server keeps rejecting the synthetic discovery config", "error", err)
			}
		case trace.IsAlreadyExists(err):
			// A user-created discovery config occupies the synthetic name and
			// Auth refuses to overwrite it. This cannot resolve on its own:
			// an admin must delete the conflicting resource. Keep trying at
			// the keep-alive cadence in case that happens.
			s.syntheticDiscoveryConfigPublished.Store(false)
			if !rejected {
				s.Log.WarnContext(ctx, "A user-created discovery config occupies the name reserved for this service's synthetic discovery config. Static discovery configuration will not be visible in the cluster until the conflicting resource is deleted.",
					"discovery_config", dc.GetName(),
					"error", err,
				)
				rejected = true
			} else {
				s.Log.DebugContext(ctx, "Synthetic discovery config name is still taken by a user-created resource", "error", err)
			}
		default:
			rejected = false
			wait = s.jitter(syntheticDiscoveryConfigRetryInterval)
			s.Log.WarnContext(ctx, "Failed to publish synthetic discovery config, will retry",
				"discovery_config", dc.GetName(),
				"retry_interval", wait,
				"error", err,
			)
		}

		select {
		case <-ctx.Done():
			return
		case <-s.clock.After(wait):
		}
	}
}
