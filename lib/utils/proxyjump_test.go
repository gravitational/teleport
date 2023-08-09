/*
Copyright 2019 Gravitational, Inc.

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

package utils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/utils/utilsaddr"
)

func TestProxyJumpParsing(t *testing.T) {
	t.Parallel()

	type tc struct {
		in  string
		out []JumpHost
	}
	testCases := []tc{
		{
			in:  "host:12345",
			out: []JumpHost{{Addr: utilsaddr.NetAddr{Addr: "host:12345", AddrNetwork: "tcp"}}},
		},
		{
			in:  "host",
			out: []JumpHost{{Addr: utilsaddr.NetAddr{Addr: "host", AddrNetwork: "tcp"}}},
		},
		{
			in:  "bob@host",
			out: []JumpHost{{Username: "bob", Addr: utilsaddr.NetAddr{Addr: "host", AddrNetwork: "tcp"}}},
		},
		{
			in:  "alice@127.0.0.1:7777",
			out: []JumpHost{{Username: "alice", Addr: utilsaddr.NetAddr{Addr: "127.0.0.1:7777", AddrNetwork: "tcp"}}},
		},
		{
			in:  "alice@127.0.0.1:7777, bob@localhost",
			out: []JumpHost{{Username: "alice", Addr: utilsaddr.NetAddr{Addr: "127.0.0.1:7777", AddrNetwork: "tcp"}}, {Username: "bob", Addr: utilsaddr.NetAddr{Addr: "localhost", AddrNetwork: "tcp"}}},
		},
		{
			in:  "alice@[::1]:7777, bob@localhost",
			out: []JumpHost{{Username: "alice", Addr: utilsaddr.NetAddr{Addr: "[::1]:7777", AddrNetwork: "tcp"}}, {Username: "bob", Addr: utilsaddr.NetAddr{Addr: "localhost", AddrNetwork: "tcp"}}},
		},
		{
			in:  "alice@domain.com@[::1]:7777, bob@localhost@localhost",
			out: []JumpHost{{Username: "alice@domain.com", Addr: utilsaddr.NetAddr{Addr: "[::1]:7777", AddrNetwork: "tcp"}}, {Username: "bob@localhost", Addr: utilsaddr.NetAddr{Addr: "localhost", AddrNetwork: "tcp"}}},
		},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.in), func(t *testing.T) {
			re, err := ParseProxyJump(tc.in)
			require.NoError(t, err)
			require.Equal(t, tc.out, re)
		})
	}
}
