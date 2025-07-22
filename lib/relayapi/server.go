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

package relayapi

import (
	"context"

	relayv1alpha "github.com/gravitational/teleport/api/gen/proto/go/teleport/relay/v1alpha"
)

type noUnkeyedLiterals struct{}

type unimplementedDiscoveryServiceServer = relayv1alpha.UnimplementedDiscoveryServiceServer

// StaticDiscoverServiceServer is a [relayv1alpha.DiscoveryServiceServer]
// implementation that responds with fixed data to the Discover rpc.
type StaticDiscoverServiceServer struct {
	_ noUnkeyedLiterals
	unimplementedDiscoveryServiceServer

	RelayGroup            string
	TunnelPublicAddr      string
	TargetConnectionCount int32
}

var _ relayv1alpha.DiscoveryServiceServer = (*StaticDiscoverServiceServer)(nil)

// Discover implements [relayv1alpha.DiscoveryServiceServer].
func (d *StaticDiscoverServiceServer) Discover(ctx context.Context, req *relayv1alpha.DiscoverRequest) (*relayv1alpha.DiscoverResponse, error) {
	return &relayv1alpha.DiscoverResponse{
		RelayGroup:            d.RelayGroup,
		TunnelPublicAddr:      d.TunnelPublicAddr,
		TargetConnectionCount: d.TargetConnectionCount,
	}, nil
}
