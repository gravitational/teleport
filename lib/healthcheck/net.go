/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package healthcheck

import (
	"context"
	"net"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/types"
)

// EndpointsResolverFunc is callback func that returns endpoints for a target.
type EndpointsResolverFunc func(ctx context.Context) ([]string, error)

// dialFunc dials an address on the given network.
type dialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// TargetDialer is a health check target which uses a net.Dialer.
type TargetDialer struct {
	// Resolver resolves the target endpoint(s).
	Resolver EndpointsResolverFunc
	// dial is used to dial network connections.
	dial dialFunc
}

// NewTargetDialer returns a new TargetDialer ready for use.
func NewTargetDialer(resolver EndpointsResolverFunc) *TargetDialer {
	return &TargetDialer{
		Resolver: resolver,
		dial:     defaultDialer().DialContext,
	}
}

// GetProtocol returns the network protocol used for checking health.
func (t *TargetDialer) GetProtocol() types.TargetHealthProtocol {
	return types.TargetHealthProtocolTCP
}

// CheckHealth checks the health of the target resource.
func (t *TargetDialer) CheckHealth(ctx context.Context) ([]string, error) {
	return t.dialEndpoints(ctx)
}

func (t *TargetDialer) dialEndpoints(ctx context.Context) ([]string, error) {
	endpoints, err := t.Resolver(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "failed to resolve target endpoints")
	}
	switch len(endpoints) {
	case 0:
		return nil, trace.NotFound("resolved zero target endpoints")
	case 1:
		return endpoints, t.dialEndpoint(ctx, endpoints[0])
	default:
		group, ctx := errgroup.WithContext(ctx)
		group.SetLimit(10)
		for _, ep := range endpoints {
			group.Go(func() error {
				return trace.Wrap(t.dialEndpoint(ctx, ep))
			})
		}
		return endpoints, group.Wait()
	}
}

func (t *TargetDialer) dialEndpoint(ctx context.Context, endpoint string) error {
	conn, err := t.dial(ctx, "tcp", endpoint)
	if err != nil {
		return trace.Wrap(err)
	}
	// an error while closing the connection could indicate an RST packet from
	// the endpoint - that's a health check failure.
	return trace.Wrap(conn.Close())
}

func defaultDialer() *net.Dialer {
	return &net.Dialer{}
}
