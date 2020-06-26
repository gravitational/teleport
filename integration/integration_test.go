/*
Copyright 2016-2019 Gravitational, Inc.

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
	"bufio"
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/pam"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	log "github.com/sirupsen/logrus"
	"gopkg.in/check.v1"
)

const (
	HostID = "00000000-0000-0000-0000-000000000000"
	Site   = "local-site"

	AllocatePortsNum = 400
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

// TestMain will re-execute Teleport to run a command if "exec" is passed to
// it as an argument. Otherwise it will run tests as normal.
func TestMain(m *testing.M) {
	// If the test is re-executing itself, execute the command that comes over
	// the pipe.
	if len(os.Args) == 2 &&
		(os.Args[1] == teleport.ExecSubCommand || os.Args[1] == teleport.ForwardSubCommand) {
		srv.RunAndExit(os.Args[1])
		return
	}

	// Otherwise run tests as normal.
	code := m.Run()
	os.Exit(code)
}

func (s *IntSuite) SetUpSuite(c *check.C) {
	var err error

	utils.InitLoggerForTests(testing.Verbose())

	SetTestTimeouts(time.Millisecond * time.Duration(100))

	s.priv, s.pub, err = testauthority.New().GenerateKeyPair("")
	c.Assert(err, check.IsNil)

	// Find AllocatePortsNum free listening ports to use.
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

// newTeleport helper returns a created but not started Teleport instance pre-configured
// with the current user os.user.Current().
func (s *IntSuite) newUnstartedTeleport(c *check.C, logins []string, enableSSH bool) *TeleInstance {
	t := NewInstance(InstanceConfig{ClusterName: Site, HostID: HostID, NodeName: Host, Ports: s.getPorts(5), Priv: s.priv, Pub: s.pub})
	// use passed logins, but use suite's default login if nothing was passed
	if len(logins) == 0 {
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

// newTeleportIoT helper returns a running Teleport instance with Host as a
// reversetunnel node.
func (s *IntSuite) newTeleportIoT(c *check.C, logins []string) *TeleInstance {
	// Create a Teleport instance with Auth/Proxy.
	mainConfig := func() *service.Config {
		tconf := service.MakeDefaultConfig()

		tconf.Console = nil

		tconf.Auth.Enabled = true

		tconf.Proxy.Enabled = true
		tconf.Proxy.DisableWebService = false
		tconf.Proxy.DisableWebInterface = true

		tconf.SSH.Enabled = false

		return tconf
	}
	main := s.newTeleportWithConfig(c, logins, nil, mainConfig())

	// Create a Teleport instance with a Node.
	nodeConfig := func() *service.Config {
		tconf := service.MakeDefaultConfig()
		tconf.Hostname = Host
		tconf.Console = nil
		tconf.Token = "token"
		tconf.AuthServers = []utils.NetAddr{
			utils.NetAddr{
				AddrNetwork: "tcp",
				Addr:        net.JoinHostPort(Loopback, main.GetPortWeb()),
			},
		}

		tconf.Auth.Enabled = false

		tconf.Proxy.Enabled = false

		tconf.SSH.Enabled = true

		return tconf
	}
	_, err := main.StartReverseTunnelNode(nodeConfig())
	c.Assert(err, check.IsNil)

	return main
}

// newTeleportWithConfig is a helper function that will create a running
// Teleport instance with the passed in user, instance secrets, and Teleport
// configuration.
func (s *IntSuite) newTeleportWithConfig(c *check.C, logins []string, instanceSecrets []*InstanceSecrets, teleportConfig *service.Config) *TeleInstance {
	t := NewInstance(InstanceConfig{ClusterName: Site, HostID: HostID, NodeName: Host, Ports: s.getPorts(5), Priv: s.priv, Pub: s.pub})

	// use passed logins, but use suite's default login if nothing was passed
	if len(logins) == 0 {
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
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

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
				LocalAuth:        services.NewBool(true),
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
		defer t.StopAll()

		// Start a node.
		nodeSSHPort := s.getPorts(1)[0]
		nodeConfig := func() *service.Config {
			tconf := service.MakeDefaultConfig()

			tconf.HostUUID = "node"
			tconf.Hostname = "node"

			tconf.SSH.Enabled = true
			tconf.SSH.Addr.Addr = net.JoinHostPort(t.Hostname, fmt.Sprintf("%v", nodeSSHPort))

			return tconf
		}
		nodeProcess, err := t.StartNode(nodeConfig())
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
					nodesInSite, err := site.GetNodes(defaults.Namespace, services.SkipValidation())
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
		endC := make(chan error)
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
			cl.Stdout = myTerm
			cl.Stdin = myTerm
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
				c.Fatalf("Stream is not getting data: %q.", string(sessionStream))
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

		// If session are being recorded at nodes, the SessionServerID should contain
		// the ID of the node. If sessions are being recorded at the proxy, then
		// SessionServerID should be that of the proxy.
		expectedServerID := nodeProcess.Config.HostUUID
		if tt.inRecordLocation == services.RecordAtProxy {
			expectedServerID = t.Process.Config.HostUUID
		}
		c.Assert(start.GetString(events.SessionServerID), check.Equals, expectedServerID)

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
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	tempdir, err := ioutil.TempDir("", "teleport-")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(tempdir)
	tempfile := filepath.Join(tempdir, "file.txt")

	// create new teleport server that will be used by all tests
	t := s.newTeleport(c, nil, true)
	defer t.StopAll()

	var tests = []struct {
		inCommand   string
		inStdin     string
		outContains string
		outFile     bool
	}{
		// 0 - echo "1\n2\n" | ssh localhost "cat -"
		// this command can be used to copy files by piping stdout to stdin over ssh.
		{
			inCommand:   "cat -",
			inStdin:     "1\n2\n",
			outContains: "1\n2\n",
			outFile:     false,
		},
		// 1 - ssh -tt locahost '/bin/sh -c "mkdir -p /tmp && echo a > /tmp/file.txt"'
		// programs like ansible execute commands like this
		{
			inCommand:   fmt.Sprintf(`/bin/sh -c "mkdir -p /tmp && echo a > %v"`, tempfile),
			inStdin:     "",
			outContains: "a",
			outFile:     true,
		},
		// 2 - ssh localhost tty
		// should print "not a tty"
		{
			inCommand:   "tty",
			inStdin:     "",
			outContains: "not a tty",
			outFile:     false,
		},
	}

	for i, tt := range tests {
		// create new teleport client
		cl, err := t.NewClient(ClientConfig{Login: s.me.Username, Cluster: Site, Host: Host, Port: t.GetPortSSHInt()})
		c.Assert(err, check.IsNil)

		// hook up stdin and stdout to a buffer for reading and writing
		inbuf := bytes.NewReader([]byte(tt.inStdin))
		outbuf := utils.NewSyncBuffer()
		cl.Stdin = inbuf
		cl.Stdout = outbuf
		cl.Stderr = outbuf

		// run command and wait a maximum of 10 seconds for it to complete
		sessionEndC := make(chan interface{})
		go func() {
			// don't check for err, because sometimes this process should fail
			// with an error and that's what the test is checking for.
			cl.SSH(context.TODO(), []string{tt.inCommand}, false)
			sessionEndC <- true
		}()
		err = waitFor(sessionEndC, time.Second*10)
		c.Assert(err, check.IsNil)

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

// TestUUIDBasedProxy verifies that attempts to proxy to nodes using ambiguous
// hostnames fails with the correct error, and that proxying by UUID succeeds.
func (s *IntSuite) TestUUIDBasedProxy(c *check.C) {
	var err error
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	t := s.newTeleport(c, nil, true)
	defer t.StopAll()

	site := t.GetSiteAPI(Site)

	// addNode adds a node to the teleport instance, returning its uuid.
	// All nodes added this way have the same hostname.
	addNode := func() (string, error) {
		nodeSSHPort := s.getPorts(1)[0]
		tconf := service.MakeDefaultConfig()

		tconf.Hostname = Host

		tconf.SSH.Enabled = true
		tconf.SSH.Addr.Addr = net.JoinHostPort(t.Hostname, fmt.Sprintf("%v", nodeSSHPort))

		node, err := t.StartNode(tconf)
		if err != nil {
			return "", trace.Wrap(err)
		}

		ident, err := node.GetIdentity(teleport.RoleNode)
		if err != nil {
			return "", trace.Wrap(err)
		}

		return ident.ID.HostID()
	}

	// add two nodes with the same hostname.
	uuid1, err := addNode()
	c.Assert(err, check.IsNil)

	uuid2, err := addNode()
	c.Assert(err, check.IsNil)

	// wait up to 10 seconds for supplied node names to show up.
	waitForNodes := func(site auth.ClientI, nodes ...string) error {
		tickCh := time.Tick(500 * time.Millisecond)
		stopCh := time.After(10 * time.Second)
	Outer:
		for _, nodeName := range nodes {
			for {
				select {
				case <-tickCh:
					nodesInSite, err := site.GetNodes(defaults.Namespace, services.SkipValidation())
					if err != nil && !trace.IsNotFound(err) {
						return trace.Wrap(err)
					}
					for _, node := range nodesInSite {
						if node.GetName() == nodeName {
							continue Outer
						}
					}
				case <-stopCh:
					return trace.BadParameter("waited 10s, did find node %s", nodeName)
				}
			}
		}
		return nil
	}

	err = waitForNodes(site, uuid1, uuid2)
	c.Assert(err, check.IsNil)

	// attempting to run a command by hostname should generate NodeIsAmbiguous error.
	_, err = runCommand(t, []string{"echo", "Hello there!"}, ClientConfig{Login: s.me.Username, Cluster: Site, Host: Host}, 1)
	c.Assert(err, check.NotNil)
	if !strings.Contains(err.Error(), teleport.NodeIsAmbiguous) {
		c.Errorf("Expected %s, got %s", teleport.NodeIsAmbiguous, err.Error())
	}

	// attempting to run a command by uuid should succeed.
	_, err = runCommand(t, []string{"echo", "Hello there!"}, ClientConfig{Login: s.me.Username, Cluster: Site, Host: uuid1}, 1)
	c.Assert(err, check.IsNil)
}

// TestInteractive covers SSH into shell and joining the same session from another client
// against a standard teleport node.
func (s *IntSuite) TestInteractiveRegular(c *check.C) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	t := s.newTeleport(c, nil, true)
	defer t.StopAll()

	s.verifySessionJoin(c, t)
}

// TestInteractive covers SSH into shell and joining the same session from another client
// against a reversetunnel node.
func (s *IntSuite) TestInteractiveReverseTunnel(c *check.C) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	// InsecureDevMode needed for IoT node handshake
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	t := s.newTeleportIoT(c, nil)
	defer t.StopAll()

	s.verifySessionJoin(c, t)
}

// TestInteractive covers SSH into shell and joining the same session from another client
func (s *IntSuite) verifySessionJoin(c *check.C, t *TeleInstance) {

	sessionEndC := make(chan interface{})

	// get a reference to site obj:
	site := t.GetSiteAPI(Site)
	c.Assert(site, check.NotNil)

	personA := NewTerminal(250)
	personB := NewTerminal(250)

	// PersonA: SSH into the server, wait one second, then type some commands on stdin:
	openSession := func() {
		cl, err := t.NewClient(ClientConfig{Login: s.me.Username, Cluster: Site, Host: Host})
		c.Assert(err, check.IsNil)
		cl.Stdout = personA
		cl.Stdin = personA
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
		cl, err := t.NewClient(ClientConfig{Login: s.me.Username, Cluster: Site, Host: Host})
		c.Assert(err, check.IsNil)
		cl.Stdout = personB
		for i := 0; i < 10; i++ {
			err = cl.Join(context.TODO(), defaults.Namespace, session.ID(sessionID), personB)
			if err == nil {
				break
			}
		}
		c.Assert(err, check.IsNil)
	}

	go openSession()
	go joinSession()

	// wait for the session to end
	err := waitFor(sessionEndC, time.Second*10)
	c.Assert(err, check.IsNil)

	// make sure the output of B is mirrored in A
	outputOfA := personA.Output(100)
	outputOfB := personB.Output(100)
	c.Assert(strings.Contains(outputOfA, outputOfB), check.Equals, true)
}

// TestShutdown tests scenario with a graceful shutdown,
// that session will be working after
func (s *IntSuite) TestShutdown(c *check.C) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	t := s.newTeleport(c, nil, true)

	// get a reference to site obj:
	site := t.GetSiteAPI(Site)
	c.Assert(site, check.NotNil)

	person := NewTerminal(250)

	// commandsC receive commands
	commandsC := make(chan string)

	// PersonA: SSH into the server, wait one second, then type some commands on stdin:
	openSession := func() {
		cl, err := t.NewClient(ClientConfig{Login: s.me.Username, Cluster: Site, Host: Host, Port: t.GetPortSSHInt()})
		c.Assert(err, check.IsNil)
		cl.Stdout = person
		cl.Stdin = person

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
			output = replaceNewlines(person.Output(1000))
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
		c.Fatalf("Failed to shut down the server.")
	}
}

type disconnectTestCase struct {
	recordingMode     string
	options           services.RoleOptions
	disconnectTimeout time.Duration
}

// TestDisconnectScenarios tests multiple scenarios with client disconnects
func (s *IntSuite) TestDisconnectScenarios(c *check.C) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	testCases := []disconnectTestCase{
		{
			recordingMode: services.RecordAtNode,
			options: services.RoleOptions{
				ClientIdleTimeout: services.NewDuration(500 * time.Millisecond),
			},
			disconnectTimeout: time.Second,
		},
		{
			recordingMode: services.RecordAtProxy,
			options: services.RoleOptions{
				ForwardAgent:      services.NewBool(true),
				ClientIdleTimeout: services.NewDuration(500 * time.Millisecond),
			},
			disconnectTimeout: time.Second,
		},
		{
			recordingMode: services.RecordAtNode,
			options: services.RoleOptions{
				DisconnectExpiredCert: services.NewBool(true),
				MaxSessionTTL:         services.NewDuration(2 * time.Second),
			},
			disconnectTimeout: 4 * time.Second,
		},
		{
			recordingMode: services.RecordAtProxy,
			options: services.RoleOptions{
				ForwardAgent:          services.NewBool(true),
				DisconnectExpiredCert: services.NewBool(true),
				MaxSessionTTL:         services.NewDuration(2 * time.Second),
			},
			disconnectTimeout: 4 * time.Second,
		},
	}
	for _, tc := range testCases {
		s.runDisconnectTest(c, tc)
	}
}

func (s *IntSuite) runDisconnectTest(c *check.C, tc disconnectTestCase) {
	t := NewInstance(InstanceConfig{
		ClusterName: Site,
		HostID:      HostID,
		NodeName:    Host,
		Ports:       s.getPorts(5),
		Priv:        s.priv,
		Pub:         s.pub,
	})

	username := s.me.Username
	role, err := services.NewRole("devs", services.RoleSpecV3{
		Options: tc.options,
		Allow: services.RoleConditions{
			Logins: []string{username},
		},
	})
	c.Assert(err, check.IsNil)
	t.AddUserWithRole(username, role)

	clusterConfig, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
		SessionRecording: tc.recordingMode,
		LocalAuth:        services.NewBool(true),
	})
	c.Assert(err, check.IsNil)

	cfg := service.MakeDefaultConfig()
	cfg.Auth.Enabled = true
	cfg.Auth.ClusterConfig = clusterConfig
	cfg.Proxy.DisableWebService = true
	cfg.Proxy.DisableWebInterface = true
	cfg.Proxy.Enabled = true
	cfg.SSH.Enabled = true

	c.Assert(t.CreateEx(nil, cfg), check.IsNil)
	c.Assert(t.Start(), check.IsNil)
	defer t.StopAll()

	// get a reference to site obj:
	site := t.GetSiteAPI(Site)
	c.Assert(site, check.NotNil)

	person := NewTerminal(250)

	// PersonA: SSH into the server, wait one second, then type some commands on stdin:
	sessionCtx, sessionCancel := context.WithCancel(context.TODO())
	openSession := func() {
		defer sessionCancel()
		cl, err := t.NewClient(ClientConfig{Login: username, Cluster: Site, Host: Host, Port: t.GetPortSSHInt()})
		c.Assert(err, check.IsNil)
		cl.Stdout = person
		cl.Stdin = person

		err = cl.SSH(context.TODO(), []string{}, false)
		if err != nil && err != io.EOF {
			c.Fatalf("expected EOF or nil, got %v instead", err)
		}
	}

	go openSession()

	enterInput(c, person, "echo start \r\n", ".*start.*")
	select {
	case <-time.After(tc.disconnectTimeout):
		c.Fatalf("timeout waiting for session to exit")
	case <-sessionCtx.Done():
		// session closed
	}
}

func enterInput(c *check.C, person *Terminal, command, pattern string) {
	person.Type(command)
	abortTime := time.Now().Add(10 * time.Second)
	var matched bool
	var output string
	for {
		output = replaceNewlines(person.Output(1000))
		matched, _ = regexp.MatchString(pattern, output)
		if matched {
			break
		}
		time.Sleep(time.Millisecond * 200)
		if time.Now().After(abortTime) {
			c.Fatalf("failed to capture pattern %q in %q", pattern, output)
		}
	}
	if !matched {
		c.Fatalf("output %q does not match pattern %q", output, pattern)
	}
}

// TestInvalidLogins validates that you can't login with invalid login or
// with invalid 'site' parameter
func (s *IntSuite) TestEnvironmentVariables(c *check.C) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	t := s.newTeleport(c, nil, true)
	defer t.StopAll()

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
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	t := s.newTeleport(c, nil, true)
	defer t.StopAll()

	cmd := []string{"echo", "success"}

	// try the wrong site:
	tc, err := t.NewClient(ClientConfig{Login: s.me.Username, Cluster: "wrong-site", Host: Host, Port: t.GetPortSSHInt()})
	c.Assert(err, check.IsNil)
	err = tc.SSH(context.TODO(), cmd, false)
	c.Assert(err, check.ErrorMatches, "cluster wrong-site not found")
}

// TestTwoClustersTunnel creates two teleport clusters: "a" and "b" and creates a
// tunnel from A to B.
//
// Two tests are run, first is when both A and B record sessions at nodes. It
// executes an SSH command on A by connecting directly to A and by connecting
// to B via B<->A tunnel. All sessions should end up in A.
//
// In the second test, sessions are recorded at B. All sessions still show up on
// A (they are Teleport nodes) but in addition, two show up on B when connecting
// over the B<->A tunnel because sessions are recorded at the proxy.
func (s *IntSuite) TestTwoClustersTunnel(c *check.C) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

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
			LocalAuth:        services.NewBool(true),
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
		for len(a.Tunnel.GetSites()) < 2 && len(b.Tunnel.GetSites()) < 2 {
			time.Sleep(time.Millisecond * 200)
			if time.Now().After(abortTime) {
				c.Fatalf("Two clusters do not see each other: tunnels are not working.")
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
			Cluster:      a.Secrets.SiteName,
			Host:         Host,
			Port:         sshPort,
			ForwardAgent: true,
		})
		tc.Stdout = &outputA
		c.Assert(err, check.IsNil)
		err = tc.SSH(context.TODO(), cmd, false)
		c.Assert(err, check.IsNil)
		c.Assert(outputA.String(), check.Equals, "hello world\n")

		// Update trusted CAs.
		err = tc.UpdateTrustedCA(context.TODO(), a.Secrets.SiteName)
		c.Assert(err, check.IsNil)

		// The known_hosts file should have two certificates, the way bytes.Split
		// works that means the output will be 3 (2 certs + 1 empty).
		buffer, err := ioutil.ReadFile(filepath.Join(tc.KeysDir, "known_hosts"))
		c.Assert(err, check.IsNil)
		parts := bytes.Split(buffer, []byte("\n"))
		c.Assert(parts, check.HasLen, 3)

		// The certs.pem file should have 2 certificates.
		buffer, err = ioutil.ReadFile(filepath.Join(tc.KeysDir, "keys", Host, "certs.pem"))
		c.Assert(err, check.IsNil)
		roots := x509.NewCertPool()
		ok := roots.AppendCertsFromPEM(buffer)
		c.Assert(ok, check.Equals, true)
		c.Assert(roots.Subjects(), check.HasLen, 2)

		// wait for active tunnel connections to be established
		waitForActiveTunnelConnections(c, b.Tunnel, a.Secrets.SiteName, 1)

		// via tunnel b->a:
		tc, err = b.NewClient(ClientConfig{
			Login:        username,
			Cluster:      a.Secrets.SiteName,
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
		err = a.StopAuth(false)
		c.Assert(err, check.IsNil)
		err = tc.SSH(context.TODO(), cmd, false)
		c.Assert(err, check.FitsTypeOf, trace.ConnectionProblem(nil, ""))

		// Reset and start "Site-A" again
		err = a.Reset()
		c.Assert(err, check.IsNil)
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

		siteA := a.GetSiteAPI(a.Secrets.SiteName)
		err = searchAndAssert(siteA, tt.outExecCountSiteA)
		c.Assert(err, check.IsNil)

		siteB := b.GetSiteAPI(b.Secrets.SiteName)
		err = searchAndAssert(siteB, tt.outExecCountSiteB)
		c.Assert(err, check.IsNil)

		// stop both sites for real
		c.Assert(b.StopAll(), check.IsNil)
		c.Assert(a.StopAll(), check.IsNil)
	}
}

// TestTwoClustersProxy checks if the reverse tunnel uses a HTTP PROXY to
// establish a connection.
func (s *IntSuite) TestTwoClustersProxy(c *check.C) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

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
	c.Assert(b.StopAll(), check.IsNil)
	c.Assert(a.StopAll(), check.IsNil)
}

// TestHA tests scenario when auth server for the cluster goes down
// and we switch to local persistent caches
func (s *IntSuite) TestHA(c *check.C) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

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
	tc, err := b.NewClient(ClientConfig{
		Login:   username,
		Cluster: "cluster-a",
		Host:    Loopback,
		Port:    sshPort,
	})
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
	c.Assert(a.StopAuth(true), check.IsNil)

	// try to execute an SSH command using the same old client  to Site-B
	// "site-A" and "site-B" reverse tunnels are supposed to reconnect,
	// and 'tc' (client) is also supposed to reconnect
	for i := 0; i < 30; i++ {
		time.Sleep(1 * time.Second)
		err = tc.SSH(context.TODO(), cmd, false)
		if err == nil {
			break
		}
	}
	c.Assert(err, check.IsNil)

	// stop cluster and remaining nodes
	c.Assert(b.StopAll(), check.IsNil)
	c.Assert(b.StopAll(), check.IsNil)
}

// TestMapRoles tests local to remote role mapping and access patterns
func (s *IntSuite) TestMapRoles(c *check.C) {
	ctx := context.Background()
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

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
	err = aux.Process.GetAuthServer().UpsertRole(ctx, role)
	c.Assert(err, check.IsNil)
	trustedClusterToken := "trusted-cluster-token"
	err = main.Process.GetAuthServer().UpsertToken(
		services.MustCreateProvisionToken(trustedClusterToken, []teleport.Role{teleport.RoleTrustedCluster}, time.Time{}))
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
	tryCreateTrustedCluster(c, aux.Process.GetAuthServer(), trustedCluster)

	nodePorts := s.getPorts(3)
	sshPort, proxyWebPort, proxySSHPort := nodePorts[0], nodePorts[1], nodePorts[2]
	c.Assert(aux.StartNodeAndProxy("aux-node", sshPort, proxyWebPort, proxySSHPort), check.IsNil)

	// wait for both sites to see each other via their reverse tunnels (for up to 10 seconds)
	abortTime := time.Now().Add(time.Second * 10)
	for len(main.Tunnel.GetSites()) < 2 && len(aux.Tunnel.GetSites()) < 2 {
		time.Sleep(time.Millisecond * 2000)
		if time.Now().After(abortTime) {
			c.Fatalf("two clusters do not see each other: tunnels are not working")
		}
	}

	// Make sure that GetNodes returns nodes in the remote site. This makes
	// sure identity aware GetNodes works for remote clusters. Testing of the
	// correct nodes that identity aware GetNodes is done in TestList.
	var nodes []services.Server
	for i := 0; i < 10; i++ {
		nodes, err = aux.Process.GetAuthServer().GetNodes(defaults.Namespace, services.SkipValidation())
		c.Assert(err, check.IsNil)
		if len(nodes) != 2 {
			time.Sleep(100 * time.Millisecond)
			continue
		}
	}
	c.Assert(nodes, check.HasLen, 2)

	cmd := []string{"echo", "hello world"}
	tc, err := main.NewClient(ClientConfig{Login: username, Cluster: clusterAux, Host: Loopback, Port: sshPort})
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
	c.Assert(main.StopAll(), check.IsNil)
	c.Assert(aux.StopAll(), check.IsNil)
}

// tryCreateTrustedCluster performs several attempts to create a trusted cluster,
// retries on connection problems and access denied errors to let caches
// propagate and services to start
func tryCreateTrustedCluster(c *check.C, authServer *auth.AuthServer, trustedCluster services.TrustedCluster) {
	ctx := context.TODO()
	for i := 0; i < 10; i++ {
		log.Debugf("Will create trusted cluster %v, attempt %v.", trustedCluster, i)
		_, err := authServer.UpsertTrustedCluster(ctx, trustedCluster)
		if err == nil {
			return
		}
		if trace.IsConnectionProblem(err) {
			log.Debugf("Retrying on connection problem: %v.", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if trace.IsAccessDenied(err) {
			log.Debugf("Retrying on access denied: %v.", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		c.Fatalf("Terminating on unexpected problem %v.", err)
	}
	c.Fatalf("Timeout creating trusted cluster")
}

// trustedClusterTest is a test setup for trusted clusters tests
type trustedClusterTest struct {
	// multiplex sets up multiplexing of the reversetunnel SSH
	// socket and the proxy's web socket
	multiplex bool
	// useJumpHost turns on jump host mode for the access
	// to the proxy instead of the proxy command
	useJumpHost bool
}

// TestTrustedClusters tests remote clusters scenarios
// using trusted clusters feature
func (s *IntSuite) TestTrustedClusters(c *check.C) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	s.trustedClusters(c, trustedClusterTest{multiplex: false})
}

// TestJumpTrustedClusters tests remote clusters scenarios
// using trusted clusters feature using jumphost connection
func (s *IntSuite) TestJumpTrustedClusters(c *check.C) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	s.trustedClusters(c, trustedClusterTest{multiplex: false, useJumpHost: true})
}

// TestMultiplexingTrustedClusters tests remote clusters scenarios
// using trusted clusters feature
func (s *IntSuite) TestMultiplexingTrustedClusters(c *check.C) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	s.trustedClusters(c, trustedClusterTest{multiplex: true})
}

func (s *IntSuite) trustedClusters(c *check.C, test trustedClusterTest) {
	ctx := context.Background()
	username := s.me.Username

	clusterMain := "cluster-main"
	clusterAux := "cluster-aux"
	main := NewInstance(InstanceConfig{ClusterName: clusterMain, HostID: HostID, NodeName: Host, Ports: s.getPorts(5), Priv: s.priv, Pub: s.pub, MultiplexProxy: test.multiplex})
	aux := NewInstance(InstanceConfig{ClusterName: clusterAux, HostID: HostID, NodeName: Host, Ports: s.getPorts(5), Priv: s.priv, Pub: s.pub})

	// main cluster has a local user and belongs to role "main-devs" and "main-admins"
	mainDevs := "main-devs"
	devsRole, err := services.NewRole(mainDevs, services.RoleSpecV3{
		Allow: services.RoleConditions{
			Logins: []string{username},
		},
	})
	c.Assert(err, check.IsNil)

	mainAdmins := "main-admins"
	adminsRole, err := services.NewRole(mainAdmins, services.RoleSpecV3{
		Allow: services.RoleConditions{
			Logins: []string{"superuser"},
		},
	})
	c.Assert(err, check.IsNil)

	main.AddUserWithRole(username, devsRole, adminsRole)

	// for role mapping test we turn on Web API on the main cluster
	// as it's used
	makeConfig := func(enableSSH bool) ([]*InstanceSecrets, *service.Config) {
		tconf := service.MakeDefaultConfig()
		tconf.Console = nil
		tconf.Proxy.DisableWebService = false
		tconf.Proxy.DisableWebInterface = true
		tconf.SSH.Enabled = enableSSH
		return nil, tconf
	}
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	c.Assert(main.CreateEx(makeConfig(false)), check.IsNil)
	c.Assert(aux.CreateEx(makeConfig(true)), check.IsNil)

	// auxiliary cluster has only a role aux-devs
	// connect aux cluster to main cluster
	// using trusted clusters, so remote user will be allowed to assume
	// role specified by mapping remote role "devs" to local role "local-devs"
	auxDevs := "aux-devs"
	auxRole, err := services.NewRole(auxDevs, services.RoleSpecV3{
		Allow: services.RoleConditions{
			Logins: []string{username},
		},
	})
	c.Assert(err, check.IsNil)
	err = aux.Process.GetAuthServer().UpsertRole(ctx, auxRole)
	c.Assert(err, check.IsNil)
	trustedClusterToken := "trusted-cluster-token"
	err = main.Process.GetAuthServer().UpsertToken(
		services.MustCreateProvisionToken(trustedClusterToken, []teleport.Role{teleport.RoleTrustedCluster}, time.Time{}))
	c.Assert(err, check.IsNil)
	// Note that the mapping omits admins role, this is to cover the scenario
	// when root cluster and leaf clusters have different role sets
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
	tryCreateTrustedCluster(c, aux.Process.GetAuthServer(), trustedCluster)

	nodePorts := s.getPorts(3)
	sshPort, proxyWebPort, proxySSHPort := nodePorts[0], nodePorts[1], nodePorts[2]
	c.Assert(aux.StartNodeAndProxy("aux-node", sshPort, proxyWebPort, proxySSHPort), check.IsNil)

	// wait for both sites to see each other via their reverse tunnels (for up to 10 seconds)
	abortTime := time.Now().Add(time.Second * 10)
	for len(main.Tunnel.GetSites()) < 2 && len(aux.Tunnel.GetSites()) < 2 {
		time.Sleep(time.Millisecond * 2000)
		if time.Now().After(abortTime) {
			c.Fatalf("two clusters do not see each other: tunnels are not working")
		}
	}

	cmd := []string{"echo", "hello world"}

	// Try and connect to a node in the Aux cluster from the Main cluster using
	// direct dialing.
	creds, err := GenerateUserCreds(UserCredsRequest{
		Process:        main.Process,
		Username:       username,
		RouteToCluster: clusterAux,
	})
	c.Assert(err, check.IsNil)

	tc, err := main.NewClientWithCreds(ClientConfig{
		Login:    username,
		Cluster:  clusterAux,
		Host:     Loopback,
		Port:     sshPort,
		JumpHost: test.useJumpHost,
	}, *creds)
	c.Assert(err, check.IsNil)

	// tell the client to trust aux cluster CAs (from secrets). this is the
	// equivalent of 'known hosts' in openssh
	auxCAS := aux.Secrets.GetCAs()
	for i := range auxCAS {
		err = tc.AddTrustedCA(auxCAS[i])
		c.Assert(err, check.IsNil)
	}

	output := &bytes.Buffer{}
	tc.Stdout = output
	c.Assert(err, check.IsNil)
	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond * 50)
		err = tc.SSH(ctx, cmd, false)
		if err == nil {
			break
		}
	}
	c.Assert(err, check.IsNil)
	c.Assert(output.String(), check.Equals, "hello world\n")

	// ListNodes expect labels as a value of host
	tc.Host = ""
	servers, err := tc.ListNodes(ctx)
	c.Assert(err, check.IsNil)
	c.Assert(servers, check.HasLen, 2)
	tc.Host = Loopback

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
		err = tc.SSH(ctx, cmd, false)
		if err != nil {
			break
		}
	}
	c.Assert(err, check.NotNil, check.Commentf("expected tunnel to close and SSH client to start failing"))

	// remove trusted cluster from aux cluster side, and recrete right after
	// this should re-establish connection
	err = aux.Process.GetAuthServer().DeleteTrustedCluster(ctx, trustedCluster.GetName())
	c.Assert(err, check.IsNil)
	_, err = aux.Process.GetAuthServer().UpsertTrustedCluster(ctx, trustedCluster)
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
		err = tc.SSH(ctx, cmd, false)
		if err == nil {
			break
		}
	}
	c.Assert(err, check.IsNil)
	c.Assert(output.String(), check.Equals, "hello world\n")

	// stop clusters and remaining nodes
	c.Assert(main.StopAll(), check.IsNil)
	c.Assert(aux.StopAll(), check.IsNil)
}

func (s *IntSuite) TestTrustedTunnelNode(c *check.C) {
	ctx := context.Background()
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
		tconf.Console = nil
		tconf.Proxy.DisableWebService = false
		tconf.Proxy.DisableWebInterface = true
		tconf.SSH.Enabled = enableSSH
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
	err = aux.Process.GetAuthServer().UpsertRole(ctx, role)
	c.Assert(err, check.IsNil)
	trustedClusterToken := "trusted-cluster-token"
	err = main.Process.GetAuthServer().UpsertToken(
		services.MustCreateProvisionToken(trustedClusterToken, []teleport.Role{teleport.RoleTrustedCluster}, time.Time{}))
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
	tryCreateTrustedCluster(c, aux.Process.GetAuthServer(), trustedCluster)

	// Create a Teleport instance with a node that dials back to the aux cluster.
	tunnelNodeHostname := "cluster-aux-node"
	nodeConfig := func() *service.Config {
		tconf := service.MakeDefaultConfig()
		tconf.Hostname = tunnelNodeHostname
		tconf.Console = nil
		tconf.Token = "token"
		tconf.AuthServers = []utils.NetAddr{
			utils.NetAddr{
				AddrNetwork: "tcp",
				Addr:        net.JoinHostPort(Loopback, aux.GetPortWeb()),
			},
		}
		tconf.Auth.Enabled = false
		tconf.Proxy.Enabled = false
		tconf.SSH.Enabled = true
		return tconf
	}
	_, err = aux.StartNode(nodeConfig())
	c.Assert(err, check.IsNil)

	// wait for both sites to see each other via their reverse tunnels (for up to 10 seconds)
	abortTime := time.Now().Add(time.Second * 10)
	for len(main.Tunnel.GetSites()) < 2 && len(aux.Tunnel.GetSites()) < 2 {
		time.Sleep(time.Millisecond * 2000)
		if time.Now().After(abortTime) {
			c.Fatalf("two clusters do not see each other: tunnels are not working")
		}
	}

	// Wait for both nodes to show up before attempting to dial to them.
	err = waitForNodeCount(main, clusterAux, 2)
	c.Assert(err, check.IsNil)

	cmd := []string{"echo", "hello world"}

	// Try and connect to a node in the Aux cluster from the Main cluster using
	// direct dialing.
	tc, err := main.NewClient(ClientConfig{
		Login:   username,
		Cluster: clusterAux,
		Host:    Loopback,
		Port:    aux.GetPortSSHInt(),
	})
	c.Assert(err, check.IsNil)
	output := &bytes.Buffer{}
	tc.Stdout = output
	c.Assert(err, check.IsNil)
	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond * 50)
		err = tc.SSH(context.TODO(), cmd, false)
		if err == nil {
			break
		}
	}
	c.Assert(err, check.IsNil)
	c.Assert(output.String(), check.Equals, "hello world\n")

	// Try and connect to a node in the Aux cluster from the Main cluster using
	// tunnel dialing.
	tunnelClient, err := main.NewClient(ClientConfig{
		Login:   username,
		Cluster: clusterAux,
		Host:    tunnelNodeHostname,
	})
	c.Assert(err, check.IsNil)
	tunnelOutput := &bytes.Buffer{}
	tunnelClient.Stdout = tunnelOutput
	c.Assert(err, check.IsNil)
	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond * 50)
		err = tunnelClient.SSH(context.Background(), cmd, false)
		if err == nil {
			break
		}
	}
	c.Assert(err, check.IsNil)
	c.Assert(tunnelOutput.String(), check.Equals, "hello world\n")

	// Stop clusters and remaining nodes.
	c.Assert(main.StopAll(), check.IsNil)
	c.Assert(aux.StopAll(), check.IsNil)
}

// TestDiscoveryRecovers ensures that discovery protocol recovers from a bad discovery
// state (all known proxies are offline).
func (s *IntSuite) TestDiscoveryRecovers(c *check.C) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	username := s.me.Username

	// create load balancer for main cluster proxies
	frontend := *utils.MustParseAddr(net.JoinHostPort(Loopback, strconv.Itoa(s.getPorts(1)[0])))
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
	for len(main.Tunnel.GetSites()) < 2 && len(remote.Tunnel.GetSites()) < 2 {
		time.Sleep(time.Millisecond * 2000)
		if time.Now().After(abortTime) {
			c.Fatalf("two clusters do not see each other: tunnels are not working")
		}
	}

	// Helper function for adding a new proxy to "main".
	addNewMainProxy := func(name string) (reversetunnel.Server, ProxyConfig) {
		c.Logf("adding main proxy %q...", name)
		nodePorts := s.getPorts(3)
		proxyReverseTunnelPort, proxyWebPort, proxySSHPort := nodePorts[0], nodePorts[1], nodePorts[2]
		newConfig := ProxyConfig{
			Name:              name,
			SSHPort:           proxySSHPort,
			WebPort:           proxyWebPort,
			ReverseTunnelPort: proxyReverseTunnelPort,
		}
		newProxy, err := main.StartProxy(newConfig)
		c.Assert(err, check.IsNil)

		// add proxy as a backend to the load balancer
		lb.AddBackend(*utils.MustParseAddr(net.JoinHostPort(Loopback, strconv.Itoa(proxyReverseTunnelPort))))
		return newProxy, newConfig
	}

	killMainProxy := func(name string) {
		c.Logf("killing main proxy %q...", name)
		for _, p := range main.Nodes {
			if !p.Config.Proxy.Enabled {
				continue
			}
			if p.Config.Hostname == name {
				reverseTunnelPort := utils.MustParseAddr(p.Config.Proxy.ReverseTunnelListenAddr.Addr).Port(0)
				c.Assert(lb.RemoveBackend(*utils.MustParseAddr(net.JoinHostPort(Loopback, strconv.Itoa(reverseTunnelPort)))), check.IsNil)
				c.Assert(p.Close(), check.IsNil)
				c.Assert(p.Wait(), check.IsNil)
				return
			}
		}
		c.Errorf("cannot close proxy %q (not found)", name)
	}

	// Helper function for testing that a proxy in main has been discovered by
	// (and is able to use reverse tunnel into) remote.  If conf is nil, main's
	// first/default proxy will be called.
	testProxyConn := func(conf *ProxyConfig, shouldFail bool) {
		clientConf := ClientConfig{
			Login:   username,
			Cluster: "cluster-remote",
			Host:    Loopback,
			Port:    remote.GetPortSSHInt(),
			Proxy:   conf,
		}
		output, err := runCommand(main, []string{"echo", "hello world"}, clientConf, 10)
		cmt := check.Commentf("testProxyConn(conf=%+v,shouldFail=%v)", conf, shouldFail)
		if shouldFail {
			c.Assert(err, check.NotNil, cmt)
		} else {
			c.Assert(err, check.IsNil, cmt)
			c.Assert(output, check.Equals, "hello world\n")
		}
	}

	// ensure that initial proxy's tunnel has been established
	waitForActiveTunnelConnections(c, main.Tunnel, "cluster-remote", 1)
	// execute the connection via initial proxy; should not fail
	testProxyConn(nil, false)

	// helper funcion for making numbered proxy names
	pname := func(n int) string {
		return fmt.Sprintf("cluster-main-proxy-%d", n)
	}

	// create first numbered proxy
	_, c0 := addNewMainProxy(pname(0))
	// check that we now have two tunnel connections
	c.Assert(waitForProxyCount(remote, "cluster-main", 2), check.IsNil)
	// check that first numbered proxy is OK.
	testProxyConn(&c0, false)
	// remove the initial proxy.
	c.Assert(lb.RemoveBackend(mainProxyAddr), check.IsNil)
	c.Assert(waitForProxyCount(remote, "cluster-main", 1), check.IsNil)

	// force bad state by iteratively removing previous proxy before
	// adding next proxy; this ensures that discovery protocol's list of
	// known proxies is all invalid.
	for i := 0; i < 6; i++ {
		prev, next := pname(i), pname(i+1)
		killMainProxy(prev)
		c.Assert(waitForProxyCount(remote, "cluster-main", 0), check.IsNil)
		_, cn := addNewMainProxy(next)
		c.Assert(waitForProxyCount(remote, "cluster-main", 1), check.IsNil)
		testProxyConn(&cn, false)
	}

	// Stop both clusters and remaining nodes.
	c.Assert(remote.StopAll(), check.IsNil)
	c.Assert(main.StopAll(), check.IsNil)
}

// TestDiscovery tests case for multiple proxies and a reverse tunnel
// agent that eventually connnects to the the right proxy
func (s *IntSuite) TestDiscovery(c *check.C) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	username := s.me.Username

	// create load balancer for main cluster proxies
	frontend := *utils.MustParseAddr(net.JoinHostPort(Loopback, strconv.Itoa(s.getPorts(1)[0])))
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
	for len(main.Tunnel.GetSites()) < 2 && len(remote.Tunnel.GetSites()) < 2 {
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
	secondProxy, err := main.StartProxy(proxyConfig)
	c.Assert(err, check.IsNil)

	// add second proxy as a backend to the load balancer
	lb.AddBackend(*utils.MustParseAddr(net.JoinHostPort(Loopback, strconv.Itoa(proxyReverseTunnelPort))))

	// At this point the main cluster should observe two tunnels
	// connected to it from remote cluster
	waitForActiveTunnelConnections(c, main.Tunnel, "cluster-remote", 1)
	waitForActiveTunnelConnections(c, secondProxy, "cluster-remote", 1)

	// execute the connection via first proxy
	cfg := ClientConfig{
		Login:   username,
		Cluster: "cluster-remote",
		Host:    Loopback,
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
		Host:    Loopback,
		Port:    remote.GetPortSSHInt(),
		Proxy:   &proxyConfig,
	}
	output, err = runCommand(main, []string{"echo", "hello world"}, cfgProxy, 10)
	c.Assert(err, check.IsNil)
	c.Assert(output, check.Equals, "hello world\n")

	// Now disconnect the main proxy and make sure it will reconnect eventually.
	c.Assert(lb.RemoveBackend(mainProxyAddr), check.IsNil)
	waitForActiveTunnelConnections(c, secondProxy, "cluster-remote", 1)

	// Requests going via main proxy should fail.
	_, err = runCommand(main, []string{"echo", "hello world"}, cfg, 1)
	c.Assert(err, check.NotNil)

	// Requests going via second proxy should succeed.
	output, err = runCommand(main, []string{"echo", "hello world"}, cfgProxy, 1)
	c.Assert(err, check.IsNil)
	c.Assert(output, check.Equals, "hello world\n")

	// Connect the main proxy back and make sure agents have reconnected over time.
	// This command is tried 10 times with 250 millisecond delay between each
	// attempt to allow the discovery request to be received and the connection
	// added to the agent pool.
	lb.AddBackend(mainProxyAddr)

	// Once the proxy is added a matching tunnel connection should be created.
	waitForActiveTunnelConnections(c, main.Tunnel, "cluster-remote", 1)
	waitForActiveTunnelConnections(c, secondProxy, "cluster-remote", 1)

	// Requests going via main proxy should succeed.
	output, err = runCommand(main, []string{"echo", "hello world"}, cfg, 40)
	c.Assert(err, check.IsNil)
	c.Assert(output, check.Equals, "hello world\n")

	// Stop one of proxies on the main cluster.
	err = main.StopProxy()
	c.Assert(err, check.IsNil)

	// Wait for the remote cluster to detect the outbound connection is gone.
	c.Assert(waitForProxyCount(remote, "cluster-main", 1), check.IsNil)

	// Stop both clusters and remaining nodes.
	c.Assert(remote.StopAll(), check.IsNil)
	c.Assert(main.StopAll(), check.IsNil)
}

// TestDiscoveryNode makes sure the discovery protocol works with nodes.
func (s *IntSuite) TestDiscoveryNode(c *check.C) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	// Create and start load balancer for proxies.
	frontend := *utils.MustParseAddr(net.JoinHostPort(Loopback, strconv.Itoa(s.getPorts(1)[0])))
	lb, err := utils.NewLoadBalancer(context.TODO(), frontend)
	c.Assert(err, check.IsNil)
	err = lb.Listen()
	c.Assert(err, check.IsNil)
	go lb.Serve()
	defer lb.Close()

	// Create a Teleport instance with Auth/Proxy.
	mainConfig := func() (*check.C, []string, []*InstanceSecrets, *service.Config) {
		tconf := service.MakeDefaultConfig()
		tconf.Console = nil

		tconf.Auth.Enabled = true

		tconf.Proxy.Enabled = true
		tconf.Proxy.TunnelPublicAddrs = []utils.NetAddr{
			utils.NetAddr{
				AddrNetwork: "tcp",
				Addr:        frontend.String(),
			},
		}
		tconf.Proxy.DisableWebService = false
		tconf.Proxy.DisableWebInterface = true

		tconf.SSH.Enabled = false

		return c, nil, nil, tconf
	}
	main := s.newTeleportWithConfig(mainConfig())
	defer main.StopAll()

	// Create a Teleport instance with a Proxy.
	nodePorts := s.getPorts(3)
	proxyReverseTunnelPort, proxyWebPort, proxySSHPort := nodePorts[0], nodePorts[1], nodePorts[2]
	proxyConfig := ProxyConfig{
		Name:              "cluster-main-proxy",
		SSHPort:           proxySSHPort,
		WebPort:           proxyWebPort,
		ReverseTunnelPort: proxyReverseTunnelPort,
	}
	proxyTunnel, err := main.StartProxy(proxyConfig)
	c.Assert(err, check.IsNil)

	proxyOneBackend := utils.MustParseAddr(net.JoinHostPort(Loopback, main.GetPortReverseTunnel()))
	lb.AddBackend(*proxyOneBackend)
	proxyTwoBackend := utils.MustParseAddr(net.JoinHostPort(Loopback, strconv.Itoa(proxyReverseTunnelPort)))
	lb.AddBackend(*proxyTwoBackend)

	// Create a Teleport instance with a Node.
	nodeConfig := func() *service.Config {
		tconf := service.MakeDefaultConfig()
		tconf.Hostname = "cluster-main-node"
		tconf.Console = nil
		tconf.Token = "token"
		tconf.AuthServers = []utils.NetAddr{
			utils.NetAddr{
				AddrNetwork: "tcp",
				Addr:        net.JoinHostPort(Loopback, main.GetPortWeb()),
			},
		}

		tconf.Auth.Enabled = false

		tconf.Proxy.Enabled = false

		tconf.SSH.Enabled = true

		return tconf
	}
	_, err = main.StartNode(nodeConfig())
	c.Assert(err, check.IsNil)

	// Wait for active tunnel connections to be established.
	waitForActiveTunnelConnections(c, main.Tunnel, Site, 1)
	waitForActiveTunnelConnections(c, proxyTunnel, Site, 1)

	// Execute the connection via first proxy.
	cfg := ClientConfig{
		Login:   s.me.Username,
		Cluster: Site,
		Host:    "cluster-main-node",
	}
	output, err := runCommand(main, []string{"echo", "hello world"}, cfg, 1)
	c.Assert(err, check.IsNil)
	c.Assert(output, check.Equals, "hello world\n")

	// Execute the connection via second proxy, should work. This command is
	// tried 10 times with 250 millisecond delay between each attempt to allow
	// the discovery request to be received and the connection added to the agent
	// pool.
	cfgProxy := ClientConfig{
		Login:   s.me.Username,
		Cluster: Site,
		Host:    "cluster-main-node",
		Proxy:   &proxyConfig,
	}

	output, err = runCommand(main, []string{"echo", "hello world"}, cfgProxy, 10)
	c.Assert(err, check.IsNil)
	c.Assert(output, check.Equals, "hello world\n")

	// Remove second proxy from LB.
	c.Assert(lb.RemoveBackend(*proxyTwoBackend), check.IsNil)
	waitForActiveTunnelConnections(c, main.Tunnel, Site, 1)

	// Requests going via main proxy will succeed. Requests going via second
	// proxy will fail.
	output, err = runCommand(main, []string{"echo", "hello world"}, cfg, 1)
	c.Assert(err, check.IsNil)
	c.Assert(output, check.Equals, "hello world\n")
	_, err = runCommand(main, []string{"echo", "hello world"}, cfgProxy, 1)
	c.Assert(err, check.NotNil)

	// Add second proxy to LB, both should have a connection.
	lb.AddBackend(*proxyTwoBackend)
	waitForActiveTunnelConnections(c, main.Tunnel, Site, 1)
	waitForActiveTunnelConnections(c, proxyTunnel, Site, 1)

	// Requests going via both proxies will succeed.
	output, err = runCommand(main, []string{"echo", "hello world"}, cfg, 1)
	c.Assert(err, check.IsNil)
	c.Assert(output, check.Equals, "hello world\n")
	output, err = runCommand(main, []string{"echo", "hello world"}, cfgProxy, 40)
	c.Assert(err, check.IsNil)
	c.Assert(output, check.Equals, "hello world\n")

	// Stop everything.
	err = proxyTunnel.Shutdown(context.Background())
	c.Assert(err, check.IsNil)
	err = main.StopAll()
	c.Assert(err, check.IsNil)
}

// waitForActiveTunnelConnections  waits for remote cluster to report a minimum number of active connections
func waitForActiveTunnelConnections(c *check.C, tunnel reversetunnel.Server, clusterName string, expectedCount int) {
	var lastCount int
	var lastErr error
	for i := 0; i < 30; i++ {
		cluster, err := tunnel.GetSite(clusterName)
		if err != nil {
			lastErr = err
			continue
		}
		lastCount = cluster.GetTunnelsCount()
		if lastCount >= expectedCount {
			return
		}
		time.Sleep(1 * time.Second)
	}
	c.Fatalf("Connections count on %v: %v, expected %v, last error: %v", clusterName, lastCount, expectedCount, lastErr)
}

// waitForProxyCount waits a set time for the proxy count in clusterName to
// reach some value.
func waitForProxyCount(t *TeleInstance, clusterName string, count int) error {
	var counts map[string]int
	start := time.Now()
	for time.Since(start) < 17*time.Second {
		counts = t.Pool.Counts()
		if counts[clusterName] == count {
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}

	return trace.BadParameter("proxy count on %v: %v (wanted %v)", clusterName, counts[clusterName], count)
}

// waitForNodeCount waits for a certain number of nodes to show up in the remote site.
func waitForNodeCount(t *TeleInstance, clusterName string, count int) error {
	for i := 0; i < 30; i++ {
		remoteSite, err := t.Tunnel.GetSite(clusterName)
		if err != nil {
			return trace.Wrap(err)
		}
		accessPoint, err := remoteSite.CachingAccessPoint()
		if err != nil {
			return trace.Wrap(err)
		}
		nodes, err := accessPoint.GetNodes(defaults.Namespace)
		if err != nil {
			return trace.Wrap(err)
		}
		if len(nodes) == count {
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return trace.BadParameter("did not find %v nodes", count)
}

// waitForTunnelConnections waits for remote tunnels connections
func waitForTunnelConnections(c *check.C, authServer *auth.AuthServer, clusterName string, expectedCount int) {
	var conns []services.TunnelConnection
	for i := 0; i < 30; i++ {
		conns, err := authServer.Presence.GetTunnelConnections(clusterName)
		if err != nil {
			c.Fatal(err)
		}
		if len(conns) == expectedCount {
			return
		}
		time.Sleep(1 * time.Second)
	}
	c.Fatalf("proxy count on %v: %v, expected %v", clusterName, len(conns), expectedCount)
}

// TestExternalClient tests if we can connect to a node in a Teleport
// cluster. Both normal and recording proxies are tested.
func (s *IntSuite) TestExternalClient(c *check.C) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

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
				LocalAuth:        services.NewBool(true),
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
		defer t.StopAll()

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
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

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
				LocalAuth:        services.NewBool(true),
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
		defer t.StopAll()

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
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

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
				LocalAuth:           services.NewBool(true),
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
		defer t.StopAll()

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
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	var err error

	// create a teleport instance with auth, proxy, and node
	makeConfig := func() (*check.C, []string, []*InstanceSecrets, *service.Config) {
		clusterConfig, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
			SessionRecording: services.RecordOff,
			LocalAuth:        services.NewBool(true),
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
	defer t.StopAll()

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
		cl.Stdout = myTerm
		cl.Stdin = myTerm
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

	// audit log should have the fact that the session occurred recorded in it
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
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

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
	}{
		// 0 - No PAM support, session should work but no PAM related output.
		{
			inEnabled:     false,
			inServiceName: "",
			outContains:   []string{},
		},
		// 1 - PAM enabled, module account and session functions return success.
		{
			inEnabled:     true,
			inServiceName: "teleport-success",
			outContains: []string{
				"Account opened successfully.",
				"Session open successfully.",
			},
		},
		// 2 - PAM enabled, module account functions fail.
		{
			inEnabled:     true,
			inServiceName: "teleport-acct-failure",
			outContains:   []string{},
		},
		// 3 - PAM enabled, module session functions fail.
		{
			inEnabled:     true,
			inServiceName: "teleport-session-failure",
			outContains:   []string{},
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
		defer t.StopAll()

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

			cl.Stdout = termSession
			cl.Stdin = termSession

			termSession.Type("\aecho hi\n\r\aexit\n\r\a")
			err = cl.SSH(context.TODO(), []string{}, false)
			c.Assert(err, check.IsNil)

			cancel()
		}()

		// Wait for the session to end or timeout after 10 seconds.
		select {
		case <-time.After(10 * time.Second):
			c.Fatalf("Timeout exceeded waiting for session to complete.")
		case <-ctx.Done():
		}

		// If any output is expected, check to make sure it was output.
		if len(tt.outContains) > 0 {
			for _, expectedOutput := range tt.outContains {
				output := termSession.Output(100)
				c.Assert(strings.Contains(output, expectedOutput), check.Equals, true)
			}
		}
	}
}

// TestRotateSuccess tests full cycle cert authority rotation
func (s *IntSuite) TestRotateSuccess(c *check.C) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

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

	runErrCh := make(chan error, 1)
	go func() {
		runErrCh <- service.Run(ctx, *config, func(cfg *service.Config) (service.Process, error) {
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
	initialCreds, err := GenerateUserCreds(UserCredsRequest{Process: svc, Username: s.me.Username})
	c.Assert(err, check.IsNil)

	l.Infof("Service started. Setting rotation state to %v", services.RotationPhaseUpdateClients)

	// start rotation
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: services.RotationPhaseInit,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	hostCA, err := svc.GetAuthServer().GetCertAuthority(services.CertAuthID{Type: services.HostCA, DomainName: Site}, false)
	c.Assert(err, check.IsNil)
	l.Debugf("Cert authority: %v", auth.CertAuthorityInfo(hostCA))

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
		Host:  Loopback,
		Port:  t.GetPortSSHInt(),
	}
	clt, err := t.NewClientWithCreds(cfg, *initialCreds)
	c.Assert(err, check.IsNil)

	// client works as is before servers have been rotated
	err = runAndMatch(clt, 8, []string{"echo", "hello world"}, ".*hello world.*")
	c.Assert(err, check.IsNil)

	l.Infof("Service reloaded. Setting rotation state to %v", services.RotationPhaseUpdateServers)

	// move to the next phase
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: services.RotationPhaseUpdateServers,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	hostCA, err = svc.GetAuthServer().GetCertAuthority(services.CertAuthID{Type: services.HostCA, DomainName: Site}, false)
	c.Assert(err, check.IsNil)
	l.Debugf("Cert authority: %v", auth.CertAuthorityInfo(hostCA))

	// wait until service reloaded
	svc, err = waitForReload(serviceC, svc)
	c.Assert(err, check.IsNil)

	// new credentials will work from this phase to others
	newCreds, err := GenerateUserCreds(UserCredsRequest{Process: svc, Username: s.me.Username})
	c.Assert(err, check.IsNil)

	clt, err = t.NewClientWithCreds(cfg, *newCreds)
	c.Assert(err, check.IsNil)

	// new client works
	err = runAndMatch(clt, 8, []string{"echo", "hello world"}, ".*hello world.*")
	c.Assert(err, check.IsNil)

	l.Infof("Service reloaded. Setting rotation state to %v.", services.RotationPhaseStandby)

	// complete rotation
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: services.RotationPhaseStandby,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	hostCA, err = svc.GetAuthServer().GetCertAuthority(services.CertAuthID{Type: services.HostCA, DomainName: Site}, false)
	c.Assert(err, check.IsNil)
	l.Debugf("Cert authority: %v", auth.CertAuthorityInfo(hostCA))

	// wait until service reloaded
	svc, err = waitForReload(serviceC, svc)
	c.Assert(err, check.IsNil)

	// new client still works
	err = runAndMatch(clt, 8, []string{"echo", "hello world"}, ".*hello world.*")
	c.Assert(err, check.IsNil)

	l.Infof("Service reloaded. Rotation has completed. Shuttting down service.")

	// shut down the service
	cancel()
	// close the service without waiting for the connections to drain
	svc.Close()

	select {
	case err := <-runErrCh:
		c.Assert(err, check.IsNil)
	case <-time.After(20 * time.Second):
		c.Fatalf("failed to shut down the server")
	}
}

// TestRotateRollback tests cert authority rollback
func (s *IntSuite) TestRotateRollback(c *check.C) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

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

	runErrCh := make(chan error, 1)
	go func() {
		runErrCh <- service.Run(ctx, *config, func(cfg *service.Config) (service.Process, error) {
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
	initialCreds, err := GenerateUserCreds(UserCredsRequest{Process: svc, Username: s.me.Username})
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
		Host:  Loopback,
		Port:  t.GetPortSSHInt(),
	}
	clt, err := t.NewClientWithCreds(cfg, *initialCreds)
	c.Assert(err, check.IsNil)

	// client works as is before servers have been rotated
	err = runAndMatch(clt, 8, []string{"echo", "hello world"}, ".*hello world.*")
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
	err = runAndMatch(clt, 8, []string{"echo", "hello world"}, ".*hello world.*")
	c.Assert(err, check.IsNil)

	l.Infof("Service reloaded. Rotation has completed. Shuttting down service.")

	// shut down the service
	cancel()
	// close the service without waiting for the connections to drain
	svc.Close()

	select {
	case err := <-runErrCh:
		c.Assert(err, check.IsNil)
	case <-time.After(20 * time.Second):
		c.Fatalf("failed to shut down the server")
	}
}

// TestRotateTrustedClusters tests CA rotation support for trusted clusters
func (s *IntSuite) TestRotateTrustedClusters(c *check.C) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

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
	runErrCh := make(chan error, 1)
	go func() {
		runErrCh <- service.Run(ctx, *config, func(cfg *service.Config) (service.Process, error) {
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

	// create auxiliary cluster and setup trust
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
	err = aux.Process.GetAuthServer().UpsertRole(ctx, role)
	c.Assert(err, check.IsNil)
	trustedClusterToken := "trusted-clsuter-token"
	err = svc.GetAuthServer().UpsertToken(
		services.MustCreateProvisionToken(trustedClusterToken, []teleport.Role{teleport.RoleTrustedCluster}, time.Time{}))
	c.Assert(err, check.IsNil)
	trustedCluster := main.Secrets.AsTrustedCluster(trustedClusterToken, services.RoleMap{
		{Remote: mainDevs, Local: []string{auxDevs}},
	})
	c.Assert(aux.Start(), check.IsNil)

	// try and upsert a trusted cluster
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	tryCreateTrustedCluster(c, aux.Process.GetAuthServer(), trustedCluster)
	waitForTunnelConnections(c, svc.GetAuthServer(), aux.Secrets.SiteName, 1)

	// capture credentials before has reload started to simulate old client
	initialCreds, err := GenerateUserCreds(UserCredsRequest{
		Process:  svc,
		Username: s.me.Username,
	})
	c.Assert(err, check.IsNil)

	// credentials should work
	cfg := ClientConfig{
		Login:   s.me.Username,
		Host:    Loopback,
		Cluster: clusterAux,
		Port:    aux.GetPortSSHInt(),
	}
	clt, err := main.NewClientWithCreds(cfg, *initialCreds)
	c.Assert(err, check.IsNil)

	err = runAndMatch(clt, 8, []string{"echo", "hello world"}, ".*hello world.*")
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
	err = runAndMatch(clt, 8, []string{"echo", "hello world"}, ".*hello world.*")
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
	newCreds, err := GenerateUserCreds(UserCredsRequest{Process: svc, Username: s.me.Username})
	c.Assert(err, check.IsNil)

	clt, err = main.NewClientWithCreds(cfg, *newCreds)
	c.Assert(err, check.IsNil)

	// new client works
	err = runAndMatch(clt, 8, []string{"echo", "hello world"}, ".*hello world.*")
	c.Assert(err, check.IsNil)

	l.Infof("Service reloaded. Setting rotation state to %v.", services.RotationPhaseStandby)

	// complete rotation
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: services.RotationPhaseStandby,
		Mode:        services.RotationModeManual,
	})
	c.Assert(err, check.IsNil)

	// wait until service reloaded
	l.Infof("Wating for service reload")
	svc, err = waitForReload(serviceC, svc)
	c.Assert(err, check.IsNil)
	l.Infof("Service reload completed, waiting for phase")

	err = waitForPhase(services.RotationPhaseStandby)
	c.Assert(err, check.IsNil)
	l.Infof("Phase completed.")

	// new client still works
	err = runAndMatch(clt, 8, []string{"echo", "hello world"}, ".*hello world.*")
	c.Assert(err, check.IsNil)

	l.Infof("Service reloaded. Rotation has completed. Shuttting down service.")

	// shut down the service
	cancel()
	// close the service without waiting for the connections to drain
	svc.Close()

	select {
	case err := <-runErrCh:
		c.Assert(err, check.IsNil)
	case <-time.After(20 * time.Second):
		c.Fatalf("failed to shut down the server")
	}
}

// TestRotateChangeSigningAlg tests the change of CA signing algorithm on
// manual rotation.
func (s *IntSuite) TestRotateChangeSigningAlg(c *check.C) {
	// Start with an instance using default signing alg.
	tconf := rotationConfig(true)
	t := NewInstance(InstanceConfig{ClusterName: Site, HostID: HostID, NodeName: Host, Ports: s.getPorts(5), Priv: s.priv, Pub: s.pub})
	logins := []string{s.me.Username}
	for _, login := range logins {
		t.AddUser(login, []string{login})
	}
	config, err := t.GenerateConfig(nil, tconf)
	c.Assert(err, check.IsNil)

	serviceC := make(chan *service.TeleportProcess, 20)
	runErrCh := make(chan error, 1)

	restart := func(svc *service.TeleportProcess, cancel func()) (*service.TeleportProcess, func()) {
		if svc != nil && cancel != nil {
			// shut down the service
			cancel()
			// close the service without waiting for the connections to drain
			err := svc.Close()
			c.Assert(err, check.IsNil)
			err = svc.Wait()
			c.Assert(err, check.IsNil)

			select {
			case err := <-runErrCh:
				c.Assert(err, check.IsNil)
			case <-time.After(20 * time.Second):
				c.Fatalf("failed to shut down the server")
			}
		}

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			runErrCh <- service.Run(ctx, *config, func(cfg *service.Config) (service.Process, error) {
				svc, err := service.NewTeleport(cfg)
				if err == nil {
					serviceC <- svc
				}
				return svc, err
			})
		}()

		svc, err = waitForProcessStart(serviceC)
		c.Assert(err, check.IsNil)
		return svc, cancel
	}

	assertSigningAlg := func(svc *service.TeleportProcess, alg string) {
		hostCA, err := svc.GetAuthServer().GetCertAuthority(services.CertAuthID{Type: services.HostCA, DomainName: Site}, false)
		c.Assert(err, check.IsNil)
		c.Assert(hostCA.GetSigningAlg(), check.Equals, alg)

		userCA, err := svc.GetAuthServer().GetCertAuthority(services.CertAuthID{Type: services.UserCA, DomainName: Site}, false)
		c.Assert(err, check.IsNil)
		c.Assert(userCA.GetSigningAlg(), check.Equals, alg)
	}

	rotate := func(svc *service.TeleportProcess, mode string) *service.TeleportProcess {
		c.Logf("rotation phase: %q", services.RotationPhaseInit)
		err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
			TargetPhase: services.RotationPhaseInit,
			Mode:        mode,
		})
		c.Assert(err, check.IsNil)

		// wait until service phase update to be broadcasted (init phase does not trigger reload)
		err = waitForProcessEvent(svc, service.TeleportPhaseChangeEvent, 10*time.Second)
		c.Assert(err, check.IsNil)

		c.Logf("rotation phase: %q", services.RotationPhaseUpdateClients)
		err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
			TargetPhase: services.RotationPhaseUpdateClients,
			Mode:        mode,
		})
		c.Assert(err, check.IsNil)

		// wait until service reload
		svc, err = waitForReload(serviceC, svc)
		c.Assert(err, check.IsNil)

		c.Logf("rotation phase: %q", services.RotationPhaseUpdateServers)
		err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
			TargetPhase: services.RotationPhaseUpdateServers,
			Mode:        mode,
		})
		c.Assert(err, check.IsNil)

		// wait until service reloaded
		svc, err = waitForReload(serviceC, svc)
		c.Assert(err, check.IsNil)

		c.Logf("rotation phase: %q", services.RotationPhaseStandby)
		err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
			TargetPhase: services.RotationPhaseStandby,
			Mode:        mode,
		})
		c.Assert(err, check.IsNil)

		// wait until service reloaded
		svc, err = waitForReload(serviceC, svc)
		c.Assert(err, check.IsNil)

		return svc
	}

	// Start the instance.
	svc, cancel := restart(nil, nil)

	c.Log("default signature algorithm due to empty config value")
	// Verify the default signing algorithm with config value empty.
	assertSigningAlg(svc, defaults.CASignatureAlgorithm)

	c.Log("change signature algorithm with custom config value and manual rotation")
	// Change the signing algorithm in config file.
	signingAlg := ssh.SigAlgoRSA
	config.CASignatureAlgorithm = &signingAlg
	svc, cancel = restart(svc, cancel)
	// Do a manual rotation - this should change the signing algorithm.
	svc = rotate(svc, services.RotationModeManual)
	assertSigningAlg(svc, ssh.SigAlgoRSA)

	c.Log("preserve signature algorithm with empty config value and manual rotation")
	// Unset the config value.
	config.CASignatureAlgorithm = nil
	svc, cancel = restart(svc, cancel)

	// Do a manual rotation - this should leave the signing algorithm
	// unaffected because config value is not set.
	svc = rotate(svc, services.RotationModeManual)
	assertSigningAlg(svc, ssh.SigAlgoRSA)

	// shut down the service
	cancel()
	// close the service without waiting for the connections to drain
	svc.Close()

	select {
	case err := <-runErrCh:
		c.Assert(err, check.IsNil)
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
			time.Sleep(500 * time.Millisecond)
			continue
		}
		out := output.String()
		out = replaceNewlines(out)
		matched, _ := regexp.MatchString(pattern, out)
		if matched {
			return nil
		}
		err = trace.CompareFailed("output %q did not match pattern %q", out, pattern)
		time.Sleep(500 * time.Millisecond)
	}
	return err
}

// TestWindowChange checks if custom Teleport window change requests are sent
// when the server side PTY changes its size.
func (s *IntSuite) TestWindowChange(c *check.C) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	t := s.newTeleport(c, nil, true)
	defer t.StopAll()

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

		cl.Stdout = personA
		cl.Stdin = personA

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

		cl.Stdout = personB
		cl.Stdin = personB

		// Change the size of the window immediately after it is created.
		cl.OnShellCreated = func(s *ssh.Session, c *ssh.Client, terminal io.ReadWriteCloser) (exit bool, err error) {
			err = s.WindowChange(48, 160)
			if err != nil {
				return true, trace.Wrap(err)
			}
			return false, nil
		}

		for i := 0; i < 10; i++ {
			err = cl.Join(context.TODO(), defaults.Namespace, session.ID(sessionID), personB)
			if err == nil {
				break
			}
		}
		c.Assert(err, check.IsNil)
	}

	// waitForOutput checks that the output of the passed in terminal contains
	// one of the strings in `outputs` until some timeout has occurred.
	waitForOutput := func(t *Terminal, outputs ...string) error {
		tickerCh := time.Tick(500 * time.Millisecond)
		timeoutCh := time.After(30 * time.Second)
		for {
			select {
			case <-tickerCh:
				out := t.Output(5000)
				for _, s := range outputs {
					if strings.Contains(out, s) {
						return nil
					}
				}
			case <-timeoutCh:
				return trace.BadParameter("timed out waiting for output, last output: %q doesn't contain any of the expected substrings: %q", t.Output(5000), outputs)
			}
		}

	}

	// Open session, the initial size will be 80x24.
	go openSession()

	// Use the "printf" command to print the terminal size on the screen and
	// make sure it is 80x25.
	personA.Type("\atput cols; tput lines\n\r\a")
	err := waitForOutput(personA, "80\r\n25", "80\n\r25", "80\n25")
	c.Assert(err, check.IsNil)

	// As soon as person B joins the session, the terminal is resized to 160x48.
	// Have another user join the session. As soon as the second shell is
	// created, the window is resized to 160x48 (see joinSession implementation).
	go joinSession()

	// Use the "printf" command to print the window size again and make sure it's
	// 160x48.
	personA.Type("\atput cols; tput lines\n\r\a")
	err = waitForOutput(personA, "160\r\n48", "160\n\r48", "160\n48")
	c.Assert(err, check.IsNil)

	// Close the session.
	personA.Type("\aexit\r\n\a")
}

// TestList checks that the list of servers returned is identity aware.
func (s *IntSuite) TestList(c *check.C) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	// Create and start a Teleport cluster with auth, proxy, and node.
	makeConfig := func() (*check.C, []string, []*InstanceSecrets, *service.Config) {
		clusterConfig, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
			SessionRecording: services.RecordOff,
			LocalAuth:        services.NewBool(true),
		})
		c.Assert(err, check.IsNil)

		tconf := service.MakeDefaultConfig()
		tconf.Hostname = "server-01"
		tconf.Auth.Enabled = true
		tconf.Auth.ClusterConfig = clusterConfig
		tconf.Proxy.Enabled = true
		tconf.Proxy.DisableWebService = true
		tconf.Proxy.DisableWebInterface = true
		tconf.SSH.Enabled = true
		tconf.SSH.Labels = map[string]string{
			"role": "worker",
		}

		return c, nil, nil, tconf
	}
	t := s.newTeleportWithConfig(makeConfig())
	defer t.StopAll()

	// Create and start a Teleport node.
	nodeSSHPort := s.getPorts(1)[0]
	nodeConfig := func() *service.Config {
		tconf := service.MakeDefaultConfig()
		tconf.Hostname = "server-02"
		tconf.SSH.Enabled = true
		tconf.SSH.Addr.Addr = net.JoinHostPort(t.Hostname, fmt.Sprintf("%v", nodeSSHPort))
		tconf.SSH.Labels = map[string]string{
			"role": "database",
		}

		return tconf
	}
	_, err := t.StartNode(nodeConfig())
	c.Assert(err, check.IsNil)

	// Get an auth client to the cluster.
	clt := t.GetSiteAPI(Site)
	c.Assert(clt, check.NotNil)

	// Wait 10 seconds for both nodes to show up to make sure they both have
	// registered themselves.
	waitForNodes := func(clt auth.ClientI, count int) error {
		tickCh := time.Tick(500 * time.Millisecond)
		stopCh := time.After(10 * time.Second)
		for {
			select {
			case <-tickCh:
				nodesInCluster, err := clt.GetNodes(defaults.Namespace, services.SkipValidation())
				if err != nil && !trace.IsNotFound(err) {
					return trace.Wrap(err)
				}
				if got, want := len(nodesInCluster), count; got == want {
					return nil
				}
			case <-stopCh:
				return trace.BadParameter("waited 10s, did find %v nodes", count)
			}
		}
	}
	err = waitForNodes(clt, 2)
	c.Assert(err, check.IsNil)

	var tests = []struct {
		inRoleName string
		inLabels   services.Labels
		inLogin    string
		outNodes   []string
	}{
		// 0 - Role has label "role:worker", only server-01 is returned.
		{
			inRoleName: "worker-only",
			inLogin:    "foo",
			inLabels:   services.Labels{"role": []string{"worker"}},
			outNodes:   []string{"server-01"},
		},
		// 1 - Role has label "role:database", only server-02 is returned.
		{
			inRoleName: "database-only",
			inLogin:    "bar",
			inLabels:   services.Labels{"role": []string{"database"}},
			outNodes:   []string{"server-02"},
		},
		// 2 - Role has wildcard label, all nodes are returned server-01 and server-2.
		{
			inRoleName: "worker-and-database",
			inLogin:    "baz",
			inLabels:   services.Labels{services.Wildcard: []string{services.Wildcard}},
			outNodes:   []string{"server-01", "server-02"},
		},
	}

	for _, tt := range tests {
		// Create role with logins and labels for this test.
		role, err := services.NewRole(tt.inRoleName, services.RoleSpecV3{
			Allow: services.RoleConditions{
				Logins:     []string{tt.inLogin},
				NodeLabels: tt.inLabels,
			},
		})
		c.Assert(err, check.IsNil)

		// Create user, role, and generate credentials.
		err = SetupUser(t.Process, tt.inLogin, []services.Role{role})
		c.Assert(err, check.IsNil)
		initialCreds, err := GenerateUserCreds(UserCredsRequest{Process: t.Process, Username: tt.inLogin})
		c.Assert(err, check.IsNil)

		// Create a Teleport client.
		cfg := ClientConfig{
			Login: tt.inLogin,
			Port:  t.GetPortSSHInt(),
		}
		userClt, err := t.NewClientWithCreds(cfg, *initialCreds)
		c.Assert(err, check.IsNil)

		// Get list of nodes and check that the returned nodes match the
		// expected nodes.
		nodes, err := userClt.ListNodes(context.Background())
		c.Assert(err, check.IsNil)
		for _, node := range nodes {
			ok := utils.SliceContainsStr(tt.outNodes, node.GetHostname())
			if !ok {
				c.Fatalf("Got nodes: %v, want: %v.", nodes, tt.outNodes)
			}
		}
	}
}

// TestCmdLabels verifies the behavior of running commands via labels
// with a mixture of regular and reversetunnel nodes.
func (s *IntSuite) TestCmdLabels(c *check.C) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	// InsecureDevMode needed for IoT node handshake
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	// Create and start a Teleport cluster with auth, proxy, and node.
	makeConfig := func() *service.Config {
		clusterConfig, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
			SessionRecording: services.RecordOff,
			LocalAuth:        services.NewBool(true),
		})
		c.Assert(err, check.IsNil)

		tconf := service.MakeDefaultConfig()
		tconf.Hostname = "server-01"
		tconf.Auth.Enabled = true
		tconf.Auth.ClusterConfig = clusterConfig
		tconf.Proxy.Enabled = true
		tconf.Proxy.DisableWebService = false
		tconf.Proxy.DisableWebInterface = true
		tconf.SSH.Enabled = true
		tconf.SSH.Labels = map[string]string{
			"role": "worker",
			"spam": "eggs",
		}

		return tconf
	}
	t := s.newTeleportWithConfig(c, nil, nil, makeConfig())
	defer t.StopAll()

	// Create and start a reversetunnel node.
	nodeConfig := func() *service.Config {
		tconf := service.MakeDefaultConfig()
		tconf.Hostname = "server-02"
		tconf.SSH.Enabled = true
		tconf.SSH.Labels = map[string]string{
			"role": "database",
			"spam": "eggs",
		}

		return tconf
	}
	_, err := t.StartReverseTunnelNode(nodeConfig())
	c.Assert(err, check.IsNil)

	// test label patterns that match both nodes, and each
	// node individually.
	tts := []struct {
		command []string
		labels  map[string]string
		expect  string
	}{
		{
			command: []string{"echo", "two"},
			labels:  map[string]string{"spam": "eggs"},
			expect:  "two\ntwo\n",
		},
		{
			command: []string{"echo", "worker"},
			labels:  map[string]string{"role": "worker"},
			expect:  "worker\n",
		},
		{
			command: []string{"echo", "database"},
			labels:  map[string]string{"role": "database"},
			expect:  "database\n",
		},
	}

	for _, tt := range tts {
		cfg := ClientConfig{
			Login:  s.me.Username,
			Labels: tt.labels,
		}

		output, err := runCommand(t, tt.command, cfg, 1)
		c.Assert(err, check.IsNil)
		c.Assert(output, check.Equals, tt.expect)
	}
}

// TestDataTransfer makes sure that a "session.data" event is emitted at the
// end of a session that matches the amount of data that was transferred.
func (s *IntSuite) TestDataTransfer(c *check.C) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	KB := 1024
	MB := 1048576

	// Create a Teleport cluster.
	main := s.newTeleport(c, nil, true)
	defer main.StopAll()

	// Create a client to the above Teleport cluster.
	clientConfig := ClientConfig{
		Login:   s.me.Username,
		Cluster: Site,
		Host:    Host,
		Port:    main.GetPortSSHInt(),
	}

	// Write 1 MB to stdout.
	command := []string{"dd", "if=/dev/zero", "bs=1024", "count=1024"}
	output, err := runCommand(main, command, clientConfig, 1)
	c.Assert(err, check.IsNil)

	// Make sure exactly 1 MB was written to output.
	c.Assert(len(output) == MB, check.Equals, true)

	// Make sure the session.data event was emitted to the audit log.
	eventFields, err := findEventInLog(main, events.SessionDataEvent)
	c.Assert(err, check.IsNil)

	// Make sure the audit event shows that 1 MB was written to the output.
	c.Assert(eventFields.GetInt(events.DataReceived) > MB, check.Equals, true)
	c.Assert(eventFields.GetInt(events.DataTransmitted) > KB, check.Equals, true)
}

func (s *IntSuite) TestBPFInteractive(c *check.C) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	// Check if BPF tests can be run on this host.
	err := canTestBPF()
	if err != nil {
		c.Skip(fmt.Sprintf("Tests for BPF functionality can not be run: %v.", err))
		return
	}

	lsPath, err := exec.LookPath("ls")
	c.Assert(err, check.IsNil)

	var tests = []struct {
		inSessionRecording string
		inBPFEnabled       bool
		outFound           bool
	}{
		// For session recorded at the node, enhanced events should be found.
		{
			inSessionRecording: services.RecordAtNode,
			inBPFEnabled:       true,
			outFound:           true,
		},
		// For session recorded at the node, but BPF is turned off, no events
		// should be found.
		{
			inSessionRecording: services.RecordAtNode,
			inBPFEnabled:       false,
			outFound:           false,
		},
		// For session recorded at the proxy, enhanced events should not be found.
		// BPF turned off simulates an OpenSSH node.
		{
			inSessionRecording: services.RecordAtProxy,
			inBPFEnabled:       false,
			outFound:           false,
		},
	}
	for _, tt := range tests {
		// Create temporary directory where cgroup2 hierarchy will be mounted.
		dir, err := ioutil.TempDir("", "cgroup-test")
		c.Assert(err, check.IsNil)
		defer os.RemoveAll(dir)

		// Create and start a Teleport cluster.
		makeConfig := func() (*check.C, []string, []*InstanceSecrets, *service.Config) {
			clusterConfig, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
				SessionRecording: tt.inSessionRecording,
				LocalAuth:        services.NewBool(true),
			})
			c.Assert(err, check.IsNil)

			// Create default config.
			tconf := service.MakeDefaultConfig()

			// Configure Auth.
			tconf.Auth.Preference.SetSecondFactor("off")
			tconf.Auth.Enabled = true
			tconf.Auth.ClusterConfig = clusterConfig

			// Configure Proxy.
			tconf.Proxy.Enabled = true
			tconf.Proxy.DisableWebService = false
			tconf.Proxy.DisableWebInterface = true

			// Configure Node. If session are being recorded at the proxy, don't enable
			// BPF to simulate an OpenSSH node.
			tconf.SSH.Enabled = true
			if tt.inBPFEnabled {
				tconf.SSH.BPF.Enabled = true
				tconf.SSH.BPF.CgroupPath = dir
			}
			return c, nil, nil, tconf
		}
		main := s.newTeleportWithConfig(makeConfig())
		defer main.StopAll()

		// Create a client terminal and context to signal when the client is done
		// with the terminal.
		term := NewTerminal(250)
		doneContext, doneCancel := context.WithCancel(context.Background())

		func() {
			client, err := main.NewClient(ClientConfig{
				Login:   s.me.Username,
				Cluster: Site,
				Host:    Host,
				Port:    main.GetPortSSHInt(),
			})
			c.Assert(err, check.IsNil)

			// Connect terminal to std{in,out} of client.
			client.Stdout = term
			client.Stdin = term

			// "Type" a command into the terminal.
			term.Type(fmt.Sprintf("\a%v\n\r\aexit\n\r\a", lsPath))
			err = client.SSH(context.TODO(), []string{}, false)
			c.Assert(err, check.IsNil)

			// Signal that the client has finished the interactive session.
			doneCancel()
		}()

		// Wait 10 seconds for the client to finish up the interactive session.
		select {
		case <-time.After(10 * time.Second):
			c.Fatalf("Timed out waiting for client to finish interactive session.")
		case <-doneContext.Done():
		}

		// Enhanced events should show up for session recorded at the node but not
		// at the proxy.
		if tt.outFound {
			_, err = findCommandEventInLog(main, events.SessionCommandEvent, lsPath)
			c.Assert(err, check.IsNil)
		} else {
			_, err = findCommandEventInLog(main, events.SessionCommandEvent, lsPath)
			c.Assert(err, check.NotNil)
		}
	}
}

func (s *IntSuite) TestBPFExec(c *check.C) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	// Check if BPF tests can be run on this host.
	err := canTestBPF()
	if err != nil {
		c.Skip(fmt.Sprintf("Tests for BPF functionality can not be run: %v.", err))
		return
	}

	lsPath, err := exec.LookPath("ls")
	c.Assert(err, check.IsNil)

	var tests = []struct {
		inSessionRecording string
		inBPFEnabled       bool
		outFound           bool
	}{
		// For session recorded at the node, enhanced events should be found.
		{
			inSessionRecording: services.RecordAtNode,
			inBPFEnabled:       true,
			outFound:           true,
		},
		// For session recorded at the node, but BPF is turned off, no events
		// should be found.
		{
			inSessionRecording: services.RecordAtNode,
			inBPFEnabled:       false,
			outFound:           false,
		},
		// For session recorded at the proxy, enhanced events should not be found.
		// BPF turned off simulates an OpenSSH node.
		{
			inSessionRecording: services.RecordAtProxy,
			inBPFEnabled:       false,
			outFound:           false,
		},
	}
	for _, tt := range tests {
		// Create temporary directory where cgroup2 hierarchy will be mounted.
		dir, err := ioutil.TempDir("", "cgroup-test")
		c.Assert(err, check.IsNil)
		defer os.RemoveAll(dir)

		// Create and start a Teleport cluster.
		makeConfig := func() (*check.C, []string, []*InstanceSecrets, *service.Config) {
			clusterConfig, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
				SessionRecording: tt.inSessionRecording,
				LocalAuth:        services.NewBool(true),
			})
			c.Assert(err, check.IsNil)

			// Create default config.
			tconf := service.MakeDefaultConfig()

			// Configure Auth.
			tconf.Auth.Preference.SetSecondFactor("off")
			tconf.Auth.Enabled = true
			tconf.Auth.ClusterConfig = clusterConfig

			// Configure Proxy.
			tconf.Proxy.Enabled = true
			tconf.Proxy.DisableWebService = false
			tconf.Proxy.DisableWebInterface = true

			// Configure Node. If session are being recorded at the proxy, don't enable
			// BPF to simulate an OpenSSH node.
			tconf.SSH.Enabled = true
			if tt.inBPFEnabled {
				tconf.SSH.BPF.Enabled = true
				tconf.SSH.BPF.CgroupPath = dir
			}
			return c, nil, nil, tconf
		}
		main := s.newTeleportWithConfig(makeConfig())
		defer main.StopAll()

		// Create a client to the above Teleport cluster.
		clientConfig := ClientConfig{
			Login:   s.me.Username,
			Cluster: Site,
			Host:    Host,
			Port:    main.GetPortSSHInt(),
		}

		// Run exec command.
		_, err = runCommand(main, []string{lsPath}, clientConfig, 1)
		c.Assert(err, check.IsNil)

		// Enhanced events should show up for session recorded at the node but not
		// at the proxy.
		if tt.outFound {
			_, err = findCommandEventInLog(main, events.SessionCommandEvent, lsPath)
			c.Assert(err, check.IsNil)
		} else {
			_, err = findCommandEventInLog(main, events.SessionCommandEvent, lsPath)
			c.Assert(err, check.NotNil)
		}
	}
}

// TestBPFSessionDifferentiation verifies that the bpf package can
// differentiate events from two different sessions. This test in turn also
// verifies the cgroup package.
func (s *IntSuite) TestBPFSessionDifferentiation(c *check.C) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	// Check if BPF tests can be run on this host.
	err := canTestBPF()
	if err != nil {
		c.Skip(fmt.Sprintf("Tests for BPF functionality can not be run: %v.", err))
		return
	}

	lsPath, err := exec.LookPath("ls")
	c.Assert(err, check.IsNil)

	// Create temporary directory where cgroup2 hierarchy will be mounted.
	dir, err := ioutil.TempDir("", "cgroup-test")
	c.Assert(err, check.IsNil)
	defer os.RemoveAll(dir)

	// Create and start a Teleport cluster.
	makeConfig := func() (*check.C, []string, []*InstanceSecrets, *service.Config) {
		clusterConfig, err := services.NewClusterConfig(services.ClusterConfigSpecV3{
			SessionRecording: services.RecordAtNode,
			LocalAuth:        services.NewBool(true),
		})
		c.Assert(err, check.IsNil)

		// Create default config.
		tconf := service.MakeDefaultConfig()

		// Configure Auth.
		tconf.Auth.Preference.SetSecondFactor("off")
		tconf.Auth.Enabled = true
		tconf.Auth.ClusterConfig = clusterConfig

		// Configure Proxy.
		tconf.Proxy.Enabled = true
		tconf.Proxy.DisableWebService = false
		tconf.Proxy.DisableWebInterface = true

		// Configure Node. If session are being recorded at the proxy, don't enable
		// BPF to simulate an OpenSSH node.
		tconf.SSH.Enabled = true
		tconf.SSH.BPF.Enabled = true
		tconf.SSH.BPF.CgroupPath = dir
		return c, nil, nil, tconf
	}
	main := s.newTeleportWithConfig(makeConfig())
	defer main.StopAll()

	// Create two client terminals and channel to signal when the clients are
	// done with the terminals.
	termA := NewTerminal(250)
	termB := NewTerminal(250)
	doneCh := make(chan bool, 2)

	// Open a terminal and type "ls" into both and exit.
	writeTerm := func(term *Terminal) {
		client, err := main.NewClient(ClientConfig{
			Login:   s.me.Username,
			Cluster: Site,
			Host:    Host,
			Port:    main.GetPortSSHInt(),
		})
		c.Assert(err, check.IsNil)

		// Connect terminal to std{in,out} of client.
		client.Stdout = term
		client.Stdin = term

		// "Type" a command into the terminal.
		term.Type(fmt.Sprintf("\a%v\n\r\aexit\n\r\a", lsPath))
		err = client.SSH(context.Background(), []string{}, false)
		c.Assert(err, check.IsNil)

		// Signal that the client has finished the interactive session.
		doneCh <- true
	}
	writeTerm(termA)
	writeTerm(termB)

	// Wait 10 seconds for both events to arrive, otherwise timeout.
	timeout := time.After(10 * time.Second)
	for i := 0; i < 2; i++ {
		select {
		case <-doneCh:
			if i == 1 {
				break
			}
		case <-timeout:
			c.Fatalf("Timed out waiting for client to finish interactive session.")
		}
	}

	// Try to find two command events from different sessions. Timeout after
	// 10 seconds.
	for i := 0; i < 10; i++ {
		sessionIDs := map[string]bool{}

		eventFields, err := eventsInLog(main.Config.DataDir+"/log/events.log", events.SessionCommandEvent)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		for _, fields := range eventFields {
			if fields.GetString(events.EventType) == events.SessionCommandEvent &&
				fields.GetString(events.Path) == lsPath {
				sessionIDs[fields.GetString(events.SessionEventID)] = true
			}
		}

		// If two command events for "ls" from different sessions, return right
		// away, test was successful.
		if len(sessionIDs) == 2 {
			return
		}
		time.Sleep(1 * time.Second)
	}
	c.Fatalf("Failed to find command events from two different sessions.")
}

// findEventInLog polls the event log looking for an event of a particular type.
func findEventInLog(t *TeleInstance, eventName string) (events.EventFields, error) {
	for i := 0; i < 10; i++ {
		eventFields, err := eventsInLog(t.Config.DataDir+"/log/events.log", eventName)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		for _, fields := range eventFields {
			eventType, ok := fields[events.EventType]
			if !ok {
				return nil, trace.BadParameter("not found")
			}
			if eventType == eventName {
				return fields, nil
			}
		}

		time.Sleep(250 * time.Millisecond)
	}
	return nil, trace.NotFound("event not found")
}

// findCommandEventInLog polls the event log looking for an event of a particular type.
func findCommandEventInLog(t *TeleInstance, eventName string, programName string) (events.EventFields, error) {
	for i := 0; i < 10; i++ {
		eventFields, err := eventsInLog(t.Config.DataDir+"/log/events.log", eventName)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		for _, fields := range eventFields {
			eventType, ok := fields[events.EventType]
			if !ok {
				continue
			}
			eventPath, ok := fields[events.Path]
			if !ok {
				continue
			}
			if eventType == eventName && eventPath == programName {
				return fields, nil
			}
		}

		time.Sleep(1 * time.Second)
	}
	return nil, trace.NotFound("event not found")
}

// eventsInLog returns all events in a log file.
func eventsInLog(path string, eventName string) ([]events.EventFields, error) {
	var ret []events.EventFields

	file, err := os.Open(path)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var fields events.EventFields
		err = json.Unmarshal(scanner.Bytes(), &fields)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		ret = append(ret, fields)
	}

	if len(ret) == 0 {
		return nil, trace.NotFound("event not found")
	}
	return ret, nil
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
	// since this helper is sometimes used for running commands on
	// multiple nodes concurrently, we use io.Pipe to protect our
	// output buffer from concurrent writes.
	read, write := io.Pipe()
	output := &bytes.Buffer{}
	doneC := make(chan struct{})
	go func() {
		io.Copy(output, read)
		close(doneC)
	}()
	tc.Stdout = write
	for i := 0; i < attempts; i++ {
		err = tc.SSH(context.TODO(), cmd, false)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
	write.Close()
	if err != nil {
		return "", trace.Wrap(err)
	}
	<-doneC
	return output.String(), nil
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
	typed   chan byte
	mu      *sync.Mutex
	written *bytes.Buffer
}

func NewTerminal(capacity int) *Terminal {
	return &Terminal{
		typed:   make(chan byte, capacity),
		mu:      &sync.Mutex{},
		written: bytes.NewBuffer(nil),
	}
}

func (t *Terminal) Type(data string) {
	for _, b := range []byte(data) {
		t.typed <- b
	}
}

// Output returns a number of first 'limit' bytes printed into this fake terminal
func (t *Terminal) Output(limit int) string {
	t.mu.Lock()
	defer t.mu.Unlock()
	buff := t.written.Bytes()
	if len(buff) > limit {
		buff = buff[:limit]
	}
	// clean up white space for easier comparison:
	return strings.TrimSpace(string(buff))
}

func (t *Terminal) Write(data []byte) (n int, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
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
			n--
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

// isRoot returns a boolean if the test is being run as root or not. Tests
// for this package must be run as root.
func canTestBPF() error {
	if os.Geteuid() != 0 {
		return trace.BadParameter("not root")
	}

	err := bpf.IsHostCompatible()
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}
