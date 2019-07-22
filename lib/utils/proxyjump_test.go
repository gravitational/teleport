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
	"gopkg.in/check.v1"
)

func (s *UtilsSuite) TestProxyJumpParsing(c *check.C) {
	type tc struct {
		in  string
		out []JumpHost
		err error
	}
	testCases := []tc{
		{
			in:  "host:port",
			out: []JumpHost{{Addr: NetAddr{Addr: "host:port", AddrNetwork: "tcp"}}},
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
	}
	for i, tc := range testCases {
		comment := check.Commentf("Test case %v: %q", i, tc.in)
		re, err := ParseProxyJump(tc.in)
		if tc.err == nil {
			c.Assert(err, check.IsNil, comment)
			c.Assert(re, check.DeepEquals, tc.out)
		} else {
			c.Assert(err, check.FitsTypeOf, tc.err)
		}
	}
}
