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

package db

import (
	"context"
	"log/slog"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// newAzureMySQLFetcher creates a fetcher for Azure MySQL.
func newAzureMySQLFetcher(config azureFetcherConfig) (common.Fetcher, error) {
	return newAzureFetcher[*azure.DBServer, azure.DBServersClient](config, &azureDBServerPlugin{})
}

// newAzureMySQLFetcher creates a fetcher for Azure PostgreSQL.
func newAzurePostgresFetcher(config azureFetcherConfig) (common.Fetcher, error) {
	return newAzureFetcher[*azure.DBServer, azure.DBServersClient](config, &azureDBServerPlugin{})
}

// azureDBServerPlugin implements azureFetcherPlugin for MySQL and PostgreSQL.
type azureDBServerPlugin struct{}

func (p *azureDBServerPlugin) GetListClient(cfg *azureFetcherConfig, subID string) (azure.DBServersClient, error) {
	switch cfg.Type {
	case types.AzureMatcherMySQL:
		client, err := cfg.AzureClients.GetAzureMySQLClient(subID)
		return client, trace.Wrap(err)
	case types.AzureMatcherPostgres:
		client, err := cfg.AzureClients.GetAzurePostgresClient(subID)
		return client, trace.Wrap(err)
	default:
		return nil, trace.BadParameter("unknown matcher type %q", cfg.Type)
	}
}

func (p *azureDBServerPlugin) GetServerLocation(server *azure.DBServer) string {
	return server.Location
}

func (p *azureDBServerPlugin) NewDatabaseFromServer(ctx context.Context, server *azure.DBServer, logger *slog.Logger) types.Database {
	if !server.IsSupported() {
		logger.DebugContext(ctx, "Skipping Azure server that does not support AAD authentication",
			"server", server.Name,
			"version", server.Properties.Version,
		)
		return nil
	}

	if !server.IsAvailable() {
		logger.DebugContext(ctx, "Skipping unavailable Azure server",
			"server", server.Name,
			"state", server.Properties.UserVisibleState)
		return nil
	}

	database, err := common.NewDatabaseFromAzureServer(server)
	if err != nil {
		logger.WarnContext(ctx, "Could not convert Azure server to database resource",
			"server", server.Name,
			"error", err,
		)
		return nil
	}
	return database
}
