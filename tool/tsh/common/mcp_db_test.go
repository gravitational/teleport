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
	"bytes"
	"context"
	"io"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	mcpclient "github.com/mark3labs/mcp-go/client"
	mcptransport "github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	dbmcp "github.com/gravitational/teleport/lib/client/db/mcp"
	clientmcp "github.com/gravitational/teleport/lib/client/mcp"
	mcpconfig "github.com/gravitational/teleport/lib/client/mcp/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	testserver "github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestMCPDBCommand(t *testing.T) {
	tmpHomePath := t.TempDir()
	connector := mockConnector(t)
	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetDatabaseUsers([]string{"postgres", "root"})
	alice.SetDatabaseNames([]string{"postgres", "defaultdb"})
	alice.SetRoles([]string{"access"})

	authProcess, err := testserver.NewTeleportProcess(
		t.TempDir(),
		testserver.WithClusterName("root"),
		testserver.WithBootstrap(connector, alice),
		testserver.WithConfig(func(cfg *servicecfg.Config) {
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			cfg.Databases.Enabled = true
			cfg.Databases.Databases = []servicecfg.Database{
				{
					Name:     "postgres1",
					Protocol: defaults.ProtocolPostgres,
					URI:      "external-pg:5432",
				},
				{
					Name:     "postgres2",
					Protocol: defaults.ProtocolPostgres,
					URI:      "external-pg:5432",
				},
				{
					Name:     "cockroach",
					Protocol: defaults.ProtocolCockroachDB,
					URI:      "external-cockroach:5432",
				},
				{
					Name:     "mysql-local",
					Protocol: defaults.ProtocolMySQL,
					URI:      "external-mysql:3306",
				},
			}
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, authProcess.Close())
		require.NoError(t, authProcess.Wait())
	})

	authServer := authProcess.GetAuthServer()
	require.NotNil(t, authServer)

	proxyAddr, err := authProcess.ProxyWebAddr()
	require.NoError(t, err)

	err = Run(t.Context(), []string{
		"login", "--insecure", "--debug", "--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, alice, connector.GetName()))
	require.NoError(t, err)

	stdin, writer := io.Pipe()
	reader, stdout := io.Pipe()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	executionCh := make(chan error)
	go func() {
		executionCh <- Run(ctx, []string{
			"mcp",
			"db",
			"start",
			"teleport://clusters/root/databases/postgres1?dbUser=postgres&dbName=postgres",
			"teleport://clusters/root/databases/postgres2?dbUser=postgres&dbName=postgres",
			"teleport://clusters/root/databases/cockroach?dbUser=root&dbName=defaultdb",
		}, setHomePath(tmpHomePath), func(c *CLIConf) error {
			c.overrideStdin = stdin
			c.OverrideStdout = stdout
			// MCP server logs are going to be discarded.
			c.overrideStderr = io.Discard

			// Fake a query tool for each database.
			addDBQueryTool := func(protocol string, nsc *dbmcp.NewServerConfig) {
				nsc.RootServer.AddTool(
					mcp.NewTool(dbmcp.ToolName(protocol, "query")),
					func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
						return nil, trace.NotImplemented("not implemented")
					},
				)
			}
			c.databaseMCPRegistryOverride = map[string]dbmcp.NewServerFunc{
				defaults.ProtocolPostgres: func(ctx context.Context, nsc *dbmcp.NewServerConfig) (dbmcp.Server, error) {
					addDBQueryTool(defaults.ProtocolPostgres, nsc)
					return &testDatabaseMCP{}, nil
				},
				defaults.ProtocolCockroachDB: func(ctx context.Context, nsc *dbmcp.NewServerConfig) (dbmcp.Server, error) {
					addDBQueryTool(defaults.ProtocolCockroachDB, nsc)
					return &testDatabaseMCP{}, nil
				},
			}
			return nil
		})
	}()

	cltTransport := mcptransport.NewIO(reader, writer, nil /* logging */)
	require.NoError(t, cltTransport.Start(t.Context()))
	clt := mcpclient.NewClient(cltTransport)
	require.NoError(t, clt.Start(t.Context()))
	defer clt.Close()

	req := mcp.InitializeRequest{}
	req.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	req.Params.ClientInfo = mcp.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		_, err = clt.Initialize(ctx, req)
		require.NoError(t, err)
		require.NoError(t, clt.Ping(ctx))
	}, time.Second, 100*time.Millisecond)

	tools, err := clt.ListTools(t.Context(), mcp.ListToolsRequest{})
	require.NoError(t, err)
	var toolNames []string
	for _, tool := range tools.Tools {
		toolNames = append(toolNames, tool.Name)
	}
	require.ElementsMatch(t, []string{
		"teleport_list_databases",
		"teleport_postgres_query",
		"teleport_cockroachdb_query",
	}, toolNames)

	// Stop the MCP server command and wait until it is finished.
	cancel()
	select {
	case err := <-executionCh:
		require.Error(t, err)
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(10 * time.Second):
		require.Fail(t, "expected the execution to be completed")
	}
}

