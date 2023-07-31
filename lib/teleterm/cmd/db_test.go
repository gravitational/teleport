// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/teleterm/gateway"
	"github.com/gravitational/teleport/lib/teleterm/gatewaytest"
	"github.com/gravitational/teleport/lib/tlsca"
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

type fakeStorage struct {
	clusters []*clusters.Cluster
}

func (f fakeStorage) GetByResourceURI(resourceURI uri.ResourceURI) (*clusters.Cluster, error) {
	for _, cluster := range f.clusters {
		if strings.HasPrefix(resourceURI.String(), cluster.URI.String()) {
			return cluster, nil
		}
	}

	return nil, trace.NotFound("not found")
}

type fakeDatabaseGateway struct {
	gateway.Database
	targetURI       uri.ResourceURI
	subresourceName string
}

func (m fakeDatabaseGateway) TargetURI() uri.ResourceURI    { return m.targetURI }
func (m fakeDatabaseGateway) TargetName() string            { return m.targetURI.GetDbName() }
func (m fakeDatabaseGateway) TargetUser() string            { return "alice" }
func (m fakeDatabaseGateway) TargetSubresourceName() string { return m.subresourceName }
func (m fakeDatabaseGateway) Protocol() string              { return defaults.ProtocolMongoDB }
func (m fakeDatabaseGateway) Log() *logrus.Entry            { return nil }
func (m fakeDatabaseGateway) LocalAddress() string          { return "localhost" }
func (m fakeDatabaseGateway) LocalPortInt() int             { return 8888 }
func (m fakeDatabaseGateway) LocalPort() string             { return "8888" }

func TestDbcmdCLICommandProviderGetCommand(t *testing.T) {
	testCases := []struct {
		name                  string
		targetSubresourceName string
	}{
		{
			name:                  "empty name",
			targetSubresourceName: "",
		},
		{
			name:                  "with name",
			targetSubresourceName: "bar",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cluster := clusters.Cluster{
				URI:  uri.NewClusterURI("quux"),
				Name: "quux",
			}
			fakeStorage := fakeStorage{
				clusters: []*clusters.Cluster{&cluster},
			}
			mockGateway := fakeDatabaseGateway{
				targetURI:       cluster.URI.AppendDB("foo"),
				subresourceName: tc.targetSubresourceName,
			}

			dbcmdCLICommandProvider := NewDBCLICommandProvider(fakeStorage, fakeExec{})
			command, err := dbcmdCLICommandProvider.GetCommand(mockGateway)

			require.NoError(t, err)
			require.NotEmpty(t, command.Args)
			require.Contains(t, command.Args[1], tc.targetSubresourceName)
			require.Contains(t, command.Args[1], mockGateway.LocalPort())
		})
	}
}

func TestDbcmdCLICommandProviderGetCommand_ReturnsErrorIfClusterIsNotFound(t *testing.T) {
	fakeStorage := fakeStorage{
		clusters: []*clusters.Cluster{},
	}
	dbcmdCLICommandProvider := NewDBCLICommandProvider(fakeStorage, fakeExec{})

	keyPairPaths := gatewaytest.MustGenAndSaveCert(t, tlsca.Identity{
		Username: "alice",
		Groups:   []string{"test-group"},
		RouteToDatabase: tlsca.RouteToDatabase{
			ServiceName: "foo",
			Protocol:    defaults.ProtocolPostgres,
			Username:    "alice",
		},
	})

	gateway, err := gateway.New(
		gateway.Config{
			TargetURI:             uri.NewClusterURI("quux").AppendDB("foo"),
			TargetName:            "foo",
			TargetUser:            "alice",
			TargetSubresourceName: "",
			Protocol:              defaults.ProtocolPostgres,
			LocalAddress:          "localhost",
			WebProxyAddr:          "localhost:1337",
			Insecure:              true,
			CertPath:              keyPairPaths.CertPath,
			KeyPath:               keyPairPaths.KeyPath,
			CLICommandProvider:    dbcmdCLICommandProvider,
			TCPPortAllocator:      gateway.NetTCPPortAllocator{},
		},
	)
	require.NoError(t, err)
	t.Cleanup(func() { gateway.Close() })

	_, err = dbcmdCLICommandProvider.GetCommand(gateway)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err), "err is not trace.NotFound")
}
