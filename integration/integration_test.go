/*
Copyright 2016-2018 Gravitational, Inc.

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
	"io/ioutil"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/pam"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
	"gopkg.in/check.v1"
)

const (
	Host   = "localhost"
	HostID = "00000000-0000-0000-0000-000000000000"
	Site   = "local-site"

	AllocatePortsNum = 300
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

// newTeleport helper returns a created but not started Teleport instance pre-configured
// with the current user os.user.Current().
func (s *IntSuite) newUnstartedTeleport(c *check.C, logins []string, enableSSH bool) *TeleInstance {
	t := NewInstance(InstanceConfig{ClusterName: Site, HostID: HostID, NodeName: Host, Ports: s.getPorts(5), Priv: s.priv, Pub: s.pub})
	// use passed logins, but use suite's default login if nothing was passed
	if logins == nil || len(logins) == 0 {
		logins = []string{s.me.Username}
	}
	for _, login := range logins {
		t.AddUser(login, []string{login})
	}
	if err := t.Create(nil, enableSSH, nil); err != nil {
		c.Fatalf("Unexpected response from Create: %v", err)
	}
	return t
}

// newTeleport helper returns a running Teleport instance pre-configured
// with the current user os.user.Current().
func (s *IntSuite) newTeleport(c *check.C, logins []string, enableSSH bool) *TeleInstance {
	t := s.newUnstartedTeleport(c, logins, enableSSH)
	if err := t.Start(); err != nil {
		c.Fatalf("Unexpected response from Start: %v", err)
	}
	return t
}

// newTeleportWithConfig is a helper function that will create a running
// Teleport instance with the passed in user, instance secrets, and Teleport
// configuration.
func (s *IntSuite) newTeleportWithConfig(c *check.C, logins []string, instanceSecrets []*InstanceSecrets, teleportConfig *service.Config) *TeleInstance {
	t := NewInstance(InstanceConfig{ClusterName: Site, HostID: HostID, NodeName: Host, Ports: s.getPorts(5), Priv: s.priv, Pub: s.pub})

	// use passed logins, but use suite's default login if nothing was passed
	if logins == nil || len(logins) == 0 {
		logins = []string{s.me.Username}
	}
	for _, login := range logins {
		t.AddUser(login, []string{login})
	}

	// create a new teleport instance with passed in configuration
	if err := t.CreateEx(instanceSecrets, teleportConfig); err != nil {
		c.Fatalf("Unexpected response from CreateEx: %v", trace.DebugReport(err))
	}
	if err := t.Start(); err != nil {
		c.Fatalf("Unexpected response from Start: %v", trace.DebugReport(err))
	}

	return t
}

// TestAuditOn creates a live session, records a bunch of data through it
// and then reads it back and compares against simulated reality.
func (s *IntSuite) TestAuditOn(c *check.C) {
	var tests = []struct {
		inRecordLocation string
		inForwardAgent   bool
		auditSessionsURI string
	}{

		// normal teleport
		{
			inRecordLocation: services.RecordAtNode,
			inForwardAgent:   false,
		},
		// recording proxy
		{
			inRecordLocation: services.RecordAtProxy,
			inForwardAgent:   true,
		},
		// normal teleport with upload to file server
		{
			inRecordLocation: services.RecordAtNode,
			inForwardAgent:   false,
			auditSessionsURI: c.MkDir(),
		},
		{
			inRecordLocation: services.RecordAtProxy,
			inForwardAgent:   false,
			auditSessionsURI: c.MkDir(),
		},
	}

	for _, tt := range tests {
		makeConfig := func() (*check.C, []string, []*InstanceSecrets, *service.Config) {
			clusterConfig, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
				SessionRecording: tt.inRecordLocation,
				Audit:            services.AuditConfig{AuditSessionsURI: tt.auditSessionsURI},
			})
			c.Assert(err, check.IsNil)

			tconf := service.MakeDefaultConfig()
			tconf.Auth.Enabled = true
			tconf.Auth.ClusterConfig = clusterConfig
			tconf.Proxy.Enabled = true
			tconf.Proxy.DisableWebService = true
			tconf.Proxy.DisableWebInterface = true
			tconf.SSH.Enabled = true
			return c, nil, nil, tconf
		}
		t := s.newTeleportWithConfig(makeConfig())
		defer t.Stop(true)

		nodeSSHPort := s.getPorts(1)[0]
		nodeProcess, err := t.StartNode("node", nodeSSHPort)
		c.Assert(err, check.IsNil)

		// get access to a authClient for the cluster
		site := t.GetSiteAPI(Site)
		c.Assert(site, check.NotNil)

		// wait 10 seconds for both nodes to show up, otherwise
		// we'll have trouble connecting to the node below.
		waitForNodes := func(site auth.ClientI, count int) error {
			tickCh := time.Tick(500 * time.Millisecond)
			stopCh := time.After(10 * time.Second)
			for {
				select {
				case <-tickCh:
					nodesInSite, err := site.GetNodes(defaults.Namespace)
					if err != nil && !trace.IsNotFound(err) {
						return trace.Wrap(err)
					}
					if got, want := len(nodesInSite), count; got == want {
						return nil
					}
				case <-stopCh:
					return trace.BadParameter("waited 10s, did find %v nodes", count)
				}
			}
		}
		err = waitForNodes(site, 2)
		c.Assert(err, check.IsNil)

		// should have no sessions:
		sessions, err := site.GetSessions(defaults.Namespace)
		c.Assert(err, check.IsNil)
		c.Assert(len(sessions), check.Equals, 0)

		// create interactive session (this goroutine is this user's terminal time)
		endC := make(chan error, 0)
		myTerm := NewTerminal(250)
		go func() {
			cl, err := t.NewClient(ClientConfig{
				Login:        s.me.Username,
				Cluster:      Site,
				Host:         Host,
				Port:         nodeSSHPort,
				ForwardAgent: tt.inForwardAgent,
			})
			c.Assert(err, check.IsNil)
			cl.Stdout = &myTerm
			cl.Stdin = &myTerm
			err = cl.SSH(context.TODO(), []string{}, false)
			endC <- err
		}()

		// wait until we've found the session in the audit log
		getSession := func(site auth.ClientI) (*session.Session, error) {
			tickCh := time.Tick(500 * time.Millisecond)
			stopCh := time.After(10 * time.Second)
			for {
				select {
				case <-tickCh:
					sessions, err = site.GetSessions(defaults.Namespace)
					if err != nil {
						return nil, trace.Wrap(err)
					}
					if len(sessions) != 1 {
						continue
					}
					return &sessions[0], nil
				case <-stopCh:
					return nil, trace.BadParameter("unable to find sessions after 10s (mode=%v)", tt.inRecordLocation)
				}
			}
		}
		session, err := getSession(site)
		c.Assert(err, check.IsNil)

		// wait for the user to join this session:
		for len(session.Parties) == 0 {
			time.Sleep(time.Millisecond * 5)
			session, err = site.GetSession(defaults.Namespace, sessions[0].ID)
			c.Assert(err, check.IsNil)
		}
		// make sure it's us who joined! :)
		c.Assert(session.Parties[0].User, check.Equals, s.me.Username)

		// lets type "echo hi" followed by "enter" and then "exit" + "enter":
		myTerm.Type("\aecho hi\n\r\aexit\n\r\a")

		// wait for session to end:
		select {
		case <-endC:
		case <-time.After(10 * time.Second):
			c.Fatalf("Timeout waiting for session to finish")
		}

		// wait for the upload of the right session to complete
		timeoutC := time.After(10 * time.Second)
	loop:
		for {
			select {
			case event := <-t.UploadEventsC:
				if event.SessionID != string(session.ID) {
					log.Debugf("Skipping mismatching session %v, expecting upload of %v.", event.SessionID, session.ID)
					continue
				}
				break loop
			case <-timeoutC:
				c.Fatalf("Timeout waiting for upload of session %v to complete to %v", session.ID, tt.auditSessionsURI)
			}
		}

		// read back the entire session (we have to try several times until we get back
		// everything because the session is closing)
		var sessionStream []byte
		for i := 0; i < 6; i++ {
			sessionStream, err = site.GetSessionChunk(defaults.Namespace, session.ID, 0, events.MaxChunkBytes)
			c.Assert(err, check.IsNil)
			if strings.Contains(string(sessionStream), "exit") {
				break
			}
			time.Sleep(time.Millisecond * 250)
			if i >= 5 {
				// session stream keeps coming back short
				c.Fatal("stream is not getting data: %q", string(sessionStream))
			}
		}

		// see what we got. It looks different based on bash settings, but here it is
		// on Ev's machine (hostname is 'edsger'):
		//
		// edsger ~: echo hi
		// hi
		// edsger ~: exit
		// logout
		//
		comment := check.Commentf("%q", string(sessionStream))
		c.Assert(strings.Contains(string(sessionStream), "echo hi"), check.Equals, true, comment)
		c.Assert(strings.Contains(string(sessionStream), "exit"), check.Equals, true, comment)

		// Wait until session.start, session.leave, and session.end events have arrived.
		getSessions := func(site auth.ClientI) ([]events.EventFields, error) {
			tickCh := time.Tick(500 * time.Millisecond)
			stopCh := time.After(10 * time.Second)
			for {
				select {
				case <-tickCh:
					// Get all session events from the backend.
					sessionEvents, err := site.GetSessionEvents(defaults.Namespace, session.ID, 0, false)
					if err != nil {
						return nil, trace.Wrap(err)
					}

					// Look through all session events for the three wanted.
					var hasStart bool
					var hasEnd bool
					var hasLeave bool
					for _, se := range sessionEvents {
						if se.GetType() == events.SessionStartEvent {
							hasStart = true
						}
						if se.GetType() == events.SessionEndEvent {
							hasEnd = true
						}
						if se.GetType() == events.SessionLeaveEvent {
							hasLeave = true
						}
					}

					// Make sure all three events were found.
					if hasStart && hasEnd && hasLeave {
						return sessionEvents, nil
					}
				case <-stopCh:
					return nil, trace.BadParameter("unable to find all session events after 10s (mode=%v)", tt.inRecordLocation)
				}
			}
		}
		history, err := getSessions(site)
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

		// if session are being recorded at nodes, then the event server_id field
		// should contain the ID of the node. if sessions are being recorded at the
		// proxy, then server_id is random so we can't check it, but it should not
		// the server_id of any of the nodes we know about.
		switch tt.inRecordLocation {
		case services.RecordAtNode:
			c.Assert(start.GetString(events.SessionServerID), check.Equals, nodeProcess.Config.HostUUID)
		case services.RecordAtProxy:
			c.Assert(start.GetString(events.SessionServerID), check.Not(check.Equals), nodeProcess.Config.HostUUID)
			c.Assert(start.GetString(events.SessionServerID), check.Not(check.Equals), t.Process.Config.HostUUID)
		}

		// make sure data is recorded properly
		out := &bytes.Buffer{}
		for _, e := range history {
			out.WriteString(getChunk(e, 1000))
		}
		recorded := replaceNewlines(out.String())
		c.Assert(recorded, check.Matches, ".*exit.*")
		c.Assert(recorded, check.Matches, ".*echo hi.*")

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
}

func replaceNewlines(in string) string {
	return regexp.MustCompile(`\r?\n`).ReplaceAllString(in, `\n`)
}

// TestInteroperability checks if Teleport and OpenSSH behave in the same way
// when executing commands.
func (s *IntSuite) TestInteroperability(c *check.C) {
	tempdir, err := ioutil.TempDir("", "teleport-")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tempdir)
	tempfile := filepath.Join(tempdir, "file.txt")

	// create new teleport server that will be used by all tests
	t := s.newTeleport(c, nil, true)
	defer t.Stop(true)

	var tests = []struct {
		inCommand   string
		inStdin     string
		outContains string
		outFile     bool
	}{
		// 0 - echo "1\n2\n" | ssh localhost "cat -"
		// this command can be used to copy files by piping stdout to stdin over ssh.
		{
			"cat -",
			"1\n2\n",
			"1\n2\n",
			false,
		},
		// 1 - ssh -tt locahost '/bin/sh -c "mkdir -p /tmp && echo a > /tmp/file.txt"'
		// programs like ansible execute commands like this
		{
			fmt.Sprintf(`/bin/sh -c "mkdir -p /tmp && echo a > %v"`, tempfile),
			"",
			"a",
			true,
		},
		// 2 - ssh localhost tty
		// should print "not a tty"
		{
			"tty",
			"",
			"not a tty",
			false,
		},
	}

	for i, tt := range tests {
		// create new teleport client
		cl, err := t.NewClient(ClientConfig{Login: s.me.Username, Cluster: Site, Host: Host, Port: t.GetPortSSHInt()})
		c.Assert(err, check.IsNil)

		// hook up stdin and stdout to a buffer for reading and writing
		inbuf := bytes.NewReader([]byte(tt.inStdin))
		outbuf := &bytes.Buffer{}
		cl.Stdin = inbuf
		cl.Stdout = outbuf
		cl.Stderr = outbuf

		// run command and wait a maximum of 10 seconds for it to complete
		sessionEndC := make(chan interface{}, 0)
		go func() {
			// don't check for err, because sometimes this process should fail
			// with an error and that's what the test is checking for.
			cl.SSH(context.TODO(), []string{tt.inCommand}, false)
			sessionEndC <- true
		}()
		waitFor(sessionEndC, time.Second*10)

		// if we are looking for the output in a file, look in the file
		// otherwise check stdout and stderr for the expected output
		if tt.outFile {
			bytes, err := ioutil.ReadFile(tempfile)
			c.Assert(err, check.IsNil)
			comment := check.Commentf("Test %v: %q does not contain: %q", i, string(bytes), tt.outContains)
			c.Assert(strings.Contains(string(bytes), tt.outContains), check.Equals, true, comment)
		} else {
			comment := check.Commentf("Test %v: %q does not contain: %q", i, outbuf.String(), tt.outContains)
			c.Assert(strings.Contains(outbuf.String(), tt.outContains), check.Equals, true, comment)
		}
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
		cl, err := t.NewClient(ClientConfig{Login: s.me.Username, Cluster: Site, Host: Host, Port: t.GetPortSSHInt()})
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
		cl, err := t.NewClient(ClientConfig{Login: s.me.Username, Cluster: Site, Host: Host, Port: t.GetPortSSHInt()})
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

// TestShutdown tests scenario with a graceful shutdown,
// that session will be working after
func (s *IntSuite) TestShutdown(c *check.C) {
	t := s.newTeleport(c, nil, true)

	// get a reference to site obj:
	site := t.GetSiteAPI(Site)
	c.Assert(site, check.NotNil)

	person := NewTerminal(250)

	// commandsC receive commands
	commandsC := make(chan string, 0)

	// PersonA: SSH into the server, wait one second, then type some commands on stdin:
	openSession := func() {
		cl, err := t.NewClient(ClientConfig{Login: s.me.Username, Cluster: Site, Host: Host, Port: t.GetPortSSHInt()})
		c.Assert(err, check.IsNil)
		cl.Stdout = &person
		cl.Stdin = &person

		go func() {
			for command := range commandsC {
				person.Type(command)
			}
		}()

		err = cl.SSH(context.TODO(), []string{}, false)
		c.Assert(err, check.IsNil)
	}

	go openSession()

	retry := func(command, pattern string) {
		person.Type(command)
		// wait for both sites to see each other via their reverse tunnels (for up to 10 seconds)
		abortTime := time.Now().Add(10 * time.Second)
		var matched bool
		var output string
		for {
			output = string(replaceNewlines(person.Output(1000)))
			matched, _ = regexp.MatchString(pattern, output)
			if matched {
				break
			}
			time.Sleep(time.Millisecond * 200)
			if time.Now().After(abortTime) {
				c.Fatalf("failed to capture output: %v", pattern)
			}
		}
		if !matched {
			c.Fatalf("output %q does not match pattern %q", output, pattern)
		}
	}

	retry("echo start \r\n", ".*start.*")

	// initiate shutdown
	ctx := context.TODO()
	shutdownContext := t.Process.StartShutdown(ctx)

	// make sure that terminal still works
	retry("echo howdy \r\n", ".*howdy.*")

	// now type exit and wait for shutdown to complete
	person.Type("exit\n\r")

	select {
	case <-shutdownContext.Done():
	case <-time.After(5 * time.Second):
		c.Fatalf("failed to shut down the server")
	}
}

// TestInvalidLogins validates that you can't login with invalid login or
// with invalid 'site' parameter
func (s *IntSuite) TestEnvironmentVariables(c *check.C) {
	t := s.newTeleport(c, nil, true)
	defer t.Stop(true)

	testKey, testVal := "TELEPORT_TEST_ENV", "howdy"
	cmd := []string{"printenv", testKey}

	// make sure sessions set run command
	tc, err := t.NewClient(ClientConfig{Login: s.me.Username, Cluster: Site, Host: Host, Port: t.GetPortSSHInt()})
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
	tc, err := t.NewClient(ClientConfig{Login: s.me.Username, Cluster: "wrong-site", Host: Host, Port: t.GetPortSSHInt()})
	c.Assert(err, check.IsNil)
	err = tc.SSH(context.TODO(), cmd, false)
	c.Assert(err, check.ErrorMatches, "cluster wrong-site not found")
}

// TestTwoClusters creates two teleport clusters: "a" and "b" and creates a
// tunnel from A to B.
//
// Two tests are run, first is when both A and B record sessions at nodes. It
// executes an SSH command on A by connecting directly to A and by connecting
// to B via B<->A tunnel. All sessions should end up in A.
//
// In the second test, sessions are recorded at B. All sessions still show up on
// A (they are Teleport nodes) but in addition, two show up on B when connecting
// over the B<->A tunnel because sessions are recorded at the proxy.
func (s *IntSuite) TestTwoClusters(c *check.C) {
	now := time.Now().In(time.UTC).Round(time.Second)

	var tests = []struct {
		inRecordLocation  string
		outExecCountSiteA int
		outExecCountSiteB int
	}{
		// normal teleport. since all events are recorded at the node, all events
		// end up on site-a and none on site-b.
		{
			services.RecordAtNode,
			3,
			0,
		},
		// recording proxy. since events are recorded at the proxy, 3 events end up
		// on site-a (because it's a teleport node so it still records at the node)
		// and 2 events end up on site-b because it's recording.
		{
			services.RecordAtProxy,
			3,
			2,
		},
	}

	for _, tt := range tests {
		// start the http proxy, we need to make sure this was not used
		ps := &proxyServer{}
		ts := httptest.NewServer(ps)
		defer ts.Close()

		// clear out any proxy environment variables
		for _, v := range []string{"http_proxy", "https_proxy", "HTTP_PROXY", "HTTPS_PROXY"} {
			os.Setenv(v, "")
		}

		username := s.me.Username

		a := NewInstance(InstanceConfig{ClusterName: "site-A", HostID: HostID, NodeName: Host, Ports: s.getPorts(5), Priv: s.priv, Pub: s.pub})
		b := NewInstance(InstanceConfig{ClusterName: "site-B", HostID: HostID, NodeName: Host, Ports: s.getPorts(5), Priv: s.priv, Pub: s.pub})

		a.AddUser(username, []string{username})
		b.AddUser(username, []string{username})

		clusterConfig, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
			SessionRecording: tt.inRecordLocation,
		})
		c.Assert(err, check.IsNil)

		acfg := service.MakeDefaultConfig()
		acfg.Auth.Enabled = true
		acfg.Proxy.Enabled = true
		acfg.Proxy.DisableWebService = true
		acfg.Proxy.DisableWebInterface = true
		acfg.SSH.Enabled = true

		bcfg := service.MakeDefaultConfig()
		bcfg.Auth.Enabled = true
		bcfg.Auth.ClusterConfig = clusterConfig
		bcfg.Proxy.Enabled = true
		bcfg.Proxy.DisableWebService = true
		bcfg.Proxy.DisableWebInterface = true
		bcfg.SSH.Enabled = false

		c.Assert(b.CreateEx(a.Secrets.AsSlice(), bcfg), check.IsNil)
		c.Assert(a.CreateEx(b.Secrets.AsSlice(), acfg), check.IsNil)

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

		// make sure the direct dialer was used and not the proxy dialer
		c.Assert(ps.Count(), check.Equals, 0)

		// if we got here, it means two sites are cross-connected. lets execute SSH commands
		sshPort := a.GetPortSSHInt()
		cmd := []string{"echo", "hello world"}

		// directly:
		tc, err := a.NewClient(ClientConfig{
			Login:        username,
			Cluster:      "site-A",
			Host:         Host,
			Port:         sshPort,
			ForwardAgent: true,
		})
		tc.Stdout = &outputA
		c.Assert(err, check.IsNil)
		err = tc.SSH(context.TODO(), cmd, false)
		c.Assert(err, check.IsNil)
		c.Assert(outputA.String(), check.Equals, "hello world\n")

		// via tunnel b->a:
		tc, err = b.NewClient(ClientConfig{
			Login:        username,
			Cluster:      "site-A",
			Host:         Host,
			Port:         sshPort,
			ForwardAgent: true,
		})
		tc.Stdout = &outputB
		c.Assert(err, check.IsNil)
		err = tc.SSH(context.TODO(), cmd, false)
		c.Assert(err, check.IsNil)
		c.Assert(outputA.String(), check.DeepEquals, outputB.String())

		// Stop "site-A" and try to connect to it again via "site-A" (expect a connection error)
		a.Stop(false)
		err = tc.SSH(context.TODO(), cmd, false)
		// debug mode will add more lines, so this check has to be flexible
		c.Assert(strings.Replace(err.Error(), "\n", "", -1), check.Matches, `.*site-A is offline.*`)

		// Reset and start "Site-A" again
		a.Reset()
		err = a.Start()
		c.Assert(err, check.IsNil)

		// try to execute an SSH command using the same old client to Site-B
		// "site-A" and "site-B" reverse tunnels are supposed to reconnect,
		// and 'tc' (client) is also supposed to reconnect
		for i := 0; i < 10; i++ {
			time.Sleep(250 * time.Millisecond)
			err = tc.SSH(context.TODO(), cmd, false)
			if err == nil {
				break
			}
		}
		c.Assert(err, check.IsNil)

		searchAndAssert := func(site auth.ClientI, count int) error {
			tickCh := time.Tick(500 * time.Millisecond)
			stopCh := time.After(5 * time.Second)

			// only look for exec events
			execQuery := fmt.Sprintf("%s=%s", events.EventType, events.ExecEvent)

			for {
				select {
				case <-tickCh:
					eventsInSite, err := site.SearchEvents(now, now.Add(1*time.Hour), execQuery, 0)
					if err != nil {
						return trace.Wrap(err)
					}

					// found the number of events we were looking for
					if got, want := len(eventsInSite), count; got == want {
						return nil
					}
				case <-stopCh:
					return trace.BadParameter("unable to find %v events after 5s", count)
				}
			}
		}

		siteA := a.GetSiteAPI("site-A")
		err = searchAndAssert(siteA, tt.outExecCountSiteA)
		c.Assert(err, check.IsNil)

		siteB := b.GetSiteAPI("site-B")
		err = searchAndAssert(siteB, tt.outExecCountSiteB)
		c.Assert(err, check.IsNil)

		// stop both sites for real
		c.Assert(b.Stop(true), check.IsNil)
		c.Assert(a.Stop(true), check.IsNil)
	}
}

// TestTwoClustersProxy checks if the reverse tunnel uses a HTTP PROXY to
// establish a connection.
func (s *IntSuite) TestTwoClustersProxy(c *check.C) {
	// start the http proxy
	ps := &proxyServer{}
	ts := httptest.NewServer(ps)
	defer ts.Close()

	// set the http_proxy environment variable
	u, err := url.Parse(ts.URL)
	c.Assert(err, check.IsNil)
	os.Setenv("http_proxy", u.Host)
	defer os.Setenv("http_proxy", "")

	username := s.me.Username

	a := NewInstance(InstanceConfig{ClusterName: "site-A", HostID: HostID, NodeName: Host, Ports: s.getPorts(5), Priv: s.priv, Pub: s.pub})
	b := NewInstance(InstanceConfig{ClusterName: "site-B", HostID: HostID, NodeName: Host, Ports: s.getPorts(5), Priv: s.priv, Pub: s.pub})

	a.AddUser(username, []string{username})
	b.AddUser(username, []string{username})

	c.Assert(b.Create(a.Secrets.AsSlice(), false, nil), check.IsNil)
	c.Assert(a.Create(b.Secrets.AsSlice(), true, nil), check.IsNil)

	c.Assert(b.Start(), check.IsNil)
	c.Assert(a.Start(), check.IsNil)

	// wait for both sites to see each other via their reverse tunnels (for up to 10 seconds)
	abortTime := time.Now().Add(time.Second * 10)
	for len(a.Tunnel.GetSites()) < 2 && len(b.Tunnel.GetSites()) < 2 {
		time.Sleep(time.Millisecond * 200)
		if time.Now().After(abortTime) {
			c.Fatalf("two sites do not see each other: tunnels are not working")
		}
	}

	// make sure the reverse tunnel went through the proxy
	c.Assert(ps.Count() > 0, check.Equals, true, check.Commentf("proxy did not intercept any connection"))

	// stop both sites for real
	c.Assert(b.Stop(true), check.IsNil)
	c.Assert(a.Stop(true), check.IsNil)
}

// TestHA tests scenario when auth server for the cluster goes down
// and we switch to local persistent caches
func (s *IntSuite) TestHA(c *check.C) {
	username := s.me.Username

	a := NewInstance(InstanceConfig{ClusterName: "cluster-a", HostID: HostID, NodeName: Host, Ports: s.getPorts(5), Priv: s.priv, Pub: s.pub})
	b := NewInstance(InstanceConfig{ClusterName: "cluster-b", HostID: HostID, NodeName: Host, Ports: s.getPorts(5), Priv: s.priv, Pub: s.pub})

	a.AddUser(username, []string{username})
	b.AddUser(username, []string{username})

	c.Assert(b.Create(a.Secrets.AsSlice(), false, nil), check.IsNil)
	c.Assert(a.Create(b.Secrets.AsSlice(), true, nil), check.IsNil)

	c.Assert(b.Start(), check.IsNil)
	c.Assert(a.Start(), check.IsNil)

	nodePorts := s.getPorts(3)
	sshPort, proxyWebPort, proxySSHPort := nodePorts[0], nodePorts[1], nodePorts[2]
	c.Assert(a.StartNodeAndProxy("cluster-a-node", sshPort, proxyWebPort, proxySSHPort), check.IsNil)

	// wait for both sites to see each other via their reverse tunnels (for up to 10 seconds)
	abortTime := time.Now().Add(time.Second * 10)
	for len(a.Tunnel.GetSites()) < 2 && len(b.Tunnel.GetSites()) < 2 {
		time.Sleep(time.Millisecond * 2000)
		if time.Now().After(abortTime) {
			c.Fatalf("two sites do not see each other: tunnels are not working")
		}
	}

	cmd := []string{"echo", "hello world"}
	tc, err := b.NewClient(ClientConfig{Login: username, Cluster: "cluster-a", Host: "127.0.0.1", Port: sshPort})
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
	main := NewInstance(InstanceConfig{ClusterName: clusterMain, HostID: HostID, NodeName: Host, Ports: s.getPorts(5), Priv: s.priv, Pub: s.pub})
	aux := NewInstance(InstanceConfig{ClusterName: clusterAux, HostID: HostID, NodeName: Host, Ports: s.getPorts(5), Priv: s.priv, Pub: s.pub})

	// main cluster has a local user and belongs to role "main-devs"
	mainDevs := "main-devs"
	role, err := services.NewRole(mainDevs, services.RoleSpecV3{
		Allow: services.RoleConditions{
			Logins: []string{username},
		},
	})
	c.Assert(err, check.IsNil)
	main.AddUserWithRole(username, role)

	// for role mapping test we turn on Web API on the main cluster
	// as it's used
	makeConfig := func(enableSSH bool) ([]*InstanceSecrets, *service.Config) {
		tconf := service.MakeDefaultConfig()
		tconf.SSH.Enabled = enableSSH
		tconf.Console = nil
		tconf.Proxy.DisableWebService = false
		tconf.Proxy.DisableWebInterface = true
		return nil, tconf
	}
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	c.Assert(main.CreateEx(makeConfig(false)), check.IsNil)
	c.Assert(aux.CreateEx(makeConfig(true)), check.IsNil)

	// auxiliary cluster has a role aux-devs
	// connect aux cluster to main cluster
	// using trusted clusters, so remote user will be allowed to assume
	// role specified by mapping remote role "devs" to local role "local-devs"
	auxDevs := "aux-devs"
	role, err = services.NewRole(auxDevs, services.RoleSpecV3{
		Allow: services.RoleConditions{
			Logins: []string{username},
		},
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

	// modify trusted cluster resource name so it would not
	// match the cluster name to check that it does not matter
	trustedCluster.SetName(main.Secrets.SiteName + "-cluster")

	c.Assert(main.Start(), check.IsNil)
	c.Assert(aux.Start(), check.IsNil)

	err = trustedCluster.CheckAndSetDefaults()
	c.Assert(err, check.IsNil)

	// try and upsert a trusted cluster
	var upsertSuccess bool
	for i := 0; i < 10; i++ {
		log.Debugf("Will create trusted cluster %v, attempt %v", trustedCluster, i)
		_, err = aux.Process.GetAuthServer().UpsertTrustedCluster(trustedCluster)
		if err != nil {
			if trace.IsConnectionProblem(err) {
				log.Debugf("retrying on connection problem: %v", err)
				continue
			}
			c.Fatalf("got non connection problem %v", err)
		}
		upsertSuccess = true
		break
	}
	// make sure we upsert a trusted cluster
	c.Assert(upsertSuccess, check.Equals, true)

	nodePorts := s.getPorts(3)
	sshPort, proxyWebPort, proxySSHPort := nodePorts[0], nodePorts[1], nodePorts[2]
	c.Assert(aux.StartNodeAndProxy("aux-node", sshPort, proxyWebPort, proxySSHPort), check.IsNil)

	// wait for both sites to see each other via their reverse tunnels (for up to 10 seconds)
	abortTime := time.Now().Add(time.Second * 10)
	for len(main.Tunnel.GetSites()) < 2 && len(main.Tunnel.GetSites()) < 2 {
		time.Sleep(time.Millisecond * 2000)
		if time.Now().After(abortTime) {
			c.Fatalf("two clusters do not see each other: tunnels are not working")
		}
	}

	cmd := []string{"echo", "hello world"}
	tc, err := main.NewClient(ClientConfig{Login: username, Cluster: clusterAux, Host: "127.0.0.1", Port: sshPort})
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

	// make sure both clusters have the right certificate authorities with the right signing keys.
	var tests = []struct {
		mainClusterName  string
		auxClusterName   string
		inCluster        *TeleInstance
		outChkMainUserCA check.Checker
		outLenMainUserCA int
		outChkMainHostCA check.Checker
		outLenMainHostCA int
		outChkAuxUserCA  check.Checker
		outLenAuxUserCA  int
		outChkAuxHostCA  check.Checker
		outLenAuxHostCA  int
	}{
		// 0 - main
		//   * User CA for main has one signing key.
		//   * Host CA for main has one signing key.
		//   * User CA for aux does not exist.
		//   * Host CA for aux has no signing keys.
		{
			main.Secrets.SiteName,
			aux.Secrets.SiteName,
			main,
			check.IsNil, 1,
			check.IsNil, 1,
			check.NotNil, 0,
			check.IsNil, 0,
		},
		// 1 - aux
		//   * User CA for main has no signing keys.
		//   * Host CA for main has no signing keys.
		//   * User CA for aux has one signing key.
		//   * Host CA for aux has one signing key.
		{
			trustedCluster.GetName(),
			aux.Secrets.SiteName,
			aux,
			check.IsNil, 0,
			check.IsNil, 0,
			check.IsNil, 1,
			check.IsNil, 1,
		},
	}

	for i, tt := range tests {
		cid := services.CertAuthID{Type: services.UserCA, DomainName: tt.mainClusterName}
		mainUserCAs, err := tt.inCluster.Process.GetAuthServer().GetCertAuthority(cid, true)
		c.Assert(err, tt.outChkMainUserCA)
		if tt.outChkMainUserCA == check.IsNil {
			c.Assert(mainUserCAs.GetSigningKeys(), check.HasLen, tt.outLenMainUserCA, check.Commentf("Test %v, Main User CA", i))
		}

		cid = services.CertAuthID{Type: services.HostCA, DomainName: tt.mainClusterName}
		mainHostCAs, err := tt.inCluster.Process.GetAuthServer().GetCertAuthority(cid, true)
		c.Assert(err, tt.outChkMainHostCA)
		if tt.outChkMainHostCA == check.IsNil {
			c.Assert(mainHostCAs.GetSigningKeys(), check.HasLen, tt.outLenMainHostCA, check.Commentf("Test %v, Main Host CA", i))
		}

		cid = services.CertAuthID{Type: services.UserCA, DomainName: tt.auxClusterName}
		auxUserCAs, err := tt.inCluster.Process.GetAuthServer().GetCertAuthority(cid, true)
		c.Assert(err, tt.outChkAuxUserCA)
		if tt.outChkAuxUserCA == check.IsNil {
			c.Assert(auxUserCAs.GetSigningKeys(), check.HasLen, tt.outLenAuxUserCA, check.Commentf("Test %v, Aux User CA", i))
		}

		cid = services.CertAuthID{Type: services.HostCA, DomainName: tt.auxClusterName}
		auxHostCAs, err := tt.inCluster.Process.GetAuthServer().GetCertAuthority(cid, true)
		c.Assert(err, tt.outChkAuxHostCA)
		if tt.outChkAuxHostCA == check.IsNil {
			c.Assert(auxHostCAs.GetSigningKeys(), check.HasLen, tt.outLenAuxHostCA, check.Commentf("Test %v, Aux Host CA", i))
		}
	}

	// stop clusters and remaining nodes
	c.Assert(main.Stop(true), check.IsNil)
	c.Assert(aux.Stop(true), check.IsNil)
}

// TestTrustedClusters tests remote clusters scenarios
// using trusted clusters feature
func (s *IntSuite) TestTrustedClusters(c *check.C) {
	s.trustedClusters(c, false)
}

// TestMultiplexingTrustedClusters tests remote clusters scenarios
// using trusted clusters feature
func (s *IntSuite) TestMultiplexingTrustedClusters(c *check.C) {
	s.trustedClusters(c, true)
}

func (s *IntSuite) trustedClusters(c *check.C, multiplex bool) {
	username := s.me.Username

	clusterMain := "cluster-main"
	clusterAux := "cluster-aux"
	main := NewInstance(InstanceConfig{ClusterName: clusterMain, HostID: HostID, NodeName: Host, Ports: s.getPorts(5), Priv: s.priv, Pub: s.pub, MultiplexProxy: multiplex})
	aux := NewInstance(InstanceConfig{ClusterName: clusterAux, HostID: HostID, NodeName: Host, Ports: s.getPorts(5), Priv: s.priv, Pub: s.pub})

	// main cluster has a local user and belongs to role "main-devs"
	mainDevs := "main-devs"
	role, err := services.NewRole(mainDevs, services.RoleSpecV3{
		Allow: services.RoleConditions{
			Logins: []string{username},
		},
	})
	c.Assert(err, check.IsNil)
	main.AddUserWithRole(username, role)

	// for role mapping test we turn on Web API on the main cluster
	// as it's used
	makeConfig := func(enableSSH bool) ([]*InstanceSecrets, *service.Config) {
		tconf := service.MakeDefaultConfig()
		tconf.SSH.Enabled = enableSSH
		tconf.Console = nil
		tconf.Proxy.DisableWebService = false
		tconf.Proxy.DisableWebInterface = true
		return nil, tconf
	}
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	c.Assert(main.CreateEx(makeConfig(false)), check.IsNil)
	c.Assert(aux.CreateEx(makeConfig(true)), check.IsNil)

	// auxiliary cluster has a role aux-devs
	// connect aux cluster to main cluster
	// using trusted clusters, so remote user will be allowed to assume
	// role specified by mapping remote role "devs" to local role "local-devs"
	auxDevs := "aux-devs"
	role, err = services.NewRole(auxDevs, services.RoleSpecV3{
		Allow: services.RoleConditions{
			Logins: []string{username},
		},
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

	// modify trusted cluster resource name so it would not
	// match the cluster name to check that it does not matter
	trustedCluster.SetName(main.Secrets.SiteName + "-cluster")

	c.Assert(main.Start(), check.IsNil)
	c.Assert(aux.Start(), check.IsNil)

	err = trustedCluster.CheckAndSetDefaults()
	c.Assert(err, check.IsNil)

	// try and upsert a trusted cluster
	var upsertSuccess bool
	for i := 0; i < 10; i++ {
		log.Debugf("Will create trusted cluster %v, attempt %v", trustedCluster, i)
		_, err = aux.Process.GetAuthServer().UpsertTrustedCluster(trustedCluster)
		if err != nil {
			if trace.IsConnectionProblem(err) {
				log.Debugf("retrying on connection problem: %v", err)
				continue
			}
			c.Fatalf("got non connection problem %v", err)
		}
		upsertSuccess = true
		break
	}
	// make sure we upsert a trusted cluster
	c.Assert(upsertSuccess, check.Equals, true)

	nodePorts := s.getPorts(3)
	sshPort, proxyWebPort, proxySSHPort := nodePorts[0], nodePorts[1], nodePorts[2]
	c.Assert(aux.StartNodeAndProxy("aux-node", sshPort, proxyWebPort, proxySSHPort), check.IsNil)

	// wait for both sites to see each other via their reverse tunnels (for up to 10 seconds)
	abortTime := time.Now().Add(time.Second * 10)
	for len(main.Tunnel.GetSites()) < 2 && len(main.Tunnel.GetSites()) < 2 {
		time.Sleep(time.Millisecond * 2000)
		if time.Now().After(abortTime) {
			c.Fatalf("two clusters do not see each other: tunnels are not working")
		}
	}

	cmd := []string{"echo", "hello world"}
	tc, err := main.NewClient(ClientConfig{Login: username, Cluster: clusterAux, Host: "127.0.0.1", Port: sshPort})
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

	// check that remote cluster has been provisioned
	remoteClusters, err := main.Process.GetAuthServer().GetRemoteClusters()
	c.Assert(err, check.IsNil)
	c.Assert(remoteClusters, check.HasLen, 1)
	c.Assert(remoteClusters[0].GetName(), check.Equals, clusterAux)

	// after removing the remote cluster, the connection will start failing
	err = main.Process.GetAuthServer().DeleteRemoteCluster(clusterAux)
	c.Assert(err, check.IsNil)
	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond * 50)
		err = tc.SSH(context.TODO(), cmd, false)
		if err != nil {
			break
		}
	}
	c.Assert(err, check.NotNil, check.Commentf("expected tunnel to close and SSH client to start failing"))

	// remove trusted cluster from aux cluster side, and recrete right after
	// this should re-establish connection
	err = aux.Process.GetAuthServer().DeleteTrustedCluster(trustedCluster.GetName())
	c.Assert(err, check.IsNil)
	_, err = aux.Process.GetAuthServer().UpsertTrustedCluster(trustedCluster)
	c.Assert(err, check.IsNil)

	// check that remote cluster has been re-provisioned
	remoteClusters, err = main.Process.GetAuthServer().GetRemoteClusters()
	c.Assert(err, check.IsNil)
	c.Assert(remoteClusters, check.HasLen, 1)
	c.Assert(remoteClusters[0].GetName(), check.Equals, clusterAux)

	// wait for both sites to see each other via their reverse tunnels (for up to 10 seconds)
	abortTime = time.Now().Add(time.Second * 10)
	for len(main.Tunnel.GetSites()) < 2 {
		time.Sleep(time.Millisecond * 2000)
		if time.Now().After(abortTime) {
			c.Fatalf("two clusters do not see each other: tunnels are not working")
		}
	}

	// connection and client should recover and work again
	output = &bytes.Buffer{}
	tc.Stdout = output
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

// TestDiscovery tests case for multiple proxies and a reverse tunnel
// agent that eventually connnects to the the right proxy
func (s *IntSuite) TestDiscovery(c *check.C) {
	username := s.me.Username

	// create load balancer for main cluster proxies
	frontend := *utils.MustParseAddr(fmt.Sprintf("127.0.0.1:%v", s.getPorts(1)[0]))
	lb, err := utils.NewLoadBalancer(context.TODO(), frontend)
	c.Assert(err, check.IsNil)
	c.Assert(lb.Listen(), check.IsNil)
	go lb.Serve()
	defer lb.Close()

	remote := NewInstance(InstanceConfig{ClusterName: "cluster-remote", HostID: HostID, NodeName: Host, Ports: s.getPorts(5), Priv: s.priv, Pub: s.pub})
	main := NewInstance(InstanceConfig{ClusterName: "cluster-main", HostID: HostID, NodeName: Host, Ports: s.getPorts(5), Priv: s.priv, Pub: s.pub})

	remote.AddUser(username, []string{username})
	main.AddUser(username, []string{username})

	c.Assert(main.Create(remote.Secrets.AsSlice(), false, nil), check.IsNil)
	mainSecrets := main.Secrets
	// switch listen address of the main cluster to load balancer
	mainProxyAddr := *utils.MustParseAddr(mainSecrets.ListenAddr)
	lb.AddBackend(mainProxyAddr)
	mainSecrets.ListenAddr = frontend.String()
	c.Assert(remote.Create(mainSecrets.AsSlice(), true, nil), check.IsNil)

	c.Assert(main.Start(), check.IsNil)
	c.Assert(remote.Start(), check.IsNil)

	// wait for both sites to see each other via their reverse tunnels (for up to 10 seconds)
	abortTime := time.Now().Add(time.Second * 10)
	for len(main.Tunnel.GetSites()) < 2 {
		time.Sleep(time.Millisecond * 2000)
		if time.Now().After(abortTime) {
			c.Fatalf("two clusters do not see each other: tunnels are not working")
		}
	}

	// start second proxy
	nodePorts := s.getPorts(3)
	proxyReverseTunnelPort, proxyWebPort, proxySSHPort := nodePorts[0], nodePorts[1], nodePorts[2]
	proxyConfig := ProxyConfig{
		Name:              "cluster-main-proxy",
		SSHPort:           proxySSHPort,
		WebPort:           proxyWebPort,
		ReverseTunnelPort: proxyReverseTunnelPort,
	}
	err = main.StartProxy(proxyConfig)
	c.Assert(err, check.IsNil)

	// add second proxy as a backend to the load balancer
	lb.AddBackend(*utils.MustParseAddr(fmt.Sprintf("127.0.0.1:%v", proxyReverseTunnelPort)))

	// At this point the remote cluster should be connected to two proxies in
	// the main cluster.
	waitForProxyCount(remote, "cluster-main", 2)

	// execute the connection via first proxy
	cfg := ClientConfig{
		Login:   username,
		Cluster: "cluster-remote",
		Host:    "127.0.0.1",
		Port:    remote.GetPortSSHInt(),
	}
	output, err := runCommand(main, []string{"echo", "hello world"}, cfg, 1)
	c.Assert(err, check.IsNil)
	c.Assert(output, check.Equals, "hello world\n")

	// Execute the connection via second proxy, should work. This command is
	// tried 10 times with 250 millisecond delay between each attempt to allow
	// the discovery request to be received and the connection added to the agent
	// pool.
	cfgProxy := ClientConfig{
		Login:   username,
		Cluster: "cluster-remote",
		Host:    "127.0.0.1",
		Port:    remote.GetPortSSHInt(),
		Proxy:   &proxyConfig,
	}
	output, err = runCommand(main, []string{"echo", "hello world"}, cfgProxy, 10)
	c.Assert(err, check.IsNil)
	c.Assert(output, check.Equals, "hello world\n")

	// now disconnect the main proxy and make sure it will reconnect eventually
	lb.RemoveBackend(mainProxyAddr)

	// requests going via main proxy will fail
	output, err = runCommand(main, []string{"echo", "hello world"}, cfg, 1)
	c.Assert(err, check.NotNil)

	// requests going via second proxy will succeed
	output, err = runCommand(main, []string{"echo", "hello world"}, cfgProxy, 1)
	c.Assert(err, check.IsNil)
	c.Assert(output, check.Equals, "hello world\n")

	// Connect the main proxy back and make sure agents have reconnected over time.
	// This command is tried 10 times with 250 millisecond delay between each
	// attempt to allow the discovery request to be received and the connection
	// added to the agent pool.
	lb.AddBackend(mainProxyAddr)
	output, err = runCommand(main, []string{"echo", "hello world"}, cfg, 10)
	c.Assert(err, check.IsNil)
	c.Assert(output, check.Equals, "hello world\n")

	// Stop one of proxies on the main cluster.
	err = main.StopProxy()
	c.Assert(err, check.IsNil)

	// Wait for the remote cluster to detect the outbound connection is gone.
	waitForProxyCount(remote, "cluster-main", 1)

	// Stop both clusters and remaining nodes.
	c.Assert(remote.Stop(true), check.IsNil)
	c.Assert(main.Stop(true), check.IsNil)
}

// waitForProxyCount waits a set time for the proxy count in clusterName to
// reach some value.
func waitForProxyCount(t *TeleInstance, clusterName string, count int) error {
	var counts map[string]int

	for i := 0; i < 20; i++ {
		counts = t.Pool.Counts()
		if counts[clusterName] == count {
			return nil
		}

		time.Sleep(250 * time.Millisecond)
	}

	return trace.BadParameter("proxy count on %v: %v", clusterName, counts[clusterName])
}

// TestExternalClient tests if we can connect to a node in a Teleport
// cluster. Both normal and recording proxies are tested.
func (s *IntSuite) TestExternalClient(c *check.C) {
	// Only run this test if we have access to the external SSH binary.
	_, err := exec.LookPath("ssh")
	if err != nil {
		c.Skip("Skipping TestExternalClient, no external SSH binary found.")
		return
	}

	var tests = []struct {
		inRecordLocation string
		inForwardAgent   bool
		inCommand        string
		outError         bool
		outExecOutput    string
	}{
		// Record at the node, forward agent. Will still work even though the agent
		// will be rejected by the proxy (agent forwarding request rejection is a
		// soft failure).
		{
			inRecordLocation: services.RecordAtNode,
			inForwardAgent:   true,
			inCommand:        "echo hello",
			outError:         false,
			outExecOutput:    "hello",
		},
		// Record at the node, don't forward agent, will work. This is the normal
		// Teleport mode of operation.
		{
			inRecordLocation: services.RecordAtNode,
			inForwardAgent:   false,
			inCommand:        "echo hello",
			outError:         false,
			outExecOutput:    "hello",
		},
		// Record at the proxy, forward agent. Will work.
		{
			inRecordLocation: services.RecordAtProxy,
			inForwardAgent:   true,
			inCommand:        "echo hello",
			outError:         false,
			outExecOutput:    "hello",
		},
		// Record at the proxy, don't forward agent, request will fail because
		// recording proxy requires an agent.
		{
			inRecordLocation: services.RecordAtProxy,
			inForwardAgent:   false,
			inCommand:        "echo hello",
			outError:         true,
			outExecOutput:    "",
		},
	}

	for _, tt := range tests {
		// Create a Teleport instance with auth, proxy, and node.
		makeConfig := func() (*check.C, []string, []*InstanceSecrets, *service.Config) {
			clusterConfig, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
				SessionRecording: tt.inRecordLocation,
			})
			c.Assert(err, check.IsNil)

			tconf := service.MakeDefaultConfig()
			tconf.Console = nil
			tconf.Auth.Enabled = true
			tconf.Auth.ClusterConfig = clusterConfig

			tconf.Proxy.Enabled = true
			tconf.Proxy.DisableWebService = true
			tconf.Proxy.DisableWebInterface = true

			tconf.SSH.Enabled = true

			return c, nil, nil, tconf
		}
		t := s.newTeleportWithConfig(makeConfig())
		defer t.Stop(true)

		// Start (and defer close) a agent that runs during this integration test.
		teleAgent, socketDirPath, socketPath, err := createAgent(
			s.me,
			t.Secrets.Users[s.me.Username].Key.Priv,
			t.Secrets.Users[s.me.Username].Key.Cert)
		c.Assert(err, check.IsNil)
		defer closeAgent(teleAgent, socketDirPath)

		// Create a *exec.Cmd that will execute the external SSH command.
		execCmd, err := externalSSHCommand(commandOptions{
			forwardAgent: tt.inForwardAgent,
			socketPath:   socketPath,
			proxyPort:    t.GetPortProxy(),
			nodePort:     t.GetPortSSH(),
			command:      tt.inCommand,
		})
		c.Assert(err, check.IsNil)

		// Execute SSH command and check the output is what we expect.
		output, err := execCmd.Output()
		if tt.outError {
			c.Assert(err, check.NotNil)
		} else {
			if err != nil {
				// If an *exec.ExitError is returned, parse it and return stderr. If this
				// is not done then c.Assert will just print a byte array for the error.
				er, ok := err.(*exec.ExitError)
				if ok {
					c.Fatalf("Unexpected error: %v", string(er.Stderr))
				}
			}
			c.Assert(err, check.IsNil)
			c.Assert(strings.TrimSpace(string(output)), check.Equals, tt.outExecOutput)
		}
	}
}

// TestControlMaster checks if multiple SSH channels can be created over the
// same connection. This is frequently used by tools like Ansible.
func (s *IntSuite) TestControlMaster(c *check.C) {
	// Only run this test if we have access to the external SSH binary.
	_, err := exec.LookPath("ssh")
	if err != nil {
		c.Skip("Skipping TestControlMaster, no external SSH binary found.")
		return
	}

	var tests = []struct {
		inRecordLocation string
	}{
		// Run tests when Teleport is recording sessions at the node.
		{
			inRecordLocation: services.RecordAtNode,
		},
		// Run tests when Teleport is recording sessions at the proxy.
		{
			inRecordLocation: services.RecordAtProxy,
		},
	}

	for _, tt := range tests {
		controlDir, err := ioutil.TempDir("", "teleport-")
		c.Assert(err, check.IsNil)
		defer os.RemoveAll(controlDir)
		controlPath := filepath.Join(controlDir, "control-path")

		// Create a Teleport instance with auth, proxy, and node.
		makeConfig := func() (*check.C, []string, []*InstanceSecrets, *service.Config) {
			clusterConfig, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
				SessionRecording: tt.inRecordLocation,
			})
			c.Assert(err, check.IsNil)

			tconf := service.MakeDefaultConfig()
			tconf.Console = nil
			tconf.Auth.Enabled = true
			tconf.Auth.ClusterConfig = clusterConfig

			tconf.Proxy.Enabled = true
			tconf.Proxy.DisableWebService = true
			tconf.Proxy.DisableWebInterface = true

			tconf.SSH.Enabled = true

			return c, nil, nil, tconf
		}
		t := s.newTeleportWithConfig(makeConfig())
		defer t.Stop(true)

		// Start (and defer close) a agent that runs during this integration test.
		teleAgent, socketDirPath, socketPath, err := createAgent(
			s.me,
			t.Secrets.Users[s.me.Username].Key.Priv,
			t.Secrets.Users[s.me.Username].Key.Cert)
		c.Assert(err, check.IsNil)
		defer closeAgent(teleAgent, socketDirPath)

		// Create and run an exec command twice with the passed in ControlPath. This
		// will cause re-use of the connection and creation of two sessions within
		// the connection.
		for i := 0; i < 2; i++ {
			execCmd, err := externalSSHCommand(commandOptions{
				forcePTY:     true,
				forwardAgent: true,
				controlPath:  controlPath,
				socketPath:   socketPath,
				proxyPort:    t.GetPortProxy(),
				nodePort:     t.GetPortSSH(),
				command:      "echo hello",
			})
			c.Assert(err, check.IsNil)

			// Execute SSH command and check the output is what we expect.
			output, err := execCmd.Output()
			if err != nil {
				// If an *exec.ExitError is returned, parse it and return stderr. If this
				// is not done then c.Assert will just print a byte array for the error.
				er, ok := err.(*exec.ExitError)
				if ok {
					c.Fatalf("Unexpected error: %v", string(er.Stderr))
				}
			}
			c.Assert(err, check.IsNil)
			c.Assert(strings.TrimSpace(string(output)), check.Equals, "hello")
		}
	}
}

// TestProxyHostKeyCheck uses the forwarding proxy to connect to a server that
// presents a host key instead of a certificate in different configurations
// for the host key checking parameter in services.ClusterConfig.
func (s *IntSuite) TestProxyHostKeyCheck(c *check.C) {
	var tests = []struct {
		inHostKeyCheck string
		outError       bool
	}{
		// disable host key checking, should be able to connect
		{
			services.HostKeyCheckNo,
			false,
		},
		// enable host key checking, should NOT be able to connect
		{
			services.HostKeyCheckYes,
			true,
		},
	}

	for _, tt := range tests {
		hostSigner, err := ssh.ParsePrivateKey(s.priv)
		c.Assert(err, check.IsNil)

		// start a ssh server that presents a host key instead of a certificate
		nodePort := s.getPorts(1)[0]
		sshNode, err := newDiscardServer(Host, nodePort, hostSigner)
		c.Assert(err, check.IsNil)
		err = sshNode.Start()
		c.Assert(err, check.IsNil)
		defer sshNode.Stop()

		// create a teleport instance with auth, proxy, and node
		makeConfig := func() (*check.C, []string, []*InstanceSecrets, *service.Config) {
			clusterConfig, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
				SessionRecording:    services.RecordAtProxy,
				ProxyChecksHostKeys: tt.inHostKeyCheck,
			})
			c.Assert(err, check.IsNil)

			tconf := service.MakeDefaultConfig()
			tconf.Console = nil
			tconf.Auth.Enabled = true
			tconf.Auth.ClusterConfig = clusterConfig

			tconf.Proxy.Enabled = true
			tconf.Proxy.DisableWebService = true
			tconf.Proxy.DisableWebInterface = true

			return c, nil, nil, tconf
		}
		t := s.newTeleportWithConfig(makeConfig())
		defer t.Stop(true)

		// create a teleport client and exec a command
		clientConfig := ClientConfig{
			Login:        s.me.Username,
			Cluster:      Site,
			Host:         Host,
			Port:         nodePort,
			ForwardAgent: true,
		}
		_, err = runCommand(t, []string{"echo hello"}, clientConfig, 1)

		// check if we were able to exec the command or not
		if tt.outError {
			c.Assert(err, check.NotNil)
		} else {
			c.Assert(err, check.IsNil)
		}
	}
}

// TestAuditOff checks that when session recording has been turned off,
// sessions are not recorded.
func (s *IntSuite) TestAuditOff(c *check.C) {
	var err error

	// create a teleport instance with auth, proxy, and node
	makeConfig := func() (*check.C, []string, []*InstanceSecrets, *service.Config) {
		clusterConfig, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
			SessionRecording: services.RecordOff,
		})
		c.Assert(err, check.IsNil)

		tconf := service.MakeDefaultConfig()
		tconf.Console = nil
		tconf.Auth.Enabled = true
		tconf.Auth.ClusterConfig = clusterConfig

		tconf.Proxy.Enabled = true
		tconf.Proxy.DisableWebService = true
		tconf.Proxy.DisableWebInterface = true

		tconf.SSH.Enabled = true

		return c, nil, nil, tconf
	}
	t := s.newTeleportWithConfig(makeConfig())
	defer t.Stop(true)

	// get access to a authClient for the cluster
	site := t.GetSiteAPI(Site)
	c.Assert(site, check.NotNil)

	// should have no sessions in it to start with
	sessions, _ := site.GetSessions(defaults.Namespace)
	c.Assert(len(sessions), check.Equals, 0)

	// create interactive session (this goroutine is this user's terminal time)
	endCh := make(chan error, 1)

	myTerm := NewTerminal(250)
	go func() {
		cl, err := t.NewClient(ClientConfig{
			Login:   s.me.Username,
			Cluster: Site,
			Host:    Host,
			Port:    t.GetPortSSHInt(),
		})
		c.Assert(err, check.IsNil)
		cl.Stdout = &myTerm
		cl.Stdin = &myTerm
		err = cl.SSH(context.TODO(), []string{}, false)
		endCh <- err
	}()

	// wait until there's a session in there:
	for i := 0; len(sessions) == 0; i++ {
		time.Sleep(time.Millisecond * 20)
		sessions, _ = site.GetSessions(defaults.Namespace)
		if i > 100 {
			c.Fatalf("Waited %v, but no sessions found", 100*20*time.Millisecond)
			return
		}
	}
	session := &sessions[0]

	// wait for the user to join this session
	for len(session.Parties) == 0 {
		time.Sleep(time.Millisecond * 5)
		session, err = site.GetSession(defaults.Namespace, sessions[0].ID)
		c.Assert(err, check.IsNil)
	}
	// make sure it's us who joined! :)
	c.Assert(session.Parties[0].User, check.Equals, s.me.Username)

	// lets type "echo hi" followed by "enter" and then "exit" + "enter":
	myTerm.Type("\aecho hi\n\r\aexit\n\r\a")

	// wait for session to end
	select {
	case <-time.After(1 * time.Minute):
		c.Fatalf("Timed out waiting for session to end.")
	case <-endCh:
	}

	// audit log should have the fact that the session occured recorded in it
	sessions, err = site.GetSessions(defaults.Namespace)
	c.Assert(err, check.IsNil)
	c.Assert(len(sessions), check.Equals, 1)

	// however, attempts to read the actual sessions should fail because it was
	// not actually recorded
	_, err = site.GetSessionChunk(defaults.Namespace, session.ID, 0, events.MaxChunkBytes)
	c.Assert(err, check.NotNil)
}

// TestPAM checks that Teleport PAM integration works correctly. In this case
// that means if the account and session modules return success, the user
// should be allowed to log in. If either the account or session module does
// not return success, the user should not be able to log in.
func (s *IntSuite) TestPAM(c *check.C) {
	// Check if TestPAM can run. For PAM tests to run, the binary must have been
	// built with PAM support and the system running the tests must have libpam
	// installed, and have the policy files installed. This test is always run
	// in a container as part of the CI/CD pipeline. To run this test locally,
	// install the pam_teleport.so module by running 'make && sudo make install'
	// from the modules/pam_teleport directory. This will install the PAM module
	// as well as the policy files.
	if !pam.BuildHasPAM() || !pam.SystemHasPAM() || !hasPAMPolicy() {
		skipMessage := "Skipping TestPAM: no policy found. To run PAM tests run " +
			"'make && sudo make install' from the modules/pam_teleport directory."
		c.Skip(skipMessage)
	}

	var tests = []struct {
		inEnabled     bool
		inServiceName string
		outContains   []string
		outError      bool
	}{
		// 0 - No PAM support, session should work but no PAM related output.
		{
			inEnabled:     false,
			inServiceName: "",
			outContains:   []string{},
			outError:      false,
		},
		// 1 - PAM enabled, module account and session functions return success.
		{
			inEnabled:     true,
			inServiceName: "teleport-success",
			outContains: []string{
				"Account opened successfully.",
				"Session open successfully.",
			},
			outError: false,
		},
		// 2 - PAM enabled, module account functions fail.
		{
			inEnabled:     true,
			inServiceName: "teleport-acct-failure",
			outContains:   []string{},
			outError:      true,
		},
		// 3 - PAM enabled, module session functions fail.
		{
			inEnabled:     true,
			inServiceName: "teleport-session-failure",
			outContains:   []string{},
			outError:      true,
		},
	}

	for _, tt := range tests {
		// Create a teleport instance with auth, proxy, and node.
		makeConfig := func() (*check.C, []string, []*InstanceSecrets, *service.Config) {
			tconf := service.MakeDefaultConfig()
			tconf.Console = nil
			tconf.Auth.Enabled = true

			tconf.Proxy.Enabled = true
			tconf.Proxy.DisableWebService = true
			tconf.Proxy.DisableWebInterface = true

			tconf.SSH.Enabled = true
			tconf.SSH.PAM.Enabled = tt.inEnabled
			tconf.SSH.PAM.ServiceName = tt.inServiceName

			return c, nil, nil, tconf
		}
		t := s.newTeleportWithConfig(makeConfig())
		defer t.Stop(true)

		termSession := NewTerminal(250)

		// Create an interactive session and write something to the terminal.
		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			cl, err := t.NewClient(ClientConfig{
				Login:   s.me.Username,
				Cluster: Site,
				Host:    Host,
				Port:    t.GetPortSSHInt(),
			})
			c.Assert(err, check.IsNil)

			cl.Stdout = &termSession
			cl.Stdin = &termSession

			termSession.Type("\aecho hi\n\r\aexit\n\r\a")
			err = cl.SSH(context.TODO(), []string{}, false)

			// If an error is expected (for example PAM does not allow a session to be
			// created), this failure needs to be checked here.
			if tt.outError {
				c.Assert(err, check.NotNil)
			} else {
				c.Assert(err, check.IsNil)
			}

			cancel()
		}()

		// Wait for the session to end or timeout after 10 seconds.
		select {
		case <-time.After(10 * time.Second):
			c.Fatalf("Timeout exceeded waiting for session to complete.")
		case <-ctx.Done():
		}

		// If any output is expected, check to make sure it was output.
		for _, expectedOutput := range tt.outContains {
			output := string(termSession.Output(100))
			c.Assert(strings.Contains(output, expectedOutput), check.Equals, true)
		}
	}
}

// TestRotateSuccess tests full cycle cert authority rotation
func (s *IntSuite) TestRotateSuccess(c *check.C) {
	for i := 0; i < getIterations(); i++ {
		s.rotateSuccess(c)
	}
}

func (s *IntSuite) rotateSuccess(c *check.C) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tconf := rotationConfig(true)
	t := NewInstance(InstanceConfig{ClusterName: Site, HostID: HostID, NodeName: Host, Ports: s.getPorts(5), Priv: s.priv, Pub: s.pub})
	logins := []string{s.me.Username}
	for _, login := range logins {
		t.AddUser(login, []string{login})
	}
	config, err := t.GenerateConfig(nil, tconf)
	c.Assert(err, check.IsNil)

	serviceC := make(chan *service.TeleportProcess, 20)

	runCtx, runCancel := context.WithCancel(context.TODO())
	go func() {
		defer runCancel()
		service.Run(ctx, *config, func(cfg *service.Config) (service.Process, error) {
			svc, err := service.NewTeleport(cfg)
			if err == nil {
				serviceC <- svc
			}
			return svc, err
		})
	}()

	l := log.WithFields(log.Fields{trace.Component: teleport.Component("test", "rotate")})

	svc, err := waitForProcessStart(serviceC)
	c.Assert(err, check.IsNil)

	// Setup user in the cluster
	err = SetupUser(svc, s.me.Username, nil)
	c.Assert(err, check.IsNil)

	// capture credentials before reload started to simulate old client
	initialCreds, err := GenerateUserCreds(svc, s.me.Username)
	c.Assert(err, check.IsNil)

	l.Infof("Service started. Setting rotation state to %v", services.RotationPhaseUpdateClients)

	// start rotation
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: services.RotationPhaseInit,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// wait until service phase update to be broadcasted (init phase does not trigger reload)
	err = waitForProcessEvent(svc, service.TeleportPhaseChangeEvent, 10*time.Second)
	c.Assert(err, check.IsNil)

	// update clients
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: services.RotationPhaseUpdateClients,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// wait until service reload
	svc, err = waitForReload(serviceC, svc)
	c.Assert(err, check.IsNil)

	cfg := ClientConfig{
		Login: s.me.Username,
		Host:  "127.0.0.1",
		Port:  t.GetPortSSHInt(),
	}
	clt, err := t.NewClientWithCreds(cfg, *initialCreds)
	c.Assert(err, check.IsNil)

	// client works as is before servers have been rotated
	err = runAndMatch(clt, 3, []string{"echo", "hello world"}, ".*hello world.*")
	c.Assert(err, check.IsNil)

	l.Infof("Service reloaded. Setting rotation state to %v", services.RotationPhaseUpdateServers)

	// move to the next phase
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: services.RotationPhaseUpdateServers,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// wait until service reloaded
	svc, err = waitForReload(serviceC, svc)
	c.Assert(err, check.IsNil)

	// new credentials will work from this phase to others
	newCreds, err := GenerateUserCreds(svc, s.me.Username)
	c.Assert(err, check.IsNil)

	clt, err = t.NewClientWithCreds(cfg, *newCreds)
	c.Assert(err, check.IsNil)

	// new client works
	err = runAndMatch(clt, 3, []string{"echo", "hello world"}, ".*hello world.*")
	c.Assert(err, check.IsNil)

	l.Infof("Service reloaded. Setting rotation state to %v.", services.RotationPhaseStandby)

	// complete rotation
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: services.RotationPhaseStandby,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// wait until service reloaded
	svc, err = waitForReload(serviceC, svc)
	c.Assert(err, check.IsNil)

	// new client still works
	err = runAndMatch(clt, 3, []string{"echo", "hello world"}, ".*hello world.*")
	c.Assert(err, check.IsNil)

	l.Infof("Service reloaded. Rotation has completed. Shuttting down service.")

	// shut down the service
	cancel()
	// close the service without waiting for the connections to drain
	svc.Close()

	select {
	case <-runCtx.Done():
	case <-time.After(20 * time.Second):
		c.Fatalf("failed to shut down the server")
	}
}

// TestRotateRollback tests cert authority rollback
func (s *IntSuite) TestRotateRollback(c *check.C) {
	for i := 0; i < getIterations(); i++ {
		s.rotateRollback(c)
	}
}

func (s *IntSuite) rotateRollback(c *check.C) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tconf := rotationConfig(true)
	t := NewInstance(InstanceConfig{ClusterName: Site, HostID: HostID, NodeName: Host, Ports: s.getPorts(5), Priv: s.priv, Pub: s.pub})
	logins := []string{s.me.Username}
	for _, login := range logins {
		t.AddUser(login, []string{login})
	}
	config, err := t.GenerateConfig(nil, tconf)
	c.Assert(err, check.IsNil)

	serviceC := make(chan *service.TeleportProcess, 20)

	runCtx, runCancel := context.WithCancel(context.TODO())
	go func() {
		defer runCancel()
		service.Run(ctx, *config, func(cfg *service.Config) (service.Process, error) {
			svc, err := service.NewTeleport(cfg)
			if err == nil {
				serviceC <- svc
			}
			return svc, err
		})
	}()

	l := log.WithFields(log.Fields{trace.Component: teleport.Component("test", "rotate")})

	svc, err := waitForProcessStart(serviceC)
	c.Assert(err, check.IsNil)

	// Setup user in the cluster
	err = SetupUser(svc, s.me.Username, nil)
	c.Assert(err, check.IsNil)

	// capture credentials before reload started to simulate old client
	initialCreds, err := GenerateUserCreds(svc, s.me.Username)
	c.Assert(err, check.IsNil)

	l.Infof("Service started. Setting rotation state to %v", services.RotationPhaseInit)

	// start rotation
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: services.RotationPhaseInit,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	err = waitForProcessEvent(svc, service.TeleportPhaseChangeEvent, 10*time.Second)
	c.Assert(err, check.IsNil)

	l.Infof("Setting rotation state to %v", services.RotationPhaseUpdateClients)

	// start rotation
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: services.RotationPhaseUpdateClients,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// wait until service reload
	svc, err = waitForReload(serviceC, svc)
	c.Assert(err, check.IsNil)

	cfg := ClientConfig{
		Login: s.me.Username,
		Host:  "127.0.0.1",
		Port:  t.GetPortSSHInt(),
	}
	clt, err := t.NewClientWithCreds(cfg, *initialCreds)
	c.Assert(err, check.IsNil)

	// client works as is before servers have been rotated
	err = runAndMatch(clt, 3, []string{"echo", "hello world"}, ".*hello world.*")
	c.Assert(err, check.IsNil)

	l.Infof("Service reloaded. Setting rotation state to %v", services.RotationPhaseUpdateServers)

	// move to the next phase
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: services.RotationPhaseUpdateServers,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// wait until service reloaded
	svc, err = waitForReload(serviceC, svc)
	c.Assert(err, check.IsNil)

	l.Infof("Service reloaded. Setting rotation state to %v.", services.RotationPhaseRollback)

	// complete rotation
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: services.RotationPhaseRollback,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// wait until service reloaded
	svc, err = waitForReload(serviceC, svc)
	c.Assert(err, check.IsNil)

	// old client works
	err = runAndMatch(clt, 3, []string{"echo", "hello world"}, ".*hello world.*")
	c.Assert(err, check.IsNil)

	l.Infof("Service reloaded. Rotation has completed. Shuttting down service.")

	// shut down the service
	cancel()
	// close the service without waiting for the connections to drain
	svc.Close()

	select {
	case <-runCtx.Done():
	case <-time.After(20 * time.Second):
		c.Fatalf("failed to shut down the server")
	}
}

// getIterations provides a simple way to add iterations to the test
// by setting environment variable "ITERATIONS", by default it returns 1
func getIterations() int {
	out := os.Getenv("ITERATIONS")
	if out == "" {
		return 1
	}
	iter, err := strconv.Atoi(out)
	if err != nil {
		panic(err)
	}
	log.Debugf("Starting tests with %v iterations.", iter)
	return iter
}

// TestRotateTrustedClusters tests CA rotation support for trusted clusters
func (s *IntSuite) TestRotateTrustedClusters(c *check.C) {
	for i := 0; i < getIterations(); i++ {
		s.rotateTrustedClusters(c)
	}
}

// rotateTrustedClusters tests CA rotation support for trusted clusters
func (s *IntSuite) rotateTrustedClusters(c *check.C) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clusterMain := "rotate-main"
	clusterAux := "rotate-aux"

	tconf := rotationConfig(false)
	main := NewInstance(InstanceConfig{ClusterName: clusterMain, HostID: HostID, NodeName: Host, Ports: s.getPorts(5), Priv: s.priv, Pub: s.pub})
	aux := NewInstance(InstanceConfig{ClusterName: clusterAux, HostID: HostID, NodeName: Host, Ports: s.getPorts(5), Priv: s.priv, Pub: s.pub})

	logins := []string{s.me.Username}
	for _, login := range logins {
		main.AddUser(login, []string{login})
	}
	config, err := main.GenerateConfig(nil, tconf)
	c.Assert(err, check.IsNil)

	serviceC := make(chan *service.TeleportProcess, 20)
	runCtx, runCancel := context.WithCancel(context.TODO())
	go func() {
		defer runCancel()
		service.Run(ctx, *config, func(cfg *service.Config) (service.Process, error) {
			svc, err := service.NewTeleport(cfg)
			if err == nil {
				serviceC <- svc
			}
			return svc, err
		})
	}()

	l := log.WithFields(log.Fields{trace.Component: teleport.Component("test", "rotate")})

	svc, err := waitForProcessStart(serviceC)
	c.Assert(err, check.IsNil)

	// main cluster has a local user and belongs to role "main-devs"
	mainDevs := "main-devs"
	role, err := services.NewRole(mainDevs, services.RoleSpecV3{
		Allow: services.RoleConditions{
			Logins: []string{s.me.Username},
		},
	})
	c.Assert(err, check.IsNil)

	err = SetupUser(svc, s.me.Username, []services.Role{role})
	c.Assert(err, check.IsNil)

	// create auxillary cluster and setup trust
	c.Assert(aux.CreateEx(nil, rotationConfig(false)), check.IsNil)

	// auxiliary cluster has a role aux-devs
	// connect aux cluster to main cluster
	// using trusted clusters, so remote user will be allowed to assume
	// role specified by mapping remote role "devs" to local role "local-devs"
	auxDevs := "aux-devs"
	role, err = services.NewRole(auxDevs, services.RoleSpecV3{
		Allow: services.RoleConditions{
			Logins: []string{s.me.Username},
		},
	})
	c.Assert(err, check.IsNil)
	err = aux.Process.GetAuthServer().UpsertRole(role, backend.Forever)
	c.Assert(err, check.IsNil)
	trustedClusterToken := "trusted-clsuter-token"
	err = svc.GetAuthServer().UpsertToken(trustedClusterToken, []teleport.Role{teleport.RoleTrustedCluster}, backend.Forever)
	c.Assert(err, check.IsNil)
	trustedCluster := main.Secrets.AsTrustedCluster(trustedClusterToken, services.RoleMap{
		{Remote: mainDevs, Local: []string{auxDevs}},
	})
	c.Assert(aux.Start(), check.IsNil)

	// try and upsert a trusted cluster
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)
	var upsertSuccess bool
	for i := 0; i < 10; i++ {
		log.Debugf("Will create trusted cluster %v, attempt %v", trustedCluster, i)
		_, err = aux.Process.GetAuthServer().UpsertTrustedCluster(trustedCluster)
		if err != nil {
			if trace.IsConnectionProblem(err) {
				log.Debugf("retrying on connection problem: %v", err)
				continue
			}
			c.Fatalf("got non connection problem %v", err)
		}
		upsertSuccess = true
		break
	}
	// make sure we upsert a trusted cluster
	c.Assert(upsertSuccess, check.Equals, true)

	// capture credentials before has reload started to simulate old client
	initialCreds, err := GenerateUserCreds(svc, s.me.Username)
	c.Assert(err, check.IsNil)

	// credentials should work
	cfg := ClientConfig{
		Login:   s.me.Username,
		Host:    "127.0.0.1",
		Cluster: clusterAux,
		Port:    aux.GetPortSSHInt(),
	}
	clt, err := main.NewClientWithCreds(cfg, *initialCreds)
	c.Assert(err, check.IsNil)

	err = runAndMatch(clt, 6, []string{"echo", "hello world"}, ".*hello world.*")
	c.Assert(err, check.IsNil)

	l.Infof("Setting rotation state to %v", services.RotationPhaseInit)

	// start rotation
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: services.RotationPhaseInit,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// wait until service phase update to be broadcasted (init phase does not trigger reload)
	err = waitForProcessEvent(svc, service.TeleportPhaseChangeEvent, 10*time.Second)
	c.Assert(err, check.IsNil)

	// waitForPhase waits until aux cluster detects the rotation
	waitForPhase := func(phase string) error {
		var lastPhase string
		for i := 0; i < 10; i++ {
			ca, err := aux.Process.GetAuthServer().GetCertAuthority(services.CertAuthID{
				Type:       services.HostCA,
				DomainName: clusterMain,
			}, false)
			c.Assert(err, check.IsNil)
			if ca.GetRotation().Phase == phase {
				return nil
			}
			lastPhase = ca.GetRotation().Phase
			time.Sleep(tconf.PollingPeriod / 2)
		}
		return trace.CompareFailed("failed to converge to phase %q, last phase %q", phase, lastPhase)
	}

	err = waitForPhase(services.RotationPhaseInit)
	c.Assert(err, check.IsNil)

	// update clients
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: services.RotationPhaseUpdateClients,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// wait until service reloaded
	svc, err = waitForReload(serviceC, svc)
	c.Assert(err, check.IsNil)

	err = waitForPhase(services.RotationPhaseUpdateClients)
	c.Assert(err, check.IsNil)

	// old client should work as is
	err = runAndMatch(clt, 6, []string{"echo", "hello world"}, ".*hello world.*")
	c.Assert(err, check.IsNil)

	l.Infof("Service reloaded. Setting rotation state to %v", services.RotationPhaseUpdateServers)

	// move to the next phase
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: services.RotationPhaseUpdateServers,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// wait until service reloaded
	svc, err = waitForReload(serviceC, svc)
	c.Assert(err, check.IsNil)

	err = waitForPhase(services.RotationPhaseUpdateServers)
	c.Assert(err, check.IsNil)

	// new credentials will work from this phase to others
	newCreds, err := GenerateUserCreds(svc, s.me.Username)
	c.Assert(err, check.IsNil)

	clt, err = main.NewClientWithCreds(cfg, *newCreds)
	c.Assert(err, check.IsNil)

	// new client works
	err = runAndMatch(clt, 3, []string{"echo", "hello world"}, ".*hello world.*")
	c.Assert(err, check.IsNil)

	l.Infof("Service reloaded. Setting rotation state to %v.", services.RotationPhaseStandby)

	// complete rotation
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: services.RotationPhaseStandby,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// wait until service reloaded
	svc, err = waitForReload(serviceC, svc)
	c.Assert(err, check.IsNil)

	err = waitForPhase(services.RotationPhaseStandby)
	c.Assert(err, check.IsNil)

	// new client still works
	err = runAndMatch(clt, 3, []string{"echo", "hello world"}, ".*hello world.*")
	c.Assert(err, check.IsNil)

	l.Infof("Service reloaded. Rotation has completed. Shuttting down service.")

	// shut down the service
	cancel()
	// close the service without waiting for the connections to drain
	svc.Close()

	select {
	case <-runCtx.Done():
	case <-time.After(20 * time.Second):
		c.Fatalf("failed to shut down the server")
	}
}

// rotationConfig sets up default config used for CA rotation tests
func rotationConfig(disableWebService bool) *service.Config {
	tconf := service.MakeDefaultConfig()
	tconf.SSH.Enabled = true
	tconf.Proxy.DisableWebService = disableWebService
	tconf.Proxy.DisableWebInterface = true
	tconf.PollingPeriod = 500 * time.Millisecond
	tconf.ClientTimeout = time.Second
	tconf.ShutdownTimeout = 2 * tconf.ClientTimeout
	return tconf
}

// waitForProcessEvent waits for process event to occur or timeout
func waitForProcessEvent(svc *service.TeleportProcess, event string, timeout time.Duration) error {
	eventC := make(chan service.Event, 1)
	svc.WaitForEvent(context.TODO(), event, eventC)
	select {
	case <-eventC:
		return nil
	case <-time.After(timeout):
		return trace.BadParameter("timeout waiting for service to broadcast event %v", event)
	}
}

// waitForProcessStart is waiting for the process to start
func waitForProcessStart(serviceC chan *service.TeleportProcess) (*service.TeleportProcess, error) {
	var svc *service.TeleportProcess
	select {
	case svc = <-serviceC:
	case <-time.After(60 * time.Second):
		return nil, trace.BadParameter("timeout waiting for service to start")
	}
	return svc, nil
}

// waitForReload waits for multiple events to happen:
//
// 1. new service to be created and started
// 2. old service, if present to shut down
//
// this helper function allows to serialize tests for reloads.
func waitForReload(serviceC chan *service.TeleportProcess, old *service.TeleportProcess) (*service.TeleportProcess, error) {
	var svc *service.TeleportProcess
	select {
	case svc = <-serviceC:
	case <-time.After(60 * time.Second):
		return nil, trace.BadParameter("timeout waiting for service to start")
	}

	eventC := make(chan service.Event, 1)
	svc.WaitForEvent(context.TODO(), service.TeleportReadyEvent, eventC)
	select {
	case <-eventC:

	case <-time.After(20 * time.Second):
		return nil, trace.BadParameter("timeout waiting for service to broadcast ready status")
	}

	// if old service is present, wait for it to complete shut down procedure
	if old != nil {
		ctx, cancel := context.WithCancel(context.TODO())
		go func() {
			defer cancel()
			old.Supervisor.Wait()
		}()
		select {
		case <-ctx.Done():
		case <-time.After(60 * time.Second):
			return nil, trace.BadParameter("timeout waiting for old service to stop")
		}
	}
	return svc, nil

}

// runAndMatch runs command and makes sure it matches the pattern
func runAndMatch(tc *client.TeleportClient, attempts int, command []string, pattern string) error {
	output := &bytes.Buffer{}
	tc.Stdout = output
	var err error
	for i := 0; i < attempts; i++ {
		err = tc.SSH(context.TODO(), command, false)
		if err != nil {
			continue
		}
		out := output.String()
		out = string(replaceNewlines(out))
		matched, _ := regexp.MatchString(pattern, out)
		if matched {
			return nil
		}
		err = trace.CompareFailed("output %q did not match pattern %q", out, pattern)
		time.Sleep(250 * time.Millisecond)
	}
	return err
}

// TestWindowChange checks if custom Teleport window change requests are sent
// when the server side PTY changes its size.
func (s *IntSuite) TestWindowChange(c *check.C) {
	t := s.newTeleport(c, nil, true)
	defer t.Stop(true)

	site := t.GetSiteAPI(Site)
	c.Assert(site, check.NotNil)

	personA := NewTerminal(250)
	personB := NewTerminal(250)

	// openSession will open a new session on a server.
	openSession := func() {
		cl, err := t.NewClient(ClientConfig{
			Login:   s.me.Username,
			Cluster: Site,
			Host:    Host,
			Port:    t.GetPortSSHInt(),
		})
		c.Assert(err, check.IsNil)

		cl.Stdout = &personA
		cl.Stdin = &personA

		err = cl.SSH(context.TODO(), []string{}, false)
		c.Assert(err, check.IsNil)
	}

	// joinSession will join the existing session on a server.
	joinSession := func() {
		// Find the existing session in the backend.
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

		cl, err := t.NewClient(ClientConfig{
			Login:   s.me.Username,
			Cluster: Site,
			Host:    Host,
			Port:    t.GetPortSSHInt(),
		})
		c.Assert(err, check.IsNil)

		cl.Stdout = &personB
		cl.Stdin = &personB

		// Change the size of the window immediately after it is created.
		cl.OnShellCreated = func(s *ssh.Session, c *ssh.Client, terminal io.ReadWriteCloser) (exit bool, err error) {
			err = s.WindowChange(48, 160)
			if err != nil {
				return true, trace.Wrap(err)
			}
			return false, nil
		}

		for i := 0; i < 10; i++ {
			err = cl.Join(context.TODO(), defaults.Namespace, session.ID(sessionID), &personB)
			if err == nil {
				break
			}
		}
		c.Assert(err, check.IsNil)
	}

	// waitForOutput checks the output of the passed in terminal of a string until
	// some timeout has occured.
	waitForOutput := func(t Terminal, s string) error {
		tickerCh := time.Tick(500 * time.Millisecond)
		timeoutCh := time.After(30 * time.Second)
		for {
			select {
			case <-tickerCh:
				if strings.Contains(t.Output(500), s) {
					return nil
				}
			case <-timeoutCh:
				return trace.BadParameter("timed out waiting for output")
			}
		}

	}

	// Open session, the initial size will be 80x24.
	go openSession()

	// Use the "printf" command to print the terminal size on the screen and
	// make sure it is 80x25.
	personA.Type("\aprintf '%s %s\n' $(tput cols) $(tput lines)\n\r\a")
	err := waitForOutput(personA, "80 25")
	c.Assert(err, check.IsNil)

	// As soon as person B joins the session, the terminal is resized to 160x48.
	// Have another user join the session. As soon as the second shell is
	// created, the window is resized to 160x48 (see joinSession implementation).
	go joinSession()

	// Use the "printf" command to print the window size again and make sure it's
	// 160x48.
	personA.Type("\aprintf '%s %s\n' $(tput cols) $(tput lines)\n\r\a")
	err = waitForOutput(personA, "160 48")
	c.Assert(err, check.IsNil)

	// Close the session.
	personA.Type("\aexit\r\n\a")
}

// runCommand is a shortcut for running SSH command, it creates a client
// connected to proxy of the passed in instance, runs the command, and returns
// the result. If multiple attempts are requested, a 250 millisecond delay is
// added between them before giving up.
func runCommand(instance *TeleInstance, cmd []string, cfg ClientConfig, attempts int) (string, error) {
	tc, err := instance.NewClient(cfg)
	if err != nil {
		return "", trace.Wrap(err)
	}
	output := &bytes.Buffer{}
	tc.Stdout = output
	for i := 0; i < attempts; i++ {
		err = tc.SSH(context.TODO(), cmd, false)
		if err == nil {
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	return output.String(), trace.Wrap(err)
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

// hasPAMPolicy checks if the three policy files needed for tests exists. If
// they do it returns true, otherwise returns false.
func hasPAMPolicy() bool {
	pamPolicyFiles := []string{
		"/etc/pam.d/teleport-acct-failure",
		"/etc/pam.d/teleport-session-failure",
		"/etc/pam.d/teleport-success",
	}

	for _, fileName := range pamPolicyFiles {
		_, err := os.Stat(fileName)
		if os.IsNotExist(err) {
			return false
		}
	}

	return true
}
