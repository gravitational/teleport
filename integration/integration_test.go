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
	"fmt"
	"io"
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

	AllocatePortsNum = 20
)

type IntSuite struct {
	ports utils.PortList
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
	var err error
	utils.InitLoggerForTests()
	SetTestTimeouts(100)

	// find 10 free litening ports to use
	s.ports, err = utils.GetFreeTCPPorts(AllocatePortsNum)
	if err != nil {
		c.Fatal(err)
	}
	s.me, _ = user.Current()

	// close & re-open stdin because 'go test' runs with os.stdin connected to /dev/null
	stdin, err := os.Open("/dev/tty")
	if err != nil {
		os.Stdin.Close()
		os.Stdin = stdin
	}
}

// newTeleport helper returns a running Teleport instance pre-configured
// with the current user os.user.Current()
func (s *IntSuite) newTeleport(c *check.C, enableSSH bool) *TeleInstance {
	username := s.me.Username
	t := NewInstance(Site, Host, s.getPorts(5), s.priv, s.pub)
	t.AddUser(username, []string{username})
	if t.Create(nil, enableSSH, nil) != nil {
		c.FailNow()
	}
	if t.Start() != nil {
		c.FailNow()
	}
	return t
}

// TestInteractive covers SSH into shell and joining the same session from another client
func (s *IntSuite) TestInteractive(c *check.C) {
	t := s.newTeleport(c, true)
	defer t.Stop()

	sessionEndC := make(chan interface{}, 0)

	// get a reference to site obj:
	siteTunnel, _ := t.Tunnel.GetSite(Site)
	site, _ := siteTunnel.GetClient()
	c.Assert(site, check.NotNil)

	personA := NewTerminal(250)
	personB := NewTerminal(250)

	// PersonB: wait for a session to become available, then join:
	joinSession := func() {
		var sessionID string
		for {
			time.Sleep(time.Millisecond * 5)
			sessions, _ := site.GetSessions()
			if len(sessions) == 0 {
				continue
			}
			sessionID = string(sessions[0].ID)
			break
		}
		cl, err := t.NewClient(s.me.Username, Site, Host, t.GetPortSSHInt())
		c.Assert(err, check.IsNil)
		cl.Output = &personB
		for i := 0; i < 10; i++ {
			time.Sleep(time.Millisecond * 5)
			err = cl.Join(sessionID, &personB)
			if err == nil {
				break
			}
		}
		c.Assert(err, check.IsNil)
	}

	// PersonA: SSH into the server, wait one second, then type some commands on stdin:
	openSession := func() {
		cl, err := t.NewClient(s.me.Username, Site, Host, t.GetPortSSHInt())
		c.Assert(err, check.IsNil)
		cl.Output = &personA
		// Person A types something into the terminal (including "exit")
		personA.Type("\acd /\n\rls -l\n\recho $(( 1+2 ))\n\rexit\n\r")
		err = cl.SSH([]string{}, false, &personA)
		c.Assert(err, check.IsNil)
		sessionEndC <- true
	}

	go openSession()
	go joinSession()

	// wait for the session to end
	waitFor(sessionEndC, time.Second*10)

	// make sure both parites saw the same output:
	c.Assert(personA.Output(30), check.DeepEquals, personB.Output(30))
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
	err = tc.SSH(cmd, false, nil)
	c.Assert(err, check.ErrorMatches, "site wrong-site not found")
}

// TestTwoSites creates two teleport sites: "a" and "b" and
// creates a tunnel from A to B.
//
// Then it executes an SSH command on A by connecting directly
// to A and by connecting to B via B<->A tunnel
func (s *IntSuite) TestTwoSites(c *check.C) {
	username := s.me.Username

	a := NewInstance("site-A", Host, s.getPorts(5), s.priv, s.pub)
	b := NewInstance("site-B", Host, s.getPorts(5), s.priv, s.pub)

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
	tc.Output = &outputA
	c.Assert(err, check.IsNil)
	err = tc.SSH(cmd, false, nil)
	c.Assert(err, check.IsNil)
	c.Assert(outputA.String(), check.Equals, "hello world\n")

	// via tunnel b->a:
	tc, err = b.NewClient(username, "site-A", Host, sshPort)
	tc.Output = &outputB
	c.Assert(err, check.IsNil)
	err = tc.SSH(cmd, false, nil)
	c.Assert(err, check.IsNil)
	c.Assert(outputA, check.DeepEquals, outputB)

	c.Assert(b.Stop(), check.IsNil)
	c.Assert(a.Stop(), check.IsNil)
}

// getPorts helper returns a range of unallocated ports available for litening on
func (s *IntSuite) getPorts(num int) []int {
	if len(s.ports) < num {
		panic("do not have enough ports! increase AllocatePortsNum constant")
	}
	ports := make([]int, num)
	for i := range ports {
		p, _ := strconv.Atoi(s.ports.Pop())
		ports[i] = p
	}
	return ports
}

// Terminal emulates stdin+stdout for integration testing
type Terminal struct {
	io.Writer
	io.Reader

	written *bytes.Buffer
	typed   chan byte
}

func NewTerminal(capacity int) Terminal {
	return Terminal{
		typed:   make(chan byte, capacity),
		written: bytes.NewBuffer([]byte{}),
	}
}

func (t *Terminal) Type(data string) {
	for _, b := range []byte(data) {
		t.typed <- b
	}
}

// Output returns a number of first num bytes printed into this fake terminal
func (t *Terminal) Output(num int) []byte {
	w := t.written.Bytes()
	if len(w) > num {
		return w[:num]
	}
	return w
}

func (t *Terminal) Write(data []byte) (n int, err error) {
	return t.written.Write(data)
}

func (t *Terminal) Read(p []byte) (n int, err error) {
	for n = 0; n < len(p); n++ {
		p[n] = <-t.typed
		if p[n] == '\r' {
			break
		}
		if p[n] == '\a' { // 'alert' used for debugging, means 'pause for 1 second'
			time.Sleep(time.Second)
			n -= 1
		}
		time.Sleep(time.Millisecond * 10)
	}
	return n, nil
}

// waitFor helper waits on a challen for up to the given timeout
func waitFor(c chan interface{}, timeout time.Duration) error {
	tick := time.Tick(timeout)
	select {
	case <-c:
		return nil
	case <-tick:
		return fmt.Errorf("timeout waiting for event")
	}
}
