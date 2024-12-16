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

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/sql/armsql"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/cloud/azure"
	"github.com/gravitational/teleport/lib/srv/discovery/common"
)

// newAzureSQLServer creates a fetcher for Azure SQL Servers.
func newAzureSQLServerFetcher(config azureFetcherConfig) (common.Fetcher, error) {
	return newAzureFetcher[*armsql.Server, azure.SQLServerClient](config, &azureSQLServerFetcher{})
}

// azureSQLServerFetcher implements azureFetcherPlugin for Azure SQL Servers.
type azureSQLServerFetcher struct{}

func (f *azureSQLServerFetcher) GetListClient(cfg *azureFetcherConfig, subID string) (azure.SQLServerClient, error) {
	client, err := cfg.AzureClients.GetAzureSQLServerClient(subID)
	return client, trace.Wrap(err)
}

func (f *azureSQLServerFetcher) GetServerLocation(server *armsql.Server) string {
	return azure.StringVal(server.Location)
}

func (f *azureSQLServerFetcher) NewDatabaseFromServer(ctx context.Context, server *armsql.Server, logger *slog.Logger) types.Database {
	database, err := common.NewDatabaseFromAzureSQLServer(server)
	if err != nil {
		logger.WarnContext(ctx, "Could not convert Azure SQL server to database resource",
			"server", azure.StringVal(server.Name),
			"error", err,
		)
		return nil
	}

	// The method used to list the SQL servers only return running servers so
	// there is no need to check the status here.
	return database
}
