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

// TODO(gavin): delete this package after updating e to not depend on it.
package endpoints

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/gcp"
	"github.com/gravitational/teleport/lib/healthcheck"
	"github.com/gravitational/teleport/lib/srv/db/healthchecks"
)

// ResolverBuilder constructs a [Resolver].
type ResolverBuilder func(ctx context.Context, db types.Database, cfg ResolverBuilderConfig) (Resolver, error)

// Resolver resolves database endpoints.
type Resolver interface {
	// Resolve resolves database endpoints.
	Resolve(ctx context.Context) ([]string, error)
}

// ResolverFn is function that implements [Resolver].
type ResolverFn func(ctx context.Context) ([]string, error)

// Resolve resolves database endpoints.
func (f ResolverFn) Resolve(ctx context.Context) ([]string, error) {
	return f(ctx)
}

// ResolverBuilderConfig is the config a for a [ResolverBuilder].
type ResolverBuilderConfig struct {
	// GCPClients are clients used to resolve GCP endpoints.
	GCPClients GCPClients
}

// GCPClients are clients used to resolve GCP endpoints.
type GCPClients interface {
	// GetSQLAdminClient returns GCP Cloud SQL Admin client.
	GetSQLAdminClient(context.Context) (gcp.SQLAdminClient, error)
	// GetAlloyDBClient returns GCP AlloyDB Admin client.
	GetAlloyDBClient(context.Context) (gcp.AlloyDBAdminClient, error)
}

// RegisterResolver registers a new database endpoint resolver.
func RegisterResolver(builder ResolverBuilder, names ...string) {
	healthchecks.RegisterHealthChecker(func(ctx context.Context, cfg healthchecks.HealthCheckerConfig) (healthcheck.HealthChecker, error) {
		resolver, err := builder(ctx, cfg.Database, ResolverBuilderConfig{GCPClients: cfg.GCPClients})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return healthcheck.NewTargetDialer(resolver.Resolve), nil
	}, names...)
}
