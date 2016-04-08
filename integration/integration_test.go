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
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

type IntSuite struct{}

// bootstrap check
func TestIntegrations(t *testing.T) { check.TestingT(t) }

var _ = check.Suite(&IntSuite{})

func (s *IntSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
	SetTestTimeouts(10)
}

func (s *IntSuite) TestEverything(c *check.C) {
	cl := NewInstance("client", 5000)
	sr := NewInstance("server", 6000)

	c.Assert(sr.Create(cl.Secrets.AsSlice(), false), check.IsNil)
	c.Assert(cl.Create(sr.Secrets.AsSlice(), true), check.IsNil)

	c.Assert(sr.Start(), check.IsNil)
	c.Assert(cl.Start(), check.IsNil)

	time.Sleep(time.Second * 3)

	cl.SSH([]string{"/bin/ls", "-l", "/"}, "127.0.0.1", 5000)
	sr.SSH([]string{"/bin/ls", "-l", "/"}, "127.0.0.1", 5000)

	c.Assert(cl.Stop(), check.IsNil)
	c.Assert(sr.Stop(), check.IsNil)
	cl = nil
	sr = nil
}
