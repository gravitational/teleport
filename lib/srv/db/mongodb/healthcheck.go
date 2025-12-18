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

package mongodb

import (
	"context"

	"github.com/gravitational/trace"
	"go.mongodb.org/mongo-driver/mongo/address"

	"github.com/gravitational/teleport/lib/healthcheck"
	"github.com/gravitational/teleport/lib/srv/db/healthchecks"
)

// NewHealthChecker returns an endpoint health checker.
// SRV URI (mongodb+srv://) is resolved to a seed list from DNS SRV record
// https://www.mongodb.com/docs/manual/reference/connection-string/#srv-connection-format
func NewHealthChecker(_ context.Context, cfg healthchecks.HealthCheckerConfig) (healthcheck.HealthChecker, error) {
	resolver := newEndpointsResolver(cfg.Database.GetURI())
	return healthcheck.NewTargetDialer(resolver.Resolve), nil
}

func newEndpointsResolver(uri string) healthcheck.EndpointsResolverFunc {
	return func(ctx context.Context) ([]string, error) {
		clientCfg, err := makeClientOptionsFromDatabaseURI(uri)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		endpoints := make([]string, 0, len(clientCfg.Hosts))
		for _, host := range clientCfg.Hosts {
			endpoints = append(endpoints, address.Address(host).String())
		}
		return endpoints, nil
	}
}
