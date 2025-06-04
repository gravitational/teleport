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
	"io"
	"testing"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	mcptransport "github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	dbmcp "github.com/gravitational/teleport/lib/client/db/mcp"
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

// testDatabaseMCP is a noop database MCP server.
type testDatabaseMCP struct{}

func (s *testDatabaseMCP) Close(_ context.Context) error { return nil }
