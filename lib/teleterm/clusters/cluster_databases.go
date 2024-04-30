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

package clusters

import (
	"context"
	"fmt"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	dbprofile "github.com/gravitational/teleport/lib/client/db"
	"github.com/gravitational/teleport/lib/client/db/dbcmd"
	libdefaults "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/tlsca"
)

// Database describes database
type Database struct {
	// URI is the database URI
	URI uri.ResourceURI
	types.Database
}

// GetDatabase returns a database
func (c *Cluster) GetDatabase(ctx context.Context, authClient auth.ClientI, dbURI uri.ResourceURI) (*Database, error) {
	var database types.Database
	dbName := dbURI.GetDbName()
	err := AddMetadataToRetryableError(ctx, func() error {
		databases, err := apiclient.GetAllResources[types.DatabaseServer](ctx, authClient, &proto.ListResourcesRequest{
			Namespace:           c.clusterClient.Namespace,
			ResourceType:        types.KindDatabaseServer,
			PredicateExpression: fmt.Sprintf(`name == "%s"`, dbName),
		})
		if err != nil {
			return trace.Wrap(err)
		}

		if len(databases) == 0 {
			return trace.NotFound("database %q not found", dbName)
		}

		database = databases[0].GetDatabase()
		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &Database{
		URI:      c.URI.AppendDB(database.GetName()),
		Database: database,
	}, err
}

func (c *Cluster) GetDatabases(ctx context.Context, authClient auth.ClientI, r *api.GetDatabasesRequest) (*GetDatabasesResponse, error) {
	var (
		page apiclient.ResourcePage[types.DatabaseServer]
		err  error
	)

	req := &proto.ListResourcesRequest{
		Namespace:           defaults.Namespace,
		ResourceType:        types.KindDatabaseServer,
		Limit:               r.Limit,
		SortBy:              types.GetSortByFromString(r.SortBy),
		StartKey:            r.StartKey,
		PredicateExpression: r.Query,
		SearchKeywords:      client.ParseSearchKeywords(r.Search, ' '),
		UseSearchAsRoles:    r.SearchAsRoles == "yes",
	}

	err = AddMetadataToRetryableError(ctx, func() error {
		page, err = apiclient.GetResourcePage[types.DatabaseServer](ctx, authClient, req)
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	response := &GetDatabasesResponse{
		StartKey:   page.NextKey,
		TotalCount: page.Total,
	}
	for _, database := range page.Resources {
		response.Databases = append(response.Databases, Database{
			URI:      c.URI.AppendDB(database.GetName()),
			Database: database.GetDatabase(),
		})
	}

	return response, nil
}

// reissueDBCerts issues new certificates for specific DB access and saves them to disk.
func (c *Cluster) reissueDBCerts(ctx context.Context, clusterClient *client.ClusterClient, routeToDatabase tlsca.RouteToDatabase) error {
	// When generating certificate for MongoDB access, database username must
	// be encoded into it. This is required to be able to tell which database
	// user to authenticate the connection as.
	if routeToDatabase.Protocol == libdefaults.ProtocolMongoDB && routeToDatabase.Username == "" {
		return trace.BadParameter("the username must be present for MongoDB connections")
	}

	// Refresh the certs to account for clusterClient.SiteName pointing at a leaf cluster.
	err := clusterClient.ReissueUserCerts(ctx, client.CertCacheKeep, client.ReissueParams{
		RouteToCluster: c.clusterClient.SiteName,
		AccessRequests: c.status.ActiveRequests.AccessRequests,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Fetch the certs for the database.
	err = clusterClient.ReissueUserCerts(ctx, client.CertCacheKeep, client.ReissueParams{
		RouteToCluster: c.clusterClient.SiteName,
		RouteToDatabase: proto.RouteToDatabase{
			ServiceName: routeToDatabase.ServiceName,
			Protocol:    routeToDatabase.Protocol,
			Username:    routeToDatabase.Username,
		},
		AccessRequests: c.status.ActiveRequests.AccessRequests,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Update the database-specific connection profile file.
	err = dbprofile.Add(ctx, c.clusterClient, routeToDatabase, c.status)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetAllowedDatabaseUsers returns allowed users for the given database based on the role set.
func (c *Cluster) GetAllowedDatabaseUsers(ctx context.Context, authClient auth.ClientI, dbURI string) ([]string, error) {
	dbResourceURI, err := uri.ParseDBURI(dbURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessChecker, err := services.NewAccessCheckerForRemoteCluster(ctx, c.status.AccessInfo(), c.status.Cluster, authClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	db, err := c.GetDatabase(ctx, authClient, dbResourceURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dbUsers, err := accessChecker.EnumerateDatabaseUsers(db)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return dbUsers.Allowed(), nil
}

type GetDatabasesResponse struct {
	Databases []Database
	// StartKey is the next key to use as a starting point.
	StartKey string
	// // TotalCount is the total number of resources available as a whole.
	TotalCount int
}

// NewDBCLICmdBuilder creates a dbcmd.CLICommandBuilder with provided cluster,
// db route, and options.
func NewDBCLICmdBuilder(cluster *Cluster, routeToDb tlsca.RouteToDatabase, options ...dbcmd.ConnectCommandFunc) *dbcmd.CLICommandBuilder {
	return dbcmd.NewCmdBuilder(
		cluster.clusterClient,
		&cluster.status,
		routeToDb,
		// TODO(ravicious): Pass the root cluster name here. cluster.Name returns leaf name for leaf
		// clusters.
		//
		// At this point it doesn't matter though because this argument is used only for
		// generating correct CA paths. We use dbcmd.WithNoTLS here which means that the CA paths aren't
		// included in the returned CLI command.
		cluster.Name,
		options...,
	)
}
