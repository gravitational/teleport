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

package common

import (
	"net"
	"sort"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"github.com/mark3labs/mcp-go/server"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	clientproto "github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	pgmcp "github.com/gravitational/teleport/lib/client/db/mcp/postgres"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/tlsca"
	listenerutils "github.com/gravitational/teleport/lib/utils/listener"
)

func onMCPStartDB(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	profile, err := tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}
	routes, err := profile.DatabasesForCluster(tc.SiteName)
	if err != nil {
		return trace.Wrap(err)
	}

	dbs, err := listAvailableMCPDatatabases(cf, tc, profile, routes)
	if err != nil {
		return trace.Wrap(err)
	}

	var mcpInfo []pgmcp.DBInfo
	for _, db := range dbs {
		// skip non-postgres databases
		if db.info.database.GetProtocol() != defaults.ProtocolPostgres {
			continue
		}

		in, out := net.Pipe()
		listener := listenerutils.NewSingleUseListener(out)
		defer listener.Close()

		cc := client.NewDBCertChecker(tc, db.info.RouteToDatabase, nil, client.WithTTL(time.Duration(cf.MinsToLive)*time.Minute))

		lp, err := alpnproxy.NewLocalProxy(
			makeBasicLocalProxyConfig(cf.Context, tc, listener, tc.InsecureSkipVerify),
			alpnproxy.WithDatabaseProtocol(db.info.Protocol),
			// alpnproxy.WithClientCert(db.cert),
			alpnproxy.WithMiddleware(cc),
			alpnproxy.WithClusterCAsIfConnUpgrade(cf.Context, tc.RootClusterCACertPool),
		)
		if err != nil {
			return trace.Wrap(err)
		}
		go func() {
			defer lp.Close()
			if err = lp.Start(cf.Context); err != nil {
				logger.ErrorContext(cf.Context, "Failed to start local ALPN proxy", "error", err)
			}
		}()

		mcpInfo = append(mcpInfo, pgmcp.DBInfo{
			RawConn:     in,
			Route:       client.RouteToDatabaseToProto(db.info.RouteToDatabase),
			Description: db.info.database.GetDescription(),
		})
	}

	mcpServer := server.NewMCPServer("teleport_databases", teleport.Version)
	sess, err := pgmcp.NewSession(cf.Context, pgmcp.NewSessionConfig{
		MCPServer:   mcpServer,
		Datatabases: mcpInfo,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer sess.Close(cf.Context)

	return trace.Wrap(
		server.NewStdioServer(mcpServer).Listen(cf.Context, cf.Stdin(), cf.Stdout()),
	)
}

type mcpDb struct {
	info  *databaseInfo
	users *dbUsers
	in    net.Conn
	route clientproto.RouteToDatabase
}

func listAvailableMCPDatatabases(cf *CLIConf, tc *client.TeleportClient, profile *client.ProfileStatus, routes []tlsca.RouteToDatabase) ([]mcpDb, error) {
	var clusterClient *client.ClusterClient
	err := client.RetryWithRelogin(cf.Context, tc, func() error {
		var err error
		clusterClient, err = tc.ConnectToCluster(cf.Context)
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer clusterClient.Close()

	// list available databases
	servers, err := apiclient.GetAllResources[types.DatabaseServer](cf.Context, clusterClient.AuthClient, tc.ResourceFilter(types.KindDatabaseServer))
	if err != nil {
		return nil, trace.Wrap(err)
	}

	accessChecker, err := services.NewAccessCheckerForRemoteCluster(cf.Context, profile.AccessInfo(), tc.SiteName, clusterClient.AuthClient)
	if err != nil {
		logger.DebugContext(cf.Context, "Failed to fetch user roles", "error", err)
	}

	databases := types.DatabaseServers(servers).ToDatabases()
	sort.Sort(types.Databases(databases))

	var dbs []mcpDb
	for _, db := range databases {
		dbInfo := &databaseInfo{
			database: db,
			RouteToDatabase: tlsca.RouteToDatabase{
				ServiceName: db.GetName(),
				Protocol:    db.GetProtocol(),
			},
		}
		// check for an active route now that we have the full db name.
		if route, ok := findActiveDatabase(db.GetName(), routes); ok {
			dbInfo.RouteToDatabase = route
			dbInfo.isActive = true
		}

		users := getDBUsers(db, accessChecker)
		// TODO better handle this
		if len(users.Allowed) < 1 || users.Allowed[0] == "*" {
			return nil, trace.BadParameter("could not take a database username from %s", strings.Join(users.Allowed, ","))
		}
		// manually fill the user info
		dbInfo.RouteToDatabase.Username = users.Allowed[0]
		// TODO make it dynamic
		// TODO grab this from the list of available (allowed) databases
		dbInfo.RouteToDatabase.Database = "postgres"

		// TODO update check and set to deal with this
		// if err := dbInfo.checkAndSetDefaults(cf, tc); err != nil {
		// 	return nil, trace.Wrap(err)
		// }

		dbs = append(dbs, mcpDb{info: dbInfo})
	}

	return dbs, nil

}