func TestMCPDBCommandFailures(t *testing.T) {
	tmpHomePath := t.TempDir()
	connector := mockConnector(t)
	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetDatabaseUsers([]string{"postgres"})
	alice.SetDatabaseNames([]string{"postgres"})
	alice.SetRoles([]string{"access"})
	clusterName := "root"

	authProcess, err := testserver.NewTeleportProcess(
		t.TempDir(),
		testserver.WithClusterName(clusterName),
		testserver.WithBootstrap(connector, alice),
		testserver.WithConfig(func(cfg *servicecfg.Config) {
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			cfg.Databases.Enabled = true
			cfg.Databases.Databases = []servicecfg.Database{
				{
					Name:     "postgres1",
					Protocol: defaults.ProtocolPostgres,
					URI:      "external-pg:5432",
				},
				{
					Name:     "postgres2",
					Protocol: defaults.ProtocolPostgres,
					URI:      "external-pg:5432",
				},
				{
					Name:     "mysql-local",
					Protocol: defaults.ProtocolMySQL,
					URI:      "external-mysql:3306",
				},
			}
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, authProcess.Close())
		require.NoError(t, authProcess.Wait())
	})

	authServer := authProcess.GetAuthServer()
	require.NotNil(t, authServer)

	proxyAddr, err := authProcess.ProxyWebAddr()
	require.NoError(t, err)

	withMockedMCPServers := func(c *CLIConf) error {
		c.databaseMCPRegistryOverride = map[string]dbmcp.NewServerFunc{
			defaults.ProtocolPostgres: func(ctx context.Context, nsc *dbmcp.NewServerConfig) (dbmcp.Server, error) {
				return &testDatabaseMCP{}, nil
			},
		}
		return nil
	}

	err = Run(t.Context(), []string{
		"login", "--insecure", "--debug", "--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, alice, connector.GetName()))
	require.NoError(t, err)

	t.Run("different clusters", func(t *testing.T) {
		err := Run(t.Context(), []string{
			"mcp",
			"db",
			"start",
			"teleport://clusters/root/databases/postgres1?dbUser=postgres&dbName=postgres",
			"teleport://clusters/other/databases/postgres2?dbUser=postgres&dbName=postgres",
		}, setHomePath(tmpHomePath), withMockedMCPServers)
		require.Error(t, err)
	})

	t.Run("duplicated databases", func(t *testing.T) {
		err := Run(t.Context(), []string{
			"mcp",
			"db",
			"start",
			"teleport://clusters/root/databases/postgres1?dbUser=postgres&dbName=postgres",
			"teleport://clusters/root/databases/postgres1?dbUser=readonly&dbName=postgres",
		}, setHomePath(tmpHomePath), withMockedMCPServers)
		require.Error(t, err)
	})
}

func TestMCPDBConfigCommand(t *testing.T) {
	clusterName := "root"
	db0, err := types.NewDatabaseV3(types.Metadata{
		Name: "pg",
	}, types.DatabaseSpecV3{
		Protocol: "protocol",
		URI:      "uri",
	})
	require.NoError(t, err)
	db1, err := types.NewDatabaseV3(types.Metadata{
		Name: "another",
	}, types.DatabaseSpecV3{
		Protocol: "protocol",
		URI:      "uri",
	})
	require.NoError(t, err)

	dbURI0 := clientmcp.NewDatabaseResourceURI(clusterName, db0.GetName(), clientmcp.WithDatabaseUser("readonly"), clientmcp.WithDatabaseName("dbname"))
	dbURI0Updated := clientmcp.NewDatabaseResourceURI(clusterName, db0.GetName(), clientmcp.WithDatabaseUser("rw"), clientmcp.WithDatabaseName("anotherdb"))
	dbURI1 := clientmcp.NewDatabaseResourceURI(clusterName, db1.GetName(), clientmcp.WithDatabaseUser("rw"), clientmcp.WithDatabaseName("dbname"))

	for name, tc := range map[string]struct {
		cf                *CLIConf
		overwriteEnv      bool
		databasesGetter   databasesGetter
		assertError       require.ErrorAssertionFunc
		initialDatabases  []string
		expectedDatabases []string
		initialEnv        map[string]string
		expectedEnv       map[string]string
	}{
		"add database to empty config": {
			cf: &CLIConf{
				DatabaseService: dbURI0.GetDatabaseServiceName(),
				DatabaseUser:    dbURI0.GetDatabaseUser(),
				DatabaseName:    dbURI0.GetDatabaseName(),
			},
			databasesGetter:   &mockDatabasesGetter{dbs: []types.Database{db0, db1}},
			assertError:       require.NoError,
			expectedDatabases: []string{dbURI0.String()},
			expectedEnv:       map[string]string{},
		},
		"append database to config": {
			cf: &CLIConf{
				DatabaseService: dbURI1.GetDatabaseServiceName(),
				DatabaseUser:    dbURI1.GetDatabaseUser(),
				DatabaseName:    dbURI1.GetDatabaseName(),
			},
			databasesGetter:   &mockDatabasesGetter{dbs: []types.Database{db0, db1}},
			assertError:       require.NoError,
			initialDatabases:  []string{dbURI0.String()},
			expectedDatabases: []string{dbURI0.String(), dbURI1.String()},
			expectedEnv:       map[string]string{},
		},
		"update existent database": {
			cf: &CLIConf{
				DatabaseService: dbURI0Updated.GetDatabaseServiceName(),
				DatabaseUser:    dbURI0Updated.GetDatabaseUser(),
				DatabaseName:    dbURI0Updated.GetDatabaseName(),
			},
			databasesGetter:   &mockDatabasesGetter{dbs: []types.Database{db0, db1}},
			assertError:       require.NoError,
			initialDatabases:  []string{dbURI0.String(), dbURI1.String()},
			expectedDatabases: []string{dbURI0Updated.String(), dbURI1.String()},
			expectedEnv:       map[string]string{},
		},
		"database not found": {
			cf: &CLIConf{
				DatabaseService: dbURI0.GetDatabaseServiceName(),
				DatabaseUser:    dbURI0.GetDatabaseUser(),
				DatabaseName:    dbURI0.GetDatabaseName(),
			},
			databasesGetter: &mockDatabasesGetter{err: trace.NotFound("database not found")},
			assertError:     require.Error,
		},
		"missing connection params": {
			cf: &CLIConf{
				DatabaseService: dbURI0Updated.GetDatabaseServiceName(),
			},
			databasesGetter: &mockDatabasesGetter{dbs: []types.Database{db0}},
			assertError:     require.Error,
		},
		"keep current environment setting": {
			cf: &CLIConf{
				DatabaseService: dbURI0.GetDatabaseServiceName(),
				DatabaseUser:    dbURI0.GetDatabaseUser(),
				DatabaseName:    dbURI0.GetDatabaseName(),
				DebugSetByUser:  true,
				Debug:           true,
			},
			databasesGetter:   &mockDatabasesGetter{dbs: []types.Database{db0, db1}},
			assertError:       require.NoError,
			initialDatabases:  []string{dbURI0.String()},
			expectedDatabases: []string{dbURI0.String()},
			initialEnv:        map[string]string{"test": "hello"},
			expectedEnv:       map[string]string{"test": "hello"},
		},
		"reset environment setting": {
			cf: &CLIConf{
				DatabaseService: dbURI0.GetDatabaseServiceName(),
				DatabaseUser:    dbURI0.GetDatabaseUser(),
				DatabaseName:    dbURI0.GetDatabaseName(),
			},
			overwriteEnv:      true,
			databasesGetter:   &mockDatabasesGetter{dbs: []types.Database{db0, db1}},
			assertError:       require.NoError,
			initialDatabases:  []string{dbURI0.String()},
			expectedDatabases: []string{dbURI0.String()},
			initialEnv:        map[string]string{"test": "hello"},
			expectedEnv:       map[string]string{},
		},
	} {
		t.Run(name, func(t *testing.T) {
			configPath := setupMockDBMCPConfig(t, tc.cf, tc.initialDatabases, tc.initialEnv)
			var buf bytes.Buffer
			tc.cf.Context = context.Background()
			tc.cf.Proxy = "proxy:3080"
			tc.cf.HomePath = t.TempDir()
			tc.cf.OverrideStdout = &buf
			mustCreateEmptyProfile(t, tc.cf)

			cmd := &mcpDBConfigCommand{
				clientConfig: mcpClientConfigFlags{
					clientConfig: configPath,
					jsonFormat:   string(mcpconfig.FormatJSONPretty),
				},
				cf:              tc.cf,
				ctx:             t.Context(),
				siteName:        clusterName,
				databasesGetter: tc.databasesGetter,
				overwriteEnv:    tc.overwriteEnv,
			}

			err := cmd.run()
			tc.assertError(t, err)
			if err != nil {
				return
			}

			jsonConfig, err := mcpconfig.LoadConfigFromFile(configPath, mcpconfig.ConfigFormatClaude)
			require.NoError(t, err)
			mcpCmd, ok := jsonConfig.GetMCPServers()[mcpDBConfigName]
			require.True(t, ok, "expected configuration to include database access server definition, but got nothing")
			require.Empty(t, cmp.Diff(mcpCmd.Args, tc.expectedDatabases, cmpopts.EquateEmpty(), cmpopts.IgnoreSliceElements(func(arg string) bool {
				// Only assert database resources on the args.
				_, err := clientmcp.ParseResourceURI(arg)
				return err != nil
			})))
			require.Empty(t, cmp.Diff(mcpCmd.Envs, tc.expectedEnv, cmpopts.EquateEmpty(), cmpopts.IgnoreMapEntries(func(key string, _ string) bool {
				// Ignore default fields, only look for additional ones.
				switch key {
				case types.HomeEnvVar, debugEnvVar, osLogEnvVar:
					return true
				default:
					return false
				}
			})))
		})
	}
}

func setupMockDBMCPConfig(t *testing.T, cf *CLIConf, databasesURIs []string, additionalEnv map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	config, err := mcpconfig.LoadConfigFromFile(configPath, mcpconfig.ConfigFormatClaude)
	require.NoError(t, err)
	require.NoError(t, config.PutMCPServer("local-everything", mcpconfig.MCPServer{
		Command: "npx",
		Args:    []string{"-y", "@modelcontextprotocol/server-everything"},
	}))
	if len(databasesURIs) > 0 {
		srv := makeLocalMCPServer(cf, append([]string{"mcp", "db", "start"}, databasesURIs...))
		for name, value := range additionalEnv {
			srv.AddEnv(name, value)
		}
		require.NoError(t, config.PutMCPServer(mcpDBConfigName, srv))
	}
	require.NoError(t, config.Save(mcpconfig.FormatJSONPretty))
	return config.Path()
}

// testDatabaseMCP is a noop database MCP server.
type testDatabaseMCP struct{}

func (s *testDatabaseMCP) Close(_ context.Context) error { return nil }

// mockDatabaseGetter is a fetch databases mock.
type mockDatabasesGetter struct {
	dbs []types.Database
	err error
}

func (m *mockDatabasesGetter) ListDatabases(_ context.Context, _ *proto.ListResourcesRequest) ([]types.Database, error) {
	return m.dbs, m.err
}
