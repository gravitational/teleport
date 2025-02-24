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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/postgresql/armpostgresqlflexibleservers"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// newAzurePostgresFlexServerFetcher creates a fetcher for Azure PostgreSQL Flexible server.
func newAzurePostgresFlexServerFetcher(config azureFetcherConfig) (common.Fetcher, error) {
	return newAzureFetcher[*armpostgresqlflexibleservers.Server, azure.PostgresFlexServersClient](config, &azurePostgresFlexServerFetcher{})
}

// newAzurePostgresFlexServerFetcher implements azureFetcherPlugin for Azure PostgreSQL Flexible server.
type azurePostgresFlexServerFetcher struct{}

// GetListClient returns a server-listing client for Azure PostgreSQL Flexible server.
func (f *azurePostgresFlexServerFetcher) GetListClient(cfg *azureFetcherConfig, subID string) (azure.PostgresFlexServersClient, error) {
	client, err := cfg.AzureClients.GetAzurePostgresFlexServersClient(subID)
	return client, trace.Wrap(err)
}

// GetServerLocation returns the location of an Azure PostgreSQL Flexible server.
func (f *azurePostgresFlexServerFetcher) GetServerLocation(server *armpostgresqlflexibleservers.Server) string {
	return azure.StringVal(server.Location)
}

// NewDatabaseFromServer converts an Azure PostgreSQL server to a Teleport database.
func (f *azurePostgresFlexServerFetcher) NewDatabaseFromServer(ctx context.Context, server *armpostgresqlflexibleservers.Server, logger *slog.Logger) types.Database {
	if !f.isAvailable(server, logger) {
		logger.DebugContext(ctx, "Skipping unavailable Azure PostgreSQL Flexible server",
			azure.StringVal(server.Name),
			azure.StringVal(server.Properties.State))
		return nil
	}

	database, err := common.NewDatabaseFromAzurePostgresFlexServer(server)
	if err != nil {
		logger.WarnContext(ctx, "Could not convert Azure PostgreSQL server to database resource",
			"server", azure.StringVal(server.Name),
			"error", err,
		)
		return nil
	}
	return database
}

// isAvailable checks the status of the server and returns true if the server
// is available.
func (f *azurePostgresFlexServerFetcher) isAvailable(server *armpostgresqlflexibleservers.Server, logger *slog.Logger) bool {
	state := armpostgresqlflexibleservers.ServerState(azure.StringVal(server.Properties.State))
	switch state {
	case armpostgresqlflexibleservers.ServerStateReady, armpostgresqlflexibleservers.ServerStateUpdating:
		return true
	case armpostgresqlflexibleservers.ServerStateDisabled,
		armpostgresqlflexibleservers.ServerStateDropping,
		armpostgresqlflexibleservers.ServerStateStarting,
		armpostgresqlflexibleservers.ServerStateStopped,
		armpostgresqlflexibleservers.ServerStateStopping:
		// server state is known and it's not available.
		return false
	}
	logger.WarnContext(context.Background(), "Assuming Azure PostgreSQL Flexible server with unknown status is available",
		"status", state,
		"server", azure.StringVal(server.Name),
	)
	return true
}
