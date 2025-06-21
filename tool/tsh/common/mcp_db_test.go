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
	"github.com/gravitational/teleport/lib/client/mcp/claude"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	testserver "github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestMCPDBCommand(t *testing.T) {
	tmpHomePath := t.TempDir()
	connector := mockConnector(t)
	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetDatabaseUsers([]string{"postgres"})
	alice.SetDatabaseNames([]string{"postgres"})
	alice.SetRoles([]string{"access"})

	authProcess := testserver.MakeTestServer(
		t,
		testserver.WithClusterName(t, "root"),
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
		}, setHomePath(tmpHomePath), func(c *CLIConf) error {
			c.overrideStdin = stdin
			c.OverrideStdout = stdout
			// MCP server logs are going to be discarded.
			c.overrideStderr = io.Discard
			c.databaseMCPRegistryOverride = map[string]dbmcp.NewServerFunc{
				defaults.ProtocolPostgres: func(ctx context.Context, nsc *dbmcp.NewServerConfig) (dbmcp.Server, error) {
					return &testDatabaseMCP{}, nil
				},
			}
			return nil
		})
	}()

	clt := mcpclient.NewClient(mcptransport.NewIO(reader, writer, nil /* logging */))
	require.NoError(t, clt.Start(t.Context()))

	req := mcp.InitializeRequest{}
	req.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	req.Params.ClientInfo = mcp.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}

	require.EventuallyWithT(t, func(collect *assert.CollectT) {
		_, err = clt.Initialize(t.Context(), req)
		require.NoError(collect, err)
		require.NoError(collect, clt.Ping(t.Context()))
	}, time.Second, 100*time.Millisecond)

	// Stop the MCP server command and wait until it is finshed.
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

	authProcess := testserver.MakeTestServer(
		t,
		testserver.WithClusterName(t, clusterName),
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

	dbURI0 := clientmcp.NewDatabaseResourceURIWithConnectParams(clusterName, db0.GetName(), "readonly", "dbname")
	dbURI0Updated := clientmcp.NewDatabaseResourceURIWithConnectParams(clusterName, db0.GetName(), "rw", "anotherdb")
	dbURI1 := clientmcp.NewDatabaseResourceURIWithConnectParams(clusterName, db1.GetName(), "rw", "dbname")

	for name, tc := range map[string]struct {
		cf                *CLIConf
		databasesGetter   databasesGetter
		assertError       require.ErrorAssertionFunc
		initialDatabases  []string
		expectedDatabases []string
	}{
		"add database to empty config": {
			cf: &CLIConf{
				DatabaseService: dbURI0.GetDatabaseServiceName(),
				DatabaseUser:    dbURI0.GetDatabaseUser(),
				DatabaseName:    dbURI0.GetDatabaseName(),
			},
			databasesGetter:   &mockDatabasesGetter{dbs: []types.Database{db0, db1}},
			assertError:       require.NoError,
			expectedDatabases: []string{dbURI0.StringWithParams()},
		},
		"append database to config": {
			cf: &CLIConf{
				DatabaseService: dbURI1.GetDatabaseServiceName(),
				DatabaseUser:    dbURI1.GetDatabaseUser(),
				DatabaseName:    dbURI1.GetDatabaseName(),
			},
			databasesGetter:   &mockDatabasesGetter{dbs: []types.Database{db0, db1}},
			assertError:       require.NoError,
			initialDatabases:  []string{dbURI0.StringWithParams()},
			expectedDatabases: []string{dbURI0.StringWithParams(), dbURI1.StringWithParams()},
		},
		"update existent database": {
			cf: &CLIConf{
				DatabaseService: dbURI0Updated.GetDatabaseServiceName(),
				DatabaseUser:    dbURI0Updated.GetDatabaseUser(),
				DatabaseName:    dbURI0Updated.GetDatabaseName(),
			},
			databasesGetter:   &mockDatabasesGetter{dbs: []types.Database{db0, db1}},
			assertError:       require.NoError,
			initialDatabases:  []string{dbURI0.StringWithParams(), dbURI1.StringWithParams()},
			expectedDatabases: []string{dbURI0Updated.StringWithParams(), dbURI1.StringWithParams()},
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
	} {
		t.Run(name, func(t *testing.T) {
			configPath := setupMockDBMCPConfig(t, tc.initialDatabases)
			var buf bytes.Buffer
			tc.cf.Context = context.Background()
			tc.cf.Proxy = "proxy:3080"
			tc.cf.HomePath = t.TempDir()
			tc.cf.OverrideStdout = &buf
			mustCreateEmptyProfile(t, tc.cf)

			cmd := &mcpDBConfigCommand{
				clientConfig: mcpClientConfigFlags{
					clientConfig: configPath,
					jsonFormat:   string(claude.FormatJSONPretty),
				},
				cf:              tc.cf,
				ctx:             t.Context(),
				siteName:        clusterName,
				databasesGetter: tc.databasesGetter,
			}

			err := cmd.run()
			tc.assertError(t, err)
			if err != nil {
				return
			}

			jsonConfig, err := claude.LoadConfigFromFile(configPath)
			require.NoError(t, err)
			mcpCmd, ok := jsonConfig.GetMCPServers()[mcpDBConfigName]
			require.True(t, ok, "expected configuration to include database access server definition, but got nothing")
			for _, uri := range tc.expectedDatabases {
				require.Contains(t, mcpCmd.Args, uri)
			}
		})
	}
}

func setupMockDBMCPConfig(t *testing.T, databasesURIs []string) string {
	t.Helper()
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	config, err := claude.LoadConfigFromFile(configPath)
	require.NoError(t, err)
	require.NoError(t, config.PutMCPServer("local-everything", claude.MCPServer{
		Command: "npx",
		Args:    []string{"-y", "@modelcontextprotocol/server-everything"},
	}))
	if len(databasesURIs) > 0 {
		require.NoError(t, config.PutMCPServer(mcpDBConfigName, claude.MCPServer{
			Command: "tsh",
			Args:    append([]string{"mcp", "db", "start"}, databasesURIs...),
		}))
	}
	require.NoError(t, config.Save(claude.FormatJSONPretty))
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
