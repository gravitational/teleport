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

package snowflake

import (
	"cmp"
	"context"
	"net"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/healthcheck"
	"github.com/gravitational/teleport/lib/srv/db/healthchecks"
)

// NewEndpointsResolver resolves an endpoint from DB URI.
func NewEndpointsResolver(_ context.Context, db types.Database) (healthcheck.Resolver, error) {
	dbURL, err := parseURI(db.GetURI())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	host := dbURL.Hostname()
	port := cmp.Or(dbURL.Port(), "443")
	hostPort := net.JoinHostPort(host, port)
	return healthcheck.EndpointsResolverFunc(func(context.Context) ([]string, error) {
		return []string{hostPort}, nil
	}), nil
}

// NewHealthChecker resolves an endpoint from DB URI.
func NewHealthChecker(ctx context.Context, cfg healthchecks.HealthCheckerConfig) (healthcheck.HealthChecker, error) {
	resolver, err := NewEndpointsResolver(ctx, cfg.Database)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return healthcheck.NewTargetDialer(resolver.Resolve), nil
}
