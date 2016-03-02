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
	. "gopkg.in/check.v1"
	"testing"
)

func TestAddrSturct(t *testing.T) { TestingT(t) }

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
