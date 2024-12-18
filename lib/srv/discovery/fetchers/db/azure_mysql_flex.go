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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/mysql/armmysqlflexibleservers"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// newAzureMySQLFlexServerFetcher creates a fetcher for Azure MySQL Flexible server.
func newAzureMySQLFlexServerFetcher(config azureFetcherConfig) (common.Fetcher, error) {
	return newAzureFetcher[*armmysqlflexibleservers.Server, azure.MySQLFlexServersClient](config, &azureMySQLFlexServerFetcher{})
}

// azureMySQLFlexServerFetcher implements azureFetcherPlugin for Azure MySQL Flexible server.
type azureMySQLFlexServerFetcher struct{}

// GetListClient returns a server-listing client for Azure MySQL Flexible server.
func (f *azureMySQLFlexServerFetcher) GetListClient(cfg *azureFetcherConfig, subID string) (azure.MySQLFlexServersClient, error) {
	client, err := cfg.AzureClients.GetAzureMySQLFlexServersClient(subID)
	return client, trace.Wrap(err)
}

// GetServerLocation returns the location of an Azure MySQL Flexible server.
func (f *azureMySQLFlexServerFetcher) GetServerLocation(server *armmysqlflexibleservers.Server) string {
	return azure.StringVal(server.Location)
}

// NewDatabaseFromServer converts an Azure MySQL Flexible server to a Teleport database.
func (f *azureMySQLFlexServerFetcher) NewDatabaseFromServer(ctx context.Context, server *armmysqlflexibleservers.Server, logger *slog.Logger) types.Database {
	if !f.isAvailable(server, logger) {
		logger.DebugContext(ctx, "Skipping unavailable Azure MySQL Flexible server",
			"server", azure.StringVal(server.Name),
			"state", azure.StringVal(server.Properties.State),
		)
		return nil
	}

	database, err := common.NewDatabaseFromAzureMySQLFlexServer(server)
	if err != nil {
		logger.WarnContext(ctx, "Could not convert Azure MySQL server to database resource",
			"server", azure.StringVal(server.Name),
			"error", err,
		)
		return nil
	}
	return database
}

// isAvailable checks the status of the server and returns true if the server
// is available.
func (f *azureMySQLFlexServerFetcher) isAvailable(server *armmysqlflexibleservers.Server, logger *slog.Logger) bool {
	state := armmysqlflexibleservers.ServerState(azure.StringVal(server.Properties.State))
	switch state {
	case armmysqlflexibleservers.ServerStateReady, armmysqlflexibleservers.ServerStateUpdating:
		return true
	case armmysqlflexibleservers.ServerStateDisabled,
		armmysqlflexibleservers.ServerStateDropping,
		armmysqlflexibleservers.ServerStateStarting,
		armmysqlflexibleservers.ServerStateStopped,
		armmysqlflexibleservers.ServerStateStopping:
		// server state is known and it's not available.
		return false
	}
	logger.WarnContext(context.Background(), "Assuming Azure MySQL Flexible server with unknown status type is available",
		"status", state,
		"server", azure.StringVal(server.Name),
	)
	return true
}
