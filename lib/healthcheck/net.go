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
	"strings"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"
)

// EndpointsResolverFunc is callback func that returns endpoints for a target.
type EndpointsResolverFunc func(ctx context.Context) ([]string, error)

// dialFunc dials an address on the given network.
type dialFunc func(ctx context.Context, network, addr string) (net.Conn, error)

// TargetDialer is a health check target which uses a net.Dialer.
type TargetDialer struct {
	// Resolver resolves the target endpoint(s).
	Resolver EndpointsResolverFunc
	// lastResolvedEndpoints are the endpoints last resolved for a health check.
	lastResolvedEndpoints []string

	// -- test fields below --

	// dial used to mock dialing in tests
	dial dialFunc
}

func (t *TargetDialer) checkAndSetDefaults() error {
	if t.Resolver == nil {
		return trace.BadParameter("missing target endpoint resolver")
	}
	if t.dial == nil {
		t.dial = defaultDialer().DialContext
	}
	return nil
}

func (t *TargetDialer) CheckHealth(ctx context.Context) error {
	return t.dialEndpoints(ctx)
}

func (t *TargetDialer) dialEndpoints(ctx context.Context) error {
	endpoints, err := t.Resolver(ctx)
	if err != nil {
		return trace.Wrap(err, "failed to resolve target endpoints")
	}
	t.lastResolvedEndpoints = endpoints
	switch len(endpoints) {
	case 0:
		return trace.NotFound("resolved zero target endpoints")
	case 1:
		return t.dialEndpoint(ctx, endpoints[0])
	default:
		group, ctx := errgroup.WithContext(ctx)
		group.SetLimit(10)
		for _, ep := range endpoints {
			group.Go(func() error {
				return trace.Wrap(t.dialEndpoint(ctx, ep))
			})
		}
		return group.Wait()
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

func (t *TargetDialer) lastAddress() string {
	return strings.Join(t.lastResolvedEndpoints, ",")
}

func defaultDialer() *net.Dialer {
	return &net.Dialer{}
}
