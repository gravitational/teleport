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

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	dbmcp "github.com/gravitational/teleport/lib/client/db/mcp"
	pgmcp "github.com/gravitational/teleport/lib/client/db/postgres/mcp"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils/listener"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

type mcpCommands struct {
	db *mcpDBCommand
}

func newMCPCommands(app *kingpin.Application) *mcpCommands {
	mcp := app.Command("mcp", "View and control proxied MCP servers.")
	return &mcpCommands{
		db: newMCPDBCommand(mcp),
	}
}

// mcpDBCommand implements `tsh mcp db` command.
type mcpDBCommand struct {
	*kingpin.CmdClause

	databaseUser        string
	databaseName        string
	labels              string
	predicateExpression string
}

func newMCPDBCommand(parent *kingpin.CmdClause) *mcpDBCommand {
	cmd := &mcpDBCommand{
		CmdClause: parent.Command("db", "Start a local MCP server for database access"),
	}

	cmd.Flag("db-user", "Database user to log in as.").Short('u').StringVar(&cmd.databaseUser)
	cmd.Flag("db-name", "Database name to log in to.").Short('n').StringVar(&cmd.databaseName)
	cmd.Flag("labels", labelHelp).StringVar(&cmd.labels)
	cmd.Flag("query", queryHelp).StringVar(&cmd.predicateExpression)
	return cmd
}

func (c *mcpDBCommand) run(cf *CLIConf) error {
	logger, _, err := logutils.Initialize(logutils.Config{
		Severity: slog.LevelInfo.String(),
		Format:   mcpLogFormat,
		Output:   mcpLogOutput,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	registry := defaultDBMCPRegistry
	if cf.databaseMCPRegistryOverride != nil {
		registry = cf.databaseMCPRegistryOverride
	}

	// Set the labels so it gets parsed when the client is created.
	cf.Labels = c.labels
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	// Avoid any input request on the command execution. This is required,
	// otherwise the MCP clients will be stuck waiting for a response.
	tc.NonInteractive = false

	sc, err := newSharedDatabaseExecClient(cf, tc)
	if err != nil {
		return trace.Wrap(err)
	}

	dbs, err := c.getDatabases(cf.Context, sc, tc.Labels)
	if err != nil {
		return trace.Wrap(err)
	}

	server := dbmcp.NewRootServer(logger)
	allDatabases, closeLocalProxies, err := c.prepareDatabases(cf.Context, registry, dbs, logger, tc, sc.profile, server)
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
func (c *mcpDBCommand) prepareDatabases(
	ctx context.Context,
	registry dbmcp.Registry,
	dbs []types.Database,
	logger *slog.Logger,
	tc *client.TeleportClient,
	profile *client.ProfileStatus,
	server *dbmcp.RootServer,
) (map[string][]*dbmcp.Database, closeLocalProxyFunc, error) {
	var (
		dbsPerProtocol = make(map[string][]*dbmcp.Database)
		closeFuncs     []closeLocalProxyFunc
	)

	for _, db := range dbs {
		if !registry.IsSupported(db.GetProtocol()) {
			logger.InfoContext(ctx, "database protocol unsupported, skipping it", "database", db.GetName(), "protocol", db.GetProtocol())
			continue
		}

		route := tlsca.RouteToDatabase{
			ServiceName: db.GetName(),
			Protocol:    db.GetProtocol(),
			Username:    c.databaseUser,
			Database:    c.databaseName,
		}

		listener := listener.InNewMemoryListener()
		cc := client.NewDBCertChecker(tc, route, nil, client.WithTTL(tc.KeyTTL))
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
			DB:           db,
			DatabaseUser: c.databaseUser,
			DatabaseName: c.databaseName,
			Addr:         listener.Addr().String(),
			// Since we're using in-memory listener we don't need to resolve the
			// address.
			LookupFunc: func(ctx context.Context, host string) (addrs []string, err error) {
				return []string{"memory"}, nil
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

func (c *mcpDBCommand) getDatabases(ctx context.Context, sc *sharedDatabaseExecClient, labels map[string]string) ([]types.Database, error) {
	dbsList, err := sc.listDatabasesWithFilter(ctx, &proto.ListResourcesRequest{
		ResourceType:        types.KindDatabaseServer,
		Namespace:           apidefaults.Namespace,
		Labels:              labels,
		PredicateExpression: c.predicateExpression,
	})

	return dbsList, trace.Wrap(err)
}

var (
	// defaultDBMCPRegistry is the default database access MCP servers registry.
	defaultDBMCPRegistry = map[string]dbmcp.NewServerFunc{
		defaults.ProtocolPostgres: pgmcp.NewServer,
	}
)

const (
	// mcpLogFormat defiens the log format of the MCP command.
	mcpLogFormat = "json"
	// mcpLogFormat defines to where the MCP command logs will be directed to.
	// The stdout is exclusively used as the MCP server transport, leaving only
	// stderr available.
	mcpLogOutput = "stderr"
)
