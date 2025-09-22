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
	"fmt"
	"log/slog"
	"maps"
	"net"
	"text/template"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/client"
	dbmcp "github.com/gravitational/teleport/lib/client/db/mcp"
	pgmcp "github.com/gravitational/teleport/lib/client/db/postgres/mcp"
	"github.com/gravitational/teleport/lib/client/mcp"
	"github.com/gravitational/teleport/lib/client/mcp/claude"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// mcpDBStartCommand implements `tsh mcp db start` command.
type mcpDBStartCommand struct {
	*kingpin.CmdClause

	cf           *CLIConf
	databaseURIs []string
}

func newMCPDBCommand(parent *kingpin.CmdClause, cf *CLIConf) *mcpDBStartCommand {
	cmd := &mcpDBStartCommand{
		CmdClause: parent.Command("start", "Start a local MCP server for database access.").Hidden(),
		cf:        cf,
	}

	cmd.Arg("uris", "List of database MCP resource URIs that will be served by the server.").Required().StringsVar(&cmd.databaseURIs)
	return cmd
}

func (c *mcpDBStartCommand) run() error {
	logger, err := initLogger(c.cf, utils.LoggingForMCP, getLoggingOptsForMCPServer(c.cf))
	if err != nil {
		return trace.Wrap(err)
	}

	registry := defaultDBMCPRegistry
	if c.cf.databaseMCPRegistryOverride != nil {
		registry = c.cf.databaseMCPRegistryOverride
	}

	tc, err := makeClient(c.cf)
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

		if _, ok := configuredDatabases[uri.WithoutParams().String()]; ok {
			return trace.BadParameter("Database %q was configured twice. MCP servers only support serving a database service only once.", uri.GetDatabaseName())
		}

		configuredDatabases[uri.WithoutParams().String()] = struct{}{}
		uris[i] = uri
	}

	server := dbmcp.NewRootServer(logger)
	allDatabases, err := c.prepareDatabases(c.cf, tc, registry, uris, logger, server)
	if err != nil {
		return trace.Wrap(err)
	}

	for protocol, newServerFunc := range registry {
		databases := allDatabases[protocol]
		if len(databases) == 0 {
			continue
		}

		srv, err := newServerFunc(c.cf.Context, &dbmcp.NewServerConfig{
			Logger:     logger,
			RootServer: server,
			Databases:  databases,
		})
		if err != nil {
			return trace.Wrap(err)
		}
		defer srv.Close(c.cf.Context)
	}

	return trace.Wrap(server.ServeStdio(c.cf.Context, c.cf.Stdin(), c.cf.Stdout()))
}

// prepareDatabases based on the available MCP servers, initialize the database
// local proxy and generate the MCP database.
func (c *mcpDBStartCommand) prepareDatabases(
	cf *CLIConf,
	tc *client.TeleportClient,
	registry dbmcp.Registry,
	uris []*mcp.ResourceURI,
	logger *slog.Logger,
	server *dbmcp.RootServer,
) (map[string][]*dbmcp.Database, error) {
	var (
		ctx            = cf.Context
		dbsPerProtocol = make(map[string][]*dbmcp.Database)
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

		mcpDB := &dbmcp.Database{
			DB:           db,
			ClusterName:  uri.GetClusterName(),
			DatabaseUser: dbUser,
			DatabaseName: dbName,
			// Connections are always handled by the TeleportClient, so here we
			// just need to return a placeholder.
			LookupFunc: func(ctx context.Context, host string) (addrs []string, err error) {
				return []string{host}, nil
			},
			DialContextFunc: func(ctx context.Context, network, addr string) (net.Conn, error) {
				conn, err := tc.DialDatabase(ctx, proto.RouteToDatabase{
					ServiceName: db.GetName(),
					Protocol:    db.GetProtocol(),
					Username:    dbUser,
					Database:    dbName,
				})
				return conn, trace.Wrap(err)
			},
		}
		dbsPerProtocol[db.GetProtocol()] = append(dbsPerProtocol[db.GetProtocol()], mcpDB)
		server.RegisterDatabase(mcpDB)
	}

	return dbsPerProtocol, nil
}

