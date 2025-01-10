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

package openssh

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

func TestWriteSSHConfig(t *testing.T) {
	tests := []struct {
		name       string
		sshVersion string
		config     *SSHConfigParameters
	}{
		{
			name:       "legacy OpenSSH - single cluster",
			sshVersion: "6.4.0",
			config: &SSHConfigParameters{
				AppName:             TshApp,
				ClusterNames:        []string{"example.com"},
				KnownHostsPath:      "/home/alice/.tsh/known_hosts",
				IdentityFilePath:    "/home/alice/.tsh/keys/example.com/bob",
				CertificateFilePath: "/home/alice/.tsh/keys/example.com/bob-ssh/example.com-cert.pub",
				ProxyHost:           "proxy.example.com",
				ProxyPort:           "443",
				ExecutablePath:      "/tmp/tsh",
			},
		},
		{
			name:       "modern OpenSSH - single cluster",
			sshVersion: "9.0.0",
			config: &SSHConfigParameters{
				AppName:             TshApp,
				ClusterNames:        []string{"example.com"},
				KnownHostsPath:      "/home/alice/.tsh/known_hosts",
				IdentityFilePath:    "/home/alice/.tsh/keys/example.com/bob",
				CertificateFilePath: "/home/alice/.tsh/keys/example.com/bob-ssh/example.com-cert.pub",
				ProxyHost:           "proxy.example.com",
				ProxyPort:           "443",
				ExecutablePath:      "/tmp/tsh",
			},
		},
		{
			name:       "modern OpenSSH - multiple clusters",
			sshVersion: "9.0.0",
			config: &SSHConfigParameters{
				AppName:             TshApp,
				ClusterNames:        []string{"root", "leaf"},
				KnownHostsPath:      "/home/alice/.tsh/known_hosts",
				IdentityFilePath:    "/home/alice/.tsh/keys/example.com/bob",
				CertificateFilePath: "/home/alice/.tsh/keys/example.com/bob-ssh/example.com-cert.pub",
				ProxyHost:           "proxy.example.com",
				ProxyPort:           "443",
				ExecutablePath:      "/tmp/tsh",
			},
		},
		{
			name:       "modern OpenSSH - single cluster with username and custom port",
			sshVersion: "9.0.0",
			config: &SSHConfigParameters{
				AppName:             TshApp,
				ClusterNames:        []string{"example.com"},
				KnownHostsPath:      "/home/alice/.tsh/known_hosts",
				IdentityFilePath:    "/home/alice/.tsh/keys/example.com/bob",
				CertificateFilePath: "/home/alice/.tsh/keys/example.com/bob-ssh/example.com-cert.pub",
				ProxyHost:           "proxy.example.com",
				ProxyPort:           "443",
				ExecutablePath:      "/tmp/tsh",
				Username:            "testuser",
				Port:                3232,
			},
		},
		{
			name:       "test shellQuote",
			sshVersion: "9.0.0",
			config: &SSHConfigParameters{
				AppName:              TbotApp,
				PureTBotProxyCommand: true,
				ClusterNames:         []string{"example.com"},
				KnownHostsPath:       "/home/alice/.tsh/known_hosts",
				IdentityFilePath:     "/home/alice/.tsh/keys/example.com/bob",
				CertificateFilePath:  "/home/alice/.tsh/keys/example.com/bob-ssh/example.com-cert.pub",
				ProxyHost:            "proxy.example.com",
				ProxyPort:            "443",
				ExecutablePath:       "/home/edoardo/$( sudo rm -rf / )/tbot",
				DestinationDir:       "/home/edo\nardo/$( sudo rm -rf / )/tbot-ou'tput",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := &strings.Builder{}
			err := WriteSSHConfig(sb, tt.config)
			if golden.ShouldSet() {
				golden.Set(t, []byte(sb.String()))
			}
			require.NoError(t, err)
			require.Equal(t, string(golden.Get(t)), sb.String())
		})
	}
}

