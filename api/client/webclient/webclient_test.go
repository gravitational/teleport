/*
Copyright 2021 Gravitational, Inc.

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

package webclient

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTunnelAddr(t *testing.T) {
	testCases := []struct {
		description      string
		sshProxySettings SSHProxySettings
		expectTunnelAddr string
	}{
		{
			description: "should use TunnelPublicAddr",
			sshProxySettings: SSHProxySettings{
				TunnelPublicAddr: "tunnel.example.com:4024",
				PublicAddr:       "proxy.example.com",
				SSHPublicAddr:    "ssh.example.com",
				TunnelListenAddr: "[::]:5024",
			},
			expectTunnelAddr: "tunnel.example.com:4024",
		},
		{
			description: "should use SSHPublicAddr and TunnelListenAddr",
			sshProxySettings: SSHProxySettings{
				SSHPublicAddr:    "ssh.example.com",
				PublicAddr:       "proxy.example.com",
				TunnelListenAddr: "[::]:5024",
			},
			expectTunnelAddr: "ssh.example.com:5024",
		},
		{
			description: "should use PublicAddr and TunnelListenAddr",
			sshProxySettings: SSHProxySettings{
				PublicAddr:       "proxy.example.com",
				TunnelListenAddr: "[::]:5024",
			},
			expectTunnelAddr: "proxy.example.com:5024",
		},
		{
			description: "should return TunnelListenAddr",
			sshProxySettings: SSHProxySettings{
				TunnelListenAddr: "[::]:5024",
			},
			expectTunnelAddr: "[::]:5024",
		},
		{
			description: "should use PublicAddr and SSHProxyTunnelListenPort",
			sshProxySettings: SSHProxySettings{
				PublicAddr: "proxy.example.com",
			},
			expectTunnelAddr: "proxy.example.com:3024",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			tunnelAddr, err := tunnelAddr(tc.sshProxySettings)
			require.NoError(t, err)
			require.Equal(t, tc.expectTunnelAddr, tunnelAddr)
		})
	}
}
