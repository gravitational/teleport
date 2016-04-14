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
	"bytes"
	"os"
	"os/user"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

const (
	Host = "127.0.0.1"
	Site = "local-site"
)

type IntSuite struct {
	ports []int
	me    *user.User
	// priv/pub pair to avoid re-generating it
	priv []byte
	pub  []byte
}

// bootstrap check
func TestIntegrations(t *testing.T) { check.TestingT(t) }

var _ = check.Suite(&IntSuite{
	priv: []byte(testauthority.Priv),
	pub:  []byte(testauthority.Pub),
})

func (s *IntSuite) TearDownSuite(c *check.C) {
	var err error
	// restore os.Stdin to its original condition: connected to /dev/null
	os.Stdin.Close()
	os.Stdin, err = os.Open("/dev/null")
	c.Assert(err, check.IsNil)
}

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

	// close & re-open stdin because 'go test' runs with os.stdin connected to /dev/null
	os.Stdin.Close()
	os.Stdin, err = os.Open("/dev/tty")
	c.Assert(err, check.IsNil)
}

// newTeleport helper returns a running Teleport instance pre-configured
// with the current user os.user.Current()
func (s *IntSuite) newTeleport(c *check.C, enableSSH bool) *TeleInstance {
	username := s.me.Username
	t := NewInstance(Site, Host, s.ports[5:], s.priv, s.pub)
	t.AddUser(username, []string{username})
	if t.Create(nil, enableSSH, nil) != nil {
		c.FailNow()
	}
	if t.Start() != nil {
		c.FailNow()
	}
	return t
}

func (s *IntSuite) _TestSessionViewing(c *check.C) {
	t := s.newTeleport(c, true)
	time.Sleep(time.Second)

	defer t.Stop()

}

// TestInvalidLogins validates that you can't login with invalid login or
// with invalid 'site' parameter
func (s *IntSuite) TestInvalidLogins(c *check.C) {
	t := s.newTeleport(c, true)
	defer t.Stop()

	cmd := []string{"echo", "success"}

	// try the wrong site:
	tc, err := t.NewClient(s.me.Username, "wrong-site", Host, t.GetPortSSHInt())
	c.Assert(err, check.IsNil)
	err = tc.SSH(cmd, false, nil, nil)
	c.Assert(err, check.ErrorMatches, "site wrong-site not found")
}

// TestTwoSites creates two teleport sites: "a" and "b" and
// creates a tunnel from A to B.
//
// Then it executes an SSH command on A by connecting directly
// to A and by connecting to B via B<->A tunnel
func (s *IntSuite) TestTwoSites(c *check.C) {
	username := s.me.Username

	a := NewInstance("site-A", Host, s.ports[:5], s.priv, s.pub)
	b := NewInstance("site-B", Host, s.ports[5:], s.priv, s.pub)

	a.AddUser(username, []string{username})
	b.AddUser(username, []string{username})

	c.Assert(b.Create(a.Secrets.AsSlice(), false, nil), check.IsNil)
	c.Assert(a.Create(b.Secrets.AsSlice(), true, nil), check.IsNil)

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

	var (
		outputA bytes.Buffer
		outputB bytes.Buffer
	)

	// if we got here, it means two sites are cross-connected. lets execute SSH commands
	sshPort := a.GetPortSSHInt()
	cmd := []string{"echo", "hello world"}

	// directly:
	tc, err := a.NewClient(username, "site-A", Host, sshPort)
	c.Assert(err, check.IsNil)
	tc.Output = &outputA
	err = tc.SSH(cmd, false, nil, nil)
	c.Assert(err, check.IsNil)
	c.Assert(outputA.String(), check.Equals, "hello world\n")

	// via tunnel b->a:
	tc, err = b.NewClient(username, "site-A", Host, sshPort)
	c.Assert(err, check.IsNil)
	tc.Output = &outputB
	err = tc.SSH(cmd, false, nil, nil)
	c.Assert(err, check.IsNil)
	c.Assert(outputA, check.DeepEquals, outputB)

	c.Assert(b.Stop(), check.IsNil)
	c.Assert(a.Stop(), check.IsNil)
}
