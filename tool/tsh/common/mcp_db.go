// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"context"
	"log/slog"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/client"
	dbmcp "github.com/gravitational/teleport/lib/client/db/mcp"
	pgmcp "github.com/gravitational/teleport/lib/client/db/postgres/mcp"
	"github.com/gravitational/teleport/lib/client/mcp"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/listener"
)

// mcpDBStartCommand implements `tsh mcp db start` command.
type mcpDBStartCommand struct {
	*kingpin.CmdClause

	databaseURIs []string
}

func newMCPDBCommand(parent *kingpin.CmdClause) *mcpDBStartCommand {
	cmd := &mcpDBStartCommand{
		CmdClause: parent.Command("start", "Start a local MCP server for database access").Hidden(),
	}

	cmd.Arg("uris", "List of database MCP resource URIs that will be served by the server").Required().StringsVar(&cmd.databaseURIs)
	return cmd
}

func (c *mcpDBStartCommand) run(cf *CLIConf) error {
	logger, err := initLogger(cf, utils.LoggingForMCP, parseLoggingOptsFromEnvAndArgv(cf))
	if err != nil {
		return trace.Wrap(err)
	}

	registry := defaultDBMCPRegistry
	if cf.databaseMCPRegistryOverride != nil {
		registry = cf.databaseMCPRegistryOverride
	}

	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	// Avoid any input request on the command execution. This is required,
	// otherwise the MCP clients will be stuck waiting for a response.
	tc.NonInteractive = false

	configuredDatabases := map[string]struct{}{}
	uris := make([]*mcp.ResourceURI, len(c.databaseURIs))
	for i, rawURI := range c.databaseURIs {
		uri, err := mcp.ParseResourceURI(rawURI)
		if err != nil {
			return trace.Wrap(err)
		}

		if !uri.IsDatabase() {
			return trace.BadParameter("%q resource must be a database", rawURI)
		}

		// TODO(gabrielcorado): support databases from different clusters.
		if uri.GetClusterName() != tc.SiteName {
			return trace.BadParameter("Databases must be from the same cluster (%q). %q is from a different cluster.", tc.SiteName, rawURI)
		}

		if _, ok := configuredDatabases[uri.String()]; ok {
			return trace.BadParameter("Database %q was configured twice. MCP servers only support serving a database service only once.", uri.String())
		}

		configuredDatabases[uri.String()] = struct{}{}
		uris[i] = uri
	}

	server := dbmcp.NewRootServer(logger)
	allDatabases, closeLocalProxies, err := c.prepareDatabases(cf, tc, registry, uris, logger, server)
	if err != nil {
		return trace.Wrap(err)
	}
	defer closeLocalProxies()

	for protocol, newServerFunc := range registry {
		databases := allDatabases[protocol]
		if len(databases) == 0 {
			continue
		}

		srv, err := newServerFunc(cf.Context, &dbmcp.NewServerConfig{
			Logger:     logger,
			RootServer: server,
			Databases:  databases,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		defer srv.Close(cf.Context)
	}

	return trace.Wrap(server.ServeStdio(cf.Context, cf.Stdin(), cf.Stdout()))
}

// closeLocalProxyFunc function used to close local proxy listeners.
type closeLocalProxyFunc func() error

// prepareDatabases based on the available MCP servers, initialize the database
// local proxy and generate the MCP database.
func (c *mcpDBStartCommand) prepareDatabases(
	cf *CLIConf,
	tc *client.TeleportClient,
	registry dbmcp.Registry,
	uris []*mcp.ResourceURI,
	logger *slog.Logger,
	server *dbmcp.RootServer,
) (map[string][]*dbmcp.Database, closeLocalProxyFunc, error) {
	var (
		ctx            = cf.Context
		dbsPerProtocol = make(map[string][]*dbmcp.Database)
		closeFuncs     []closeLocalProxyFunc
	)

	for _, uri := range uris {
		serviceName := uri.GetDatabaseServiceName()
		dbUser := uri.GetDatabaseUser()
		dbName := uri.GetDatabaseName()

		route := tlsca.RouteToDatabase{
			ServiceName: serviceName,
			Username:    dbUser,
			Database:    dbName,
		}

		info, err := getDatabaseInfo(cf, tc, []tlsca.RouteToDatabase{route})
		if err != nil {
			logger.InfoContext(ctx, "failed to retrieve database information", "database", serviceName, "error", err)
			continue
		}

		db, err := info.GetDatabase(ctx, tc)
		if err != nil {
			logger.InfoContext(ctx, "failed to load database information", "database", serviceName, "error", err)
			continue
		}

		if !registry.IsSupported(db.GetProtocol()) {
			logger.InfoContext(ctx, "database protocol unsupported, skipping it", "database", serviceName, "protocol", db.GetProtocol())
			continue
		}

		route.Protocol = db.GetProtocol()
		cc := client.NewDBCertChecker(tc, route, nil, client.WithTTL(tc.KeyTTL))
		// This avoids having the middleware to refresh the certificate if there
		// is a certificate available on disk.
		cert, err := loadDBCertificate(tc, route.ServiceName)
		if err == nil {
			cc.SetCert(cert)
		}

		listener := listener.NewInMemoryListener()
		lp, err := alpnproxy.NewLocalProxy(
			makeBasicLocalProxyConfig(ctx, tc, listener, tc.InsecureSkipVerify),
			alpnproxy.WithDatabaseProtocol(route.Protocol),
			alpnproxy.WithMiddleware(cc),
			alpnproxy.WithClusterCAsIfConnUpgrade(ctx, tc.RootClusterCACertPool),
		)
		if err != nil {
			_ = listener.Close()
			logger.ErrorContext(ctx, "failed to start local proxy for database, skipping it", "database", db.GetName(), "error", err)
			continue
		}
		go func() {
			defer lp.Close()
			if err = lp.Start(ctx); err != nil {
				logger.WarnContext(ctx, "failed to start local ALPN proxy", "error", err)
			}
		}()

		mcpDB := &dbmcp.Database{
			DB:                     db,
			ClusterName:            uri.GetClusterName(),
			DatabaseUser:           dbUser,
			DatabaseName:           dbName,
			Addr:                   listener.Addr().String(),
			ExternalErrorRetriever: cc,
			// Since we're using in-memory listener we don't need to resolve the
			// address.
			LookupFunc: func(ctx context.Context, host string) (addrs []string, err error) {
				return []string{listener.Addr().String()}, nil
			},
			DialContextFunc: listener.DialContext,
		}
		dbsPerProtocol[db.GetProtocol()] = append(dbsPerProtocol[db.GetProtocol()], mcpDB)
		server.RegisterDatabase(mcpDB)
		closeFuncs = append(closeFuncs, listener.Close)
	}

	return dbsPerProtocol, func() error {
		var errs []error
		for _, closeFunc := range closeFuncs {
			errs = append(errs, closeFunc())
		}

		return trace.NewAggregate(errs...)
	}, nil
}

var (
	// defaultDBMCPRegistry is the default database access MCP servers registry.
	defaultDBMCPRegistry = map[string]dbmcp.NewServerFunc{
		defaults.ProtocolPostgres: pgmcp.NewServer,
	}
)
