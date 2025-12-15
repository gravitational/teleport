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

package sftp

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

var parseTestCases = []struct {
	name     string
	in       string
	dest     Target
	errCheck require.ErrorAssertionFunc
}{
	{
		name: "full spec of the remote destination",
		in:   "root@remote.host:/etc/nginx.conf",
		dest: Target{
			Login: "root",
			Addr: &utils.NetAddr{
				Addr:        "remote.host:8080",
				AddrNetwork: "tcp",
			},
			Path: "/etc/nginx.conf",
		},
	},
	{
		name: "spec with just the remote host",
		in:   "remote.host:/etc/nginx.co:nf",
		dest: Target{
			Addr: &utils.NetAddr{
				Addr:        "remote.host:8080",
				AddrNetwork: "tcp",
			},
			Path: "/etc/nginx.co:nf",
		},
	},
	{
		name: "ipv6 remote destination address",
		in:   "[::1]:/etc/nginx.co:nf",
		dest: Target{
			Addr: &utils.NetAddr{
				Addr:        "[::1]:8080",
				AddrNetwork: "tcp",
			},
			Path: "/etc/nginx.co:nf",
		},
	},
	{
		name: "full spec of the remote destination using ipv4 address",
		in:   "root@123.123.123.123:/var/www/html/",
		dest: Target{
			Login: "root",
			Addr: &utils.NetAddr{
				Addr:        "123.123.123.123:8080",
				AddrNetwork: "tcp",
			},
			Path: "/var/www/html/",
		},
	},
	{
		name: "target location using wildcard",
		in:   "myusername@myremotehost.com:/home/hope/*",
		dest: Target{
			Login: "myusername",
			Addr: &utils.NetAddr{
				Addr:        "myremotehost.com:8080",
				AddrNetwork: "tcp",
			},
			Path: "/home/hope/*",
		},
	},
	{
		name: "complex login",
		in:   "complex@example.com@remote.com:/anything.txt",
		dest: Target{
			Login: "complex@example.com",
			Addr: &utils.NetAddr{
				Addr:        "remote.com:8080",
				AddrNetwork: "tcp",
			},
			Path: "/anything.txt",
		},
	},
	{
		name: "implicit user's home directory",
		in:   "root@remote.host:",
		dest: Target{
			Login: "root",
			Addr: &utils.NetAddr{
				Addr:        "remote.host:8080",
				AddrNetwork: "tcp",
			},
			Path: ".",
		},
	},
	{
		name: "no login and '@' in path",
		in:   "remote.host:/some@file",
		dest: Target{
			Addr: &utils.NetAddr{
				Addr:        "remote.host:8080",
				AddrNetwork: "tcp",
			},
			Path: "/some@file",
		},
	},
	{
		name: "no login, '@' and ':' in path",
		in:   "remote.host:/some@remote:file",
		dest: Target{
			Addr: &utils.NetAddr{
				Addr:        "remote.host:8080",
				AddrNetwork: "tcp",
			},
			Path: "/some@remote:file",
		},
	},
	{
		name: "complex login, IPv6 addr and ':' in path",
		in:   "complex@user@[::1]:/remote:file",
		dest: Target{
			Login: "complex@user",
			Addr: &utils.NetAddr{
				Addr:        "[::1]:8080",
				AddrNetwork: "tcp",
			},
			Path: "/remote:file",
		},
	},
	{
		name: "filename with timestamp",
		in:   "user@server.com:/tmp/user-2022-03-10T09:49:23-98cd2a03/file.txt",
		dest: Target{
			Login: "user",
			Addr: &utils.NetAddr{
				Addr:        "server.com:8080",
				AddrNetwork: "tcp",
			},
			Path: "/tmp/user-2022-03-10T09:49:23-98cd2a03/file.txt",
		},
	},
	{
		name: "filename with '@' suffix",
		in:   "user@server:file@",
		dest: Target{
			Login: "user",
			Addr: &utils.NetAddr{
				Addr:        "server:8080",
				AddrNetwork: "tcp",
			},
			Path: "file@",
		},
	},
	{
		name: "filename with IPv6 address",
		in:   "user@server:file[::1]name",
		dest: Target{
			Login: "user",
			Addr: &utils.NetAddr{
				Addr:        "server:8080",
				AddrNetwork: "tcp",
			},
			Path: "file[::1]name",
		},
	},
	{
		name: "IPv6 address and filename with IPv6 address",
		in:   "user@[::1]:file[::1]name",
		dest: Target{
			Login: "user",
			Addr: &utils.NetAddr{
				Addr:        "[::1]:8080",
				AddrNetwork: "tcp",
			},
			Path: "file[::1]name",
		},
	},
	{
		name: "IPv6 address and filename with IPv6 address and '@'s",
		in:   "user@[::1]:file@[::1]@name",
		dest: Target{
			Login: "user",
			Addr: &utils.NetAddr{
				Addr:        "[::1]:8080",
				AddrNetwork: "tcp",
			},
			Path: "file@[::1]@name",
		},
	},
	{
		name: "path only",
		in:   "path/to/somewhere",
		dest: Target{
			Path: "path/to/somewhere",
		},
	},
	{
		name: "missing path",
		in:   "user@server",
		errCheck: func(t require.TestingT, err error, i ...any) {
			require.EqualError(t, err, fmt.Sprintf("%q is missing a path, use form [[user@]host:]path", i[0]))
		},
	},
	{
		name: "missing host",
		in:   "user@:/foo",
		errCheck: func(t require.TestingT, err error, i ...any) {
			require.EqualError(t, err, fmt.Sprintf("%q is missing a host, use form [[user@]host:]path", i[0]))
		},
	},
	{
		name: "invalid IPv6 addr, only one colon",
		in:   "[user]@[:",
		errCheck: func(t require.TestingT, err error, i ...any) {
			require.EqualError(t, err, fmt.Sprintf("%q has an invalid host, host cannot contain '[' unless it is an IPv6 address", i[0]))
		},
	},
	{
		name: "invalid IPv6 addr, only one colon",
		in:   "[user]@[::1:file",
		errCheck: func(t require.TestingT, err error, i ...any) {
			require.EqualError(t, err, fmt.Sprintf("%q has an invalid host, host cannot contain '[' or ':' unless it is an IPv6 address", i[0]))
		},
	},
	{
		name: "missing path with IPv6 addr",
		in:   "[user]@[::1]",
		errCheck: func(t require.TestingT, err error, i ...any) {
			require.EqualError(t, err, fmt.Sprintf("%q is missing a path, use form [[user@]host:]path", i[0]))
		},
	},
}

