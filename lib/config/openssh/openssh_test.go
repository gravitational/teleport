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

	"github.com/coreos/go-semver/semver"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/golden"
)

func TestParseSSHVersion(t *testing.T) {
	tests := []struct {
		str     string
		version *semver.Version
		err     bool
	}{
		{
			str:     "OpenSSH_8.2p1 Ubuntu-4ubuntu0.4, OpenSSL 1.1.1f  31 Mar 2020",
			version: semver.New("8.2.1"),
		},
		{
			str:     "OpenSSH_8.8p1, OpenSSL 1.1.1m  14 Dec 2021",
			version: semver.New("8.8.1"),
		},
		{
			str:     "OpenSSH_7.5p1, OpenSSL 1.0.2s-freebsd  28 May 2019",
			version: semver.New("7.5.1"),
		},
		{
			str:     "OpenSSH_7.9p1 Raspbian-10+deb10u2, OpenSSL 1.1.1d  10 Sep 2019",
			version: semver.New("7.9.1"),
		},
		{
			// Couldn't find a full example but in theory patch is optional:
			str:     "OpenSSH_8.1 foo",
			version: semver.New("8.1.0"),
		},
		{
			str: "Teleport v8.0.0-dev.40 git:v8.0.0-dev.40-0-ge9194c256 go1.17.2",
			err: true,
		},
	}

	for _, test := range tests {
		version, err := parseSSHVersion(test.str)
		if test.err {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.True(t, version.Equal(*test.version), "got version = %v, want = %v", version, test.version)
		}
	}
}

func TestSSHConfig_GetSSHConfig(t *testing.T) {
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
			c := &SSHConfig{
				getSSHVersion: func() (*semver.Version, error) {
					return semver.New(tt.sshVersion), nil
				},
				log: logrus.New(),
			}

			sb := &strings.Builder{}
			err := c.GetSSHConfig(sb, tt.config)
			if golden.ShouldSet() {
				golden.Set(t, []byte(sb.String()))
			}
			require.NoError(t, err)
			require.Equal(t, string(golden.Get(t)), sb.String())
		})
	}
}

func TestSSHConfig_GetMuxedSSHConfig(t *testing.T) {
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
			c := &SSHConfig{
				getSSHVersion: func() (*semver.Version, error) {
					return semver.New(tt.sshVersion), nil
				},
				log: logrus.New(),
			}

			sb := &strings.Builder{}
			err := c.GetMuxedSSHConfig(sb, tt.config)
			if golden.ShouldSet() {
				golden.Set(t, []byte(sb.String()))
			}
			require.NoError(t, err)
			require.Equal(t, string(golden.Get(t)), sb.String())
		})
	}
}
