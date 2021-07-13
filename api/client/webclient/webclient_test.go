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
	testTunnelAddr := func(settings SSHProxySettings, expectedTunnelAddr string) func(*testing.T) {
		return func(t *testing.T) {
			tunnelAddr, err := tunnelAddr(settings)
			require.NoError(t, err)
			require.Equal(t, expectedTunnelAddr, tunnelAddr)
		}
	}

	t.Run("should use TunnelPublicAddr", testTunnelAddr(
		SSHProxySettings{
			TunnelPublicAddr: "tunnel.example.com:4024",
			PublicAddr:       "proxy.example.com",
			SSHPublicAddr:    "ssh.example.com",
			TunnelListenAddr: "[::]:5024",
		},
		"tunnel.example.com:4024",
	))
	t.Run("should use SSHPublicAddr and TunnelListenAddr", testTunnelAddr(
		SSHProxySettings{
			SSHPublicAddr:    "ssh.example.com",
			PublicAddr:       "proxy.example.com",
			TunnelListenAddr: "[::]:5024",
		},
		"ssh.example.com:5024",
	))
	t.Run("should use PublicAddr and TunnelListenAddr", testTunnelAddr(
		SSHProxySettings{
			PublicAddr:       "proxy.example.com",
			TunnelListenAddr: "[::]:5024",
		},
		"proxy.example.com:5024",
	))
	t.Run("should return TunnelListenAddr", testTunnelAddr(
		SSHProxySettings{
			TunnelListenAddr: "[::]:5024",
		},
		"[::]:5024",
	))
	t.Run("should use PublicAddr and SSHProxyTunnelListenPort", testTunnelAddr(
		SSHProxySettings{
			PublicAddr: "proxy.example.com",
		},
		"proxy.example.com:3024",
	))
}
