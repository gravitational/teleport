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
	"context"
	"fmt"
	"io"
	"os"
	"os/user"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"gopkg.in/check.v1"
)

const (
	Host   = "localhost"
	HostID = "00000000-0000-0000-0000-000000000000"
	Site   = "local-site"

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

var _ = check.Suite(&IntSuite{})

func (s *IntSuite) TearDownSuite(c *check.C) {
	var err error
	// restore os.Stdin to its original condition: connected to /dev/null
	os.Stdin.Close()
	os.Stdin, err = os.Open("/dev/null")
	c.Assert(err, check.IsNil)
}

func (s *IntSuite) SetUpTest(c *check.C) {
	os.RemoveAll(client.FullProfilePath(""))
}

func (s *IntSuite) SetUpSuite(c *check.C) {
	var err error
	utils.InitLoggerForTests()
	SetTestTimeouts(time.Millisecond * time.Duration(100))

	s.priv, s.pub, err = testauthority.New().GenerateKeyPair("")
	c.Assert(err, check.IsNil)

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
	t := NewInstance(Site, HostID, Host, s.getPorts(5), s.priv, s.pub)
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
	defer t.Stop(true)

	// should have no sessions:
	sessions, _ := site.GetSessions(defaults.Namespace)
	c.Assert(len(sessions), check.Equals, 0)

	// create interactive session (this goroutine is this user's terminal time)
	endC := make(chan error, 0)
	myTerm := NewTerminal(250)
	go func() {
		cl, err := t.NewClient(s.me.Username, Site, Host, t.GetPortSSHInt())
		c.Assert(err, check.IsNil)
		cl.Stdout = &myTerm
		cl.Stdin = &myTerm
		err = cl.SSH(context.TODO(), []string{}, false)
		endC <- err
	}()

	// wait until there's a session in there:
	for i := 0; len(sessions) == 0; i++ {
		time.Sleep(time.Millisecond * 20)
		sessions, _ = site.GetSessions(defaults.Namespace)
		if i > 100 {
			// waited too long
			c.FailNow()
			return
		}
	}
	session := &sessions[0]

	// wait for the user to join this session:
	for len(session.Parties) == 0 {
		time.Sleep(time.Millisecond * 5)
		session, err = site.GetSession(defaults.Namespace, sessions[0].ID)
		c.Assert(err, check.IsNil)
	}
	// make sure it's us who joined! :)
	c.Assert(session.Parties[0].User, check.Equals, s.me.Username)

	// lets add something to the session stream:
	// write 1MB chunk
	bigChunk := make([]byte, 1024*1024)
	err = site.PostSessionChunk(defaults.Namespace, session.ID, bytes.NewReader(bigChunk))
	c.Assert(err, check.Equals, nil)

	// then add small prefix:
	err = site.PostSessionChunk(defaults.Namespace, session.ID, bytes.NewBufferString("\nsuffix"))
	c.Assert(err, check.Equals, nil)

	// lets type "echo hi" followed by "enter" and then "exit" + "enter":
	myTerm.Type("\aecho hi\n\r\aexit\n\r\a")

	// wait for session to end:
	<-endC

	// read back the entire session (we have to try several times until we get back
	// everything because the session is closing)
	const expectedLen = 1048600
	var sessionStream []byte
	for i := 0; len(sessionStream) < expectedLen; i++ {
		sessionStream, err = site.GetSessionChunk(defaults.Namespace, session.ID, 0, events.MaxChunkBytes)
		c.Assert(err, check.IsNil)
		time.Sleep(time.Millisecond * 250)
		if i > 10 {
			// session stream keeps coming back short
			c.Fatalf("stream is too short: <%d", expectedLen)
		}
	}

	// see what we got. It looks different based on bash settings, but here it is
	// on Ev's machine (hostname is 'edsger'):
	//
	// edsger ~: echo hi
	// hi
	// edsger ~: exit
	// logout
	// <1MB of zeros here>
	// suffix
	//
	c.Assert(strings.Contains(string(sessionStream), "echo hi"), check.Equals, true)
	c.Assert(strings.Contains(string(sessionStream), "\nsuffix"), check.Equals, true)

	// now lets look at session events:
	history, err := site.GetSessionEvents(defaults.Namespace, session.ID, 0)
	c.Assert(err, check.IsNil)

	getChunk := func(e events.EventFields, maxlen int) string {
		offset := e.GetInt("offset")
		length := e.GetInt("bytes")
		if length == 0 {
			return ""
		}
		if length > maxlen {
			length = maxlen
		}
		return string(sessionStream[offset : offset+length])
	}

	findByType := func(et string) events.EventFields {
		for _, e := range history {
			if e.GetType() == et {
				return e
			}
		}
		return nil
	}

	// there should alwys be 'session.start' event (and it must be first)
	first := history[0]
	start := findByType(events.SessionStartEvent)
	c.Assert(start, check.DeepEquals, first)
	c.Assert(start.GetInt("bytes"), check.Equals, 0)
	c.Assert(start.GetString(events.SessionEventID) != "", check.Equals, true)
	c.Assert(start.GetString(events.TerminalSize) != "", check.Equals, true)

	// find "\nsuffix" write and find our huge 1MB chunk
	prefixFound, hugeChunkFound := false, false
	for _, e := range history {
		if getChunk(e, 10) == "\nsuffix" {
			prefixFound = true
		}
		if e.GetInt("bytes") == 1048576 {
			hugeChunkFound = true
		}
	}
	c.Assert(prefixFound, check.Equals, true)
	c.Assert(hugeChunkFound, check.Equals, true)

	// there should alwys be 'session.end' event
	end := findByType(events.SessionEndEvent)
	c.Assert(end, check.NotNil)
	c.Assert(end.GetInt("bytes"), check.Equals, 0)
	c.Assert(end.GetString(events.SessionEventID) != "", check.Equals, true)

	// there should alwys be 'session.leave' event
	leave := findByType(events.SessionLeaveEvent)
	c.Assert(leave, check.NotNil)
	c.Assert(leave.GetInt("bytes"), check.Equals, 0)
	c.Assert(leave.GetString(events.SessionEventID) != "", check.Equals, true)

	// all of them should have a proper time:
	for _, e := range history {
		c.Assert(e.GetTime("time").IsZero(), check.Equals, false)
	}
}

// TestInteractive covers SSH into shell and joining the same session from another client
func (s *IntSuite) TestInteractive(c *check.C) {
	t := s.newTeleport(c, nil, true)
	defer t.Stop(true)

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
		cl.Stdout = &personA
		cl.Stdin = &personA
		// Person A types something into the terminal (including "exit")
		personA.Type("\aecho hi\n\r\aexit\n\r\a")
		err = cl.SSH(context.TODO(), []string{}, false)
		c.Assert(err, check.IsNil)
		sessionEndC <- true
	}

	// PersonB: wait for a session to become available, then join:
	joinSession := func() {
		var sessionID string
		for {
			time.Sleep(time.Millisecond)
			sessions, _ := site.GetSessions(defaults.Namespace)
			if len(sessions) == 0 {
				continue
			}
			sessionID = string(sessions[0].ID)
			break
		}
		cl, err := t.NewClient(s.me.Username, Site, Host, t.GetPortSSHInt())
		c.Assert(err, check.IsNil)
		cl.Stdout = &personB
		for i := 0; i < 10; i++ {
			err = cl.Join(context.TODO(), defaults.Namespace, session.ID(sessionID), &personB)
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

	// make sure the output of B is mirrored in A
	outputOfA := string(personA.Output(100))
	outputOfB := string(personB.Output(100))
	c.Assert(strings.Contains(outputOfA, outputOfB), check.Equals, true)
}

// TestInvalidLogins validates that you can't login with invalid login or
// with invalid 'site' parameter
func (s *IntSuite) TestEnvironmentVariables(c *check.C) {
	t := s.newTeleport(c, nil, true)
	defer t.Stop(true)

	testKey, testVal := "TELEPORT_TEST_ENV", "howdy"
	cmd := []string{"printenv", testKey}

	// make sure sessions set run command
	tc, err := t.NewClient(s.me.Username, Site, Host, t.GetPortSSHInt())
	c.Assert(err, check.IsNil)

	tc.Env = map[string]string{testKey: testVal}
	out := &bytes.Buffer{}
	tc.Stdout = out
	tc.Stdin = nil
	err = tc.SSH(context.TODO(), cmd, false)

	c.Assert(err, check.IsNil)
	c.Assert(strings.TrimSpace(out.String()), check.Equals, testVal)
}

// TestInvalidLogins validates that you can't login with invalid login or
// with invalid 'site' parameter
func (s *IntSuite) TestInvalidLogins(c *check.C) {
	t := s.newTeleport(c, nil, true)
	defer t.Stop(true)

	cmd := []string{"echo", "success"}

	// try the wrong site:
	tc, err := t.NewClient(s.me.Username, "wrong-site", Host, t.GetPortSSHInt())
	c.Assert(err, check.IsNil)
	err = tc.SSH(context.TODO(), cmd, false)
	c.Assert(err, check.ErrorMatches, "cluster wrong-site not found")
}

// TestTwoClusters creates two teleport clusters: "a" and "b" and
// creates a tunnel from A to B.
//
// Then it executes an SSH command on A by connecting directly
// to A and by connecting to B via B<->A tunnel
func (s *IntSuite) TestTwoClusters(c *check.C) {
	username := s.me.Username

	a := NewInstance("site-A", HostID, Host, s.getPorts(5), s.priv, s.pub)
	b := NewInstance("site-B", HostID, Host, s.getPorts(5), s.priv, s.pub)

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
	tc.Stdout = &outputA
	c.Assert(err, check.IsNil)
	err = tc.SSH(context.TODO(), cmd, false)
	c.Assert(err, check.IsNil)
	c.Assert(outputA.String(), check.Equals, "hello world\n")

	// via tunnel b->a:
	tc, err = b.NewClient(username, "site-A", Host, sshPort)
	tc.Stdout = &outputB
	c.Assert(err, check.IsNil)
	err = tc.SSH(context.TODO(), cmd, false)
	c.Assert(err, check.IsNil)
	c.Assert(outputA.String(), check.DeepEquals, outputB.String())

	// Stop "site-A" and try to connect to it again via "site-A" (expect a connection error)
	a.Stop(false)
	err = tc.SSH(context.TODO(), cmd, false)
	c.Assert(err, check.ErrorMatches, `failed connecting to node localhost. site-A is offline`)

	// Reset and start "Site-A" again
	a.Reset()
	err = a.Start()
	c.Assert(err, check.IsNil)

	// try to execute an SSH command using the same old client  to Site-B
	// "site-A" and "site-B" reverse tunnels are supposed to reconnect,
	// and 'tc' (client) is also supposed to reconnect
	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond * 5)
		err = tc.SSH(context.TODO(), cmd, false)
		if err == nil {
			break
		}
	}
	c.Assert(err, check.IsNil)

	// stop both sites for realZ
	c.Assert(b.Stop(true), check.IsNil)
	c.Assert(a.Stop(true), check.IsNil)
}

