/*
Copyright 2016 Gravitational, Inc.

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

package integration

import (
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

type IntSuite struct {
	portA int
	portB int
}

// bootstrap check
func TestIntegrations(t *testing.T) { check.TestingT(t) }

var _ = check.Suite(&IntSuite{})

func (s *IntSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
	SetTestTimeouts(10)

	// find 10 free litening ports to use
	fp, err := utils.GetFreeTCPPorts(10)
	if err != nil {
		c.Fatal(err)
	}
	s.portA, _ = strconv.Atoi(fp[0])
	s.portB, _ = strconv.Atoi(fp[5])
}

// TestTwoSites creates two teleport sites: "a" and "b" and
// creates a tunnel from A to B.
//
// Then it executes an SSH command on A by connecting directly
// to A and by connecting to B via B<->A tunnel
func (s *IntSuite) TestEverything(c *check.C) {
	a := NewInstance("site-A", s.portA)
	b := NewInstance("site-B", s.portB)

	c.Assert(b.Create(a.Secrets.AsSlice(), false), check.IsNil)
	c.Assert(a.Create(b.Secrets.AsSlice(), true), check.IsNil)

	c.Assert(b.Start(), check.IsNil)
	c.Assert(a.Start(), check.IsNil)

	time.Sleep(time.Second * 2)

	// directly:
	outputA, err := a.SSH([]string{"echo", "hello world"}, "site-A", "127.0.0.1", s.portA)
	c.Assert(err, check.IsNil)
	c.Assert(string(outputA), check.Equals, "hello world\n")
	// via tunnel b->a:
	outputB, err := b.SSH([]string{"echo", "hello world"}, "site-A", "127.0.0.1", s.portA)
	c.Assert(err, check.IsNil)
	c.Assert(outputA, check.DeepEquals, outputB)

	c.Assert(b.Stop(), check.IsNil)
	c.Assert(a.Stop(), check.IsNil)
}
