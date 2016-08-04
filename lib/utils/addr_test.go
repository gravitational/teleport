/*
Copyright 2015 Gravitational, Inc.

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
	"net"
	"strings"
	"testing"

	. "gopkg.in/check.v1"
	"gopkg.in/yaml.v2"
)

func TestAddrStruct(t *testing.T) { TestingT(t) }

type AddrTestSuite struct {
}

var _ = Suite(&AddrTestSuite{})

func (s *AddrTestSuite) TestParseHostPort(c *C) {
	// success
	addr, err := ParseHostPortAddr("localhost:22", -1)
	c.Assert(err, IsNil)
	c.Assert(addr.AddrNetwork, Equals, "tcp")
	c.Assert(addr.Addr, Equals, "localhost:22")

	// success
	addr, err = ParseHostPortAddr("localhost", 1111)
	c.Assert(err, IsNil)
	c.Assert(addr.AddrNetwork, Equals, "tcp")
	c.Assert(addr.Addr, Equals, "localhost:1111")

	// missing port
	addr, err = ParseHostPortAddr("localhost", -1)
	c.Assert(err, NotNil)
	c.Assert(addr, IsNil)
}

func (s *AddrTestSuite) TestEmpty(c *C) {
	var a NetAddr
	c.Assert(a.IsEmpty(), Equals, true)
}

func (s *AddrTestSuite) TestParse(c *C) {
	addr, err := ParseAddr("tcp://one:25/path")
	c.Assert(err, IsNil)
	c.Assert(addr, NotNil)
	c.Assert(addr.Addr, Equals, "one:25")
	c.Assert(addr.Path, Equals, "/path")
	c.Assert(addr.FullAddress(), Equals, "tcp://one:25")
	c.Assert(addr.IsEmpty(), Equals, false)
}

func (s *AddrTestSuite) TestParseDefaults(c *C) {
	addr, err := ParseAddr("host:25")
	c.Assert(err, IsNil)
	c.Assert(addr, NotNil)
	c.Assert(addr.Addr, Equals, "host:25")
	c.Assert(addr.FullAddress(), Equals, "tcp://host:25")
	c.Assert(addr.IsEmpty(), Equals, false)
}

func (s *AddrTestSuite) TestReplaceLocalhost(c *C) {
	var result string
	result = ReplaceLocalhost("10.10.1.1", "192.168.1.100:399")
	c.Assert(result, Equals, "10.10.1.1")
	result = ReplaceLocalhost("10.10.1.1:22", "192.168.1.100:399")
	c.Assert(result, Equals, "10.10.1.1:22")
	result = ReplaceLocalhost("127.0.0.1:22", "192.168.1.100:399")
	c.Assert(result, Equals, "192.168.1.100:22")
	result = ReplaceLocalhost("0.0.0.0:22", "192.168.1.100:399")
	c.Assert(result, Equals, "192.168.1.100:22")
}

func (s *AddrTestSuite) TestLocalAddrs(c *C) {
	testCases := []struct {
		in       string
		expected bool
	}{
		{in: "127.0.0.1:5000", expected: true},
		{in: "localhost:5000", expected: true},
		{in: "127.0.0.2:5000", expected: true},
		{in: "tcp://127.0.0.2:5000", expected: true},
		{in: "tcp://10.0.0.3:5000", expected: false},
		{in: "tcp://hostname:5000", expected: false},
	}
	for i, testCase := range testCases {
		addr, err := ParseAddr(testCase.in)
		c.Assert(err, IsNil)
		c.Assert(addr.IsLocal(), Equals, testCase.expected,
			Commentf("test case %v, %v should be local(%v)", i, testCase.in, testCase.expected))
	}
}

func (s *AddrTestSuite) TestLoopbackAddrs(c *C) {
	testCases := []struct {
		in       string
		expected bool
	}{
		{in: "localhost", expected: true},
		{in: "localhost:5000", expected: true},
		{in: "127.0.0.2:4003", expected: true},
		{in: "", expected: false},
		{in: "bad-host.example.com", expected: false},
		{in: "bad-host.example.com:443", expected: false},
	}
	for i, testCase := range testCases {
		c.Assert(IsLoopback(testCase.in), Equals, testCase.expected,
			Commentf("test case %v, %v should be loopback(%v)", i, testCase.in, testCase.expected))
	}
}

func (s *AddrTestSuite) TestGuessesIPAddress(c *C) {
	var testCases = []struct {
		addrs    []net.Addr
		expected net.IP
		comment  string
	}{
		{
			addrs: []net.Addr{
				&net.IPAddr{IP: net.ParseIP("10.0.100.80")},
				&net.IPAddr{IP: net.ParseIP("192.13.1.80")},
				&net.IPAddr{IP: net.ParseIP("172.192.12.1")},
			},
			expected: net.ParseIP("10.0.100.80"),
			comment:  "prefers 10.x.y.z",
		},
		{
			addrs: []net.Addr{
				&net.IPAddr{IP: net.ParseIP("192.13.1.80")},
				&net.IPAddr{IP: net.ParseIP("172.192.12.1")},
			},
			expected: net.ParseIP("192.13.1.80"),
			comment:  "prefers 192.x.y.z",
		},
		{
			addrs: []net.Addr{
				&net.IPAddr{IP: net.ParseIP("172.192.12.1")},
				&net.IPAddr{IP: net.ParseIP("52.35.21.180")},
			},
			expected: net.ParseIP("172.192.12.1"),
			comment:  "prefers 172.x.y.z",
		},
		{
			addrs: []net.Addr{
				&net.IPAddr{IP: net.ParseIP("192.192.12.1")},
				&net.IPAddr{IP: net.ParseIP("192.192.12.2")},
				&net.IPAddr{IP: net.ParseIP("52.35.21.180")},
			},
			expected: net.ParseIP("192.192.12.2"),
			comment:  "prefers last",
		},
		{
			addrs: []net.Addr{
				&net.IPAddr{IP: net.ParseIP("52.35.21.180")},
				&net.IPAddr{IP: net.ParseIP("fe80::af:6dff:fefd:150f")},
			},
			expected: net.ParseIP("52.35.21.180"),
			comment:  "ignores IPv6",
		},
		{
			addrs: []net.Addr{
				&net.IPAddr{IP: net.ParseIP("fe80::af:6dff:fefd:150f")},
			},
			expected: net.ParseIP("127.0.0.1"),
			comment:  "falls back to loopback",
		},
	}
	for _, testCase := range testCases {
		ip := guessHostIP(testCase.addrs)
		c.Assert(ip, DeepEquals, testCase.expected, Commentf(testCase.comment))
	}
}

func (s *AddrTestSuite) TestMarshal(c *C) {
	testCases := []struct {
		in       *NetAddr
		expected string
	}{
		{in: &NetAddr{Addr: "localhost:5000"}, expected: "localhost:5000"},
		{in: &NetAddr{AddrNetwork: "tcp", Addr: "localhost:5000"}, expected: "tcp://localhost:5000"},
		{in: &NetAddr{AddrNetwork: "tcp", Addr: "localhost:5000", Path: "/path"}, expected: "tcp://localhost:5000/path"},
		{in: &NetAddr{AddrNetwork: "unix", Path: "/path"}, expected: "unix:///path"},
	}

	for i, testCase := range testCases {
		bytes, err := yaml.Marshal(testCase.in)
		c.Assert(err, IsNil)
		c.Assert(strings.TrimSpace(string(bytes)), Equals, testCase.expected,
			Commentf("test case %v, %v should be marshalled to: %v", i, testCase.in, testCase.expected))
	}
}

func (s *AddrTestSuite) TestUnmarshal(c *C) {
	testCases := []struct {
		in       string
		expected *NetAddr
	}{
		{in: "localhost:5000", expected: &NetAddr{AddrNetwork: "tcp", Addr: "localhost:5000"}},
		{in: "tcp://localhost:5000/path", expected: &NetAddr{AddrNetwork: "tcp", Addr: "localhost:5000", Path: "/path"}},
		{in: "unix:///path", expected: &NetAddr{AddrNetwork: "unix", Addr: "/path"}},
	}

	for i, testCase := range testCases {
		addr := &NetAddr{}
		err := yaml.Unmarshal([]byte(testCase.in), addr)
		c.Assert(err, IsNil)
		c.Assert(addr, DeepEquals, testCase.expected,
			Commentf("test case %v, %v should be unmarshalled to: %v", i, testCase.in, testCase.expected))
	}
}
