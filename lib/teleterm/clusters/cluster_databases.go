/*
Copyright 2021 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package clusters

import (
	"context"

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
func (c *Cluster) GetDatabase(ctx context.Context, dbURI uri.ResourceURI) (*Database, error) {
	// TODO(ravicious): Fetch a single db instead of filtering the response from GetDatabases.
	// https://github.com/gravitational/teleport/pull/14690#discussion_r927720600
	dbs, err := c.getAllDatabases(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, db := range dbs {
		if db.URI == dbURI {
			return &db, nil
		}
	}

	return nil, trace.NotFound("database is not found: %v", dbURI)
}

// GetDatabases returns databases
// TODO(ravicious): Remove this method in favor of fetching a single database in GetDatabase.
// https://github.com/gravitational/teleport/pull/14690#discussion_r927720600
func (c *Cluster) getAllDatabases(ctx context.Context) ([]Database, error) {
	var dbs []types.Database
	err := addMetadataToRetryableError(ctx, func() error {
		proxyClient, err := c.clusterClient.ConnectToProxy(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()

		dbs, err = proxyClient.FindDatabasesByFilters(ctx, proto.ListResourcesRequest{
			Namespace:    defaults.Namespace,
			ResourceType: types.KindDatabaseServer,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var responseDbs []Database
	for _, db := range dbs {
		responseDbs = append(responseDbs, Database{
			URI:      c.URI.AppendDB(db.GetName()),
			Database: db,
		})
	}

	return responseDbs, nil
}

func (c *Cluster) GetDatabases(ctx context.Context, r *api.GetDatabasesRequest) (*GetDatabasesResponse, error) {
	var (
		page        apiclient.ResourcePage[types.DatabaseServer]
		authClient  auth.ClientI
		proxyClient *client.ProxyClient
		err         error
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

	err = addMetadataToRetryableError(ctx, func() error {
		proxyClient, err = c.clusterClient.ConnectToProxy(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		defer proxyClient.Close()

		authClient, err = proxyClient.ConnectToCluster(ctx, c.clusterClient.SiteName)
		if err != nil {
			return trace.Wrap(err)
		}
		defer authClient.Close()

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
func (c *Cluster) reissueDBCerts(ctx context.Context, routeToDatabase tlsca.RouteToDatabase) error {
	// When generating certificate for MongoDB access, database username must
	// be encoded into it. This is required to be able to tell which database
	// user to authenticate the connection as.
	if routeToDatabase.Protocol == libdefaults.ProtocolMongoDB && routeToDatabase.Username == "" {
		return trace.BadParameter("the username must be present for MongoDB connections")
	}

	err := addMetadataToRetryableError(ctx, func() error {
		// Refresh the certs to account for clusterClient.SiteName pointing at a leaf cluster.
		err := c.clusterClient.ReissueUserCerts(ctx, client.CertCacheKeep, client.ReissueParams{
			RouteToCluster: c.clusterClient.SiteName,
			AccessRequests: c.status.ActiveRequests.AccessRequests,
		})
		if err != nil {
			return trace.Wrap(err)
		}

		// Fetch the certs for the database.
		err = c.clusterClient.ReissueUserCerts(ctx, client.CertCacheKeep, client.ReissueParams{
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

		return nil
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
func (c *Cluster) GetAllowedDatabaseUsers(ctx context.Context, dbURI string) ([]string, error) {
	var authClient auth.ClientI
	var proxyClient *client.ProxyClient

	dbResourceURI, err := uri.ParseDBURI(dbURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	err = addMetadataToRetryableError(ctx, func() error {
		proxyClient, err = c.clusterClient.ConnectToProxy(ctx)
		if err != nil {
			return trace.Wrap(err)
		}

		return nil
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer proxyClient.Close()

	authClient, err = proxyClient.ConnectToCluster(ctx, c.clusterClient.SiteName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer authClient.Close()

	accessChecker, err := services.NewAccessCheckerForRemoteCluster(ctx, c.status.AccessInfo(), c.status.Cluster, authClient)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	db, err := c.GetDatabase(ctx, dbResourceURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dbUsers := accessChecker.EnumerateDatabaseUsers(db)

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
