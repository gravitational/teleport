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
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

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

func TestNewDBCLICommand(t *testing.T) {
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
			mockGateway := fakeDatabaseGateway{
				targetURI:       cluster.URI.AppendDB("foo"),
				subresourceName: tc.targetSubresourceName,
			}

			command, err := newDBCLICommandWithExecer(&cluster, mockGateway, fakeExec{})

			require.NoError(t, err)
			require.Len(t, command.Args, 2)
			require.Contains(t, command.Args[1], tc.targetSubresourceName)
			require.Contains(t, command.Args[1], mockGateway.LocalPort())
		})
	}
}
