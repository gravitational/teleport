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

package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
)

type fakeExec struct{}

func (f fakeExec) RunCommand(cmd string, _ ...string) ([]byte, error) {
	return []byte(""), nil
}

func (f fakeExec) LookPath(path string) (string, error) {
	return "", nil
}

func (f fakeExec) Command(name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	cmd.Path = filepath.Base(cmd.Path)
	return cmd
}

type fakeDatabaseGateway struct {
	gateway.Database
	targetURI       uri.ResourceURI
	subresourceName string
	protocol        string
}

func (m fakeDatabaseGateway) TargetURI() uri.ResourceURI    { return m.targetURI }
func (m fakeDatabaseGateway) TargetName() string            { return m.targetURI.GetDbName() }
func (m fakeDatabaseGateway) TargetUser() string            { return "alice" }
func (m fakeDatabaseGateway) TargetSubresourceName() string { return m.subresourceName }
func (m fakeDatabaseGateway) Protocol() string              { return m.protocol }
func (m fakeDatabaseGateway) Log() *slog.Logger             { return nil }
func (m fakeDatabaseGateway) LocalAddress() string          { return "localhost" }
func (m fakeDatabaseGateway) LocalPortInt() int             { return 8888 }
func (m fakeDatabaseGateway) LocalPort() string             { return "8888" }

func TestNewDBCLICommand(t *testing.T) {
	// TODO mock other types
	authClient := &mockAuthClient{
		database: &types.DatabaseV3{
			Spec: types.DatabaseSpecV3{
				Protocol: types.DatabaseProtocolMongoDB,
			},
		},
	}

	testCases := []struct {
		name                  string
		targetSubresourceName string
		argsCount             int
		protocol              string
		checkCmds             func(*testing.T, fakeDatabaseGateway, Cmds)
	}{
		{
			name:                  "empty name",
			protocol:              defaults.ProtocolMongoDB,
			targetSubresourceName: "",
			checkCmds:             checkMongoCmds,
		},
		{
			name:                  "with name",
			protocol:              defaults.ProtocolMongoDB,
			targetSubresourceName: "bar",
			checkCmds:             checkMongoCmds,
		},
		{
			name:                  "custom handling of DynamoDB does not blow up",
			targetSubresourceName: "bar",
			protocol:              defaults.ProtocolDynamoDB,
			checkCmds:             checkArgsNotEmpty,
		},
		{
			name:                  "custom handling of Spanner does not blow up",
			targetSubresourceName: "bar",
			protocol:              defaults.ProtocolSpanner,
			checkCmds:             checkArgsNotEmpty,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cluster := clusters.Cluster{
				URI:  uri.NewClusterURI("quux"),
				Name: "quux",
			}
			mockGateway := fakeDatabaseGateway{
				targetURI:       cluster.URI.AppendDB("foo"),
				subresourceName: tc.targetSubresourceName,
				protocol:        tc.protocol,
			}

			cmds, err := newDBCLICommandWithExecer(context.Background(), &cluster, mockGateway, fakeExec{}, authClient)
			require.NoError(t, err)

			tc.checkCmds(t, mockGateway, cmds)
		})
	}
}

type mockAuthClient struct {
	authclient.ClientI
	database *types.DatabaseV3
}

func (m *mockAuthClient) GetResources(_ context.Context, _ *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error) {
	if m.database == nil {
		return nil, trace.NotFound("not found")
	}
	return &proto.ListResourcesResponse{
		Resources: []*proto.PaginatedResource{{
			Resource: &proto.PaginatedResource_DatabaseServer{
				DatabaseServer: &types.DatabaseServerV3{
					Spec: types.DatabaseServerSpecV3{
						Database: m.database,
					},
				},
			},
		}},
		TotalCount: 1,
	}, nil
}

func checkMongoCmds(t *testing.T, gw fakeDatabaseGateway, cmds Cmds) {
	t.Helper()
	require.Len(t, cmds.Exec.Args, 2)
	require.Len(t, cmds.Preview.Args, 2)

	execConnString := cmds.Exec.Args[1]
	previewConnString := cmds.Preview.Args[1]

	require.Contains(t, execConnString, gw.TargetSubresourceName())
	require.Contains(t, previewConnString, gw.TargetSubresourceName())
	require.Contains(t, execConnString, gw.LocalPort())
	require.Contains(t, previewConnString, gw.LocalPort())

	// Verify that the preview cmd has exec cmd conn string wrapped in quotes.
	require.NotContains(t, execConnString, "\"")
	expectedPreviewConnString := fmt.Sprintf("%q", execConnString)
	require.Equal(t, expectedPreviewConnString, previewConnString)
}

func checkArgsNotEmpty(t *testing.T, gw fakeDatabaseGateway, cmds Cmds) {
	t.Helper()
	require.NotEmpty(t, cmds.Exec.Args)
	require.NotEmpty(t, cmds.Preview.Args)
}
