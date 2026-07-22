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

package dynamodb

import (
	"cmp"
	"context"
	"net"

	"github.com/gravitational/trace"
	"golang.org/x/net/http/httpproxy"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/healthcheck"
	"github.com/gravitational/teleport/lib/srv/db/healthchecks"
	"github.com/gravitational/teleport/lib/utils/aws/dynamodbutils"
)

// NewHealthChecker resolves endpoints from DB URI.
func NewHealthChecker(ctx context.Context, cfg healthchecks.HealthCheckerConfig) (healthcheck.HealthChecker, error) {
	resolver, err := NewEndpointsResolver(ctx, cfg.Database)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return healthcheck.NewTargetDialer(resolver.Resolve), nil
}

// NewEndpointsResolver resolves endpoints from DB URI.
func NewEndpointsResolver(_ context.Context, db types.Database) (healthcheck.Resolver, error) {
	aws := db.GetAWS()
	fips := dynamodbutils.IsFIPSEnabled()
	resolverFns := []resolverFn{
		resolveDynamoDBEndpoint,
		resolveDynamoDBStreamsEndpoint,
	}
	return healthcheck.EndpointsResolverFunc(func(ctx context.Context) ([]string, error) {
		addrs := make([]string, 0, len(resolverFns))
		for _, resolve := range resolverFns {
			re, err := resolve(ctx, aws.Region, aws.AccountID, fips)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			// Not all of our DB engines respect http proxy env vars, but this one does.
			// The endpoint resolved for TCP health checks should be the one that the
			// agent will actually connect to, since often proxy env vars are set to
			// accommodate self-imposed network restrictions that force external traffic
			// to go through a proxy.
			proxyFunc := httpproxy.FromEnvironment().ProxyFunc()
			proxyURL, err := proxyFunc(re.URL)
			if err != nil {
				return nil, trace.Wrap(err)
			}

			if proxyURL != nil {
				re.URL = proxyURL
			}
			host := re.URL.Hostname()
			port := cmp.Or(re.URL.Port(), "443")
			addrs = append(addrs, net.JoinHostPort(host, port))
		}
		return addrs, nil
	}), nil
}
