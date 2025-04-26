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

package db

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/healthcheck"
	"github.com/gravitational/teleport/lib/srv/db/mongodb"
	"github.com/gravitational/teleport/lib/srv/db/mysql"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
)

type resolverClients interface {
	mysql.ResolverClients
}

// getEndpointsResolver gets a health check endpoint resolver for the database.
func getEndpointsResolver(
	ctx context.Context,
	db types.Database,
	clients resolverClients,
) (healthcheck.EndpointsResolverFunc, error) {
	// TODO(gavin): add resolvers for all database protocols that we support
	switch db.GetProtocol() {
	case types.DatabaseProtocolPostgreSQL:
		return postgres.NewEndpointsResolverFunc(db.GetURI()), nil
	case types.DatabaseProtocolMySQL:
		return mysql.NewEndpointsResolverFunc(ctx, db, clients), nil
	case types.DatabaseProtocolMongoDB:
		return mongodb.NewEndpointsResolverFunc(db.GetURI()), nil
	default:
		// this should be unreachable because we check that a database supports
		// health checks before adding it to the health check manager, but we
		// have to satisfy the compiler
		return nil, trace.NotImplemented("endpoint health checks for protocol %q are not supported", db.GetProtocol())
	}
}
