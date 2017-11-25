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
	"io/ioutil"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
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

	AllocatePortsNum = 200
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
// with the current user os.user.Current().
func (s *IntSuite) newTeleport(c *check.C, logins []string, enableSSH bool) *TeleInstance {
	t := NewInstance(Site, HostID, Host, s.getPorts(5), s.priv, s.pub)
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
	if err := t.Start(); err != nil {
		c.Fatalf("Unexpected response from Start: %v", err)
	}
	return t
}

// newTeleportWithConfig is a helper function that will create a running
// Teleport instance with the passed in user, instance secrets, and Teleport
// configuration.
func (s *IntSuite) newTeleportWithConfig(c *check.C, logins []string, instanceSecrets []*InstanceSecrets, teleportConfig *service.Config) *TeleInstance {
	t := NewInstance(Site, HostID, Host, s.getPorts(5), s.priv, s.pub)

	// use passed logins, but use suite's default login if nothing was passed
	if logins == nil || len(logins) == 0 {
		logins = []string{s.me.Username}
	}
	for _, login := range logins {
		t.AddUser(login, []string{login})
	}

	// create a new teleport instance with passed in configuration
	if err := t.CreateEx(instanceSecrets, teleportConfig); err != nil {
		c.Fatalf("Unexpected response from CreateEx: %v", err)
	}
	if err := t.Start(); err != nil {
		c.Fatalf("Unexpected response from Start: %v", err)
	}

	return t
}

// TestAudit creates a live session, records a bunch of data through it (>5MB)
// and then reads it back and compares against simulated reality.
func (s *IntSuite) TestAudit(c *check.C) {
	var tests = []struct {
		inRecordLocation string
		inForwardAgent   bool
	}{
		// normal teleport
		{
			services.RecordAtNode,
			false,
		},
		// recording proxy
		{
			services.RecordAtProxy,
			true,
		},
	}

	for _, tt := range tests {
		makeConfig := func() (*check.C, []string, []*InstanceSecrets, *service.Config) {
			clusterConfig, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
				SessionRecording: tt.inRecordLocation,
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
					if err != nil {
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
					if len(sessions) > 0 {
						return &sessions[0], nil
					}
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

		// if session are being recorded at nodes, the event server_id field should contain
		// the ID of the node. if sessions are being recorded at the proxy, then server_id
		// should be that of the proxy
		expectedServerID := nodeProcess.Config.HostUUID
		if tt.inRecordLocation == services.RecordAtProxy {
			expectedServerID = t.Process.Config.HostUUID
		}
		c.Assert(start.GetString(events.SessionServerID), check.Equals, expectedServerID)

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

		a := NewInstance("site-A", HostID, Host, s.getPorts(5), s.priv, s.pub)
		b := NewInstance("site-B", HostID, Host, s.getPorts(5), s.priv, s.pub)

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
			time.Sleep(time.Millisecond * 5)
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
					eventsInSite, err := site.SearchEvents(now, now.Add(1*time.Hour), execQuery)
					if err != nil {
						return trace.Wrap(err)
					}

					// found the number of events we were looking for
					if got, want := len(eventsInSite), count; got == want {
						return nil
					}
				case <-stopCh:
					return trace.BadParameter("unable to find %v events after 5s: %v", count)
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
	c.Assert(a.StartNodeAndProxy("cluster-a-node", sshPort, proxyWebPort, proxySSHPort), check.IsNil)

	// wait for both sites to see each other via their reverse tunnels (for up to 10 seconds)
	abortTime := time.Now().Add(time.Second * 10)
	for len(b.Tunnel.GetSites()) < 2 && len(b.Tunnel.GetSites()) < 2 {
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
	main := NewInstance(clusterMain, HostID, Host, s.getPorts(5), s.priv, s.pub)
	aux := NewInstance(clusterAux, HostID, Host, s.getPorts(5), s.priv, s.pub)

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
		err = aux.Process.GetAuthServer().UpsertTrustedCluster(trustedCluster)
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

	remote := NewInstance("cluster-remote", HostID, Host, s.getPorts(5), s.priv, s.pub)
	main := NewInstance("cluster-main", HostID, Host, s.getPorts(5), s.priv, s.pub)

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

	// execute the connection via first proxy
	cfg := ClientConfig{Login: username, Cluster: "cluster-remote", Host: "127.0.0.1", Port: remote.GetPortSSHInt()}
	output, err := runCommand(main, []string{"echo", "hello world"}, cfg, 1)
	c.Assert(err, check.IsNil)
	c.Assert(output, check.Equals, "hello world\n")

	// execute the connection via second proxy, should work
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

	// connect the main proxy back and make sure agents have reconnected over time
	lb.AddBackend(mainProxyAddr)
	output, err = runCommand(main, []string{"echo", "hello world"}, cfg, 10)
	c.Assert(err, check.IsNil)
	c.Assert(output, check.Equals, "hello world\n")

	// stop cluster and remaining nodes
	c.Assert(remote.Stop(true), check.IsNil)
	c.Assert(main.Stop(true), check.IsNil)
}

// TestExternalClient tests if we can connect to a node in a Teleport
// cluster. Both normal and recording proxies are tested.
func (s *IntSuite) TestExternalClient(c *check.C) {
	// only run this test if we have access to the external ssh binary
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
		// record at the node, forward agent. will still work even though the agent
		// will be rejected by the proxy (agent forwarding request rejection is a
		// soft failure)
		{
			services.RecordAtNode,
			true,
			"echo hello",
			false,
			"hello",
		},
		// record at the node, don't forward agent, will work. this is the normal
		// teleport mode of operation.
		{
			services.RecordAtNode,
			false,
			"echo hello",
			false,
			"hello",
		},
		// record at the proxy, forward agent. will work.
		{
			services.RecordAtProxy,
			true,
			"echo hello",
			false,
			"hello",
		},
		// record at the proxy, don't forward agent, request will fail because
		// recording proxy requires an agent.
		{
			services.RecordAtProxy,
			false,
			"echo hello",
			true,
			"",
		},
	}

	for _, tt := range tests {
		// create a teleport instance with auth, proxy, and node
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

		// start (and defer close) a agent that runs during this integration test
		teleAgent, socketDirPath, socketPath, err := createAgent(
			s.me,
			t.Secrets.Users[s.me.Username].Key.Priv,
			t.Secrets.Users[s.me.Username].Key.Cert)
		c.Assert(err, check.IsNil)
		defer closeAgent(teleAgent, socketDirPath)

		// create a *exec.Cmd that will execute the external ssh command
		execCmd, err := externalSSHCommand(
			tt.inForwardAgent,
			socketPath,
			t.GetPortProxy(),
			t.GetPortSSH(),
			tt.inCommand)
		c.Assert(err, check.IsNil)

		// execute ssh command and check the output is what we expect
		output, err := execCmd.Output()
		if tt.outError {
			c.Assert(err, check.NotNil)
		} else {
			c.Assert(err, check.IsNil)
			c.Assert(strings.TrimSpace(string(output)), check.Equals, tt.outExecOutput)
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

// runCommand is a shortcut for running SSH command, it creates
// a client connected to proxy hosted by instance
// and returns the result
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
		time.Sleep(time.Millisecond * 50)
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
