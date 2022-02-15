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

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	dbprofile "github.com/gravitational/teleport/lib/client/db"
	libdefaults "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/gravitational/trace"
)

// Database describes database
type Database struct {
	// URI is the database URI
	URI uri.ResourceURI
	types.Database
}

// GetDatabase returns a database
func (c *Cluster) GetDatabase(ctx context.Context, dbURI string) (*Database, error) {
	dbs, err := c.GetDatabases(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	for _, db := range dbs {
		if db.URI.String() == dbURI {
			return &db, nil
		}
	}

	return nil, trace.NotFound("database is not found: %v", dbURI)
}

// GetDatabases returns databases
func (c *Cluster) GetDatabases(ctx context.Context) ([]Database, error) {
	proxyClient, err := c.clusterClient.ConnectToProxy(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer proxyClient.Close()

	dbservers, err := proxyClient.FindDatabaseServersByFilters(ctx, proto.ListResourcesRequest{
		Namespace: defaults.Namespace,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	dbs := []Database{}
	for _, srv := range dbservers {
		dbs = append(dbs, Database{
			URI:      c.URI.AppendDB(srv.GetHostID()),
			Database: srv.GetDatabase(),
		})
	}

	return dbs, nil
}

// ReissueDBCerts issues new certificates for specific DB access
func (c *Cluster) ReissueDBCerts(ctx context.Context, user string, db types.Database) error {
	// When generating certificate for MongoDB access, database username must
	// be encoded into it. This is required to be able to tell which database
	// user to authenticate the connection as.
	if db.GetProtocol() == libdefaults.ProtocolMongoDB && user == "" {
		return trace.BadParameter("please provide the database user name using --db-user flag")
	}

	err := c.clusterClient.ReissueUserCerts(ctx, client.CertCacheKeep, client.ReissueParams{
		RouteToCluster: c.clusterClient.SiteName,
		RouteToDatabase: proto.RouteToDatabase{
			ServiceName: db.GetName(),
			Protocol:    db.GetProtocol(),
			Username:    user,
		},
		AccessRequests: c.status.ActiveRequests.AccessRequests,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// Update the database-specific connection profile file.
	err = dbprofile.Add(c.clusterClient, tlsca.RouteToDatabase{
		ServiceName: db.GetName(),
		Protocol:    db.GetProtocol(),
		Username:    user,
	}, c.status)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
