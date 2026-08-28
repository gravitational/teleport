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

package utils

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
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
			out: []JumpHost{{Addr: NetAddr{Addr: "host:12345", AddrNetwork: "tcp"}}},
		},
		{
			in:  "host",
			out: []JumpHost{{Addr: NetAddr{Addr: "host", AddrNetwork: "tcp"}}},
		},
		{
			in:  "bob@host",
			out: []JumpHost{{Username: "bob", Addr: NetAddr{Addr: "host", AddrNetwork: "tcp"}}},
		},
		{
			in:  "alice@127.0.0.1:7777",
			out: []JumpHost{{Username: "alice", Addr: NetAddr{Addr: "127.0.0.1:7777", AddrNetwork: "tcp"}}},
		},
		{
			in:  "alice@127.0.0.1:7777, bob@localhost",
			out: []JumpHost{{Username: "alice", Addr: NetAddr{Addr: "127.0.0.1:7777", AddrNetwork: "tcp"}}, {Username: "bob", Addr: NetAddr{Addr: "localhost", AddrNetwork: "tcp"}}},
		},
		{
			in:  "alice@[::1]:7777, bob@localhost",
			out: []JumpHost{{Username: "alice", Addr: NetAddr{Addr: "[::1]:7777", AddrNetwork: "tcp"}}, {Username: "bob", Addr: NetAddr{Addr: "localhost", AddrNetwork: "tcp"}}},
		},
		{
			in:  "alice@domain.com@[::1]:7777, bob@localhost@localhost",
			out: []JumpHost{{Username: "alice@domain.com", Addr: NetAddr{Addr: "[::1]:7777", AddrNetwork: "tcp"}}, {Username: "bob@localhost", Addr: NetAddr{Addr: "localhost", AddrNetwork: "tcp"}}},
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
