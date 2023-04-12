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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils"
)

func TestDestinationParsing(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		comment string
		in      string
		dest    Destination
		err     error
	}{
		{
			comment: "full spec of the remote destination",
			in:      "root@remote.host:/etc/nginx.conf",
			dest:    Destination{Login: "root", Host: utils.NetAddr{Addr: "remote.host", AddrNetwork: "tcp"}, Path: "/etc/nginx.conf"},
		},
		{
			comment: "spec with just the remote host",
			in:      "remote.host:/etc/nginx.co:nf",
			dest:    Destination{Host: utils.NetAddr{Addr: "remote.host", AddrNetwork: "tcp"}, Path: "/etc/nginx.co:nf"},
		},
		{
			comment: "ipv6 remote destination address",
			in:      "[::1]:/etc/nginx.co:nf",
			dest:    Destination{Host: utils.NetAddr{Addr: "[::1]", AddrNetwork: "tcp"}, Path: "/etc/nginx.co:nf"},
		},
		{
			comment: "full spec of the remote destination using ipv4 address",
			in:      "root@123.123.123.123:/var/www/html/",
			dest:    Destination{Login: "root", Host: utils.NetAddr{Addr: "123.123.123.123", AddrNetwork: "tcp"}, Path: "/var/www/html/"},
		},
		{
			comment: "target location using wildcard",
			in:      "myusername@myremotehost.com:/home/hope/*",
			dest:    Destination{Login: "myusername", Host: utils.NetAddr{Addr: "myremotehost.com", AddrNetwork: "tcp"}, Path: "/home/hope/*"},
		},
		{
			comment: "complex login",
			in:      "complex@example.com@remote.com:/anything.txt",
			dest:    Destination{Login: "complex@example.com", Host: utils.NetAddr{Addr: "remote.com", AddrNetwork: "tcp"}, Path: "/anything.txt"},
		},
		{
			comment: "implicit user's home directory",
			in:      "root@remote.host:",
			dest:    Destination{Login: "root", Host: utils.NetAddr{Addr: "remote.host", AddrNetwork: "tcp"}, Path: "."},
		},
	}
	for _, tt := range testCases {
		t.Run(tt.comment, func(t *testing.T) {
			resp, err := ParseDestination(tt.in)
			if tt.err != nil {
				require.IsType(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			require.Empty(t, cmp.Diff(resp, &tt.dest))
		})
	}
}
