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
	"os"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	discoveryservicev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryservice/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

const (
	// heartbeatCheckPeriod is how often the announcer rebuilds the payload
	// and checks it for changes; a changed spec is announced within this
	// period rather than waiting for the next scheduled renewal.
	heartbeatCheckPeriod = 5 * time.Second
	// heartbeatTTL is how long a heartbeat resource lives without renewal; a
	// dead service's resource expires within this window, making liveness
	// observable by absence.
	heartbeatTTL = apidefaults.ServerAnnounceTTL
	// heartbeatRetryPeriod is how long the announcer waits before retrying
	// after a transient announce failure.
	heartbeatRetryPeriod = time.Minute
	// heartbeatNotImplementedRetryPeriod is how long the announcer waits
	// between probes after the auth server reported that it does not support
	// discovery service heartbeats, so heartbeating resumes without an agent
	// restart once auth is upgraded.
	heartbeatNotImplementedRetryPeriod = time.Hour
)

// heartbeatAnnouncePeriod returns the jittered renewal interval: half the TTL
// plus up to a tenth, matching the pattern used by other service heartbeats
// so fleet announces do not synchronize.
func (s *Server) heartbeatAnnouncePeriod() time.Duration {
	return heartbeatTTL/2 + s.jitter(heartbeatTTL/10)
}

// Announcer upserts this service's configuration heartbeat resource. It is
// satisfied by the auth client; a nil announcer disables heartbeating.
type Announcer interface {
	UpsertDiscoveryService(ctx context.Context, svc *discoveryservicev1.DiscoveryService) (*discoveryservicev1.DiscoveryService, error)
}

// buildSelfHeartbeat assembles this service's discovery_service resource from
// current state. The spec must stay deterministic for identical state (no
// clocks or counters): announce-on-change compares specs, so any machine-speed
// value here would turn the change detector into an announce firehose.
//
// The spec's matcher and binding fields are populated in a follow-up change;
// this heartbeat carries identity, group claim, and liveness.
func (s *Server) buildSelfHeartbeat() *discoveryservicev1.DiscoveryService {
	hostname := s.Hostname
	if hostname == "" {
		hostname, _ = os.Hostname()
	}

	return discoveryservicev1.DiscoveryService_builder{
		Kind:    types.KindDiscoveryService,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name:    s.ServerID,
			Expires: timestamppb.New(s.clock.Now().UTC().Add(heartbeatTTL)),
		}.Build(),
		Spec: discoveryservicev1.DiscoveryServiceSpec_builder{
			Hostname:        hostname,
			TeleportVersion: teleport.Version,
			DiscoveryGroup:  s.DiscoveryGroup,
			PollInterval:    durationpb.New(s.PollInterval),
		}.Build(),
		Status: &discoveryservicev1.DiscoveryServiceStatus{},
	}.Build()
}

// startHeartbeatAnnouncer runs the announce loop until the server context is
// done: a full upsert on start, at every jittered renewal interval, and
// within heartbeatCheckPeriod of the spec changing. Transient failures retry
// after heartbeatRetryPeriod; a NotImplemented auth server is probed at a
// slow cadence so heartbeating resumes after an auth upgrade without an
// agent restart. Announce outcomes are reported through OnHeartbeat when set,
// driving process readiness.
func (s *Server) startHeartbeatAnnouncer() {
	go func() {
		var lastAnnouncedSpec *discoveryservicev1.DiscoveryServiceSpec
		var nextAnnounce time.Time // zero: announce immediately
		var loggedNotImplemented bool

		ticker := s.clock.NewTicker(heartbeatCheckPeriod)
		defer ticker.Stop()
		for {
			hb := s.buildSelfHeartbeat()
			specChanged := lastAnnouncedSpec == nil || !proto.Equal(hb.GetSpec(), lastAnnouncedSpec)
			if specChanged || !s.clock.Now().Before(nextAnnounce) {
				_, err := s.DiscoveryServiceAnnouncer.UpsertDiscoveryService(s.ctx, hb)
				switch {
				case err == nil:
					lastAnnouncedSpec = hb.GetSpec()
					nextAnnounce = s.clock.Now().Add(s.heartbeatAnnouncePeriod())
					s.onHeartbeat(nil)
				case trace.IsNotImplemented(err):
					if !loggedNotImplemented {
						s.Log.WarnContext(s.ctx, "Auth server does not support discovery service heartbeats; will retry periodically until it is upgraded")
						loggedNotImplemented = true
					}
					// The service is healthy even though the heartbeat has
					// nowhere to land; report ready and probe again later.
					lastAnnouncedSpec = hb.GetSpec()
					nextAnnounce = s.clock.Now().Add(heartbeatNotImplementedRetryPeriod)
					s.onHeartbeat(nil)
				default:
					s.Log.WarnContext(s.ctx, "Failed to announce discovery service heartbeat", "error", err)
					lastAnnouncedSpec = hb.GetSpec()
					nextAnnounce = s.clock.Now().Add(s.jitter(heartbeatRetryPeriod))
					s.onHeartbeat(err)
				}
			}
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.Chan():
			}
		}
	}()
}

// onHeartbeat reports an announce outcome to the process, if a callback is
// configured.
func (s *Server) onHeartbeat(err error) {
	if s.OnHeartbeat != nil {
		s.OnHeartbeat(err)
	}
}
