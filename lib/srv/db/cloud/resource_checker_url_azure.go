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

package cloud

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/azure"
	"github.com/gravitational/teleport/lib/defaults"
)

func (c *urlChecker) checkAzure(ctx context.Context, database types.Database) error {
	// TODO check by fetching the resources from Azure and compare the URLs.
	if err := c.checkIsAzureEndpoint(ctx, database); err != nil {
		return trace.Wrap(err)
	}
	c.logger.DebugContext(ctx, "Azure database URL validated", "database", database.GetName())
	return nil
}

func (c *urlChecker) checkIsAzureEndpoint(ctx context.Context, database types.Database) error {
	switch database.GetProtocol() {
	case defaults.ProtocolRedis:
		return trace.Wrap(requireDatabaseIsEndpoint(ctx, database, azure.IsCacheForRedisEndpoint))

	case defaults.ProtocolMySQL, defaults.ProtocolPostgres:
		return trace.Wrap(requireDatabaseIsEndpoint(ctx, database, azure.IsDatabaseEndpoint))

	case defaults.ProtocolSQLServer:
		return trace.Wrap(requireDatabaseIsEndpoint(ctx, database, azure.IsMSSQLServerEndpoint))
	}
	c.logger.DebugContext(ctx, "URL checker does not support Azure database protocol",
		"database_type", database.GetType(),
		"database_protocol", database.GetProtocol(),
	)
	return nil
}
