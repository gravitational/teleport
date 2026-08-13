/**
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package main

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPublicHostFor(t *testing.T) {
	tests := []struct {
		name string
		host string
		want string
	}{
		{name: "no DOCKER_HOST", host: "", want: "localhost"},
		{name: "local socket", host: "unix:///var/run/docker.sock", want: "localhost"},
		{name: "remote tcp", host: "tcp://linux-box:2375", want: "linux-box"},
		{name: "remote tcp without port", host: "tcp://10.0.1.42", want: "10.0.1.42"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var u *url.URL
			if test.host != "" {
				parsed, err := url.Parse(test.host)
				require.NoError(t, err)
				u = parsed
			}

			got, err := publicHostFor(u)
			require.NoError(t, err)
			require.Equal(t, test.want, got)
		})
	}
}

func TestParseSSHConfig(t *testing.T) {
	// Trimmed `ssh -G linux-box` output for a Host alias that rewrites both hostname and port.
	const withAlias = `user ryan
hostname 10.0.1.42
port 2222
addressfamily any
identityfile ~/.ssh/id_ed25519
requesttty auto
`

	const noPort = `user ryan
hostname linux-box.internal
addressfamily any
`

	tests := []struct {
		name         string
		out          string
		wantHostname string
		wantPort     string
	}{
		{name: "alias rewrites hostname and port", out: withAlias, wantHostname: "10.0.1.42", wantPort: "2222"},
		{name: "port defaults to 22", out: noPort, wantHostname: "linux-box.internal", wantPort: "22"},
		{name: "no hostname reported", out: "user ryan\n", wantHostname: "", wantPort: "22"},
		{name: "empty output", out: "", wantHostname: "", wantPort: "22"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hostname, port := parseSSHConfig([]byte(test.out))

			require.Equal(t, test.wantHostname, hostname)
			require.Equal(t, test.wantPort, port)
		})
	}
}
