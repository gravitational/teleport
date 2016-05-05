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
	"io/ioutil"
	"os"
	"os/user"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"gopkg.in/check.v1"
)

const (
	Host = "127.0.0.1"
	Site = "local-site"

	AllocatePortsNum = 100
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
func (s *IntSuite) newTeleport(c *check.C, logins []string, enableSSH bool) *TeleInstance {
	t := NewInstance(Site, Host, s.getPorts(5), s.priv, s.pub)
	// use passed logins, but use suite's default login if nothing was passed
	if logins == nil || len(logins) == 0 {
		logins = []string{s.me.Username}
	}
	for _, login := range logins {
		t.AddUser(login, []string{login})
	}

	if t.Create(nil, enableSSH, nil) != nil {
		c.FailNow()
	}
	if t.Start() != nil {
		c.FailNow()
	}
	return t
}

// TestAudit creates a live session, records a bunch of data through it (>5MB) and
// then reads it back and compares against simulated reality
func (s *IntSuite) TestAudit(c *check.C) {
	var err error

	// create server and get the reference to the site API:
	t := s.newTeleport(c, nil, true)
	site := t.GetSiteAPI(Site)
	c.Assert(site, check.NotNil)
	defer t.Stop()

	// should have no sessions:
	sessions, _ := site.GetSessions()
	c.Assert(len(sessions), check.Equals, 0)

	// create interactive session (this goroutine is this user's terminal time)
	endC := make(chan error, 0)
	myTerm := NewTerminal(250)
	go func() {
		cl, err := t.NewClient(s.me.Username, Site, Host, t.GetPortSSHInt())
		c.Assert(err, check.IsNil)
		cl.Output = &myTerm

		endC <- cl.SSH([]string{}, false, &myTerm)
	}()

	// wait until there's a session in there:
	for len(sessions) == 0 {
		time.Sleep(time.Millisecond * 5)
		sessions, _ = site.GetSessions()
	}
	session := &sessions[0]
	// wait for the user to join this session:
	for len(session.Parties) == 0 {
		time.Sleep(time.Millisecond * 5)
		session, err = site.GetSession(sessions[0].ID)
		c.Assert(err, check.IsNil)
	}
	// make sure it's us who joined! :)
	c.Assert(session.Parties[0].User, check.Equals, s.me.Username)

	// lets type "echo hi" followed by "enter" and then "exit" + "enter":
	myTerm.Type("\aecho hi\n\r\aexit\n\r\a")

	// wait for session to end:
	<-endC

	// using 'session writer' lets add something to the session streaam:
	w, err := site.GetSessionWriter(session.ID)
	c.Assert(err, check.IsNil)
	// write 32Kb chunk
	bigChunk := make([]byte, 1024*32)
	n, err := w.Write(bigChunk)
	c.Assert(err, check.Equals, nil)
	c.Assert(n, check.Equals, len(bigChunk))
	// then add small prefix:
	w.Write([]byte("\nsuffix"))
	w.Close()

	// read back the entire session:
	r, err := site.GetSessionReader(session.ID, 0)
	c.Assert(err, check.IsNil)
	sessionStream, err := ioutil.ReadAll(r)
	c.Assert(err, check.IsNil)
	c.Assert(len(sessionStream) > len(bigChunk), check.Equals, true)
	r.Close()

	// see what we got. It looks different based on bash settings, but here it is
	// on Ev's machine (hostname is 'edsger'):
	//
	// edsger ~: echo hi
	// hi
	// edsger ~: exit
	// logout
	// <5MB of zeros here>
	// suffix
	//
	c.Assert(strings.Contains(string(sessionStream), "echo hi"), check.Equals, true)
	c.Assert(strings.HasSuffix(string(sessionStream), "\nsuffix"), check.Equals, true)

	// now lets look at session events:
	history, err := site.GetSessionEvents(session.ID, 0)
	c.Assert(err, check.IsNil)
	first := history[0]
	beforeLast := history[len(history)-2]
	last := history[len(history)-1]

	getChunk := func(e events.EventFields) string {
		offset := e.GetInt("offset")
		length := e.GetInt("bytes")
		if length == 0 {
			return ""
		}
		c.Assert(offset+length <= len(sessionStream), check.Equals, true)
		return string(sessionStream[offset : offset+length])
	}

	// last two are manually-typed (32Kb chunk and "suffix"):
	c.Assert(last.GetString(events.EventType), check.Equals, "print")
	c.Assert(beforeLast.GetString(events.EventType), check.Equals, "print")
	c.Assert(last.GetInt("bytes"), check.Equals, len("\nsuffix"))
	c.Assert(beforeLast.GetInt("bytes"), check.Equals, len(bigChunk))

	// 10th chunk should be printed "hi":
	c.Assert(strings.HasPrefix(getChunk(history[10]), "hi"), check.Equals, true)

	// 1st should be "session.start"
	c.Assert(first.GetString(events.EventType), check.Equals, events.SessionStartEvent)

	// last-3 should be "session.end", and the one before - "session.leave"
	endEvent := history[len(history)-3]
	leaveEvent := history[len(history)-4]
	c.Assert(endEvent.GetString(events.EventType), check.Equals, events.SessionEndEvent)
	c.Assert(leaveEvent.GetString(events.EventType), check.Equals, events.SessionLeaveEvent)

	// session events should have session ID assigned
	c.Assert(first.GetString(events.SessionEventID) != "", check.Equals, true)
	c.Assert(endEvent.GetString(events.SessionEventID) != "", check.Equals, true)
	c.Assert(leaveEvent.GetString(events.SessionEventID) != "", check.Equals, true)

	// all of them should have a proper time:
	for _, e := range history {
		c.Assert(e.GetTime("time").IsZero(), check.Equals, false)
	}
}

// TestInteractive covers SSH into shell and joining the same session from another client
func (s *IntSuite) TestInteractive(c *check.C) {
	t := s.newTeleport(c, nil, true)
	defer t.Stop()

	sessionEndC := make(chan interface{}, 0)

	// get a reference to site obj:
	site := t.GetSiteAPI(Site)
	c.Assert(site, check.NotNil)

	personA := NewTerminal(250)
	personB := NewTerminal(250)

	// PersonA: SSH into the server, wait one second, then type some commands on stdin:
	openSession := func() {
		cl, err := t.NewClient(s.me.Username, Site, Host, t.GetPortSSHInt())
		c.Assert(err, check.IsNil)
		cl.Output = &personA
		// Person A types something into the terminal (including "exit")
		personA.Type("\aecho hi\n\r\aexit\n\r\a")
		err = cl.SSH([]string{}, false, &personA)
		c.Assert(err, check.IsNil)
		sessionEndC <- true
	}

	// PersonB: wait for a session to become available, then join:
	joinSession := func() {
		var sessionID string
		for {
			time.Sleep(time.Millisecond)
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
			err = cl.Join(session.ID(sessionID), &personB)
			if err == nil {
				break
			}
		}
		c.Assert(err, check.IsNil)
	}

	go openSession()
	go joinSession()

	// wait for the session to end
	waitFor(sessionEndC, time.Second*10)

	// make sure both parites saw the same output:
	c.Assert(personA.Output(100)[50:], check.DeepEquals, personB.Output(100)[50:])
}

// TestInvalidLogins validates that you can't login with invalid login or
// with invalid 'site' parameter
func (s *IntSuite) TestInvalidLogins(c *check.C) {
	t := s.newTeleport(c, nil, true)
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
