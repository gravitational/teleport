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
	"os/user"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

type IntSuite struct {
	ports []int
	me    *user.User
}

// bootstrap check
func TestIntegrations(t *testing.T) { check.TestingT(t) }

var _ = check.Suite(&IntSuite{})

func (s *IntSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()
	SetTestTimeouts(100)

	// find 10 free litening ports to use
	fp, err := utils.GetFreeTCPPorts(10)
	if err != nil {
		c.Fatal(err)
	}
	for _, port := range fp {
		p, _ := strconv.Atoi(port)
		s.ports = append(s.ports, p)
	}
	s.me, _ = user.Current()
}

// TestTwoSites creates two teleport sites: "a" and "b" and
// creates a tunnel from A to B.
//
// Then it executes an SSH command on A by connecting directly
// to A and by connecting to B via B<->A tunnel
func (s *IntSuite) TestEverything(c *check.C) {
	username := s.me.Username
	priv := []byte(testauthority.Priv)
	pub := []byte(testauthority.Pub)

	a := NewInstance("site-A", "127.0.0.1", s.ports[:5], priv, pub)
	b := NewInstance("site-B", "127.0.0.1", s.ports[5:], priv, pub)

	a.AddUser(username, []string{username})
	b.AddUser(username, []string{username})

	c.Assert(b.Create(a.Secrets.AsSlice(), false), check.IsNil)
	c.Assert(a.Create(b.Secrets.AsSlice(), true), check.IsNil)

	c.Assert(b.Start(), check.IsNil)
	c.Assert(a.Start(), check.IsNil)

	// wait for both sites to see each other via their reverse tunnels (for up to 10 seconds)
	abortTime := time.Now().Add(time.Second * 10)
	for len(b.Tunnel.GetSites()) < 2 && len(b.Tunnel.GetSites()) < 2 {
		time.Sleep(time.Millisecond * 200)
		if time.Now().After(abortTime) {
			c.Fatalf("two sites do not see each other: tunnels are not working")
		}
	}

	// if we got here, it means two sites are cross-connected. lets execute SSH commands
	sshPort := a.GetPortSSHInt()
	cmd := []string{"echo", "hello world"}

	// directly:
	outputA, err := a.SSH(username, cmd, "site-A", "127.0.0.1", sshPort)
	c.Assert(err, check.IsNil)
	c.Assert(string(outputA), check.Equals, "hello world\n")
	// via tunnel b->a:
	outputB, err := b.SSH(username, cmd, "site-A", "127.0.0.1", sshPort)
	c.Assert(err, check.IsNil)
	c.Assert(outputA, check.DeepEquals, outputB)

	c.Assert(b.Stop(), check.IsNil)
	c.Assert(a.Stop(), check.IsNil)
}
