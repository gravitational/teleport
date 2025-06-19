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

package mcp

import (
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
)

func TestRegisterDatabase(t *testing.T) {
	server := NewRootServer(slog.New(slog.DiscardHandler))
	databases := []*Database{
		buildDatabase(t, "first"),
		buildDatabase(t, "second"),
		buildDatabase(t, "third"),
	}
	// sort databases by name to ensure the same order every test.
	slices.SortFunc(databases, func(a, b *Database) int {
		return strings.Compare(a.ResourceURI().String(), b.ResourceURI().String())
	})

	for _, db := range databases {
		server.RegisterDatabase(db)
	}

	clt := buildClient(t, server)
	t.Run("Resources", func(t *testing.T) {
		listResult, err := clt.ListResources(t.Context(), mcp.ListResourcesRequest{})
		require.NoError(t, err)
		require.Len(t, listResult.Resources, len(databases))

		// sort the result using the same field as databases to avoid flaky
		// test.
		slices.SortFunc(listResult.Resources, func(a, b mcp.Resource) int {
			return strings.Compare(a.URI, b.URI)
		})

		for i, r := range listResult.Resources {
			req := mcp.ReadResourceRequest{}
			req.Params.URI = r.URI
			readResult, err := clt.ReadResource(t.Context(), req)
			require.NoError(t, err)
			require.Len(t, readResult.Contents, 1)
			assertDatabaseResource(t, databases[i], readResult.Contents[0])
		}
	})

	t.Run("Tool", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Name = listDatabasesToolName
		res, err := clt.CallTool(t.Context(), req)
		require.NoError(t, err)
		require.False(t, res.IsError)
		require.Len(t, res.Content, len(databases))

		for _, c := range res.Content {
			require.IsType(t, mcp.EmbeddedResource{}, c)
			require.IsType(t, mcp.TextResourceContents{}, c.(mcp.EmbeddedResource).Resource)
		}

		// Although we're not sorting by the URI directly, the only field that
		// is different across the databases is their name and URI (which would
		// cause them to have the same order). So here we sort by the YAML
		// contents to avoid having to decode.
		slices.SortFunc(res.Content, func(a, b mcp.Content) int {
			resourceA := a.(mcp.EmbeddedResource).Resource.(mcp.TextResourceContents)
			resourceB := b.(mcp.EmbeddedResource).Resource.(mcp.TextResourceContents)
			return strings.Compare(resourceA.Text, resourceB.Text)
		})

		for i, c := range res.Content {
			content := c.(mcp.EmbeddedResource)
			assertDatabaseResource(t, databases[i], content.Resource)
		}
	})
}

func TestEmptyDatabasesServer(t *testing.T) {
	server := NewRootServer(slog.New(slog.DiscardHandler))

	clt := buildClient(t, server)
	t.Run("Resources", func(t *testing.T) {
		_, err := clt.ListResources(t.Context(), mcp.ListResourcesRequest{})
		require.Error(t, err)
	})

	t.Run("Tool", func(t *testing.T) {
		req := mcp.CallToolRequest{}
		req.Params.Name = listDatabasesToolName
		res, err := clt.CallTool(t.Context(), req)
		require.NoError(t, err)
		require.True(t, res.IsError)

		require.Len(t, res.Content, 1)
		content := res.Content[0]
		require.IsType(t, mcp.TextContent{}, content)
		textError := content.(mcp.TextContent).Text
		require.Contains(t, textError, EmptyDatabasesListErrorMessage, "expected empty databases error but got: %s", textError)
	})
}

func assertDatabaseResource(t *testing.T, db *Database, resource mcp.ResourceContents) {
	t.Helper()
	require.IsType(t, mcp.TextResourceContents{}, resource)
	contents := resource.(mcp.TextResourceContents)
	var database DatabaseResource
	require.Equal(t, databaseResourceMIMEType, contents.MIMEType)
	require.NoError(t, yaml.Unmarshal([]byte(contents.Text), &database))
	require.Empty(t, cmp.Diff(buildDatabaseResource(db), database, cmpopts.IgnoreFields(types.Metadata{}, "Namespace")))
}

func buildDatabase(t *testing.T, name string) *Database {
	t.Helper()

	db, err := types.NewDatabaseV3(types.Metadata{
		Name:   name,
		Labels: map[string]string{"env": "test"},
	}, types.DatabaseSpecV3{
		Protocol: defaults.ProtocolPostgres,
		URI:      "localhost:5432",
	})
	require.NoError(t, err)

	return &Database{
		DB:          db,
		ClusterName: "root",
		Addr:        "localhost:5555",
	}
}

func buildClient(t *testing.T, server *RootServer) *mcpclient.Client {
	t.Helper()

	clt, err := mcpclient.NewInProcessClient(server.MCPServer)
	require.NoError(t, err)
	t.Cleanup(func() { clt.Close() })
	require.NoError(t, clt.Start(t.Context()))

	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "test-client",
		Version: "1.0.0",
	}

	_, err = clt.Initialize(t.Context(), initRequest)
	require.NoError(t, err)
	require.NoError(t, clt.Ping(t.Context()))
	return clt
}