// databasesGetter is the interface used to retrieve available
// databases using filters.
type databasesGetter interface {
	// ListDatabases returns all registered databases.
	ListDatabases(ctx context.Context, customFilter *proto.ListResourcesRequest) ([]types.Database, error)
}

// mcpDBConfigCommand implements `tsh mcp db config` command.
type mcpDBConfigCommand struct {
	*kingpin.CmdClause

	clientConfig mcpClientConfigFlags
	ctx          context.Context
	cf           *CLIConf
	siteName     string
	overwriteEnv bool

	// databasesGetter used to retrieve databases information. Can be mocked in
	// tests.
	databasesGetter databasesGetter
}

func newMCPDBconfigCommand(parent *kingpin.CmdClause, cf *CLIConf) *mcpDBConfigCommand {
	cmd := &mcpDBConfigCommand{
		CmdClause: parent.Command("config", "Print client configuration details."),
		ctx:       cf.Context,
		cf:        cf,
	}

	cmd.Flag("db-user", "Database user to log in as.").Short('u').StringVar(&cf.DatabaseUser)
	cmd.Flag("db-name", "Database name to log in to.").Short('n').StringVar(&cf.DatabaseName)
	cmd.Flag("overwrite", "Overwrites command and environment variable from the config file.").BoolVar(&cmd.overwriteEnv)
	cmd.Arg("name", "Database service name.").StringVar(&cf.DatabaseService)
	cmd.clientConfig.addToCmd(cmd.CmdClause)
	cmd.Alias(mcpDBConfigHelp)
	return cmd
}

