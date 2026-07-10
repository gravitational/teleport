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
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/gravitational/teleport"
	discoveryservicev1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/discoveryservice/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
)

const (
	// heartbeatAnnouncePeriod is the cadence of the configuration heartbeat.
	heartbeatAnnouncePeriod = 30 * time.Second
	// heartbeatTTL is how long a heartbeat resource lives without renewal;
	// a dead service's resource expires within this window, making liveness
	// observable by absence.
	heartbeatTTL = 3 * heartbeatAnnouncePeriod
)

// Announcer upserts this service's configuration heartbeat resource. It is
// satisfied by the auth client; a nil announcer disables heartbeating.
type Announcer interface {
	UpsertDiscoveryService(ctx context.Context, svc *discoveryservicev1.DiscoveryService) (*discoveryservicev1.DiscoveryService, error)
}

// buildSelfHeartbeat assembles this service's discovery_service resource from
// current state. The payload must stay deterministic for identical state (no
// clocks in the spec): announce-on-change diffing depends on it.
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

// startHeartbeatAnnouncer periodically announces this service's configuration
// heartbeat until the server context is done. On NotImplemented from an older
// auth server it logs once and stops permanently; the service must keep
// discovering (and keep reporting ready) without a heartbeat.
func (s *Server) startHeartbeatAnnouncer() {
	go func() {
		ticker := s.clock.NewTicker(heartbeatAnnouncePeriod)
		defer ticker.Stop()
		for {
			if _, err := s.DiscoveryServiceAnnouncer.UpsertDiscoveryService(s.ctx, s.buildSelfHeartbeat()); err != nil {
				if trace.IsNotImplemented(err) {
					s.Log.WarnContext(s.ctx, "Auth server does not support discovery service heartbeats; heartbeating disabled until upgrade")
					return
				}
				s.Log.WarnContext(s.ctx, "Failed to announce discovery service heartbeat", "error", err)
			}
			select {
			case <-s.ctx.Done():
				return
			case <-ticker.Chan():
			}
		}
	}()
}