func TestWriteMuxedSSHConfig(t *testing.T) {
	tests := []struct {
		name       string
		sshVersion string
		config     *MuxedSSHConfigParameters
	}{
		{
			name:       "legacy OpenSSH - single cluster",
			sshVersion: "7.4.0",
			config: &MuxedSSHConfigParameters{
				AppName:         TbotApp,
				ClusterNames:    []string{"example.com"},
				KnownHostsPath:  "/opt/machine-id/known_hosts",
				MuxSocketPath:   "/opt/machine-id/v1.sock",
				AgentSocketPath: "/opt/machine-id/agent.sock",
				ProxyCommand:    []string{"/bin/fdpass-teleport", "foo"},
			},
		},
		{
			name:       "modern OpenSSH - single cluster",
			sshVersion: "9.0.0",
			config: &MuxedSSHConfigParameters{
				AppName:         TbotApp,
				ClusterNames:    []string{"example.com"},
				KnownHostsPath:  "/opt/machine-id/known_hosts",
				MuxSocketPath:   "/opt/machine-id/v1.sock",
				AgentSocketPath: "/opt/machine-id/agent.sock",
				ProxyCommand:    []string{"/bin/fdpass-teleport", "foo"},
			},
		},
		{
			name:       "modern OpenSSH - multiple clusters",
			sshVersion: "9.0.0",
			config: &MuxedSSHConfigParameters{
				AppName:         TbotApp,
				ClusterNames:    []string{"example.com", "example.org"},
				KnownHostsPath:  "/opt/machine-id/known_hosts",
				MuxSocketPath:   "/opt/machine-id/v1.sock",
				AgentSocketPath: "/opt/machine-id/agent.sock",
				ProxyCommand:    []string{"/bin/fdpass-teleport", "foo"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := &strings.Builder{}
			err := WriteMuxedSSHConfig(sb, tt.config)
			if golden.ShouldSet() {
				golden.Set(t, []byte(sb.String()))
			}
			require.NoError(t, err)
			require.Equal(t, string(golden.Get(t)), sb.String())
		})
	}
}

func TestWriteClusterSSHConfig(t *testing.T) {
	tests := []struct {
		name       string
		sshVersion string
		config     *ClusterSSHConfigParameters
	}{
		{
			name:       "legacy OpenSSH",
			sshVersion: "7.4.0",
			config: &ClusterSSHConfigParameters{
				AppName:             TbotApp,
				ClusterName:         "example.teleport.sh",
				DestinationDir:      "/opt/machine-id",
				KnownHostsPath:      "/opt/machine-id/example.teleport.sh.known_hosts",
				CertificateFilePath: "/opt/machine-id/key-cert.pub",
				IdentityFilePath:    "/opt/machine-id/key",
				ExecutablePath:      "/bin/tbot",
				ProxyHost:           "example.teleport.sh",
				ProxyPort:           "443",
				Port:                1234,
				Insecure:            true,
				FIPS:                true,
				TLSRouting:          true,
				ConnectionUpgrade:   true,
				Resume:              true,
			},
		},
		{
			name:       "modern OpenSSH",
			sshVersion: "9.0.0",
			config: &ClusterSSHConfigParameters{
				AppName:             TbotApp,
				ClusterName:         "example.teleport.sh",
				DestinationDir:      "/opt/machine-id",
				KnownHostsPath:      "/opt/machine-id/example.teleport.sh.known_hosts",
				CertificateFilePath: "/opt/machine-id/key-cert.pub",
				IdentityFilePath:    "/opt/machine-id/key",
				ExecutablePath:      "/bin/tbot",
				ProxyHost:           "example.teleport.sh",
				ProxyPort:           "443",
				Port:                1234,
				Insecure:            false,
				FIPS:                false,
				TLSRouting:          false,
				ConnectionUpgrade:   false,
				Resume:              false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := &strings.Builder{}
			err := WriteClusterSSHConfig(sb, tt.config)
			if golden.ShouldSet() {
				golden.Set(t, []byte(sb.String()))
			}
			require.NoError(t, err)
			require.Equal(t, string(golden.Get(t)), sb.String())
		})
	}
}
