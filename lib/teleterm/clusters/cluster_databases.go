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
	"crypto/tls"
	"fmt"

	"github.com/gravitational/trace"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/db/dbcmd"
	"github.com/gravitational/teleport/lib/services"
	dbrole "github.com/gravitational/teleport/lib/srv/db/common/role"
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
func (c *Cluster) GetDatabase(ctx context.Context, authClient authclient.ClientI, dbURI uri.ResourceURI) (*Database, error) {
	var database types.Database
	dbName := dbURI.GetDbName()
	err := AddMetadataToRetryableError(ctx, func() error {
		databases, err := apiclient.GetAllResources[types.DatabaseServer](ctx, authClient, &proto.ListResourcesRequest{
			Namespace:           defaults.Namespace,
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

// reissueDBCerts issues new certificates for specific DB access and saves them to disk.
func (c *Cluster) reissueDBCerts(ctx context.Context, clusterClient *client.ClusterClient, routeToDatabase tlsca.RouteToDatabase) (tls.Certificate, error) {
	if dbrole.RequireDatabaseUserMatcher(routeToDatabase.Protocol) && routeToDatabase.Username == "" {
		return tls.Certificate{}, trace.BadParameter("the username must be present")
	}

	// Refresh the certs to account for clusterClient.SiteName pointing at a leaf cluster.
	err := clusterClient.ReissueUserCerts(ctx, client.CertCacheKeep, client.ReissueParams{
		RouteToCluster: c.clusterClient.SiteName,
		AccessRequests: c.status.ActiveRequests,
	})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	key, _, err := clusterClient.IssueUserCertsWithMFA(ctx, client.ReissueParams{
		RouteToCluster: c.clusterClient.SiteName,
		RouteToDatabase: proto.RouteToDatabase{
			ServiceName: routeToDatabase.ServiceName,
			Protocol:    routeToDatabase.Protocol,
			Username:    routeToDatabase.Username,
		},
		AccessRequests: c.status.ActiveRequests,
		RequesterName:  proto.UserCertsRequest_TSH_DB_LOCAL_PROXY_TUNNEL,
		TTL:            c.clusterClient.KeyTTL,
	})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	dbCert, err := key.DBTLSCert(routeToDatabase.ServiceName)
	return dbCert, trace.Wrap(err)
}

// GetAllowedDatabaseUsers returns allowed users for the given database based on the role set.
func (c *Cluster) GetAllowedDatabaseUsers(ctx context.Context, authClient authclient.ClientI, dbURI string) ([]string, error) {
	dbResourceURI, err := uri.ParseDBURI(dbURI)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessChecker, err := services.NewAccessCheckerForRemoteCluster(ctx, c.status.AccessInfo(), c.clusterClient.SiteName, authClient)
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