// TestHA tests scenario when auth server for the cluster goes down
// and we switch to local persistent caches
func (s *IntSuite) TestHA(c *check.C) {
	username := s.me.Username

	a := NewInstance("cluster-a", HostID, Host, s.getPorts(5), s.priv, s.pub)
	b := NewInstance("cluster-b", HostID, Host, s.getPorts(5), s.priv, s.pub)

	a.AddUser(username, []string{username})
	b.AddUser(username, []string{username})

	c.Assert(b.Create(a.Secrets.AsSlice(), false, nil), check.IsNil)
	c.Assert(a.Create(b.Secrets.AsSlice(), true, nil), check.IsNil)

	c.Assert(b.Start(), check.IsNil)
	c.Assert(a.Start(), check.IsNil)

	nodePorts := s.getPorts(3)
	sshPort, proxyWebPort, proxySSHPort := nodePorts[0], nodePorts[1], nodePorts[2]
	c.Assert(a.StartNode("cluster-a-node", sshPort, proxyWebPort, proxySSHPort), check.IsNil)

	// wait for both sites to see each other via their reverse tunnels (for up to 10 seconds)
	abortTime := time.Now().Add(time.Second * 10)
	for len(b.Tunnel.GetSites()) < 2 && len(b.Tunnel.GetSites()) < 2 {
		time.Sleep(time.Millisecond * 2000)
		if time.Now().After(abortTime) {
			c.Fatalf("two sites do not see each other: tunnels are not working")
		}
	}

	cmd := []string{"echo", "hello world"}
	tc, err := b.NewClient(username, "cluster-a", "127.0.0.1", sshPort)
	c.Assert(err, check.IsNil)
	output := &bytes.Buffer{}
	tc.Stdout = output
	c.Assert(err, check.IsNil)
	// try to execute an SSH command using the same old client  to Site-B
	// "site-A" and "site-B" reverse tunnels are supposed to reconnect,
	// and 'tc' (client) is also supposed to reconnect
	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond * 50)
		err = tc.SSH(context.TODO(), cmd, false)
		if err == nil {
			break
		}
	}
	c.Assert(err, check.IsNil)
	c.Assert(output.String(), check.Equals, "hello world\n")
	// stop auth server a now
	c.Assert(a.Stop(true), check.IsNil)

	// try to execute an SSH command using the same old client  to Site-B
	// "site-A" and "site-B" reverse tunnels are supposed to reconnect,
	// and 'tc' (client) is also supposed to reconnect
	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond * 50)
		err = tc.SSH(context.TODO(), cmd, false)
		if err == nil {
			break
		}
	}
	c.Assert(err, check.IsNil)

	// stop cluster and remaining nodes
	c.Assert(b.Stop(true), check.IsNil)
	c.Assert(b.StopNodes(), check.IsNil)
}

