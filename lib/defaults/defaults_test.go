/*
Copyright 2016 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package defaults

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

func TestMakeAddr(t *testing.T) {
	addr := makeAddr("example.com", 3022)
	require.NotNil(t, addr)
	require.Equal(t, "tcp://example.com:3022", addr.FullAddress())
}

func TestDefaultAddresses(t *testing.T) {
	table := map[string]*utils.NetAddr{
		"tcp://0.0.0.0:3025":   AuthListenAddr(),
		"tcp://127.0.0.1:3025": AuthConnectAddr(),
		"tcp://0.0.0.0:3023":   ProxyListenAddr(),
		"tcp://0.0.0.0:3080":   ProxyWebListenAddr(),
		"tcp://0.0.0.0:3022":   SSHServerListenAddr(),
		"tcp://0.0.0.0:3024":   ReverseTunnelListenAddr(),
	}
	for expected, actual := range table {
		require.NotNil(t, actual)
		require.Equal(t, expected, actual.FullAddress())
	}
}

func TestReadableDatabaseProtocol(t *testing.T) {
	require.Equal(t, "Microsoft SQL Server", fmt.Sprint(ReadableDatabaseProtocol(ProtocolSQLServer)))
	require.Equal(t, "unknown", fmt.Sprint(ReadableDatabaseProtocol("unknown")))
}
