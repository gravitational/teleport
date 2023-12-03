/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package clusters

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
)

type GatewayCreator struct {
	resolver Resolver
}

func NewGatewayCreator(resolver Resolver) GatewayCreator {
	return GatewayCreator{
		resolver: resolver,
	}
}

func (g GatewayCreator) CreateGateway(ctx context.Context, params CreateGatewayParams) (gateway.Gateway, error) {
	cluster, _, err := g.resolver.ResolveCluster(params.TargetURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	gateway, err := cluster.CreateGateway(ctx, params)
	return gateway, trace.Wrap(err)
}

// Resolver is a subset of [Storage], mostly so that it's possible to provide a mock implementation
// in tests.
type Resolver interface {
	// ResolveCluster returns a cluster from storage given the URI. See [Storage.ResolveCluster].
	ResolveCluster(uri.ResourceURI) (*Cluster, *client.TeleportClient, error)
}