func TestParseTarget(t *testing.T) {
	t.Parallel()

	for _, tt := range parseTestCases {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := ParseTarget(tt.in, 8080)
			if tt.errCheck == nil {
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(resp, tt.dest))
			} else {
				tt.errCheck(t, err, tt.in)
			}
		})
	}
}

func FuzzParseTarget(f *testing.F) {
	for _, tt := range parseTestCases {
		f.Add(tt.in)
	}

	f.Fuzz(func(t *testing.T, input string) {
		_, _ = ParseTarget(input, 8080)
	})
}

func TestParseSources(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		inputSources    []string
		assert          assert.ErrorAssertionFunc
		expectedSources Sources
	}{
		{
			name:         "ok one source",
			inputSources: []string{"alice@foo:/path/to/thing"},
			assert:       assert.NoError,
			expectedSources: Sources{
				Login: "alice",
				Addr:  utils.MustParseAddr("foo:8080"),
				Paths: []string{"/path/to/thing"},
			},
		},
		{
			name:         "ok multiple sources",
			inputSources: []string{"alice@foo:/path/one", "alice@foo:/path/two"},
			assert:       assert.NoError,
			expectedSources: Sources{
				Login: "alice",
				Addr:  utils.MustParseAddr("foo:8080"),
				Paths: []string{"/path/one", "/path/two"},
			},
		},
		{
			name:   "no sources",
			assert: assert.Error,
		},
		{
			name:         "sources from different hosts",
			inputSources: []string{"alice@foo:/path", "/local/path"},
			assert:       assert.Error,
		},
		{
			name:         "sources with different logins",
			inputSources: []string{"alice@foo:/path/one", "bob@foo:/path/two"},
			assert:       assert.Error,
		},
	}
	for _, tc := range tests {
		sources, err := ParseSources(tc.inputSources, 8080)
		tc.assert(t, err)
		assert.Equal(t, tc.expectedSources, sources)
	}
}

func TestIsRemotePath(t *testing.T) {
	t.Parallel()
	accept := []struct {
		name  string
		input string
	}{
		{
			name:  "remote path",
			input: "foo:path/to/bar",
		},
		{
			name:  "remote path with user",
			input: "user@foo:/path/to/bar",
		},
		{
			name:  "empty path",
			input: "foo:",
		},
		{
			name:  "remote with no slashes",
			input: "foo:bar",
		},
		{
			name:  "fake Windows path",
			input: `foo:\valid\unix\file\name\weirdly`,
		},
	}
	for _, tc := range accept {
		t.Run("accept "+tc.name, func(t *testing.T) {
			require.True(t, IsRemotePath(tc.input))
		})
	}
	reject := []struct {
		name  string
		input string
	}{
		{
			name:  "local path",
			input: "path/to/bar",
		},
		{
			name:  "Windows absolute path",
			input: `C:\path\to\bar`,
		},
		{
			name:  "local path with colon",
			input: "/foo:bar",
		},
		{
			name: "empty path",
		},
	}
	for _, tc := range reject {
		t.Run("reject "+tc.name, func(t *testing.T) {
			require.False(t, IsRemotePath(tc.input))
		})
	}
}