// TODO(gabrielcorado): support generating config for multiple databases at once.
func (m *mcpDBConfigCommand) run() error {
	if m.databasesGetter == nil {
		tc, err := makeClient(m.cf)
		if err != nil {
			return trace.Wrap(err)
		}

		m.databasesGetter = tc
		m.siteName = tc.SiteName
	}

	databases, err := m.databasesGetter.ListDatabases(m.ctx, &proto.ListResourcesRequest{
		Namespace:           apidefaults.Namespace,
		ResourceType:        types.KindDatabaseServer,
		PredicateExpression: makeDiscoveredNameOrNamePredicate(m.cf.DatabaseService),
		// TODO(gabrielcorado): support requesting access.
		UseSearchAsRoles: false,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	db, err := chooseOneDatabase(m.cf, databases)
	if err != nil {
		return trace.Wrap(err)
	}

	// TODO(gabrielcorado): support having the flags empty and assume the values
	// based on the role and database.
	if m.cf.DatabaseUser == "" || m.cf.DatabaseName == "" {
		return trace.BadParameter("You must specify --db-user and --db-name flags used to connect to the database")
	}

	dbURI := mcp.NewDatabaseResourceURI(m.siteName, db.GetName(), mcp.WithDatabaseUser(m.cf.DatabaseUser), mcp.WithDatabaseName(m.cf.DatabaseName))
	switch {
	case m.clientConfig.isSet():
		return trace.Wrap(m.updateClientConfig(dbURI))
	default:
		return trace.Wrap(m.printJSONWithHint(dbURI))
	}
}

func (m *mcpDBConfigCommand) printJSONWithHint(dbURI mcp.ResourceURI) error {
	config := claude.NewConfig()
	// Since the database is being added to a "fresh" config file the database
	// will always be new and we can ignore the additional message as well.
	if _, _, err := m.addDatabaseToConfig(config, dbURI); err != nil {
		return trace.Wrap(err)
	}

	w := m.cf.Stdout()
	if _, err := fmt.Fprintln(w, "Here is a sample JSON configuration for launching Teleport MCP servers:"); err != nil {
		return trace.Wrap(err)
	}
	if err := config.Write(w, claude.FormatJSONOption(m.clientConfig.jsonFormat)); err != nil {
		return trace.Wrap(err)
	}
	if _, err := fmt.Fprintf(w, `
If you already have an entry for %q server, add the following database resource URI to the command arguments list:
%s

`, mcpDBConfigName, dbURI.String()); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(m.clientConfig.printHint(w))
}

// TODO(gabrielcorado): support updating multiple databases at once.
func (m *mcpDBConfigCommand) updateClientConfig(dbURI mcp.ResourceURI) error {
	config, err := m.clientConfig.loadConfig()
	if err != nil {
		return trace.Wrap(err)
	}
	preexistentDB, commandChanged, err := m.addDatabaseToConfig(config, dbURI)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := config.Save(claude.FormatJSONOption(m.clientConfig.jsonFormat)); err != nil {
		return trace.Wrap(err)
	}

	templateData := struct {
		Name          string
		ConfigPath    string
		ConfigName    string
		PreexistentDB bool
		EnvChanged    bool
		OverwriteEnv  bool
	}{
		Name:          dbURI.GetDatabaseServiceName(),
		ConfigPath:    config.Path(),
		ConfigName:    mcpDBConfigName,
		PreexistentDB: preexistentDB,
		EnvChanged:    commandChanged,
		OverwriteEnv:  m.overwriteEnv,
	}

	return trace.Wrap(mcpDBConfigMessageTemplate.Execute(m.cf.Stdout(), templateData))
}

// addDatabaseToConfig adds the provided database, merging with existent
// databases configured. This function returns a additional message to be
// displayed to users.
func (m *mcpDBConfigCommand) addDatabaseToConfig(config claudeConfig, dbURI mcp.ResourceURI) (bool, bool, error) {
	var (
		dbs        []string
		updated    bool
		envChanged bool
		server     = makeLocalMCPServer(m.cf, nil /* args */)
	)
	if existentServer, ok := config.GetMCPServers()[mcpDBConfigName]; ok {
		// For most common cases we want to keep the environment variables
		// unchanged. However, in case users want a "fresh start" they can
		// provide a flag so we overwrite them with default values.
		if !maps.Equal(server.Envs, existentServer.Envs) {
			envChanged = true
			if !m.overwriteEnv {
				server.Envs = existentServer.Envs
			}
		}

		for _, arg := range existentServer.Args {
			// We're only interested in resources, any flags or other command
			// parts will be discarded.
			uri, err := mcp.ParseResourceURI(arg)
			if err != nil {
				continue
			}

			if !uri.IsDatabase() {
				return false, false, trace.BadParameter("resource %q on config is not a database", uri.String())
			}

			if uri.WithoutParams().Equal(dbURI.WithoutParams()) {
				dbs = append(dbs, dbURI.String())
				updated = true
			} else {
				dbs = append(dbs, uri.String())
			}
		}
	}

	if !updated {
		dbs = append(dbs, dbURI.String())
	}

	server.Args = append([]string{"mcp", "db", "start"}, dbs...)
	return updated, envChanged, trace.Wrap(config.PutMCPServer(mcpDBConfigName, server))
}

var (
	// defaultDBMCPRegistry is the default database access MCP servers registry.
	defaultDBMCPRegistry = map[string]dbmcp.NewServerFunc{
		defaults.ProtocolPostgres:    pgmcp.NewServer,
		defaults.ProtocolCockroachDB: pgmcp.NewServerForCockroachDB,
	}
)

// mcpDBConfigName is the configuration name that is managed by the config
// command.
const mcpDBConfigName = "teleport-databases"

// mcpDBConfigMessageTemplate is the MCP db config message template.
var mcpDBConfigMessageTemplate = template.Must(template.New("").Funcs(template.FuncMap{
	"quote": func(s string) string { return fmt.Sprintf("%q", s) },
}).Parse(`{{ if .PreexistentDB -}}Updated{{ else }}Added{{ end }} database {{ .Name | quote }} on the client configuration at:
{{ .ConfigPath }}

Teleport database access MCP server is named {{ .ConfigName | quote }} in this configuration.

You may need to restart your client to reload these new configurations.

{{- if (and (.EnvChanged) (not .OverwriteEnv)) }}

Environment variables have changed, but existing values will be preserved.
To overwrite them, rerun this command with the --overwrite flag.
{{- end }}
`))
