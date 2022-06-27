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

package clusters

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/teleterm/api/uri"
	"github.com/gravitational/teleport/lib/teleterm/gateway"

	"github.com/gravitational/trace"

	"github.com/stretchr/testify/require"
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
	clusters []*Cluster
}

func (f fakeStorage) GetByResourceURI(resourceURI string) (*Cluster, error) {
	for _, cluster := range f.clusters {
		if strings.HasPrefix(resourceURI, cluster.URI.String()) {
			return cluster, nil
		}
	}

	return nil, trace.NotFound("not found")
}

func TestDbcmdCLICommandProviderGetCommand(t *testing.T) {
	testCases := []struct {
		targetSubresourceName string
	}{
		{
			targetSubresourceName: "",
		},
		{
			targetSubresourceName: "bar",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.targetSubresourceName, func(t *testing.T) {
			cluster := Cluster{
				URI:  uri.NewClusterURI("quux"),
				Name: "quux",
				clusterClient: &client.TeleportClient{
					Config: client.Config{
						SiteName: "",
					},
				},
			}
			localPort := "1337"
			gateway := gateway.Gateway{
				Config: gateway.Config{
					TargetURI:             cluster.URI.AppendDB("foo").String(),
					TargetName:            "foo",
					TargetSubresourceName: tc.targetSubresourceName,
					Protocol:              defaults.ProtocolPostgres,
					LocalAddress:          "localhost",
					LocalPort:             localPort,
				},
			}
			fakeStorage := fakeStorage{
				clusters: []*Cluster{&cluster},
			}
			dbcmdCLICommandProvider := NewDbcmdCLICommandProvider(fakeStorage, fakeExec{})

			command, err := dbcmdCLICommandProvider.GetCommand(&gateway)

			require.NoError(t, err)
			require.NotEmpty(t, command)
			require.Contains(t, command, tc.targetSubresourceName)
			require.Contains(t, command, localPort)
		})
	}
}

func TestDbcmdCLICommandProviderGetCommand_ReturnsErrorIfClusterIsNotFound(t *testing.T) {
	gateway := gateway.Gateway{
		Config: gateway.Config{
			TargetURI:             uri.NewClusterURI("quux").AppendDB("foo").String(),
			TargetName:            "foo",
			TargetSubresourceName: "",
			Protocol:              defaults.ProtocolPostgres,
			LocalAddress:          "localhost",
			LocalPort:             "12345",
		},
	}
	fakeStorage := fakeStorage{
		clusters: []*Cluster{},
	}
	dbcmdCLICommandProvider := NewDbcmdCLICommandProvider(fakeStorage, fakeExec{})

	_, err := dbcmdCLICommandProvider.GetCommand(&gateway)
	require.Error(t, err)
	require.True(t, trace.IsNotFound(err), "err is not trace.NotFound")
}