// TestMapRoles tests local to remote role mapping and access patterns
func (s *IntSuite) TestMapRoles(c *check.C) {
	username := s.me.Username

	clusterMain := "cluster-main"
	clusterAux := "cluster-aux"
	main := NewInstance(clusterMain, HostID, Host, s.getPorts(5), s.priv, s.pub)
	aux := NewInstance(clusterAux, HostID, Host, s.getPorts(5), s.priv, s.pub)

	// main cluster has a local user and belongs to role "main-devs"
	mainDevs := "main-devs"
	role, err := services.NewRole(mainDevs, services.RoleSpecV2{
		Logins: []string{username},
	})
	c.Assert(err, check.IsNil)
	main.AddUserWithRole(username, role)

	c.Assert(main.CreateDevmode(nil, false, nil), check.IsNil)
	c.Assert(aux.CreateDevmode(nil, true, nil), check.IsNil)

	// auxillary cluster has a role aux-devs
	// connect aux cluster to main cluster
	// using trusted clusters, so remote user will be allowed to assume
	// role specified by mapping remote role "devs" to local role "local-devs"
	auxDevs := "aux-devs"
	role, err = services.NewRole(auxDevs, services.RoleSpecV2{
		Logins: []string{username},
	})
	c.Assert(err, check.IsNil)
	err = aux.Process.GetAuthServer().UpsertRole(role, backend.Forever)
	c.Assert(err, check.IsNil)
	trustedClusterToken := "trusted-clsuter-token"
	err = main.Process.GetAuthServer().UpsertToken(trustedClusterToken, []teleport.Role{teleport.RoleTrustedCluster}, backend.Forever)
	c.Assert(err, check.IsNil)
	trustedCluster := main.Secrets.AsTrustedCluster(trustedClusterToken, services.RoleMap{
		{Remote: mainDevs, Local: []string{auxDevs}},
	})

	c.Assert(main.Start(), check.IsNil)
	c.Assert(aux.Start(), check.IsNil)

	err = trustedCluster.CheckAndSetDefaults()
	c.Assert(err, check.IsNil)
	abortTime := time.Now().Add(2 * time.Second)
	for {
		log.Debugf("Will create trusted cluster %v", trustedCluster)
		err = aux.Process.GetAuthServer().UpsertTrustedCluster(trustedCluster)
		if err == nil {
			break
		}
		if trace.IsConnectionProblem(err) {
			log.Debugf("retrying on connection problem: %v", err)
		} else {
			c.Fatalf("got non connection problem %v", err)
		}
		time.Sleep(time.Millisecond * 300)
		if time.Now().After(abortTime) {
			c.Fatalf("failed to add trusted cluster")
		}
	}
	nodePorts := s.getPorts(3)
	sshPort, proxyWebPort, proxySSHPort := nodePorts[0], nodePorts[1], nodePorts[2]
	c.Assert(aux.StartNode("aux-node", sshPort, proxyWebPort, proxySSHPort), check.IsNil)

	// wait for both sites to see each other via their reverse tunnels (for up to 10 seconds)
	abortTime = time.Now().Add(time.Second * 10)
	for len(main.Tunnel.GetSites()) < 2 && len(main.Tunnel.GetSites()) < 2 {
		time.Sleep(time.Millisecond * 2000)
		if time.Now().After(abortTime) {
			c.Fatalf("two clusters do not see each other: tunnels are not working")
		}
	}

	cmd := []string{"echo", "hello world"}
	tc, err := main.NewClient(username, clusterAux, "127.0.0.1", sshPort)
	c.Assert(err, check.IsNil)
	output := &bytes.Buffer{}
	tc.Stdout = output
	c.Assert(err, check.IsNil)
	// try to execute an SSH command using the same old client  to Site-B
	// "site-A" and "site-B" reverse tunnels are supposed to reconnect,
	// and 'tc' (client) is also supposed to reconnect
	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond * 50)
		err = tc.SSH(context.TODO(), cmd, false)
		if err == nil {
			break
		}
	}
	c.Assert(err, check.IsNil)
	c.Assert(output.String(), check.Equals, "hello world\n")

	// stop clusters and remaining nodes
	c.Assert(main.Stop(true), check.IsNil)
	c.Assert(aux.Stop(true), check.IsNil)
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

// Output returns a number of first 'limit' bytes printed into this fake terminal
func (t *Terminal) Output(limit int) string {
	buff := t.written.Bytes()
	if len(buff) > limit {
		buff = buff[:limit]
	}
	// clean up white space for easier comparison:
	return strings.TrimSpace(string(buff))
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
