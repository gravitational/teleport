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

package clickhouse

import (
	"cmp"
	"context"
	"net"

	"github.com/gravitational/trace"
	"golang.org/x/net/http/httpproxy"

	"github.com/gravitational/teleport/lib/healthcheck"
	"github.com/gravitational/teleport/lib/srv/db/healthchecks"
)

// NewHTTPEndpointHealthChecker resolves a ClickHouse HTTP endpoint from DB URI.
func NewHTTPEndpointHealthChecker(_ context.Context, cfg healthchecks.HealthCheckerConfig) (healthcheck.HealthChecker, error) {
	dbURL, err := getURL(cfg.Database)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Not all of our DB engines respect http proxy env vars, but this one does.
	// The endpoint resolved for TCP health checks should be the one that the
	// agent will actually connect to, since often proxy env vars are set to
	// accommodate self-imposed network restrictions that force external traffic
	// to go through a proxy.
	proxyFunc := httpproxy.FromEnvironment().ProxyFunc()
	proxyURL, err := proxyFunc(dbURL)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if proxyURL != nil {
		dbURL = proxyURL
	}
	host := dbURL.Hostname()
	port := cmp.Or(dbURL.Port(), "443")
	hostPort := net.JoinHostPort(host, port)
	return healthcheck.NewTargetDialer(func(context.Context) ([]string, error) {
		return []string{hostPort}, nil
	}), nil
}

// NewNativeEndpointHealthChecker resolves a ClickHouse native endpoint from DB URI.
func NewNativeEndpointHealthChecker(_ context.Context, cfg healthchecks.HealthCheckerConfig) (healthcheck.HealthChecker, error) {
	endpoint, err := getNativeEndpoint(cfg.Database)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return healthcheck.NewTargetDialer(func(context.Context) ([]string, error) {
		return []string{endpoint}, nil
	}), nil
}
