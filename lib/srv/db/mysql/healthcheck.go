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

package mysql

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/time/rate"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/healthcheck"
	"github.com/gravitational/teleport/lib/srv/db/cloud"
	"github.com/gravitational/teleport/lib/srv/db/endpoints"
	"github.com/gravitational/teleport/lib/srv/db/healthchecks"
)

// NewHealthChecker creates a new MySQL endpoint health checker.
func NewHealthChecker(_ context.Context, cfg healthchecks.HealthCheckerConfig) (healthcheck.HealthChecker, error) {
	resolver, err := newEndpointsResolver(cfg.Database, cfg.GCPClients)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return healthcheck.NewTargetDialer(resolver.Resolve), nil
}

// resolverClients are API clients needed to resolve MySQL endpoints.
type resolverClients interface {
	// GetSQLAdminClient returns GCP Cloud SQL Admin client.
	GetSQLAdminClient(context.Context) (gcp.SQLAdminClient, error)
}

func newEndpointsResolver(db types.Database, clients resolverClients) (endpoints.Resolver, error) {
	switch {
	case db.IsCloudSQL():
		return newCloudSQLEndpointResolver(db, clients), nil
	default:
		return endpoints.ResolverFn(func(ctx context.Context) ([]string, error) {
			return []string{db.GetURI()}, nil
		}), nil
	}
}

func newCloudSQLEndpointResolver(db types.Database, clients resolverClients) endpoints.Resolver {
	// avoid checking the ssl mode more than once every 15 minutes.
	sometimes := rate.Sometimes{Interval: 15 * time.Minute}
	var requireSSL bool
	return endpoints.ResolverFn(func(ctx context.Context) ([]string, error) {
		var requireSSLErr error
		sometimes.Do(func() {
			clt, err := clients.GetSQLAdminClient(ctx)
			if err != nil {
				requireSSLErr = trace.Wrap(err)
				return
			}

			requireSSL, err = cloud.GetGCPRequireSSL(ctx, db, clt)
			if err != nil && !trace.IsAccessDenied(err) {
				requireSSLErr = trace.Wrap(err)
				return
			}
		})
		if requireSSLErr != nil {
			return nil, trace.Wrap(requireSSLErr)
		}
		if requireSSL {
			return []string{getGCPTLSAddress(db.GetURI())}, nil
		}
		return []string{db.GetURI()}, nil
	})
}
