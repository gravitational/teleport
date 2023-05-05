/*
Copyright 2023 Gravitational, Inc.

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

package sftp

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

func TestParseDestination(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		in       string
		dest     Destination
		errCheck require.ErrorAssertionFunc
	}{
		{
			name: "full spec of the remote destination",
			in:   "root@remote.host:/etc/nginx.conf",
			dest: Destination{
				Login: "root",
				Host: &utils.NetAddr{
					Addr:        "remote.host",
					AddrNetwork: "tcp",
				},
				Path: "/etc/nginx.conf",
			},
		},
		{
			name: "spec with just the remote host",
			in:   "remote.host:/etc/nginx.co:nf",
			dest: Destination{
				Host: &utils.NetAddr{
					Addr:        "remote.host",
					AddrNetwork: "tcp",
				},
				Path: "/etc/nginx.co:nf",
			},
		},
		{
			name: "ipv6 remote destination address",
			in:   "[::1]:/etc/nginx.co:nf",
			dest: Destination{
				Host: &utils.NetAddr{
					Addr:        "[::1]",
					AddrNetwork: "tcp",
				},
				Path: "/etc/nginx.co:nf",
			},
		},
		{
			name: "full spec of the remote destination using ipv4 address",
			in:   "root@123.123.123.123:/var/www/html/",
			dest: Destination{
				Login: "root",
				Host: &utils.NetAddr{
					Addr:        "123.123.123.123",
					AddrNetwork: "tcp",
				},
				Path: "/var/www/html/",
			},
		},
		{
			name: "target location using wildcard",
			in:   "myusername@myremotehost.com:/home/hope/*",
			dest: Destination{
				Login: "myusername",
				Host: &utils.NetAddr{
					Addr:        "myremotehost.com",
					AddrNetwork: "tcp",
				},
				Path: "/home/hope/*",
			},
		},
		{
			name: "complex login",
			in:   "complex@example.com@remote.com:/anything.txt",
			dest: Destination{
				Login: "complex@example.com",
				Host: &utils.NetAddr{
					Addr:        "remote.com",
					AddrNetwork: "tcp",
				},
				Path: "/anything.txt",
			},
		},
		{
			name: "implicit user's home directory",
			in:   "root@remote.host:",
			dest: Destination{
				Login: "root",
				Host: &utils.NetAddr{
					Addr:        "remote.host",
					AddrNetwork: "tcp",
				},
				Path: ".",
			},
		},
		{
			name: "no login and '@' in path",
			in:   "remote.host:/some@file",
			dest: Destination{
				Host: &utils.NetAddr{
					Addr:        "remote.host",
					AddrNetwork: "tcp",
				},
				Path: "/some@file",
			},
		},
		{
			name: "no login, '@' and ':' in path",
			in:   "remote.host:/some@remote:file",
			dest: Destination{
				Host: &utils.NetAddr{
					Addr:        "remote.host",
					AddrNetwork: "tcp",
				},
				Path: "/some@remote:file",
			},
		},
		{
			name: "complex login, IPv6 addr and ':' in path",
			in:   "complex@user@[::1]:/remote:file",
			dest: Destination{
				Login: "complex@user",
				Host: &utils.NetAddr{
					Addr:        "[::1]",
					AddrNetwork: "tcp",
				},
				Path: "/remote:file",
			},
		},
		{
			name: "filename with timestamp",
			in:   "user@server.com:/tmp/user-2022-03-10T09:49:23-98cd2a03/file.txt",
			dest: Destination{
				Login: "user",
				Host: &utils.NetAddr{
					Addr:        "server.com",
					AddrNetwork: "tcp",
				},
				Path: "/tmp/user-2022-03-10T09:49:23-98cd2a03/file.txt",
			},
		},
		{
			name: "filename with '@' suffix",
			in:   "user@server:file@",
			dest: Destination{
				Login: "user",
				Host: &utils.NetAddr{
					Addr:        "server",
					AddrNetwork: "tcp",
				},
				Path: "file@",
			},
		},
		{
			name: "filename with IPv6 address",
			in:   "user@server:file[::1]name",
			dest: Destination{
				Login: "user",
				Host: &utils.NetAddr{
					Addr:        "server",
					AddrNetwork: "tcp",
				},
				Path: "file[::1]name",
			},
		},
		{
			name: "IPv6 address and filename with IPv6 address",
			in:   "user@[::1]:file[::1]name",
			dest: Destination{
				Login: "user",
				Host: &utils.NetAddr{
					Addr:        "[::1]",
					AddrNetwork: "tcp",
				},
				Path: "file[::1]name",
			},
		},
		{
			name: "IPv6 address and filename with IPv6 address and '@'s",
			in:   "user@[::1]:file@[::1]@name",
			dest: Destination{
				Login: "user",
				Host: &utils.NetAddr{
					Addr:        "[::1]",
					AddrNetwork: "tcp",
				},
				Path: "file@[::1]@name",
			},
		},
		{
			name: "missing path",
			in:   "user@server",
			errCheck: func(t require.TestingT, err error, i ...interface{}) {
				require.EqualError(t, err, fmt.Sprintf("%q is missing a path, use form [user@]host:[path]", i[0]))
			},
		},
		{
			name: "missing host",
			in:   "user@:/foo",
			errCheck: func(t require.TestingT, err error, i ...interface{}) {
				require.EqualError(t, err, fmt.Sprintf("%q is missing a host, use form [user@]host:[path]", i[0]))
			},
		},
		{
			name: "invalid IPv6 addr, only one colon",
			in:   "[user]@[:",
			errCheck: func(t require.TestingT, err error, i ...interface{}) {
				require.EqualError(t, err, fmt.Sprintf("%q has an invalid host, host cannot contain '[' unless it is an IPv6 address", i[0]))
			},
		},
		{
			name: "invalid IPv6 addr, only one colon",
			in:   "[user]@[::1:file",
			errCheck: func(t require.TestingT, err error, i ...interface{}) {
				require.EqualError(t, err, fmt.Sprintf("%q has an invalid host, host cannot contain '[' or ':' unless it is an IPv6 address", i[0]))
			},
		},
		{
			name: "missing path with IPv6 addr",
			in:   "[user]@[::1]",
			errCheck: func(t require.TestingT, err error, i ...interface{}) {
				require.EqualError(t, err, fmt.Sprintf("%q is missing a path, use form [user@]host:[path]", i[0]))
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := ParseDestination(tt.in)
			if tt.errCheck == nil {
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(resp, &tt.dest))
			} else {
				tt.errCheck(t, err, tt.in)
			}
		})
	}
}

func FuzzParseDestination(f *testing.F) {
	f.Fuzz(func(t *testing.T, input string) {
		_, _ = ParseDestination(input)
	})
}
