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
	"io/fs"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime/pprof"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keypaths"
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
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

const (
	HostID = "00000000-0000-0000-0000-000000000000"
	Site   = "local-site"
)

type integrationTestSuite struct {
	me *user.User
	// priv/pub pair to avoid re-generating it
	priv []byte
	pub  []byte
	// log defines the test-specific logger
	log utils.Logger
}

func newSuite(t *testing.T) *integrationTestSuite {
	SetTestTimeouts(time.Millisecond * time.Duration(100))

	suite := &integrationTestSuite{}

	var err error
	suite.priv, suite.pub, err = testauthority.New().GenerateKeyPair("")
	require.NoError(t, err)

	// Find AllocatePortsNum free listening ports to use.
	suite.me, _ = user.Current()

	// close & re-open stdin because 'go test' runs with os.stdin connected to /dev/null
	stdin, err := os.Open("/dev/tty")
	if err == nil {
		os.Stdin.Close()
		os.Stdin = stdin
	}

	t.Cleanup(func() {
		// restore os.Stdin to its original condition: connected to /dev/null
		os.Stdin.Close()
		os.Stdin, err = os.Open("/dev/null")
		require.NoError(t, err)
	})

	return suite
}

type integrationTest func(t *testing.T, suite *integrationTestSuite)

func (s *integrationTestSuite) bind(test integrationTest) func(t *testing.T) {
	return func(t *testing.T) {
		// Attempt to set a logger for the test. Be warned that parts of the
		// Teleport codebase do not honour the logger passed in via config and
		// will create their own. Do not expect to catch _all_ output with this.
		s.log = utils.NewLoggerForTests()
		os.RemoveAll(profile.FullProfilePath(""))
		t.Cleanup(func() { s.log = nil })
		test(t, s)
	}
}

// newTeleportWithConfig is a helper function that will create a running
// Teleport instance with the passed in user, instance secrets, and Teleport
// configuration.
func (s *integrationTestSuite) newTeleportWithConfig(t *testing.T, logins []string, instanceSecrets []*InstanceSecrets, teleportConfig *service.Config) *TeleInstance {
	teleport := s.newTeleportInstance()

	// use passed logins, but use suite's default login if nothing was passed
	if len(logins) == 0 {
		logins = []string{s.me.Username}
	}
	for _, login := range logins {
		teleport.AddUser(login, []string{login})
	}

	// create a new teleport instance with passed in configuration
	if err := teleport.CreateEx(t, instanceSecrets, teleportConfig); err != nil {
		t.Fatalf("Unexpected response from CreateEx: %v", trace.DebugReport(err))
	}
	if err := teleport.Start(); err != nil {
		t.Fatalf("Unexpected response from Start: %v", trace.DebugReport(err))
	}

	return teleport
}

// TestIntegrations acts as the master test suite for all integration tests
// requiring standardised setup and teardown.
func TestIntegrations(t *testing.T) {
	suite := newSuite(t)

	t.Run("AuditOff", suite.bind(testAuditOff))
	t.Run("AuditOn", suite.bind(testAuditOn))
	t.Run("BPFExec", suite.bind(testBPFExec))
	t.Run("BPFInteractive", suite.bind(testBPFInteractive))
	t.Run("BPFSessionDifferentiation", suite.bind(testBPFSessionDifferentiation))
	t.Run("CmdLabels", suite.bind(testCmdLabels))
	t.Run("ControlMaster", suite.bind(testControlMaster))
	t.Run("CustomReverseTunnel", suite.bind(testCustomReverseTunnel))
	t.Run("DataTransfer", suite.bind(testDataTransfer))
	t.Run("Disconnection", suite.bind(testDisconnectScenarios))
	t.Run("Discovery", suite.bind(testDiscovery))
	t.Run("DiscoveryNode", suite.bind(testDiscoveryNode))
	t.Run("DiscoveryRecovers", suite.bind(testDiscoveryRecovers))
	t.Run("EnvironmentVars", suite.bind(testEnvironmentVariables))
	t.Run("ExecEvents", suite.bind(testExecEvents))
	t.Run("ExternalClient", suite.bind(testExternalClient))
	t.Run("HA", suite.bind(testHA))
	t.Run("Interactive (Regular)", suite.bind(testInteractiveRegular))
	t.Run("Interactive (Reverse Tunnel)", suite.bind(testInteractiveReverseTunnel))
	t.Run("Interoperability", suite.bind(testInteroperability))
	t.Run("InvalidLogin", suite.bind(testInvalidLogins))
	t.Run("JumpTrustedClusters", suite.bind(testJumpTrustedClusters))
	t.Run("JumpTrustedClustersWithLabels", suite.bind(testJumpTrustedClustersWithLabels))
	t.Run("List", suite.bind(testList))
	t.Run("MapRoles", suite.bind(testMapRoles))
	t.Run("MultiplexingTrustedClusters", suite.bind(testMultiplexingTrustedClusters))
	t.Run("PAM", suite.bind(testPAM))
	t.Run("PortForwarding", suite.bind(testPortForwarding))
	t.Run("ProxyHostKeyCheck", suite.bind(testProxyHostKeyCheck))
	t.Run("RotateChangeSigningAlg", suite.bind(testRotateChangeSigningAlg))
	t.Run("RotateRollback", suite.bind(testRotateRollback))
	t.Run("RotateSuccess", suite.bind(testRotateSuccess))
	t.Run("RotateTrustedClusters", suite.bind(testRotateTrustedClusters))
	t.Run("SessionStartContainsAccessRequest", suite.bind(testSessionStartContainsAccessRequest))
	t.Run("SessionStreaming", suite.bind(testSessionStreaming))
	t.Run("SSHExitCode", suite.bind(testSSHExitCode))
	t.Run("Shutdown", suite.bind(testShutdown))
	t.Run("TrustedClusters", suite.bind(testTrustedClusters))
	t.Run("TrustedClustersWithLabels", suite.bind(testTrustedClustersWithLabels))
	t.Run("TrustedTunnelNode", suite.bind(testTrustedTunnelNode))
	t.Run("TwoClustersProxy", suite.bind(testTwoClustersProxy))
	t.Run("TwoClustersTunnel", suite.bind(testTwoClustersTunnel))
	t.Run("UUIDBasedProxy", suite.bind(testUUIDBasedProxy))
	t.Run("WindowChange", suite.bind(testWindowChange))
}

// testAuditOn creates a live session, records a bunch of data through it
// and then reads it back and compares against simulated reality.
func testAuditOn(t *testing.T, suite *integrationTestSuite) {
	ctx := context.Background()

	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	tests := []struct {
		comment          string
		inRecordLocation string
		inForwardAgent   bool
		auditSessionsURI string
	}{
		{
			comment:          "normal teleport",
			inRecordLocation: types.RecordAtNode,
			inForwardAgent:   false,
		}, {
			comment:          "recording proxy",
			inRecordLocation: types.RecordAtProxy,
			inForwardAgent:   true,
		}, {
			comment:          "normal teleport with upload to file server",
			inRecordLocation: types.RecordAtNode,
			inForwardAgent:   false,
			auditSessionsURI: t.TempDir(),
		}, {
			inRecordLocation: types.RecordAtProxy,
			inForwardAgent:   false,
			auditSessionsURI: t.TempDir(),
		}, {
			comment:          "normal teleport, sync recording",
			inRecordLocation: types.RecordAtNodeSync,
			inForwardAgent:   false,
		}, {
			comment:          "recording proxy, sync recording",
			inRecordLocation: types.RecordAtProxySync,
			inForwardAgent:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.comment, func(t *testing.T) {
			makeConfig := func() (*testing.T, []string, []*InstanceSecrets, *service.Config) {
				auditConfig, err := types.NewClusterAuditConfig(types.ClusterAuditConfigSpecV2{
					AuditSessionsURI: tt.auditSessionsURI,
				})
				require.NoError(t, err)

				recConfig, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
					Mode: tt.inRecordLocation,
				})
				require.NoError(t, err)

				tconf := suite.defaultServiceConfig()
				tconf.Auth.Enabled = true
				tconf.Auth.AuditConfig = auditConfig
				tconf.Auth.SessionRecordingConfig = recConfig
				tconf.Proxy.Enabled = true
				tconf.Proxy.DisableWebService = true
				tconf.Proxy.DisableWebInterface = true
				tconf.SSH.Enabled = true
				return t, nil, nil, tconf
			}
			teleport := suite.newTeleportWithConfig(makeConfig())
			defer teleport.StopAll()

			// Start a node.
			nodeSSHPort := ports.PopInt()
			nodeConfig := func() *service.Config {
				tconf := suite.defaultServiceConfig()

				tconf.HostUUID = "node"
				tconf.Hostname = "node"

				tconf.SSH.Enabled = true
				tconf.SSH.Addr.Addr = net.JoinHostPort(teleport.Hostname, fmt.Sprintf("%v", nodeSSHPort))

				return tconf
			}
			nodeProcess, err := teleport.StartNode(nodeConfig())
			require.NoError(t, err)

			// get access to a authClient for the cluster
			site := teleport.GetSiteAPI(Site)
			require.NotNil(t, site)

			// wait 10 seconds for both nodes to show up, otherwise
			// we'll have trouble connecting to the node below.
			waitForNodes := func(site auth.ClientI, count int) error {
				tickCh := time.Tick(500 * time.Millisecond)
				stopCh := time.After(10 * time.Second)
				for {
					select {
					case <-tickCh:
						nodesInSite, err := site.GetNodes(ctx, apidefaults.Namespace)
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
			require.NoError(t, err)

			// should have no sessions:
			sessions, err := site.GetSessions(apidefaults.Namespace)
			require.NoError(t, err)
			require.Empty(t, sessions)

			// create interactive session (this goroutine is this user's terminal time)
			endC := make(chan error)
			myTerm := NewTerminal(250)
			go func() {
				cl, err := teleport.NewClient(t, ClientConfig{
					Login:        suite.me.Username,
					Cluster:      Site,
					Host:         Host,
					Port:         nodeSSHPort,
					ForwardAgent: tt.inForwardAgent,
				})
				require.NoError(t, err)
				cl.Stdout = myTerm
				cl.Stdin = myTerm

				err = cl.SSH(context.TODO(), []string{}, false)
				endC <- err
			}()

			// wait until we've found the session in the audit log
			getSession := func(site auth.ClientI) (*session.Session, error) {
				timeout, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				sessions, err := waitForSessionToBeEstablished(timeout, apidefaults.Namespace, site)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				return &sessions[0], nil
			}
			session, err := getSession(site)
			require.NoError(t, err)
			sessionID := session.ID

			// wait for the user to join this session:
			for len(session.Parties) == 0 {
				time.Sleep(time.Millisecond * 5)
				session, err = site.GetSession(apidefaults.Namespace, sessionID)
				require.NoError(t, err)
			}
			// make sure it's us who joined! :)
			require.Equal(t, suite.me.Username, session.Parties[0].User)

			// lets type "echo hi" followed by "enter" and then "exit" + "enter":

			myTerm.Type("\aecho hi\n\r\aexit\n\r\a")

			// wait for session to end:
			select {
			case <-endC:
			case <-time.After(10 * time.Second):
				t.Fatalf("%s: Timeout waiting for session to finish.", tt.comment)
			}

			// wait for the upload of the right session to complete
			timeoutC := time.After(10 * time.Second)
		loop:
			for {
				select {
				case event := <-teleport.UploadEventsC:
					if event.SessionID != string(session.ID) {
						t.Logf("Skipping mismatching session %v, expecting upload of %v.", event.SessionID, session.ID)
						continue
					}
					break loop
				case <-timeoutC:
					dumpGoroutineProfile()
					t.Fatalf("%s: Timeout waiting for upload of session %v to complete to %v",
						tt.comment, session.ID, tt.auditSessionsURI)
				}
			}

			// read back the entire session (we have to try several times until we get back
			// everything because the session is closing)
			var sessionStream []byte
			for i := 0; i < 6; i++ {
				sessionStream, err = site.GetSessionChunk(apidefaults.Namespace, session.ID, 0, events.MaxChunkBytes)
				require.NoError(t, err)
				if strings.Contains(string(sessionStream), "exit") {
					break
				}
				time.Sleep(time.Millisecond * 250)
				if i >= 5 {
					// session stream keeps coming back short
					t.Fatalf("%s: Stream is not getting data: %q.", tt.comment, string(sessionStream))
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
			text := string(sessionStream)
			require.Contains(t, text, "echo hi")
			require.Contains(t, text, "exit")

			// Wait until session.start, session.leave, and session.end events have arrived.
			getSessions := func(site auth.ClientI) ([]events.EventFields, error) {
				tickCh := time.Tick(500 * time.Millisecond)
				stopCh := time.After(10 * time.Second)
				for {
					select {
					case <-tickCh:
						// Get all session events from the backend.
						sessionEvents, err := site.GetSessionEvents(apidefaults.Namespace, session.ID, 0, false)
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
			require.NoError(t, err)

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

			// there should always be 'session.start' event (and it must be first)
			first := history[0]
			start := findByType(events.SessionStartEvent)
			require.Equal(t, first, start)
			require.Equal(t, 0, start.GetInt("bytes"))
			require.Equal(t, string(sessionID), start.GetString(events.SessionEventID))
			require.NotEmpty(t, start.GetString(events.TerminalSize))

			// If session are being recorded at nodes, the SessionServerID should contain
			// the ID of the node. If sessions are being recorded at the proxy, then
			// SessionServerID should be that of the proxy.
			expectedServerID := nodeProcess.Config.HostUUID
			if services.IsRecordAtProxy(tt.inRecordLocation) {
				expectedServerID = teleport.Process.Config.HostUUID
			}
			require.Equal(t, expectedServerID, start.GetString(events.SessionServerID))

			// make sure data is recorded properly
			out := &bytes.Buffer{}
			for _, e := range history {
				out.WriteString(getChunk(e, 1000))
			}
			recorded := replaceNewlines(out.String())
			require.Regexp(t, ".*exit.*", recorded)
			require.Regexp(t, ".*echo hi.*", recorded)

			// there should always be 'session.end' event
			end := findByType(events.SessionEndEvent)
			require.NotNil(t, end)
			require.Equal(t, 0, end.GetInt("bytes"))
			require.Equal(t, string(sessionID), end.GetString(events.SessionEventID))

			// there should always be 'session.leave' event
			leave := findByType(events.SessionLeaveEvent)
			require.NotNil(t, leave)
			require.Equal(t, 0, leave.GetInt("bytes"))
			require.Equal(t, string(sessionID), leave.GetString(events.SessionEventID))

			// all of them should have a proper time
			for _, e := range history {
				require.False(t, e.GetTime("time").IsZero())
			}
		})
	}
}

// testInteroperability checks if Teleport and OpenSSH behave in the same way
// when executing commands.
func testInteroperability(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	tempdir := t.TempDir()
	tempfile := filepath.Join(tempdir, "file.txt")

	// create new teleport server that will be used by all tests
	teleport := suite.newTeleport(t, nil, true)
	defer teleport.StopAll()

	tests := []struct {
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
		t.Run(fmt.Sprintf("Test %d: %s", i, strings.Fields(tt.inCommand)[0]), func(t *testing.T) {
			// create new teleport client
			cl, err := teleport.NewClient(t, ClientConfig{Login: suite.me.Username, Cluster: Site, Host: Host, Port: teleport.GetPortSSHInt()})
			require.NoError(t, err)

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
			require.NoError(t, err)

			// if we are looking for the output in a file, look in the file
			// otherwise check stdout and stderr for the expected output
			if tt.outFile {
				bytes, err := ioutil.ReadFile(tempfile)
				require.NoError(t, err)
				require.Contains(t, string(bytes), tt.outContains)
			} else {
				require.Contains(t, outbuf.String(), tt.outContains)
			}
		})
	}
}

// TestMain will re-execute Teleport to run a command if "exec" is passed to
// it as an argument. Otherwise it will run tests as normal.
func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
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

// newUnstartedTeleport helper returns a created but not started Teleport instance pre-configured
// with the current user os.user.Current().
func (s *integrationTestSuite) newUnstartedTeleport(t *testing.T, logins []string, enableSSH bool) *TeleInstance {
	teleport := s.newTeleportInstance()
	// use passed logins, but use suite's default login if nothing was passed
	if len(logins) == 0 {
		logins = []string{s.me.Username}
	}
	for _, login := range logins {
		teleport.AddUser(login, []string{login})
	}
	require.NoError(t, teleport.Create(t, nil, enableSSH, nil))
	return teleport
}

// newTeleport helper returns a running Teleport instance pre-configured
// with the current user os.user.Current().
func (s *integrationTestSuite) newTeleport(t *testing.T, logins []string, enableSSH bool) *TeleInstance {
	teleport := s.newUnstartedTeleport(t, logins, enableSSH)
	require.NoError(t, teleport.Start())
	return teleport
}

// newTeleportIoT helper returns a running Teleport instance with Host as a
// reversetunnel node.
func (s *integrationTestSuite) newTeleportIoT(t *testing.T, logins []string) *TeleInstance {
	// Create a Teleport instance with Auth/Proxy.
	mainConfig := func() *service.Config {
		tconf := s.defaultServiceConfig()
		tconf.Auth.Enabled = true

		tconf.Proxy.Enabled = true
		tconf.Proxy.DisableWebService = false
		tconf.Proxy.DisableWebInterface = true

		tconf.SSH.Enabled = false

		return tconf
	}
	main := s.newTeleportWithConfig(t, logins, nil, mainConfig())

	// Create a Teleport instance with a Node.
	nodeConfig := func() *service.Config {
		tconf := s.defaultServiceConfig()
		tconf.Hostname = Host
		tconf.Token = "token"
		tconf.AuthServers = []utils.NetAddr{
			{
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
	require.NoError(t, err)

	return main
}

func replaceNewlines(in string) string {
	return regexp.MustCompile(`\r?\n`).ReplaceAllString(in, `\n`)
}

// TestUUIDBasedProxy verifies that attempts to proxy to nodes using ambiguous
// hostnames fails with the correct error, and that proxying by UUID succeeds.
func testUUIDBasedProxy(t *testing.T, suite *integrationTestSuite) {
	ctx := context.Background()

	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	teleportSvr := suite.newTeleport(t, nil, true)
	defer teleportSvr.StopAll()

	site := teleportSvr.GetSiteAPI(Site)

	// addNode adds a node to the teleport instance, returning its uuid.
	// All nodes added this way have the same hostname.
	addNode := func() (string, error) {
		nodeSSHPort := ports.PopInt()
		tconf := suite.defaultServiceConfig()
		tconf.Hostname = Host

		tconf.SSH.Enabled = true
		tconf.SSH.Addr.Addr = net.JoinHostPort(teleportSvr.Hostname, fmt.Sprintf("%v", nodeSSHPort))

		node, err := teleportSvr.StartNode(tconf)
		if err != nil {
			return "", trace.Wrap(err)
		}

		ident, err := node.GetIdentity(types.RoleNode)
		if err != nil {
			return "", trace.Wrap(err)
		}

		return ident.ID.HostID()
	}

	// add two nodes with the same hostname.
	uuid1, err := addNode()
	require.NoError(t, err)

	uuid2, err := addNode()
	require.NoError(t, err)

	// wait up to 10 seconds for supplied node names to show up.
	waitForNodes := func(site auth.ClientI, nodes ...string) error {
		tickCh := time.Tick(500 * time.Millisecond)
		stopCh := time.After(10 * time.Second)
	Outer:
		for _, nodeName := range nodes {
			for {
				select {
				case <-tickCh:
					nodesInSite, err := site.GetNodes(ctx, apidefaults.Namespace)
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
	require.NoError(t, err)

	// attempting to run a command by hostname should generate NodeIsAmbiguous error.
	_, err = runCommand(t, teleportSvr, []string{"echo", "Hello there!"}, ClientConfig{Login: suite.me.Username, Cluster: Site, Host: Host}, 1)
	require.Error(t, err)
	if !strings.Contains(err.Error(), teleport.NodeIsAmbiguous) {
		require.FailNowf(t, "Expected %s, got %s", teleport.NodeIsAmbiguous, err.Error())
	}

	// attempting to run a command by uuid should succeed.
	_, err = runCommand(t, teleportSvr, []string{"echo", "Hello there!"}, ClientConfig{Login: suite.me.Username, Cluster: Site, Host: uuid1}, 1)
	require.NoError(t, err)
}

// testInteractive covers SSH into shell and joining the same session from another client
// against a standard teleport node.
func testInteractiveRegular(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	teleport := suite.newTeleport(t, nil, true)
	defer teleport.StopAll()

	verifySessionJoin(t, suite.me.Username, teleport)
}

// TestInteractiveReverseTunnel covers SSH into shell and joining the same session from another client
// against a reversetunnel node.
func testInteractiveReverseTunnel(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	// InsecureDevMode needed for IoT node handshake
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	teleport := suite.newTeleportIoT(t, nil)
	defer teleport.StopAll()

	verifySessionJoin(t, suite.me.Username, teleport)
}

// TestCustomReverseTunnel tests that the SSH node falls back to configured
// proxy address if it cannot connect via the proxy address from the reverse
// tunnel discovery query.
// See https://github.com/gravitational/teleport/issues/4141 for context.
func testCustomReverseTunnel(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	// InsecureDevMode needed for IoT node handshake
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	failingListener, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	failingAddr := failingListener.Addr().String()
	failingListener.Close()

	// Create a Teleport instance with Auth/Proxy.
	conf := suite.defaultServiceConfig()
	conf.Auth.Enabled = true
	conf.Proxy.Enabled = true
	conf.Proxy.DisableWebService = false
	conf.Proxy.DisableWebInterface = true
	conf.Proxy.DisableDatabaseProxy = true
	conf.Proxy.TunnelPublicAddrs = []utils.NetAddr{
		{
			// Connect on the address that refuses connection on purpose
			// to test address fallback behavior
			Addr:        failingAddr,
			AddrNetwork: "tcp",
		},
	}
	conf.SSH.Enabled = false

	instanceConfig := suite.defaultInstanceConfig()
	instanceConfig.Ports = webReverseTunnelMuxPortSetup()
	main := NewInstance(instanceConfig)

	require.NoError(t, main.CreateEx(t, nil, conf))
	require.NoError(t, main.Start())
	defer main.StopAll()

	// Create a Teleport instance with a Node.
	nodeConf := suite.defaultServiceConfig()
	nodeConf.Hostname = Host
	nodeConf.Token = "token"
	nodeConf.Auth.Enabled = false
	nodeConf.Proxy.Enabled = false
	nodeConf.SSH.Enabled = true
	t.Setenv(apidefaults.TunnelPublicAddrEnvar, main.GetWebAddr())

	// verify the node is able to join the cluster
	_, err = main.StartReverseTunnelNode(nodeConf)
	require.NoError(t, err)
}

// verifySessionJoin covers SSH into shell and joining the same session from another client
func verifySessionJoin(t *testing.T, username string, teleport *TeleInstance) {
	// get a reference to site obj:
	site := teleport.GetSiteAPI(Site)
	require.NotNil(t, site)

	personA := NewTerminal(250)
	personB := NewTerminal(250)

	// PersonA: SSH into the server, wait one second, then type some commands on stdin:
	sessionA := make(chan error)
	openSession := func() {
		cl, err := teleport.NewClient(t, ClientConfig{Login: username, Cluster: Site, Host: Host})
		if err != nil {
			sessionA <- trace.Wrap(err)
			return
		}
		cl.Stdout = personA
		cl.Stdin = personA
		// Person A types something into the terminal (including "exit")
		personA.Type("\aecho hi\n\r\aexit\n\r\a")
		sessionA <- cl.SSH(context.TODO(), []string{}, false)
	}

	// PersonB: wait for a session to become available, then join:
	sessionB := make(chan error)
	joinSession := func() {
		sessionTimeoutCtx, sessionTimeoutCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer sessionTimeoutCancel()
		sessions, err := waitForSessionToBeEstablished(sessionTimeoutCtx, apidefaults.Namespace, site)
		if err != nil {
			sessionB <- trace.Wrap(err)
			return
		}

		sessionID := string(sessions[0].ID)
		cl, err := teleport.NewClient(t, ClientConfig{Login: username, Cluster: Site, Host: Host})
		if err != nil {
			sessionB <- trace.Wrap(err)
			return
		}

		timeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-timeoutCtx.Done():
				sessionB <- timeoutCtx.Err()
				return

			case <-ticker.C:
				err := cl.Join(context.TODO(), apidefaults.Namespace, session.ID(sessionID), personB)
				if err == nil {
					sessionB <- nil
					return
				}
			}
		}
	}

	go openSession()
	go joinSession()

	// wait for the sessions to end
	err := waitForError(sessionA, time.Second*10)
	require.NoError(t, err)

	err = waitForError(sessionB, time.Second*10)
	require.NoError(t, err)

	// make sure the output of B is mirrored in A
	outputOfA := personA.Output(100)
	outputOfB := personB.Output(100)
	require.Contains(t, outputOfA, outputOfB)
}

// TestShutdown tests scenario with a graceful shutdown,
// that session will be working after
func testShutdown(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	teleport := suite.newTeleport(t, nil, true)

	// get a reference to site obj:
	site := teleport.GetSiteAPI(Site)
	require.NotNil(t, site)

	person := NewTerminal(250)

	// commandsC receive commands
	commandsC := make(chan string)

	// PersonA: SSH into the server, wait one second, then type some commands on stdin:
	openSession := func() {
		cl, err := teleport.NewClient(t, ClientConfig{Login: suite.me.Username, Cluster: Site, Host: Host, Port: teleport.GetPortSSHInt()})
		require.NoError(t, err)
		cl.Stdout = person
		cl.Stdin = person

		go func() {
			for command := range commandsC {
				person.Type(command)
			}
		}()

		err = cl.SSH(context.TODO(), []string{}, false)
		require.NoError(t, err)
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
				require.FailNowf(t, "failed to capture output: %v", pattern)
			}
		}
		if !matched {
			require.FailNowf(t, "output %q does not match pattern %q", output, pattern)
		}
	}

	retry("echo start \r\n", ".*start.*")

	// initiate shutdown
	ctx := context.TODO()
	shutdownContext := teleport.Process.StartShutdown(ctx)

	// make sure that terminal still works
	retry("echo howdy \r\n", ".*howdy.*")

	// now type exit and wait for shutdown to complete
	person.Type("exit\n\r")

	select {
	case <-shutdownContext.Done():
	case <-time.After(5 * time.Second):
		require.FailNow(t, "Failed to shut down the server.")
	}
}

// errorVerifier is a function type for functions that check that a given
// error is what was expected. Implementations are expected top return nil
// if the supplied error is as expected, or an descriptive error if is is
// not
type errorVerifier func(error) error

func errorContains(text string) errorVerifier {
	return func(err error) error {
		if err == nil || !strings.Contains(err.Error(), text) {
			return fmt.Errorf("Expected error to contain %q, got: %v", text, err)
		}
		return nil
	}
}

type disconnectTestCase struct {
	recordingMode     string
	options           types.RoleOptions
	disconnectTimeout time.Duration
	concurrentConns   int
	sessCtlTimeout    time.Duration
	postFunc          func(context.Context, *testing.T, *TeleInstance)

	// verifyError checks if `err` reflects the error expected by the test scenario.
	// It returns nil if yes, non-nil otherwise.
	// It is important for verifyError to not do assertions using `*testing.T`
	// itself, as those assertions must run in the main test goroutine, but
	// verifyError runs in a different goroutine.
	verifyError errorVerifier
}

// TestDisconnectScenarios tests multiple scenarios with client disconnects
func testDisconnectScenarios(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	testCases := []disconnectTestCase{
		{
			recordingMode: types.RecordAtNode,
			options: types.RoleOptions{
				ClientIdleTimeout: types.NewDuration(500 * time.Millisecond),
			},
			disconnectTimeout: time.Second,
		}, {
			recordingMode: types.RecordAtProxy,
			options: types.RoleOptions{
				ForwardAgent:      types.NewBool(true),
				ClientIdleTimeout: types.NewDuration(500 * time.Millisecond),
			},
			disconnectTimeout: time.Second,
		}, {
			recordingMode: types.RecordAtNode,
			options: types.RoleOptions{
				DisconnectExpiredCert: types.NewBool(true),
				MaxSessionTTL:         types.NewDuration(2 * time.Second),
			},
			disconnectTimeout: 4 * time.Second,
		}, {
			recordingMode: types.RecordAtProxy,
			options: types.RoleOptions{
				ForwardAgent:          types.NewBool(true),
				DisconnectExpiredCert: types.NewBool(true),
				MaxSessionTTL:         types.NewDuration(2 * time.Second),
			},
			disconnectTimeout: 4 * time.Second,
		}, {
			// "verify that concurrent connection limits are applied when recording at node",
			recordingMode: types.RecordAtNode,
			options: types.RoleOptions{
				MaxConnections: 1,
			},
			disconnectTimeout: 1 * time.Second,
			concurrentConns:   2,
			verifyError:       errorContains("administratively prohibited"),
		}, {
			// "verify that concurrent connection limits are applied when recording at proxy",
			recordingMode: types.RecordAtProxy,
			options: types.RoleOptions{
				ForwardAgent:   types.NewBool(true),
				MaxConnections: 1,
			},
			disconnectTimeout: 1 * time.Second,
			concurrentConns:   2,
			verifyError:       errorContains("administratively prohibited"),
		}, {
			// "verify that lost connections to auth server terminate controlled conns",
			recordingMode: types.RecordAtNode,
			options: types.RoleOptions{
				MaxConnections: 1,
			},
			disconnectTimeout: time.Second,
			sessCtlTimeout:    500 * time.Millisecond,
			// use postFunc to wait for the semaphore to be acquired and a session
			// to be started, then shut down the auth server.
			postFunc: func(ctx context.Context, t *testing.T, teleport *TeleInstance) {
				site := teleport.GetSiteAPI(Site)
				var sems []types.Semaphore
				var err error
				for i := 0; i < 6; i++ {
					sems, err = site.GetSemaphores(ctx, types.SemaphoreFilter{
						SemaphoreKind: types.SemaphoreKindConnection,
					})
					if err == nil && len(sems) > 0 {
						break
					}
					select {
					case <-time.After(time.Millisecond * 100):
					case <-ctx.Done():
						return
					}
				}
				require.NoError(t, err)
				require.Len(t, sems, 1)

				timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
				defer cancel()

				ss, err := waitForSessionToBeEstablished(timeoutCtx, apidefaults.Namespace, site)
				require.NoError(t, err)
				require.Len(t, ss, 1)
				require.Nil(t, teleport.StopAuth(false))
			},
		},
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("Test %d", i), func(t *testing.T) {
			runDisconnectTest(t, suite, tc)
		})
	}
}

func runDisconnectTest(t *testing.T, suite *integrationTestSuite, tc disconnectTestCase) {
	teleport := suite.newTeleportInstance()

	username := suite.me.Username
	role, err := types.NewRole("devs", types.RoleSpecV4{
		Options: tc.options,
		Allow: types.RoleConditions{
			Logins: []string{username},
		},
	})
	require.NoError(t, err)
	teleport.AddUserWithRole(username, role)

	netConfig, err := types.NewClusterNetworkingConfigFromConfigFile(types.ClusterNetworkingConfigSpecV2{
		SessionControlTimeout: types.Duration(tc.sessCtlTimeout),
	})
	require.NoError(t, err)

	recConfig, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
		Mode: tc.recordingMode,
	})
	require.NoError(t, err)

	cfg := suite.defaultServiceConfig()
	cfg.Auth.Enabled = true
	cfg.Auth.NetworkingConfig = netConfig
	cfg.Auth.SessionRecordingConfig = recConfig
	cfg.Proxy.DisableWebService = true
	cfg.Proxy.DisableWebInterface = true
	cfg.Proxy.Enabled = true
	cfg.SSH.Enabled = true

	require.NoError(t, teleport.CreateEx(t, nil, cfg))
	require.NoError(t, teleport.Start())
	defer teleport.StopAll()

	// get a reference to site obj:
	site := teleport.GetSiteAPI(Site)
	require.NotNil(t, site)

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	if tc.concurrentConns < 1 {
		// test cases that don't specify concurrentConns are single-connection tests.
		tc.concurrentConns = 1
	}

	asyncErrors := make(chan error, 1)

	for i := 0; i < tc.concurrentConns; i++ {
		person := NewTerminal(250)

		openSession := func() {
			defer cancel()
			cl, err := teleport.NewClient(t, ClientConfig{Login: username, Cluster: Site, Host: Host, Port: teleport.GetPortSSHInt()})
			require.NoError(t, err)
			cl.Stdout = person
			cl.Stdin = person

			err = cl.SSH(ctx, []string{}, false)
			select {
			case <-ctx.Done():
				// either we timed out, or a different session
				// triggered closure.
				return
			default:
			}

			if tc.verifyError != nil {
				if badErrorErr := tc.verifyError(err); badErrorErr != nil {
					asyncErrors <- badErrorErr
				}
			} else if err != nil && !trace.IsEOF(err) && !isSSHError(err) {
				asyncErrors <- fmt.Errorf("expected EOF, ExitError, or nil, got %v instead", err)
				return
			}
		}

		go openSession()

		go func() {
			err := enterInput(ctx, person, "echo start \r\n", ".*start.*")
			if err != nil {
				asyncErrors <- err
			}
		}()
	}

	if tc.postFunc != nil {
		// test case modifies the teleport instance after session start
		tc.postFunc(ctx, t, teleport)
	}

	select {
	case <-time.After(tc.disconnectTimeout + time.Second):
		dumpGoroutineProfile()
		require.FailNowf(t, "timeout", "%s timeout waiting for session to exit: %+v", timeNow(), tc)

	case ae := <-asyncErrors:
		require.FailNow(t, "Async error", ae.Error())

	case <-ctx.Done():
		// session closed.  a test case is successful if the first
		// session to close encountered the expected error variant.
	}
}

func isSSHError(err error) bool {
	switch trace.Unwrap(err).(type) {
	case *ssh.ExitError, *ssh.ExitMissingError:
		return true
	default:
		return false
	}
}

func timeNow() string {
	return time.Now().Format(time.StampMilli)
}

// enterInput simulates entering user input into a terminal and awaiting a
// response. Returns an error if the given response text doesn't match
// the supplied regexp string.
func enterInput(ctx context.Context, person *Terminal, command, pattern string) error {
	person.Type(command)
	abortTime := time.Now().Add(10 * time.Second)
	var matched bool
	var output string
	for {
		output = replaceNewlines(person.Output(1000))
		matched, _ = regexp.MatchString(pattern, output)
		if matched {
			return nil
		}
		select {
		case <-time.After(time.Millisecond * 50):
		case <-ctx.Done():
			// cancellation means that we don't care about the input being
			// confirmed anymore; not equivalent to a timeout.
			return nil
		}
		if time.Now().After(abortTime) {
			return fmt.Errorf("failed to capture pattern %q in %q", pattern, output)
		}
	}
}

// TestInvalidLogins validates that you can't login with invalid login or
// with invalid 'site' parameter
func testEnvironmentVariables(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	teleport := suite.newTeleport(t, nil, true)
	defer teleport.StopAll()

	testKey, testVal := "TELEPORT_TEST_ENV", "howdy"
	cmd := []string{"printenv", testKey}

	// make sure sessions set run command
	tc, err := teleport.NewClient(t, ClientConfig{Login: suite.me.Username, Cluster: Site, Host: Host, Port: teleport.GetPortSSHInt()})
	require.NoError(t, err)

	tc.Env = map[string]string{testKey: testVal}
	out := &bytes.Buffer{}
	tc.Stdout = out
	tc.Stdin = nil
	err = tc.SSH(context.TODO(), cmd, false)

	require.NoError(t, err)
	require.Equal(t, testVal, strings.TrimSpace(out.String()))
}

// TestInvalidLogins validates that you can't login with invalid login or
// with invalid 'site' parameter
func testInvalidLogins(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	teleport := suite.newTeleport(t, nil, true)
	defer teleport.StopAll()

	cmd := []string{"echo", "success"}

	// try the wrong site:
	tc, err := teleport.NewClient(t, ClientConfig{Login: suite.me.Username, Cluster: "wrong-site", Host: Host, Port: teleport.GetPortSSHInt()})
	require.NoError(t, err)
	err = tc.SSH(context.TODO(), cmd, false)
	require.Regexp(t, "cluster wrong-site not found", err.Error())
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
func testTwoClustersTunnel(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	now := time.Now().In(time.UTC).Round(time.Second)

	tests := []struct {
		inRecordLocation  string
		outExecCountSiteA int
		outExecCountSiteB int
	}{
		// normal teleport. since all events are recorded at the node, all events
		// end up on site-a and none on site-b.
		{
			types.RecordAtNode,
			3,
			0,
		},
		// recording proxy. since events are recorded at the proxy, 3 events end up
		// on site-a (because it's a teleport node so it still records at the node)
		// and 2 events end up on site-b because it's recording.
		{
			types.RecordAtProxy,
			3,
			2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.inRecordLocation, func(t *testing.T) {
			twoClustersTunnel(t, suite, now, tt.inRecordLocation, tt.outExecCountSiteA, tt.outExecCountSiteB)
		})
	}

	log.Info("Tests done. Cleaning up.")
}

func twoClustersTunnel(t *testing.T, suite *integrationTestSuite, now time.Time, proxyRecordMode string, execCountSiteA, execCountSiteB int) {
	// start the http proxy, we need to make sure this was not used
	ps := &proxyServer{}
	ts := httptest.NewServer(ps)
	defer ts.Close()

	// clear out any proxy environment variables
	for _, v := range []string{"http_proxy", "https_proxy", "HTTP_PROXY", "HTTPS_PROXY"} {
		t.Setenv(v, "")
	}

	username := suite.me.Username

	a := suite.newNamedTeleportInstance(t, "site-A")
	b := suite.newNamedTeleportInstance(t, "site-B")

	a.AddUser(username, []string{username})
	b.AddUser(username, []string{username})

	recConfig, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
		Mode: proxyRecordMode,
	})
	require.NoError(t, err)

	acfg := suite.defaultServiceConfig()
	acfg.Auth.Enabled = true
	acfg.Proxy.Enabled = true
	acfg.Proxy.DisableWebService = true
	acfg.Proxy.DisableWebInterface = true
	acfg.SSH.Enabled = true

	bcfg := suite.defaultServiceConfig()
	bcfg.Auth.Enabled = true
	bcfg.Auth.SessionRecordingConfig = recConfig
	bcfg.Proxy.Enabled = true
	bcfg.Proxy.DisableWebService = true
	bcfg.Proxy.DisableWebInterface = true
	bcfg.SSH.Enabled = false

	require.NoError(t, b.CreateEx(t, a.Secrets.AsSlice(), bcfg))
	t.Cleanup(func() { require.NoError(t, b.StopAll()) })
	require.NoError(t, a.CreateEx(t, b.Secrets.AsSlice(), acfg))
	t.Cleanup(func() { require.NoError(t, a.StopAll()) })

	require.NoError(t, b.Start())
	require.NoError(t, a.Start())

	// wait for both sites to see each other via their reverse tunnels (for up to 10 seconds)
	clustersAreCrossConnected := func() bool {
		return len(checkGetClusters(t, a.Tunnel)) >= 2 && len(checkGetClusters(t, b.Tunnel)) >= 2
	}
	require.Eventually(t,
		clustersAreCrossConnected,
		10*time.Second /* waitFor */, 200*time.Millisecond, /* tick */
		"Two clusters do not see each other: tunnels are not working.")

	var (
		outputA bytes.Buffer
		outputB bytes.Buffer
	)

	// make sure the direct dialer was used and not the proxy dialer
	require.Zero(t, ps.Count())

	// if we got here, it means two sites are cross-connected. lets execute SSH commands
	sshPort := a.GetPortSSHInt()
	cmd := []string{"echo", "hello world"}

	// directly:
	tc, err := a.NewClient(t, ClientConfig{
		Login:        username,
		Cluster:      a.Secrets.SiteName,
		Host:         Host,
		Port:         sshPort,
		ForwardAgent: true,
	})
	tc.Stdout = &outputA
	require.NoError(t, err)
	err = tc.SSH(context.TODO(), cmd, false)
	require.NoError(t, err)
	require.Equal(t, "hello world\n", outputA.String())

	// Update trusted CAs.
	err = tc.UpdateTrustedCA(context.TODO(), a.Secrets.SiteName)
	require.NoError(t, err)

	// The known_hosts file should have two certificates, the way bytes.Split
	// works that means the output will be 3 (2 certs + 1 empty).
	buffer, err := ioutil.ReadFile(keypaths.KnownHostsPath(tc.KeysDir))
	require.NoError(t, err)
	parts := bytes.Split(buffer, []byte("\n"))
	require.Len(t, parts, 3)

	roots := x509.NewCertPool()
	werr := filepath.Walk(keypaths.CAsDir(tc.KeysDir, Host), func(path string, info fs.FileInfo, err error) error {
		require.NoError(t, err)
		if info.IsDir() {
			return nil
		}
		buffer, err = ioutil.ReadFile(path)
		require.NoError(t, err)
		ok := roots.AppendCertsFromPEM(buffer)
		require.True(t, ok)
		return nil
	})
	require.NoError(t, werr)
	ok := roots.AppendCertsFromPEM(buffer)
	require.True(t, ok)
	require.Len(t, roots.Subjects(), 2)

	// wait for active tunnel connections to be established
	waitForActiveTunnelConnections(t, b.Tunnel, a.Secrets.SiteName, 1)

	// via tunnel b->a:
	tc, err = b.NewClient(t, ClientConfig{
		Login:        username,
		Cluster:      a.Secrets.SiteName,
		Host:         Host,
		Port:         sshPort,
		ForwardAgent: true,
	})
	tc.Stdout = &outputB
	require.NoError(t, err)
	err = tc.SSH(context.TODO(), cmd, false)
	require.NoError(t, err)
	require.Equal(t, outputA.String(), outputB.String())

	// Stop "site-A" and try to connect to it again via "site-A" (expect a connection error)
	require.NoError(t, a.StopAuth(false))
	err = tc.SSH(context.TODO(), cmd, false)
	require.IsType(t, err, trace.ConnectionProblem(nil, ""))

	// Reset and start "Site-A" again
	require.NoError(t, a.Reset())
	require.NoError(t, a.Start())

	// try to execute an SSH command using the same old client to Site-B
	// "site-A" and "site-B" reverse tunnels are supposed to reconnect,
	// and 'tc' (client) is also supposed to reconnect
	var sshErr error
	tcHasReconnected := func() bool {
		sshErr = tc.SSH(context.TODO(), cmd, false)
		return sshErr == nil
	}
	require.Eventually(t, tcHasReconnected, 10*time.Second, 250*time.Millisecond,
		"Timed out waiting for Site A to restart: %v", sshErr)

	clientHasEvents := func(site auth.ClientI, count int) func() bool {
		// only look for exec events
		eventTypes := []string{events.ExecEvent}

		return func() bool {
			eventsInSite, _, err := site.SearchEvents(now, now.Add(1*time.Hour), apidefaults.Namespace, eventTypes, 0, types.EventOrderAscending, "")
			require.NoError(t, err)
			return len(eventsInSite) == count
		}
	}

	siteA := a.GetSiteAPI(a.Secrets.SiteName)
	require.Eventually(t, clientHasEvents(siteA, execCountSiteA), 5*time.Second, 500*time.Millisecond,
		"Failed to find %d events on Site A after 5s", execCountSiteA)

	siteB := b.GetSiteAPI(b.Secrets.SiteName)
	require.Eventually(t, clientHasEvents(siteB, execCountSiteB), 5*time.Second, 500*time.Millisecond,
		"Failed to find %d events on Site B after 5s", execCountSiteB)
}

// TestTwoClustersProxy checks if the reverse tunnel uses a HTTP PROXY to
// establish a connection.
func testTwoClustersProxy(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	// start the http proxy
	ps := &proxyServer{}
	ts := httptest.NewServer(ps)
	defer ts.Close()

	// set the http_proxy environment variable
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)
	t.Setenv("http_proxy", u.Host)

	username := suite.me.Username

	a := suite.newNamedTeleportInstance(t, "site-A")
	b := suite.newNamedTeleportInstance(t, "site-B")

	a.AddUser(username, []string{username})
	b.AddUser(username, []string{username})

	require.NoError(t, b.Create(t, a.Secrets.AsSlice(), false, nil))
	defer b.StopAll()
	require.NoError(t, a.Create(t, b.Secrets.AsSlice(), true, nil))
	defer a.StopAll()

	require.NoError(t, b.Start())
	require.NoError(t, a.Start())

	// wait for both sites to see each other via their reverse tunnels (for up to 10 seconds)
	abortTime := time.Now().Add(time.Second * 10)
	for len(checkGetClusters(t, a.Tunnel)) < 2 && len(checkGetClusters(t, b.Tunnel)) < 2 {
		time.Sleep(time.Millisecond * 200)
		require.False(t, time.Now().After(abortTime), "two sites do not see each other: tunnels are not working")
	}

	// make sure the reverse tunnel went through the proxy
	require.Greater(t, ps.Count(), 0, "proxy did not intercept any connection")

	// stop both sites for real
	require.NoError(t, b.StopAll())
	require.NoError(t, a.StopAll())
}

// TestHA tests scenario when auth server for the cluster goes down
// and we switch to local persistent caches
func testHA(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	username := suite.me.Username

	a := suite.newNamedTeleportInstance(t, "cluster-a")
	b := suite.newNamedTeleportInstance(t, "cluster-b")

	a.AddUser(username, []string{username})
	b.AddUser(username, []string{username})

	require.NoError(t, b.Create(t, a.Secrets.AsSlice(), true, nil))
	require.NoError(t, a.Create(t, b.Secrets.AsSlice(), true, nil))

	require.NoError(t, b.Start())
	require.NoError(t, a.Start())

	nodePorts := ports.PopIntSlice(3)
	sshPort, proxyWebPort, proxySSHPort := nodePorts[0], nodePorts[1], nodePorts[2]
	require.NoError(t, a.StartNodeAndProxy("cluster-a-node", sshPort, proxyWebPort, proxySSHPort))

	// wait for both sites to see each other via their reverse tunnels (for up to 10 seconds)
	abortTime := time.Now().Add(time.Second * 10)
	for len(checkGetClusters(t, a.Tunnel)) < 2 && len(checkGetClusters(t, b.Tunnel)) < 2 {
		time.Sleep(time.Millisecond * 2000)
		require.False(t, time.Now().After(abortTime), "two sites do not see each other: tunnels are not working")
	}

	cmd := []string{"echo", "hello world"}
	tc, err := b.NewClient(t, ClientConfig{
		Login:   username,
		Cluster: "cluster-a",
		Host:    Loopback,
		Port:    sshPort,
	})
	require.NoError(t, err)

	output := &bytes.Buffer{}
	tc.Stdout = output
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
	require.NoError(t, err)
	require.Equal(t, "hello world\n", output.String())

	// Stop cluster "a" to force existing tunnels to close.
	require.NoError(t, a.StopAuth(true))

	// Reset KeyPair set by the first start by ACME. After introducing the ALPN TLS listener TLS proxy
	// certs are generated even if WebService and WebInterface was disabled and only DisableTLS
	// flag skips the TLS cert initialization. the First start call creates the ACME certs
	// where Resets() call deletes certs dir thus KeyPairs is no longer valid.
	a.Config.Proxy.KeyPairs = nil

	// Restart cluster "a".
	require.NoError(t, a.Reset())
	require.NoError(t, a.Start())

	// Wait for the tunnels to reconnect.
	abortTime = time.Now().Add(time.Second * 10)
	for len(checkGetClusters(t, a.Tunnel)) < 2 && len(checkGetClusters(t, b.Tunnel)) < 2 {
		time.Sleep(time.Millisecond * 2000)
		require.False(t, time.Now().After(abortTime), "two sites do not see each other: tunnels are not working")
	}

	// try to execute an SSH command using the same old client to site-B
	// "site-A" and "site-B" reverse tunnels are supposed to reconnect,
	// and 'tc' (client) is also supposed to reconnect
	for i := 0; i < 30; i++ {
		time.Sleep(1 * time.Second)
		err = tc.SSH(context.TODO(), cmd, false)
		if err == nil {
			break
		}
	}
	require.NoError(t, err)

	// stop cluster and remaining nodes
	require.NoError(t, a.StopAll())
	require.NoError(t, b.StopAll())
}

// TestMapRoles tests local to remote role mapping and access patterns
func testMapRoles(t *testing.T, suite *integrationTestSuite) {
	ctx := context.Background()
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	username := suite.me.Username

	clusterMain := "cluster-main"
	clusterAux := "cluster-aux"

	main := suite.newNamedTeleportInstance(t, clusterMain)
	aux := suite.newNamedTeleportInstance(t, clusterAux)

	// main cluster has a local user and belongs to role "main-devs"
	mainDevs := "main-devs"
	role, err := types.NewRole(mainDevs, types.RoleSpecV4{
		Allow: types.RoleConditions{
			Logins: []string{username},
		},
	})
	require.NoError(t, err)
	main.AddUserWithRole(username, role)

	// for role mapping test we turn on Web API on the main cluster
	// as it's used
	makeConfig := func(enableSSH bool) (*testing.T, []*InstanceSecrets, *service.Config) {
		tconf := suite.defaultServiceConfig()
		tconf.SSH.Enabled = enableSSH
		tconf.Proxy.DisableWebService = false
		tconf.Proxy.DisableWebInterface = true
		return t, nil, tconf
	}
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	require.NoError(t, main.CreateEx(makeConfig(false)))
	require.NoError(t, aux.CreateEx(makeConfig(true)))

	// auxiliary cluster has a role aux-devs
	// connect aux cluster to main cluster
	// using trusted clusters, so remote user will be allowed to assume
	// role specified by mapping remote role "devs" to local role "local-devs"
	auxDevs := "aux-devs"
	role, err = types.NewRole(auxDevs, types.RoleSpecV4{
		Allow: types.RoleConditions{
			Logins: []string{username},
		},
	})
	require.NoError(t, err)
	err = aux.Process.GetAuthServer().UpsertRole(ctx, role)
	require.NoError(t, err)
	trustedClusterToken := "trusted-cluster-token"
	err = main.Process.GetAuthServer().UpsertToken(ctx,
		services.MustCreateProvisionToken(trustedClusterToken, []types.SystemRole{types.RoleTrustedCluster}, time.Time{}))
	require.NoError(t, err)
	trustedCluster := main.AsTrustedCluster(trustedClusterToken, types.RoleMap{
		{Remote: mainDevs, Local: []string{auxDevs}},
	})

	// modify trusted cluster resource name so it would not
	// match the cluster name to check that it does not matter
	trustedCluster.SetName(main.Secrets.SiteName + "-cluster")

	require.NoError(t, main.Start())
	require.NoError(t, aux.Start())

	err = trustedCluster.CheckAndSetDefaults()
	require.NoError(t, err)

	// try and upsert a trusted cluster
	tryCreateTrustedCluster(t, aux.Process.GetAuthServer(), trustedCluster)
	waitForTunnelConnections(t, main.Process.GetAuthServer(), clusterAux, 1)

	nodePorts := ports.PopIntSlice(3)
	sshPort, proxyWebPort, proxySSHPort := nodePorts[0], nodePorts[1], nodePorts[2]
	require.NoError(t, aux.StartNodeAndProxy("aux-node", sshPort, proxyWebPort, proxySSHPort))

	// wait for both sites to see each other via their reverse tunnels (for up to 10 seconds)
	abortTime := time.Now().Add(time.Second * 10)
	for len(checkGetClusters(t, main.Tunnel)) < 2 && len(checkGetClusters(t, aux.Tunnel)) < 2 {
		time.Sleep(time.Millisecond * 2000)
		require.False(t, time.Now().After(abortTime), "two clusters do not see each other: tunnels are not working")
	}

	// Make sure that GetNodes returns nodes in the remote site. This makes
	// sure identity aware GetNodes works for remote clusters. Testing of the
	// correct nodes that identity aware GetNodes is done in TestList.
	var nodes []types.Server
	for i := 0; i < 10; i++ {
		nodes, err = aux.Process.GetAuthServer().GetNodes(ctx, apidefaults.Namespace)
		require.NoError(t, err)
		if len(nodes) != 2 {
			time.Sleep(100 * time.Millisecond)
			continue
		}
	}
	require.Len(t, nodes, 2)

	cmd := []string{"echo", "hello world"}
	tc, err := main.NewClient(t, ClientConfig{Login: username, Cluster: clusterAux, Host: Loopback, Port: sshPort})
	require.NoError(t, err)
	output := &bytes.Buffer{}
	tc.Stdout = output
	require.NoError(t, err)
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
	require.NoError(t, err)
	require.Equal(t, "hello world\n", output.String())

	// make sure both clusters have the right certificate authorities with the right signing keys.
	tests := []struct {
		name                       string
		mainClusterName            string
		auxClusterName             string
		inCluster                  *TeleInstance
		outChkMainUserCA           require.ErrorAssertionFunc
		outChkMainUserCAPrivateKey require.ValueAssertionFunc
		outChkMainHostCA           require.ErrorAssertionFunc
		outChkMainHostCAPrivateKey require.ValueAssertionFunc
		outChkAuxUserCA            require.ErrorAssertionFunc
		outChkAuxUserCAPrivateKey  require.ValueAssertionFunc
		outChkAuxHostCA            require.ErrorAssertionFunc
		outChkAuxHostCAPrivateKey  require.ValueAssertionFunc
	}{
		// 0 - main
		//   * User CA for main has one signing key.
		//   * Host CA for main has one signing key.
		//   * User CA for aux does not exist.
		//   * Host CA for aux has no signing keys.
		{
			name:                       "main",
			mainClusterName:            main.Secrets.SiteName,
			auxClusterName:             aux.Secrets.SiteName,
			inCluster:                  main,
			outChkMainUserCA:           require.NoError,
			outChkMainUserCAPrivateKey: require.NotEmpty,
			outChkMainHostCA:           require.NoError,
			outChkMainHostCAPrivateKey: require.NotEmpty,
			outChkAuxUserCA:            require.Error,
			outChkAuxUserCAPrivateKey:  require.Empty,
			outChkAuxHostCA:            require.NoError,
			outChkAuxHostCAPrivateKey:  require.Empty,
		},
		// 1 - aux
		//   * User CA for main has no signing keys.
		//   * Host CA for main has no signing keys.
		//   * User CA for aux has one signing key.
		//   * Host CA for aux has one signing key.
		{
			name:                       "aux",
			mainClusterName:            trustedCluster.GetName(),
			auxClusterName:             aux.Secrets.SiteName,
			inCluster:                  aux,
			outChkMainUserCA:           require.NoError,
			outChkMainUserCAPrivateKey: require.Empty,
			outChkMainHostCA:           require.NoError,
			outChkMainHostCAPrivateKey: require.Empty,
			outChkAuxUserCA:            require.NoError,
			outChkAuxUserCAPrivateKey:  require.NotEmpty,
			outChkAuxHostCA:            require.NoError,
			outChkAuxHostCAPrivateKey:  require.NotEmpty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cid := types.CertAuthID{Type: types.UserCA, DomainName: tt.mainClusterName}
			mainUserCAs, err := tt.inCluster.Process.GetAuthServer().GetCertAuthority(cid, true)
			tt.outChkMainUserCA(t, err)
			if err == nil {
				tt.outChkMainUserCAPrivateKey(t, mainUserCAs.GetActiveKeys().SSH[0].PrivateKey)
			}

			cid = types.CertAuthID{Type: types.HostCA, DomainName: tt.mainClusterName}
			mainHostCAs, err := tt.inCluster.Process.GetAuthServer().GetCertAuthority(cid, true)
			tt.outChkMainHostCA(t, err)
			if err == nil {
				tt.outChkMainHostCAPrivateKey(t, mainHostCAs.GetActiveKeys().SSH[0].PrivateKey)
			}

			cid = types.CertAuthID{Type: types.UserCA, DomainName: tt.auxClusterName}
			auxUserCAs, err := tt.inCluster.Process.GetAuthServer().GetCertAuthority(cid, true)
			tt.outChkAuxUserCA(t, err)
			if err == nil {
				tt.outChkAuxUserCAPrivateKey(t, auxUserCAs.GetActiveKeys().SSH[0].PrivateKey)
			}

			cid = types.CertAuthID{Type: types.HostCA, DomainName: tt.auxClusterName}
			auxHostCAs, err := tt.inCluster.Process.GetAuthServer().GetCertAuthority(cid, true)
			tt.outChkAuxHostCA(t, err)
			if err == nil {
				tt.outChkAuxHostCAPrivateKey(t, auxHostCAs.GetActiveKeys().SSH[0].PrivateKey)
			}
		})
	}

	// stop clusters and remaining nodes
	require.NoError(t, main.StopAll())
	require.NoError(t, aux.StopAll())
}

// tryCreateTrustedCluster performs several attempts to create a trusted cluster,
// retries on connection problems and access denied errors to let caches
// propagate and services to start
//
// Duplicated in tool/tsh/tsh_test.go
func tryCreateTrustedCluster(t *testing.T, authServer *auth.Server, trustedCluster types.TrustedCluster) {
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
		require.FailNow(t, "Terminating on unexpected problem", "%v.", err)
	}
	require.FailNow(t, "Timeout creating trusted cluster")
}

// trustedClusterTest is a test setup for trusted clusters tests
type trustedClusterTest struct {
	// multiplex sets up multiplexing of the reversetunnel SSH
	// socket and the proxy's web socket
	multiplex bool
	// useJumpHost turns on jump host mode for the access
	// to the proxy instead of the proxy command
	useJumpHost bool
	// useLabels turns on trusted cluster labels and
	// verifies RBAC
	useLabels bool
}

// TestTrustedClusters tests remote clusters scenarios
// using trusted clusters feature
func testTrustedClusters(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	trustedClusters(t, suite, trustedClusterTest{multiplex: false})
}

// TestTrustedClustersWithLabels tests remote clusters scenarios
// using trusted clusters feature and access labels
func testTrustedClustersWithLabels(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	trustedClusters(t, suite, trustedClusterTest{multiplex: false, useLabels: true})
}

// TestJumpTrustedClusters tests remote clusters scenarios
// using trusted clusters feature using jumphost connection
func testJumpTrustedClusters(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	trustedClusters(t, suite, trustedClusterTest{multiplex: false, useJumpHost: true})
}

// TestJumpTrustedClusters tests remote clusters scenarios
// using trusted clusters feature using jumphost connection
func testJumpTrustedClustersWithLabels(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	trustedClusters(t, suite, trustedClusterTest{multiplex: false, useJumpHost: true, useLabels: true})
}

// TestMultiplexingTrustedClusters tests remote clusters scenarios
// using trusted clusters feature
func testMultiplexingTrustedClusters(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	trustedClusters(t, suite, trustedClusterTest{multiplex: true})
}

func standardPortsOrMuxSetup(mux bool) *InstancePorts {
	if mux {
		return webReverseTunnelMuxPortSetup()
	}
	return standardPortSetup()
}

func trustedClusters(t *testing.T, suite *integrationTestSuite, test trustedClusterTest) {
	ctx := context.Background()
	username := suite.me.Username

	clusterMain := "cluster-main"
	clusterAux := "cluster-aux"
	main := NewInstance(InstanceConfig{
		ClusterName: clusterMain,
		HostID:      HostID,
		NodeName:    Host,
		Priv:        suite.priv,
		Pub:         suite.pub,
		log:         suite.log,
		Ports:       standardPortsOrMuxSetup(test.multiplex),
	})
	aux := suite.newNamedTeleportInstance(t, clusterAux)

	// main cluster has a local user and belongs to role "main-devs" and "main-admins"
	mainDevs := "main-devs"
	devsRole, err := types.NewRole(mainDevs, types.RoleSpecV4{
		Allow: types.RoleConditions{
			Logins: []string{username},
		},
	})
	// If the test is using labels, the cluster will be labeled
	// and user will be granted access if labels match.
	// Otherwise, to preserve backwards-compatibility
	// roles with no labels will grant access to clusters with no labels.
	if test.useLabels {
		devsRole.SetClusterLabels(types.Allow, types.Labels{"access": []string{"prod"}})
	}
	require.NoError(t, err)

	mainAdmins := "main-admins"
	adminsRole, err := types.NewRole(mainAdmins, types.RoleSpecV4{
		Allow: types.RoleConditions{
			Logins: []string{"superuser"},
		},
	})
	require.NoError(t, err)

	main.AddUserWithRole(username, devsRole, adminsRole)

	// Ops users can only access remote clusters with label 'access': 'ops'
	mainOps := "main-ops"
	mainOpsRole, err := types.NewRole(mainOps, types.RoleSpecV4{
		Allow: types.RoleConditions{
			Logins:        []string{username},
			ClusterLabels: types.Labels{"access": []string{"ops"}},
		},
	})
	require.NoError(t, err)
	main.AddUserWithRole(mainOps, mainOpsRole, adminsRole)

	// for role mapping test we turn on Web API on the main cluster
	// as it's used
	makeConfig := func(enableSSH bool) (*testing.T, []*InstanceSecrets, *service.Config) {
		tconf := suite.defaultServiceConfig()
		tconf.Proxy.DisableWebService = false
		tconf.Proxy.DisableWebInterface = true
		tconf.SSH.Enabled = enableSSH
		return t, nil, tconf
	}
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	require.NoError(t, main.CreateEx(makeConfig(false)))
	require.NoError(t, aux.CreateEx(makeConfig(true)))

	// auxiliary cluster has only a role aux-devs
	// connect aux cluster to main cluster
	// using trusted clusters, so remote user will be allowed to assume
	// role specified by mapping remote role "devs" to local role "local-devs"
	auxDevs := "aux-devs"
	auxRole, err := types.NewRole(auxDevs, types.RoleSpecV4{
		Allow: types.RoleConditions{
			Logins: []string{username},
		},
	})
	require.NoError(t, err)
	err = aux.Process.GetAuthServer().UpsertRole(ctx, auxRole)
	require.NoError(t, err)

	trustedClusterToken := "trusted-cluster-token"
	tokenResource, err := types.NewProvisionToken(trustedClusterToken, []types.SystemRole{types.RoleTrustedCluster}, time.Time{})
	require.NoError(t, err)
	if test.useLabels {
		meta := tokenResource.GetMetadata()
		meta.Labels = map[string]string{"access": "prod"}
		tokenResource.SetMetadata(meta)
	}
	err = main.Process.GetAuthServer().UpsertToken(ctx, tokenResource)
	require.NoError(t, err)
	// Note that the mapping omits admins role, this is to cover the scenario
	// when root cluster and leaf clusters have different role sets
	trustedCluster := main.AsTrustedCluster(trustedClusterToken, types.RoleMap{
		{Remote: mainDevs, Local: []string{auxDevs}},
		{Remote: mainOps, Local: []string{auxDevs}},
	})

	// modify trusted cluster resource name so it would not
	// match the cluster name to check that it does not matter
	trustedCluster.SetName(main.Secrets.SiteName + "-cluster")

	require.NoError(t, main.Start())
	require.NoError(t, aux.Start())

	err = trustedCluster.CheckAndSetDefaults()
	require.NoError(t, err)

	// try and upsert a trusted cluster
	tryCreateTrustedCluster(t, aux.Process.GetAuthServer(), trustedCluster)
	waitForTunnelConnections(t, main.Process.GetAuthServer(), clusterAux, 1)

	nodePorts := ports.PopIntSlice(3)
	sshPort, proxyWebPort, proxySSHPort := nodePorts[0], nodePorts[1], nodePorts[2]
	require.NoError(t, aux.StartNodeAndProxy("aux-node", sshPort, proxyWebPort, proxySSHPort))

	// wait for both sites to see each other via their reverse tunnels (for up to 10 seconds)
	abortTime := time.Now().Add(time.Second * 10)
	for len(checkGetClusters(t, main.Tunnel)) < 2 && len(checkGetClusters(t, aux.Tunnel)) < 2 {
		time.Sleep(time.Millisecond * 2000)
		require.False(t, time.Now().After(abortTime), "two clusters do not see each other: tunnels are not working")
	}

	cmd := []string{"echo", "hello world"}

	// Try and connect to a node in the Aux cluster from the Main cluster using
	// direct dialing.
	creds, err := GenerateUserCreds(UserCredsRequest{
		Process:        main.Process,
		Username:       username,
		RouteToCluster: clusterAux,
	})
	require.NoError(t, err)

	tc, err := main.NewClientWithCreds(ClientConfig{
		Login:    username,
		Cluster:  clusterAux,
		Host:     Loopback,
		Port:     sshPort,
		JumpHost: test.useJumpHost,
	}, *creds)
	require.NoError(t, err)

	// tell the client to trust aux cluster CAs (from secrets). this is the
	// equivalent of 'known hosts' in openssh
	auxCAS := aux.Secrets.GetCAs(t)
	for i := range auxCAS {
		err = tc.AddTrustedCA(auxCAS[i])
		require.NoError(t, err)
	}

	output := &bytes.Buffer{}
	tc.Stdout = output
	require.NoError(t, err)
	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond * 50)
		err = tc.SSH(ctx, cmd, false)
		if err == nil {
			break
		}
	}
	require.NoError(t, err)
	require.Equal(t, "hello world\n", output.String())

	// Try and generate user creds for Aux cluster as ops user.
	_, err = GenerateUserCreds(UserCredsRequest{
		Process:        main.Process,
		Username:       mainOps,
		RouteToCluster: clusterAux,
	})
	require.True(t, trace.IsNotFound(err))

	// ListNodes expect labels as a value of host
	tc.Host = ""
	servers, err := tc.ListNodes(ctx)
	require.NoError(t, err)
	require.Len(t, servers, 2)
	tc.Host = Loopback

	// check that remote cluster has been provisioned
	remoteClusters, err := main.Process.GetAuthServer().GetRemoteClusters()
	require.NoError(t, err)
	require.Len(t, remoteClusters, 1)
	require.Equal(t, clusterAux, remoteClusters[0].GetName())

	// after removing the remote cluster, the connection will start failing
	err = main.Process.GetAuthServer().DeleteRemoteCluster(clusterAux)
	require.NoError(t, err)
	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond * 50)
		err = tc.SSH(ctx, cmd, false)
		if err != nil {
			break
		}
	}
	require.Error(t, err, "expected tunnel to close and SSH client to start failing")

	// remove trusted cluster from aux cluster side, and recrete right after
	// this should re-establish connection
	err = aux.Process.GetAuthServer().DeleteTrustedCluster(ctx, trustedCluster.GetName())
	require.NoError(t, err)
	_, err = aux.Process.GetAuthServer().UpsertTrustedCluster(ctx, trustedCluster)
	require.NoError(t, err)

	// check that remote cluster has been re-provisioned
	remoteClusters, err = main.Process.GetAuthServer().GetRemoteClusters()
	require.NoError(t, err)
	require.Len(t, remoteClusters, 1)
	require.Equal(t, clusterAux, remoteClusters[0].GetName())

	// wait for both sites to see each other via their reverse tunnels (for up to 10 seconds)
	abortTime = time.Now().Add(time.Second * 10)
	for len(checkGetClusters(t, main.Tunnel)) < 2 {
		time.Sleep(time.Millisecond * 2000)
		require.False(t, time.Now().After(abortTime), "two clusters do not see each other: tunnels are not working")
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
	require.NoError(t, err)
	require.Equal(t, "hello world\n", output.String())

	// stop clusters and remaining nodes
	require.NoError(t, main.StopAll())
	require.NoError(t, aux.StopAll())
}

func checkGetClusters(t *testing.T, tun reversetunnel.Server) []reversetunnel.RemoteSite {
	clusters, err := tun.GetSites()
	require.NoError(t, err)
	return clusters
}

func testTrustedTunnelNode(t *testing.T, suite *integrationTestSuite) {
	ctx := context.Background()
	username := suite.me.Username

	clusterMain := "cluster-main"
	clusterAux := "cluster-aux"
	main := suite.newNamedTeleportInstance(t, clusterMain)
	aux := suite.newNamedTeleportInstance(t, clusterAux)

	// main cluster has a local user and belongs to role "main-devs"
	mainDevs := "main-devs"
	role, err := types.NewRole(mainDevs, types.RoleSpecV4{
		Allow: types.RoleConditions{
			Logins: []string{username},
		},
	})
	require.NoError(t, err)
	main.AddUserWithRole(username, role)

	// for role mapping test we turn on Web API on the main cluster
	// as it's used
	makeConfig := func(enableSSH bool) (*testing.T, []*InstanceSecrets, *service.Config) {
		tconf := suite.defaultServiceConfig()
		tconf.Proxy.DisableWebService = false
		tconf.Proxy.DisableWebInterface = true
		tconf.SSH.Enabled = enableSSH
		return t, nil, tconf
	}
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	require.NoError(t, main.CreateEx(makeConfig(false)))
	require.NoError(t, aux.CreateEx(makeConfig(true)))

	// auxiliary cluster has a role aux-devs
	// connect aux cluster to main cluster
	// using trusted clusters, so remote user will be allowed to assume
	// role specified by mapping remote role "devs" to local role "local-devs"
	auxDevs := "aux-devs"
	role, err = types.NewRole(auxDevs, types.RoleSpecV4{
		Allow: types.RoleConditions{
			Logins: []string{username},
		},
	})
	require.NoError(t, err)
	err = aux.Process.GetAuthServer().UpsertRole(ctx, role)
	require.NoError(t, err)
	trustedClusterToken := "trusted-cluster-token"
	err = main.Process.GetAuthServer().UpsertToken(ctx,
		services.MustCreateProvisionToken(trustedClusterToken, []types.SystemRole{types.RoleTrustedCluster}, time.Time{}))
	require.NoError(t, err)
	trustedCluster := main.AsTrustedCluster(trustedClusterToken, types.RoleMap{
		{Remote: mainDevs, Local: []string{auxDevs}},
	})

	// modify trusted cluster resource name so it would not
	// match the cluster name to check that it does not matter
	trustedCluster.SetName(main.Secrets.SiteName + "-cluster")

	require.NoError(t, main.Start())
	require.NoError(t, aux.Start())

	err = trustedCluster.CheckAndSetDefaults()
	require.NoError(t, err)

	// try and upsert a trusted cluster
	tryCreateTrustedCluster(t, aux.Process.GetAuthServer(), trustedCluster)
	waitForTunnelConnections(t, main.Process.GetAuthServer(), clusterAux, 1)

	// Create a Teleport instance with a node that dials back to the aux cluster.
	tunnelNodeHostname := "cluster-aux-node"
	nodeConfig := func() *service.Config {
		tconf := suite.defaultServiceConfig()
		tconf.Hostname = tunnelNodeHostname
		tconf.Token = "token"
		tconf.AuthServers = []utils.NetAddr{
			{
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
	require.NoError(t, err)

	// wait for both sites to see each other via their reverse tunnels (for up to 10 seconds)
	abortTime := time.Now().Add(time.Second * 10)
	for len(checkGetClusters(t, main.Tunnel)) < 2 && len(checkGetClusters(t, aux.Tunnel)) < 2 {
		time.Sleep(time.Millisecond * 2000)
		require.False(t, time.Now().After(abortTime), "two clusters do not see each other: tunnels are not working")
	}

	// Wait for both nodes to show up before attempting to dial to them.
	err = waitForNodeCount(ctx, main, clusterAux, 2)
	require.NoError(t, err)

	cmd := []string{"echo", "hello world"}

	// Try and connect to a node in the Aux cluster from the Main cluster using
	// direct dialing.
	tc, err := main.NewClient(t, ClientConfig{
		Login:   username,
		Cluster: clusterAux,
		Host:    Loopback,
		Port:    aux.GetPortSSHInt(),
	})
	require.NoError(t, err)
	output := &bytes.Buffer{}
	tc.Stdout = output
	require.NoError(t, err)
	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond * 50)
		err = tc.SSH(context.TODO(), cmd, false)
		if err == nil {
			break
		}
	}
	require.NoError(t, err)
	require.Equal(t, "hello world\n", output.String())

	// Try and connect to a node in the Aux cluster from the Main cluster using
	// tunnel dialing.
	tunnelClient, err := main.NewClient(t, ClientConfig{
		Login:   username,
		Cluster: clusterAux,
		Host:    tunnelNodeHostname,
	})
	require.NoError(t, err)
	tunnelOutput := &bytes.Buffer{}
	tunnelClient.Stdout = tunnelOutput
	require.NoError(t, err)
	for i := 0; i < 10; i++ {
		time.Sleep(time.Millisecond * 50)
		err = tunnelClient.SSH(context.Background(), cmd, false)
		if err == nil {
			break
		}
	}
	require.NoError(t, err)
	require.Equal(t, "hello world\n", tunnelOutput.String())

	// Stop clusters and remaining nodes.
	require.NoError(t, main.StopAll())
	require.NoError(t, aux.StopAll())
}

// TestDiscoveryRecovers ensures that discovery protocol recovers from a bad discovery
// state (all known proxies are offline).
func testDiscoveryRecovers(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	username := suite.me.Username

	// create load balancer for main cluster proxies
	frontend := *utils.MustParseAddr(net.JoinHostPort(Loopback, strconv.Itoa(ports.PopInt())))
	lb, err := utils.NewLoadBalancer(context.TODO(), frontend)
	require.NoError(t, err)
	require.NoError(t, lb.Listen())
	go lb.Serve()
	defer lb.Close()

	remote := suite.newNamedTeleportInstance(t, "cluster-remote")
	main := suite.newNamedTeleportInstance(t, "cluster-main")

	remote.AddUser(username, []string{username})
	main.AddUser(username, []string{username})

	require.NoError(t, main.Create(t, remote.Secrets.AsSlice(), false, nil))
	mainSecrets := main.Secrets
	// switch listen address of the main cluster to load balancer
	mainProxyAddr := *utils.MustParseAddr(mainSecrets.TunnelAddr)
	lb.AddBackend(mainProxyAddr)
	mainSecrets.TunnelAddr = frontend.String()
	require.NoError(t, remote.Create(t, mainSecrets.AsSlice(), true, nil))

	require.NoError(t, main.Start())
	require.NoError(t, remote.Start())

	// wait for both sites to see each other via their reverse tunnels (for up to 10 seconds)
	abortTime := time.Now().Add(time.Second * 10)
	for len(checkGetClusters(t, main.Tunnel)) < 2 && len(checkGetClusters(t, remote.Tunnel)) < 2 {
		time.Sleep(time.Millisecond * 2000)
		require.False(t, time.Now().After(abortTime), "two clusters do not see each other: tunnels are not working")
	}

	// Helper function for adding a new proxy to "main".
	addNewMainProxy := func(name string) (reversetunnel.Server, ProxyConfig) {
		t.Logf("adding main proxy %q...", name)
		nodePorts := ports.PopIntSlice(3)
		proxyReverseTunnelPort, proxyWebPort, proxySSHPort := nodePorts[0], nodePorts[1], nodePorts[2]
		newConfig := ProxyConfig{
			Name:              name,
			SSHPort:           proxySSHPort,
			WebPort:           proxyWebPort,
			ReverseTunnelPort: proxyReverseTunnelPort,
		}
		newProxy, err := main.StartProxy(newConfig)
		require.NoError(t, err)

		// add proxy as a backend to the load balancer
		lb.AddBackend(*utils.MustParseAddr(net.JoinHostPort(Loopback, strconv.Itoa(proxyReverseTunnelPort))))
		return newProxy, newConfig
	}

	killMainProxy := func(name string) {
		t.Logf("killing main proxy %q...", name)
		for _, p := range main.Nodes {
			if !p.Config.Proxy.Enabled {
				continue
			}
			if p.Config.Hostname == name {
				reverseTunnelPort := utils.MustParseAddr(p.Config.Proxy.ReverseTunnelListenAddr.Addr).Port(0)
				require.NoError(t, lb.RemoveBackend(*utils.MustParseAddr(net.JoinHostPort(Loopback, strconv.Itoa(reverseTunnelPort)))))
				require.NoError(t, p.Close())
				require.NoError(t, p.Wait())
				return
			}
		}
		t.Errorf("cannot close proxy %q (not found)", name)
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
		output, err := runCommand(t, main, []string{"echo", "hello world"}, clientConf, 10)
		if shouldFail {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, "hello world\n", output)
		}
	}

	// ensure that initial proxy's tunnel has been established
	waitForActiveTunnelConnections(t, main.Tunnel, "cluster-remote", 1)
	// execute the connection via initial proxy; should not fail
	testProxyConn(nil, false)

	// helper funcion for making numbered proxy names
	pname := func(n int) string {
		return fmt.Sprintf("cluster-main-proxy-%d", n)
	}

	// create first numbered proxy
	_, c0 := addNewMainProxy(pname(0))
	// check that we now have two tunnel connections
	require.NoError(t, waitForProxyCount(remote, "cluster-main", 2))
	// check that first numbered proxy is OK.
	testProxyConn(&c0, false)
	// remove the initial proxy.
	require.NoError(t, lb.RemoveBackend(mainProxyAddr))
	require.NoError(t, waitForProxyCount(remote, "cluster-main", 1))

	// force bad state by iteratively removing previous proxy before
	// adding next proxy; this ensures that discovery protocol's list of
	// known proxies is all invalid.
	for i := 0; i < 6; i++ {
		prev, next := pname(i), pname(i+1)
		killMainProxy(prev)
		require.NoError(t, waitForProxyCount(remote, "cluster-main", 0))
		_, cn := addNewMainProxy(next)
		require.NoError(t, waitForProxyCount(remote, "cluster-main", 1))
		testProxyConn(&cn, false)
	}

	// Stop both clusters and remaining nodes.
	require.NoError(t, remote.StopAll())
	require.NoError(t, main.StopAll())
}

// TestDiscovery tests case for multiple proxies and a reverse tunnel
// agent that eventually connnects to the the right proxy
func testDiscovery(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	username := suite.me.Username

	// create load balancer for main cluster proxies
	frontend := *utils.MustParseAddr(net.JoinHostPort(Loopback, strconv.Itoa(ports.PopInt())))
	lb, err := utils.NewLoadBalancer(context.TODO(), frontend)
	require.NoError(t, err)
	require.NoError(t, lb.Listen())
	go lb.Serve()
	defer lb.Close()

	remote := suite.newNamedTeleportInstance(t, "cluster-remote")
	main := suite.newNamedTeleportInstance(t, "cluster-main")

	remote.AddUser(username, []string{username})
	main.AddUser(username, []string{username})

	require.NoError(t, main.Create(t, remote.Secrets.AsSlice(), false, nil))
	mainSecrets := main.Secrets
	// switch listen address of the main cluster to load balancer
	mainProxyAddr := *utils.MustParseAddr(mainSecrets.TunnelAddr)
	lb.AddBackend(mainProxyAddr)
	mainSecrets.TunnelAddr = frontend.String()
	require.NoError(t, remote.Create(t, mainSecrets.AsSlice(), true, nil))

	require.NoError(t, main.Start())
	require.NoError(t, remote.Start())

	// wait for both sites to see each other via their reverse tunnels (for up to 10 seconds)
	abortTime := time.Now().Add(time.Second * 10)
	for len(checkGetClusters(t, main.Tunnel)) < 2 && len(checkGetClusters(t, remote.Tunnel)) < 2 {
		time.Sleep(time.Millisecond * 2000)
		require.False(t, time.Now().After(abortTime), "two clusters do not see each other: tunnels are not working")
	}

	// start second proxy
	nodePorts := ports.PopIntSlice(3)
	proxyReverseTunnelPort, proxyWebPort, proxySSHPort := nodePorts[0], nodePorts[1], nodePorts[2]
	proxyConfig := ProxyConfig{
		Name:              "cluster-main-proxy",
		SSHPort:           proxySSHPort,
		WebPort:           proxyWebPort,
		ReverseTunnelPort: proxyReverseTunnelPort,
	}
	secondProxy, err := main.StartProxy(proxyConfig)
	require.NoError(t, err)

	// add second proxy as a backend to the load balancer
	lb.AddBackend(*utils.MustParseAddr(net.JoinHostPort(Loopback, strconv.Itoa(proxyReverseTunnelPort))))

	// At this point the main cluster should observe two tunnels
	// connected to it from remote cluster
	waitForActiveTunnelConnections(t, main.Tunnel, "cluster-remote", 1)
	waitForActiveTunnelConnections(t, secondProxy, "cluster-remote", 1)

	// execute the connection via first proxy
	cfg := ClientConfig{
		Login:   username,
		Cluster: "cluster-remote",
		Host:    Loopback,
		Port:    remote.GetPortSSHInt(),
	}
	output, err := runCommand(t, main, []string{"echo", "hello world"}, cfg, 1)
	require.NoError(t, err)
	require.Equal(t, "hello world\n", output)

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
	output, err = runCommand(t, main, []string{"echo", "hello world"}, cfgProxy, 10)
	require.NoError(t, err)
	require.Equal(t, "hello world\n", output)

	// Now disconnect the main proxy and make sure it will reconnect eventually.
	require.NoError(t, lb.RemoveBackend(mainProxyAddr))
	waitForActiveTunnelConnections(t, secondProxy, "cluster-remote", 1)

	// Requests going via main proxy should fail.
	_, err = runCommand(t, main, []string{"echo", "hello world"}, cfg, 1)
	require.Error(t, err)

	// Requests going via second proxy should succeed.
	output, err = runCommand(t, main, []string{"echo", "hello world"}, cfgProxy, 1)
	require.NoError(t, err)
	require.Equal(t, "hello world\n", output)

	// Connect the main proxy back and make sure agents have reconnected over time.
	// This command is tried 10 times with 250 millisecond delay between each
	// attempt to allow the discovery request to be received and the connection
	// added to the agent pool.
	lb.AddBackend(mainProxyAddr)

	// Once the proxy is added a matching tunnel connection should be created.
	waitForActiveTunnelConnections(t, main.Tunnel, "cluster-remote", 1)
	waitForActiveTunnelConnections(t, secondProxy, "cluster-remote", 1)

	// Requests going via main proxy should succeed.
	output, err = runCommand(t, main, []string{"echo", "hello world"}, cfg, 40)
	require.NoError(t, err)
	require.Equal(t, "hello world\n", output)

	// Stop one of proxies on the main cluster.
	err = main.StopProxy()
	require.NoError(t, err)

	// Wait for the remote cluster to detect the outbound connection is gone.
	require.NoError(t, waitForProxyCount(remote, "cluster-main", 1))

	// Stop both clusters and remaining nodes.
	require.NoError(t, remote.StopAll())
	require.NoError(t, main.StopAll())
}

// TestDiscoveryNode makes sure the discovery protocol works with nodes.
func testDiscoveryNode(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	// Create and start load balancer for proxies.
	frontend := *utils.MustParseAddr(net.JoinHostPort(Loopback, strconv.Itoa(ports.PopInt())))
	lb, err := utils.NewLoadBalancer(context.TODO(), frontend)
	require.NoError(t, err)
	err = lb.Listen()
	require.NoError(t, err)
	go lb.Serve()
	defer lb.Close()

	// Create a Teleport instance with Auth/Proxy.
	mainConfig := func() (*testing.T, []string, []*InstanceSecrets, *service.Config) {
		tconf := suite.defaultServiceConfig()

		tconf.Auth.Enabled = true

		tconf.Proxy.Enabled = true
		tconf.Proxy.TunnelPublicAddrs = []utils.NetAddr{
			{
				AddrNetwork: "tcp",
				Addr:        frontend.String(),
			},
		}
		tconf.Proxy.DisableWebService = false
		tconf.Proxy.DisableWebInterface = true
		tconf.Proxy.DisableALPNSNIListener = true

		tconf.SSH.Enabled = false

		return t, nil, nil, tconf
	}
	main := suite.newTeleportWithConfig(mainConfig())
	defer main.StopAll()

	// Create a Teleport instance with a Proxy.
	nodePorts := ports.PopIntSlice(3)
	proxyReverseTunnelPort, proxyWebPort, proxySSHPort := nodePorts[0], nodePorts[1], nodePorts[2]
	proxyConfig := ProxyConfig{
		Name:              "cluster-main-proxy",
		SSHPort:           proxySSHPort,
		WebPort:           proxyWebPort,
		ReverseTunnelPort: proxyReverseTunnelPort,
	}
	proxyTunnel, err := main.StartProxy(proxyConfig)
	require.NoError(t, err)

	proxyOneBackend := utils.MustParseAddr(net.JoinHostPort(Loopback, main.GetPortReverseTunnel()))
	lb.AddBackend(*proxyOneBackend)
	proxyTwoBackend := utils.MustParseAddr(net.JoinHostPort(Loopback, strconv.Itoa(proxyReverseTunnelPort)))
	lb.AddBackend(*proxyTwoBackend)

	// Create a Teleport instance with a Node.
	nodeConfig := func() *service.Config {
		tconf := suite.defaultServiceConfig()
		tconf.Hostname = "cluster-main-node"
		tconf.Token = "token"
		tconf.AuthServers = []utils.NetAddr{
			{
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
	require.NoError(t, err)

	// Wait for active tunnel connections to be established.
	waitForActiveTunnelConnections(t, main.Tunnel, Site, 1)
	waitForActiveTunnelConnections(t, proxyTunnel, Site, 1)

	// Execute the connection via first proxy.
	cfg := ClientConfig{
		Login:   suite.me.Username,
		Cluster: Site,
		Host:    "cluster-main-node",
	}
	output, err := runCommand(t, main, []string{"echo", "hello world"}, cfg, 1)
	require.NoError(t, err)
	require.Equal(t, "hello world\n", output)

	// Execute the connection via second proxy, should work. This command is
	// tried 10 times with 250 millisecond delay between each attempt to allow
	// the discovery request to be received and the connection added to the agent
	// pool.
	cfgProxy := ClientConfig{
		Login:   suite.me.Username,
		Cluster: Site,
		Host:    "cluster-main-node",
		Proxy:   &proxyConfig,
	}

	output, err = runCommand(t, main, []string{"echo", "hello world"}, cfgProxy, 10)
	require.NoError(t, err)
	require.Equal(t, "hello world\n", output)

	// Remove second proxy from LB.
	require.NoError(t, lb.RemoveBackend(*proxyTwoBackend))
	waitForActiveTunnelConnections(t, main.Tunnel, Site, 1)

	// Requests going via main proxy will succeed. Requests going via second
	// proxy will fail.
	output, err = runCommand(t, main, []string{"echo", "hello world"}, cfg, 1)
	require.NoError(t, err)
	require.Equal(t, "hello world\n", output)
	_, err = runCommand(t, main, []string{"echo", "hello world"}, cfgProxy, 1)
	require.Error(t, err)

	// Add second proxy to LB, both should have a connection.
	lb.AddBackend(*proxyTwoBackend)
	waitForActiveTunnelConnections(t, main.Tunnel, Site, 1)
	waitForActiveTunnelConnections(t, proxyTunnel, Site, 1)

	// Requests going via both proxies will succeed.
	output, err = runCommand(t, main, []string{"echo", "hello world"}, cfg, 1)
	require.NoError(t, err)
	require.Equal(t, "hello world\n", output)
	output, err = runCommand(t, main, []string{"echo", "hello world"}, cfgProxy, 40)
	require.NoError(t, err)
	require.Equal(t, "hello world\n", output)

	// Stop everything.
	err = proxyTunnel.Shutdown(context.Background())
	require.NoError(t, err)
	err = main.StopAll()
	require.NoError(t, err)
}

// waitForActiveTunnelConnections  waits for remote cluster to report a minimum number of active connections
func waitForActiveTunnelConnections(t *testing.T, tunnel reversetunnel.Server, clusterName string, expectedCount int) {
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
	t.Fatalf("Connections count on %v: %v, expected %v, last error: %v", clusterName, lastCount, expectedCount, lastErr)
}

// waitForProxyCount waits a set time for the proxy count in clusterName to
// reach some value.
func waitForProxyCount(t *TeleInstance, clusterName string, count int) error {
	var counts map[string]int
	start := time.Now()
	for time.Since(start) < 17*time.Second {
		counts = t.RemoteClusterWatcher.Counts()
		if counts[clusterName] == count {
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}

	return trace.BadParameter("proxy count on %v: %v (wanted %v)", clusterName, counts[clusterName], count)
}

// waitForNodeCount waits for a certain number of nodes to show up in the remote site.
func waitForNodeCount(ctx context.Context, t *TeleInstance, clusterName string, count int) error {
	const (
		deadline     = time.Second * 30
		iterWaitTime = time.Second
	)

	err := utils.RetryStaticFor(deadline, iterWaitTime, func() error {
		remoteSite, err := t.Tunnel.GetSite(clusterName)
		if err != nil {
			return trace.Wrap(err)
		}
		accessPoint, err := remoteSite.CachingAccessPoint()
		if err != nil {
			return trace.Wrap(err)
		}
		nodes, err := accessPoint.GetNodes(ctx, apidefaults.Namespace)
		if err != nil {
			return trace.Wrap(err)
		}
		if len(nodes) == count {
			return nil
		}
		return trace.BadParameter("did not find %v nodes", count)

	})
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// waitForTunnelConnections waits for remote tunnels connections
func waitForTunnelConnections(t *testing.T, authServer *auth.Server, clusterName string, expectedCount int) {
	var conns []types.TunnelConnection
	for i := 0; i < 30; i++ {
		conns, err := authServer.Presence.GetTunnelConnections(clusterName)
		require.NoError(t, err)
		if len(conns) == expectedCount {
			return
		}
		time.Sleep(1 * time.Second)
	}
	require.Len(t, conns, expectedCount)
}

// TestExternalClient tests if we can connect to a node in a Teleport
// cluster. Both normal and recording proxies are tested.
func testExternalClient(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	// Only run this test if we have access to the external SSH binary.
	_, err := exec.LookPath("ssh")
	if err != nil {
		t.Skip("Skipping TestExternalClient, no external SSH binary found.")
		return
	}

	tests := []struct {
		desc             string
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
			desc:             "Record at Node with Agent Forwarding",
			inRecordLocation: types.RecordAtNode,
			inForwardAgent:   true,
			inCommand:        "echo hello",
			outError:         false,
			outExecOutput:    "hello",
		},
		// Record at the node, don't forward agent, will work. This is the normal
		// Teleport mode of operation.
		{
			desc:             "Record at Node without Agent Forwarding",
			inRecordLocation: types.RecordAtNode,
			inForwardAgent:   false,
			inCommand:        "echo hello",
			outError:         false,
			outExecOutput:    "hello",
		},
		// Record at the proxy, forward agent. Will work.
		{
			desc:             "Record at Proxy with Agent Forwarding",
			inRecordLocation: types.RecordAtProxy,
			inForwardAgent:   true,
			inCommand:        "echo hello",
			outError:         false,
			outExecOutput:    "hello",
		},
		// Record at the proxy, don't forward agent, request will fail because
		// recording proxy requires an agent.
		{
			desc:             "Record at Proxy without Agent Forwarding",
			inRecordLocation: types.RecordAtProxy,
			inForwardAgent:   false,
			inCommand:        "echo hello",
			outError:         true,
			outExecOutput:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// Create a Teleport instance with auth, proxy, and node.
			makeConfig := func() (*testing.T, []string, []*InstanceSecrets, *service.Config) {
				recConfig, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
					Mode: tt.inRecordLocation,
				})
				require.NoError(t, err)

				tconf := suite.defaultServiceConfig()
				tconf.Auth.Enabled = true
				tconf.Auth.SessionRecordingConfig = recConfig

				tconf.Proxy.Enabled = true
				tconf.Proxy.DisableWebService = true
				tconf.Proxy.DisableWebInterface = true

				tconf.SSH.Enabled = true

				return t, nil, nil, tconf
			}
			teleport := suite.newTeleportWithConfig(makeConfig())
			defer teleport.StopAll()

			// Start (and defer close) a agent that runs during this integration test.
			teleAgent, socketDirPath, socketPath, err := createAgent(
				suite.me,
				teleport.Secrets.Users[suite.me.Username].Key.Priv,
				teleport.Secrets.Users[suite.me.Username].Key.Cert)
			require.NoError(t, err)
			defer closeAgent(teleAgent, socketDirPath)

			// Create a *exec.Cmd that will execute the external SSH command.
			execCmd, err := externalSSHCommand(commandOptions{
				forwardAgent: tt.inForwardAgent,
				socketPath:   socketPath,
				proxyPort:    teleport.GetPortProxy(),
				nodePort:     teleport.GetPortSSH(),
				command:      tt.inCommand,
			})
			require.NoError(t, err)

			// Execute SSH command and check the output is what we expect.
			output, err := execCmd.Output()
			if tt.outError {
				require.Error(t, err)
			} else {
				if err != nil {
					// If an *exec.ExitError is returned, parse it and return stderr. If this
					// is not done then c.Assert will just print a byte array for the error.
					er, ok := err.(*exec.ExitError)
					if ok {
						t.Fatalf("Unexpected error: %v", string(er.Stderr))
					}
				}
				require.NoError(t, err)
				require.Equal(t, tt.outExecOutput, strings.TrimSpace(string(output)))
			}
		})
	}
}

// TestControlMaster checks if multiple SSH channels can be created over the
// same connection. This is frequently used by tools like Ansible.
func testControlMaster(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	// Only run this test if we have access to the external SSH binary.
	_, err := exec.LookPath("ssh")
	if err != nil {
		t.Skip("Skipping TestControlMaster, no external SSH binary found.")
		return
	}

	tests := []struct {
		inRecordLocation string
	}{
		// Run tests when Teleport is recording sessions at the node.
		{
			inRecordLocation: types.RecordAtNode,
		},
		// Run tests when Teleport is recording sessions at the proxy.
		{
			inRecordLocation: types.RecordAtProxy,
		},
	}

	for _, tt := range tests {
		controlDir, err := ioutil.TempDir("", "teleport-")
		require.NoError(t, err)
		defer os.RemoveAll(controlDir)
		controlPath := filepath.Join(controlDir, "control-path")

		// Create a Teleport instance with auth, proxy, and node.
		makeConfig := func() (*testing.T, []string, []*InstanceSecrets, *service.Config) {
			recConfig, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
				Mode: tt.inRecordLocation,
			})
			require.NoError(t, err)

			tconf := suite.defaultServiceConfig()
			tconf.Auth.Enabled = true
			tconf.Auth.SessionRecordingConfig = recConfig

			tconf.Proxy.Enabled = true
			tconf.Proxy.DisableWebService = true
			tconf.Proxy.DisableWebInterface = true

			tconf.SSH.Enabled = true

			return t, nil, nil, tconf
		}
		teleport := suite.newTeleportWithConfig(makeConfig())
		defer teleport.StopAll()

		// Start (and defer close) a agent that runs during this integration test.
		teleAgent, socketDirPath, socketPath, err := createAgent(
			suite.me,
			teleport.Secrets.Users[suite.me.Username].Key.Priv,
			teleport.Secrets.Users[suite.me.Username].Key.Cert)
		require.NoError(t, err)
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
				proxyPort:    teleport.GetPortProxy(),
				nodePort:     teleport.GetPortSSH(),
				command:      "echo hello",
			})
			require.NoError(t, err)

			// Execute SSH command and check the output is what we expect.
			output, err := execCmd.Output()
			if err != nil {
				// If an *exec.ExitError is returned, parse it and return stderr. If this
				// is not done then c.Assert will just print a byte array for the error.
				er, ok := err.(*exec.ExitError)
				if ok {
					t.Fatalf("Unexpected error: %v", string(er.Stderr))
				}
			}
			require.NoError(t, err)
			require.Equal(t, "hello", strings.TrimSpace(string(output)))
		}
	}
}

// testProxyHostKeyCheck uses the forwarding proxy to connect to a server that
// presents a host key instead of a certificate in different configurations
// for the host key checking parameter in services.ClusterConfig.
func testProxyHostKeyCheck(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	tests := []struct {
		desc           string
		inHostKeyCheck bool
		outError       bool
	}{
		// disable host key checking, should be able to connect
		{
			desc:           "Disabled",
			inHostKeyCheck: false,
			outError:       false,
		},
		// enable host key checking, should NOT be able to connect
		{
			desc:           "Enabled",
			inHostKeyCheck: true,
			outError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			hostSigner, err := ssh.ParsePrivateKey(suite.priv)
			require.NoError(t, err)

			// start a ssh server that presents a host key instead of a certificate
			nodePort := ports.PopInt()
			sshNode, err := newDiscardServer(Host, nodePort, hostSigner)
			require.NoError(t, err)
			err = sshNode.Start()
			require.NoError(t, err)
			defer sshNode.Stop()

			// create a teleport instance with auth, proxy, and node
			makeConfig := func() (*testing.T, []string, []*InstanceSecrets, *service.Config) {
				recConfig, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
					Mode:                types.RecordAtProxy,
					ProxyChecksHostKeys: types.NewBoolOption(tt.inHostKeyCheck),
				})
				require.NoError(t, err)

				tconf := suite.defaultServiceConfig()
				tconf.Auth.Enabled = true
				tconf.Auth.SessionRecordingConfig = recConfig

				tconf.Proxy.Enabled = true
				tconf.Proxy.DisableWebService = true
				tconf.Proxy.DisableWebInterface = true

				return t, nil, nil, tconf
			}
			teleport := suite.newTeleportWithConfig(makeConfig())
			defer teleport.StopAll()

			// create a teleport client and exec a command
			clientConfig := ClientConfig{
				Login:        suite.me.Username,
				Cluster:      Site,
				Host:         Host,
				Port:         nodePort,
				ForwardAgent: true,
			}
			_, err = runCommand(t, teleport, []string{"echo hello"}, clientConfig, 1)

			// check if we were able to exec the command or not
			if tt.outError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// testAuditOff checks that when session recording has been turned off,
// sessions are not recorded.
func testAuditOff(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	var err error

	// create a teleport instance with auth, proxy, and node
	makeConfig := func() (*testing.T, []string, []*InstanceSecrets, *service.Config) {
		recConfig, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
			Mode: types.RecordOff,
		})
		require.NoError(t, err)

		tconf := suite.defaultServiceConfig()
		tconf.Auth.Enabled = true
		tconf.Auth.SessionRecordingConfig = recConfig

		tconf.Proxy.Enabled = true
		tconf.Proxy.DisableWebService = true
		tconf.Proxy.DisableWebInterface = true

		tconf.SSH.Enabled = true

		return t, nil, nil, tconf
	}
	teleport := suite.newTeleportWithConfig(makeConfig())
	defer teleport.StopAll()

	// get access to a authClient for the cluster
	site := teleport.GetSiteAPI(Site)
	require.NotNil(t, site)

	// should have no sessions in it to start with
	sessions, _ := site.GetSessions(apidefaults.Namespace)
	require.Len(t, sessions, 0)

	// create interactive session (this goroutine is this user's terminal time)
	endCh := make(chan error, 1)

	myTerm := NewTerminal(250)
	go func() {
		cl, err := teleport.NewClient(t, ClientConfig{
			Login:   suite.me.Username,
			Cluster: Site,
			Host:    Host,
			Port:    teleport.GetPortSSHInt(),
		})
		require.NoError(t, err)
		cl.Stdout = myTerm
		cl.Stdin = myTerm
		err = cl.SSH(context.TODO(), []string{}, false)
		endCh <- err
	}()

	// wait until there's a session in there:
	timeoutCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	sessions, err = waitForSessionToBeEstablished(timeoutCtx, apidefaults.Namespace, site)
	require.NoError(t, err)
	session := &sessions[0]

	// wait for the user to join this session
	for len(session.Parties) == 0 {
		time.Sleep(time.Millisecond * 5)
		session, err = site.GetSession(apidefaults.Namespace, sessions[0].ID)
		require.NoError(t, err)
	}
	// make sure it's us who joined! :)
	require.Equal(t, suite.me.Username, session.Parties[0].User)

	// lets type "echo hi" followed by "enter" and then "exit" + "enter":
	myTerm.Type("\aecho hi\n\r\aexit\n\r\a")

	// wait for session to end
	select {
	case <-time.After(1 * time.Minute):
		t.Fatalf("Timed out waiting for session to end.")
	case <-endCh:
	}

	// audit log should have the fact that the session occurred recorded in it
	sessions, err = site.GetSessions(apidefaults.Namespace)
	require.NoError(t, err)
	require.Len(t, sessions, 1)

	// however, attempts to read the actual sessions should fail because it was
	// not actually recorded
	_, err = site.GetSessionChunk(apidefaults.Namespace, session.ID, 0, events.MaxChunkBytes)
	require.Error(t, err)
}

// testPAM checks that Teleport PAM integration works correctly. In this case
// that means if the account and session modules return success, the user
// should be allowed to log in. If either the account or session module does
// not return success, the user should not be able to log in.
func testPAM(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	// Check if TestPAM can run. For PAM tests to run, the binary must have
	// been built with PAM support and the system running the tests must have
	// libpam installed, and have the policy files installed. This test is
	// always run in a container as part of the CI/CD pipeline. To run this
	// test locally, install the pam_teleport.so module by running 'sudo make
	// install' from the build.assets/pam/ directory. This will install the PAM
	// module as well as the policy files.
	if !pam.BuildHasPAM() || !pam.SystemHasPAM() || !hasPAMPolicy() {
		skipMessage := "Skipping TestPAM: no policy found. To run PAM tests run " +
			"'sudo make install' from the build.assets/pam/ directory."
		t.Skip(skipMessage)
	}

	tests := []struct {
		desc          string
		inEnabled     bool
		inServiceName string
		inUsePAMAuth  bool
		outContains   []string
		environment   map[string]string
	}{
		// 0 - No PAM support, session should work but no PAM related output.
		{
			desc:          "Disabled",
			inEnabled:     false,
			inServiceName: "",
			inUsePAMAuth:  true,
			outContains:   []string{},
		},
		// 1 - PAM enabled, module account and session functions return success.
		{
			desc:          "Enabled with Module Account & Session functions succeeding",
			inEnabled:     true,
			inServiceName: "teleport-success",
			inUsePAMAuth:  true,
			outContains: []string{
				"pam_sm_acct_mgmt OK",
				"pam_sm_authenticate OK",
				"pam_sm_open_session OK",
				"pam_sm_close_session OK",
			},
		},
		// 2 - PAM enabled, module account and session functions return success.
		{
			desc:          "Enabled with Module & Session functions succeeding",
			inEnabled:     true,
			inServiceName: "teleport-success",
			inUsePAMAuth:  false,
			outContains: []string{
				"pam_sm_acct_mgmt OK",
				"pam_sm_open_session OK",
				"pam_sm_close_session OK",
			},
		},
		// 3 - PAM enabled, module account functions fail.
		{
			desc:          "Enabled with all functions failing",
			inEnabled:     true,
			inServiceName: "teleport-acct-failure",
			inUsePAMAuth:  true,
			outContains:   []string{},
		},
		// 4 - PAM enabled, module session functions fail.
		{
			desc:          "Enabled with Module & Session functions failing",
			inEnabled:     true,
			inServiceName: "teleport-session-failure",
			inUsePAMAuth:  true,
			outContains:   []string{},
		},
		// 5 - PAM enabled, custom environment variables are passed.
		{
			desc:          "Enabled with custom environment",
			inEnabled:     true,
			inServiceName: "teleport-custom-env",
			inUsePAMAuth:  false,
			outContains: []string{
				"pam_sm_acct_mgmt OK",
				"pam_sm_open_session OK",
				"pam_sm_close_session OK",
				"pam_custom_envs OK",
			},
			environment: map[string]string{
				"FIRST_NAME": "JOHN",
				"LAST_NAME":  "DOE",
				"OTHER":      "{{ external.testing }}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// Create a teleport instance with auth, proxy, and node.
			makeConfig := func() (*testing.T, []string, []*InstanceSecrets, *service.Config) {
				tconf := suite.defaultServiceConfig()
				tconf.Auth.Enabled = true

				tconf.Proxy.Enabled = true
				tconf.Proxy.DisableWebService = true
				tconf.Proxy.DisableWebInterface = true

				tconf.SSH.Enabled = true
				tconf.SSH.PAM.Enabled = tt.inEnabled
				tconf.SSH.PAM.ServiceName = tt.inServiceName
				tconf.SSH.PAM.UsePAMAuth = tt.inUsePAMAuth
				tconf.SSH.PAM.Environment = tt.environment

				return t, nil, nil, tconf
			}
			teleport := suite.newTeleportWithConfig(makeConfig())
			defer teleport.StopAll()

			termSession := NewTerminal(250)

			// Create an interactive session and write something to the terminal.
			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				cl, err := teleport.NewClient(t, ClientConfig{
					Login:   suite.me.Username,
					Cluster: Site,
					Host:    Host,
					Port:    teleport.GetPortSSHInt(),
				})
				require.NoError(t, err)

				cl.Stdout = termSession
				cl.Stdin = termSession

				termSession.Type("\aecho hi\n\r\aexit\n\r\a")
				err = cl.SSH(context.TODO(), []string{}, false)
				if !isSSHError(err) {
					require.NoError(t, err)
				}

				cancel()
			}()

			// Wait for the session to end or timeout after 10 seconds.
			select {
			case <-time.After(10 * time.Second):
				dumpGoroutineProfile()
				t.Fatalf("Timeout exceeded waiting for session to complete.")
			case <-ctx.Done():
			}

			// If any output is expected, check to make sure it was output.
			if len(tt.outContains) > 0 {
				for _, expectedOutput := range tt.outContains {
					output := termSession.Output(1024)
					t.Logf("got output: %q; want output to contain: %q", output, expectedOutput)
					require.Contains(t, output, expectedOutput)
				}
			}
		})
	}
}

// testRotateSuccess tests full cycle cert authority rotation
func testRotateSuccess(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	teleport := suite.newTeleportInstance()
	defer teleport.StopAll()

	logins := []string{suite.me.Username}
	for _, login := range logins {
		teleport.AddUser(login, []string{login})
	}

	tconf := suite.rotationConfig(true)
	config, err := teleport.GenerateConfig(t, nil, tconf)
	require.NoError(t, err)

	// Enable Kubernetes service to test issue where the `KubernetesReady` event was not properly propagated
	// and in the case where Kube service was enabled cert rotation flow was broken.
	enableKubernetesService(t, config)

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

	svc, err := waitForProcessStart(serviceC)
	require.NoError(t, err)
	defer svc.Shutdown(context.TODO())

	// Setup user in the cluster
	err = SetupUser(svc, suite.me.Username, nil)
	require.NoError(t, err)

	// capture credentials before reload started to simulate old client
	initialCreds, err := GenerateUserCreds(UserCredsRequest{Process: svc, Username: suite.me.Username})
	require.NoError(t, err)

	t.Logf("Service started. Setting rotation state to %v", types.RotationPhaseUpdateClients)

	// start rotation
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: types.RotationPhaseInit,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	hostCA, err := svc.GetAuthServer().GetCertAuthority(types.CertAuthID{Type: types.HostCA, DomainName: Site}, false)
	require.NoError(t, err)
	t.Logf("Cert authority: %v", auth.CertAuthorityInfo(hostCA))

	// wait until service phase update to be broadcasted (init phase does not trigger reload)
	err = waitForProcessEvent(svc, service.TeleportPhaseChangeEvent, 10*time.Second)
	require.NoError(t, err)

	// update clients
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: types.RotationPhaseUpdateClients,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	// wait until service reload
	svc, err = suite.waitForReload(serviceC, svc)
	require.NoError(t, err)
	defer svc.Shutdown(context.TODO())

	cfg := ClientConfig{
		Login:   suite.me.Username,
		Cluster: Site,
		Host:    Loopback,
		Port:    teleport.GetPortSSHInt(),
	}
	clt, err := teleport.NewClientWithCreds(cfg, *initialCreds)
	require.NoError(t, err)

	// client works as is before servers have been rotated
	err = runAndMatch(clt, 8, []string{"echo", "hello world"}, ".*hello world.*")
	require.NoError(t, err)

	t.Logf("Service reloaded. Setting rotation state to %v", types.RotationPhaseUpdateServers)

	// move to the next phase
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: types.RotationPhaseUpdateServers,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	hostCA, err = svc.GetAuthServer().GetCertAuthority(types.CertAuthID{Type: types.HostCA, DomainName: Site}, false)
	require.NoError(t, err)
	t.Logf("Cert authority: %v", auth.CertAuthorityInfo(hostCA))

	// wait until service reloaded
	svc, err = suite.waitForReload(serviceC, svc)
	require.NoError(t, err)
	defer svc.Shutdown(context.TODO())

	// new credentials will work from this phase to others
	newCreds, err := GenerateUserCreds(UserCredsRequest{Process: svc, Username: suite.me.Username})
	require.NoError(t, err)

	clt, err = teleport.NewClientWithCreds(cfg, *newCreds)
	require.NoError(t, err)

	// new client works
	err = runAndMatch(clt, 8, []string{"echo", "hello world"}, ".*hello world.*")
	require.NoError(t, err)

	t.Logf("Service reloaded. Setting rotation state to %v.", types.RotationPhaseStandby)

	// complete rotation
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: types.RotationPhaseStandby,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	hostCA, err = svc.GetAuthServer().GetCertAuthority(types.CertAuthID{Type: types.HostCA, DomainName: Site}, false)
	require.NoError(t, err)
	t.Logf("Cert authority: %v", auth.CertAuthorityInfo(hostCA))

	// wait until service reloaded
	svc, err = suite.waitForReload(serviceC, svc)
	require.NoError(t, err)
	defer svc.Shutdown(context.TODO())

	// new client still works
	err = runAndMatch(clt, 8, []string{"echo", "hello world"}, ".*hello world.*")
	require.NoError(t, err)

	t.Logf("Service reloaded. Rotation has completed. Shutting down service.")

	// shut down the service
	cancel()
	// close the service without waiting for the connections to drain
	svc.Close()

	select {
	case err := <-runErrCh:
		require.NoError(t, err)
	case <-time.After(20 * time.Second):
		t.Fatalf("failed to shut down the server")
	}
}

// TestRotateRollback tests cert authority rollback
func testRotateRollback(t *testing.T, s *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tconf := s.rotationConfig(true)
	teleport := s.newTeleportInstance()
	logins := []string{s.me.Username}
	for _, login := range logins {
		teleport.AddUser(login, []string{login})
	}
	config, err := teleport.GenerateConfig(t, nil, tconf)
	require.NoError(t, err)

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

	svc, err := waitForProcessStart(serviceC)
	require.NoError(t, err)

	// Setup user in the cluster
	err = SetupUser(svc, s.me.Username, nil)
	require.NoError(t, err)

	// capture credentials before reload started to simulate old client
	initialCreds, err := GenerateUserCreds(UserCredsRequest{Process: svc, Username: s.me.Username})
	require.NoError(t, err)

	t.Logf("Service started. Setting rotation state to %q.", types.RotationPhaseInit)

	// start rotation
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: types.RotationPhaseInit,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	err = waitForProcessEvent(svc, service.TeleportPhaseChangeEvent, 10*time.Second)
	require.NoError(t, err)

	t.Logf("Setting rotation state to %q.", types.RotationPhaseUpdateClients)

	// start rotation
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: types.RotationPhaseUpdateClients,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	// wait until service reload
	svc, err = s.waitForReload(serviceC, svc)
	require.NoError(t, err)

	cfg := ClientConfig{
		Login:   s.me.Username,
		Cluster: Site,
		Host:    Loopback,
		Port:    teleport.GetPortSSHInt(),
	}
	clt, err := teleport.NewClientWithCreds(cfg, *initialCreds)
	require.NoError(t, err)

	// client works as is before servers have been rotated
	err = runAndMatch(clt, 8, []string{"echo", "hello world"}, ".*hello world.*")
	require.NoError(t, err)

	t.Logf("Service reloaded. Setting rotation state to %q.", types.RotationPhaseUpdateServers)

	// move to the next phase
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: types.RotationPhaseUpdateServers,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	// wait until service reloaded
	svc, err = s.waitForReload(serviceC, svc)
	require.NoError(t, err)

	t.Logf("Service reloaded. Setting rotation state to %q.", types.RotationPhaseRollback)

	// complete rotation
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: types.RotationPhaseRollback,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	// wait until service reloaded
	svc, err = s.waitForReload(serviceC, svc)
	require.NoError(t, err)

	// old client works
	err = runAndMatch(clt, 8, []string{"echo", "hello world"}, ".*hello world.*")
	require.NoError(t, err)

	t.Log("Service reloaded. Rotation has completed. Shuttting down service.")

	// shut down the service
	cancel()
	// close the service without waiting for the connections to drain
	svc.Close()

	select {
	case err := <-runErrCh:
		require.NoError(t, err)
	case <-time.After(20 * time.Second):
		t.Fatalf("failed to shut down the server")
	}
}

// TestRotateTrustedClusters tests CA rotation support for trusted clusters
func testRotateTrustedClusters(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	t.Cleanup(func() { tr.Stop() })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	clusterMain := "rotate-main"
	clusterAux := "rotate-aux"

	tconf := suite.rotationConfig(false)
	main := suite.newNamedTeleportInstance(t, clusterMain)
	aux := suite.newNamedTeleportInstance(t, clusterAux)

	logins := []string{suite.me.Username}
	for _, login := range logins {
		main.AddUser(login, []string{login})
	}
	config, err := main.GenerateConfig(t, nil, tconf)
	require.NoError(t, err)

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

	svc, err := waitForProcessStart(serviceC)
	require.NoError(t, err)

	// main cluster has a local user and belongs to role "main-devs"
	mainDevs := "main-devs"
	role, err := types.NewRole(mainDevs, types.RoleSpecV4{
		Allow: types.RoleConditions{
			Logins: []string{suite.me.Username},
		},
	})
	require.NoError(t, err)

	err = SetupUser(svc, suite.me.Username, []types.Role{role})
	require.NoError(t, err)

	// create auxiliary cluster and setup trust
	require.NoError(t, aux.CreateEx(t, nil, suite.rotationConfig(false)))

	// auxiliary cluster has a role aux-devs
	// connect aux cluster to main cluster
	// using trusted clusters, so remote user will be allowed to assume
	// role specified by mapping remote role "devs" to local role "local-devs"
	auxDevs := "aux-devs"
	role, err = types.NewRole(auxDevs, types.RoleSpecV4{
		Allow: types.RoleConditions{
			Logins: []string{suite.me.Username},
		},
	})
	require.NoError(t, err)
	err = aux.Process.GetAuthServer().UpsertRole(ctx, role)
	require.NoError(t, err)
	trustedClusterToken := "trusted-cluster-token"
	err = svc.GetAuthServer().UpsertToken(ctx,
		services.MustCreateProvisionToken(trustedClusterToken, []types.SystemRole{types.RoleTrustedCluster}, time.Time{}))
	require.NoError(t, err)
	trustedCluster := main.AsTrustedCluster(trustedClusterToken, types.RoleMap{
		{Remote: mainDevs, Local: []string{auxDevs}},
	})
	require.NoError(t, aux.Start())

	// try and upsert a trusted cluster
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	tryCreateTrustedCluster(t, aux.Process.GetAuthServer(), trustedCluster)
	waitForTunnelConnections(t, svc.GetAuthServer(), aux.Secrets.SiteName, 1)

	// capture credentials before reload has started to simulate old client
	initialCreds, err := GenerateUserCreds(UserCredsRequest{
		Process:  svc,
		Username: suite.me.Username,
	})
	require.NoError(t, err)

	// credentials should work
	cfg := ClientConfig{
		Login:   suite.me.Username,
		Host:    Loopback,
		Cluster: clusterAux,
		Port:    aux.GetPortSSHInt(),
	}
	clt, err := main.NewClientWithCreds(cfg, *initialCreds)
	require.NoError(t, err)

	err = runAndMatch(clt, 8, []string{"echo", "hello world"}, ".*hello world.*")
	require.NoError(t, err)

	t.Logf("Setting rotation state to %v", types.RotationPhaseInit)

	// start rotation
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: types.RotationPhaseInit,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	// wait until service phase update to be broadcast (init phase does not trigger reload)
	err = waitForProcessEvent(svc, service.TeleportPhaseChangeEvent, 10*time.Second)
	require.NoError(t, err)

	// waitForPhase waits until aux cluster detects the rotation
	waitForPhase := func(phase string) error {
		ctx, cancel := context.WithTimeout(context.Background(), tconf.PollingPeriod*10)
		defer cancel()

		watcher, err := services.NewCertAuthorityWatcher(ctx, services.CertAuthorityWatcherConfig{
			ResourceWatcherConfig: services.ResourceWatcherConfig{
				Component: teleport.ComponentProxy,
				Clock:     tconf.Clock,
				Client:    aux.GetSiteAPI(clusterAux),
			},
			WatchHostCA: true,
		})
		if err != nil {
			return err
		}
		defer watcher.Close()

		var lastPhase string
		for i := 0; i < 10; i++ {
			select {
			case <-ctx.Done():
				return trace.CompareFailed("failed to converge to phase %q, last phase %q", phase, lastPhase)
			case cas := <-watcher.CertAuthorityC:
				for _, ca := range cas {
					if ca.GetClusterName() == clusterMain &&
						ca.GetType() == types.HostCA &&
						ca.GetRotation().Phase == phase {
						return nil
					}
					lastPhase = ca.GetRotation().Phase
				}
			}
		}
		return trace.CompareFailed("failed to converge to phase %q, last phase %q", phase, lastPhase)
	}

	err = waitForPhase(types.RotationPhaseInit)
	require.NoError(t, err)

	// update clients
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: types.RotationPhaseUpdateClients,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	// wait until service reloaded
	svc, err = suite.waitForReload(serviceC, svc)
	require.NoError(t, err)

	err = waitForPhase(types.RotationPhaseUpdateClients)
	require.NoError(t, err)

	// old client should work as is
	err = runAndMatch(clt, 8, []string{"echo", "hello world"}, ".*hello world.*")
	require.NoError(t, err)

	t.Logf("Service reloaded. Setting rotation state to %v", types.RotationPhaseUpdateServers)

	// move to the next phase
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: types.RotationPhaseUpdateServers,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	// wait until service reloaded
	svc, err = suite.waitForReload(serviceC, svc)
	require.NoError(t, err)

	err = waitForPhase(types.RotationPhaseUpdateServers)
	require.NoError(t, err)

	// new credentials will work from this phase to others
	newCreds, err := GenerateUserCreds(UserCredsRequest{Process: svc, Username: suite.me.Username})
	require.NoError(t, err)

	clt, err = main.NewClientWithCreds(cfg, *newCreds)
	require.NoError(t, err)

	// new client works
	err = runAndMatch(clt, 8, []string{"echo", "hello world"}, ".*hello world.*")
	require.NoError(t, err)

	t.Logf("Service reloaded. Setting rotation state to %v.", types.RotationPhaseStandby)

	// complete rotation
	err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
		TargetPhase: types.RotationPhaseStandby,
		Mode:        types.RotationModeManual,
	})
	require.NoError(t, err)

	// wait until service reloaded
	t.Log("Waiting for service reload.")
	svc, err = suite.waitForReload(serviceC, svc)
	require.NoError(t, err)
	t.Log("Service reload completed, waiting for phase.")

	err = waitForPhase(types.RotationPhaseStandby)
	require.NoError(t, err)
	t.Log("Phase completed.")

	// new client still works
	err = runAndMatch(clt, 8, []string{"echo", "hello world"}, ".*hello world.*")
	require.NoError(t, err)

	t.Log("Service reloaded. Rotation has completed. Shutting down service.")

	// shut down the service
	cancel()
	// close the service without waiting for the connections to drain
	require.NoError(t, svc.Close())

	select {
	case err := <-runErrCh:
		require.NoError(t, err)
	case <-time.After(20 * time.Second):
		t.Fatalf("failed to shut down the server")
	}
}

// TestRotateChangeSigningAlg tests the change of CA signing algorithm on
// manual rotation.
func testRotateChangeSigningAlg(t *testing.T, suite *integrationTestSuite) {
	// Start with an instance using default signing alg.
	tconf := suite.rotationConfig(true)
	teleport := suite.newTeleportInstance()
	logins := []string{suite.me.Username}
	for _, login := range logins {
		teleport.AddUser(login, []string{login})
	}
	config, err := teleport.GenerateConfig(t, nil, tconf)
	require.NoError(t, err)

	serviceC := make(chan *service.TeleportProcess, 20)
	runErrCh := make(chan error, 1)

	restart := func(svc *service.TeleportProcess, cancel func()) (*service.TeleportProcess, func()) {
		if svc != nil && cancel != nil {
			// shut down the service
			cancel()
			// close the service without waiting for the connections to drain
			err := svc.Close()
			require.NoError(t, err)
			err = svc.Wait()
			require.NoError(t, err)

			select {
			case err := <-runErrCh:
				require.NoError(t, err)
			case <-time.After(20 * time.Second):
				t.Fatalf("failed to shut down the server")
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
		require.NoError(t, err)
		return svc, cancel
	}

	assertSigningAlg := func(svc *service.TeleportProcess, alg string) {
		hostCA, err := svc.GetAuthServer().GetCertAuthority(types.CertAuthID{Type: types.HostCA, DomainName: Site}, false)
		require.NoError(t, err)
		require.Equal(t, alg, sshutils.GetSigningAlgName(hostCA))

		userCA, err := svc.GetAuthServer().GetCertAuthority(types.CertAuthID{Type: types.UserCA, DomainName: Site}, false)
		require.NoError(t, err)
		require.Equal(t, alg, sshutils.GetSigningAlgName(userCA))
	}

	rotate := func(svc *service.TeleportProcess, mode string) *service.TeleportProcess {
		t.Logf("Rotation phase: %q.", types.RotationPhaseInit)
		err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
			TargetPhase: types.RotationPhaseInit,
			Mode:        mode,
		})
		require.NoError(t, err)

		// wait until service phase update to be broadcasted (init phase does not trigger reload)
		err = waitForProcessEvent(svc, service.TeleportPhaseChangeEvent, 10*time.Second)
		require.NoError(t, err)

		t.Logf("Rotation phase: %q.", types.RotationPhaseUpdateClients)
		err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
			TargetPhase: types.RotationPhaseUpdateClients,
			Mode:        mode,
		})
		require.NoError(t, err)

		// wait until service reload
		svc, err = suite.waitForReload(serviceC, svc)
		require.NoError(t, err)

		t.Logf("Rotation phase: %q.", types.RotationPhaseUpdateServers)
		err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
			TargetPhase: types.RotationPhaseUpdateServers,
			Mode:        mode,
		})
		require.NoError(t, err)

		// wait until service reloaded
		svc, err = suite.waitForReload(serviceC, svc)
		require.NoError(t, err)

		t.Logf("rotation phase: %q", types.RotationPhaseStandby)
		err = svc.GetAuthServer().RotateCertAuthority(auth.RotateRequest{
			TargetPhase: types.RotationPhaseStandby,
			Mode:        mode,
		})
		require.NoError(t, err)

		// wait until service reloaded
		svc, err = suite.waitForReload(serviceC, svc)
		require.NoError(t, err)

		return svc
	}

	// Start the instance.
	svc, cancel := restart(nil, nil)

	t.Log("default signature algorithm due to empty config value")
	// Verify the default signing algorithm with config value empty.
	assertSigningAlg(svc, defaults.CASignatureAlgorithm)

	t.Log("change signature algorithm with custom config value and manual rotation")
	// Change the signing algorithm in config file.
	signingAlg := ssh.SigAlgoRSA
	config.CASignatureAlgorithm = &signingAlg
	svc, cancel = restart(svc, cancel)
	// Do a manual rotation - this should change the signing algorithm.
	svc = rotate(svc, types.RotationModeManual)
	assertSigningAlg(svc, ssh.SigAlgoRSA)

	t.Log("preserve signature algorithm with empty config value and manual rotation")
	// Unset the config value.
	config.CASignatureAlgorithm = nil
	svc, cancel = restart(svc, cancel)

	// Do a manual rotation - this should leave the signing algorithm
	// unaffected because config value is not set.
	svc = rotate(svc, types.RotationModeManual)
	assertSigningAlg(svc, ssh.SigAlgoRSA)

	// shut down the service
	cancel()
	// close the service without waiting for the connections to drain
	svc.Close()

	select {
	case err := <-runErrCh:
		require.NoError(t, err)
	case <-time.After(20 * time.Second):
		t.Fatalf("failed to shut down the server")
	}
}

// rotationConfig sets up default config used for CA rotation tests
func (s *integrationTestSuite) rotationConfig(disableWebService bool) *service.Config {
	tconf := s.defaultServiceConfig()
	tconf.SSH.Enabled = true
	tconf.Proxy.DisableWebService = disableWebService
	tconf.Proxy.DisableWebInterface = true
	tconf.PollingPeriod = 500 * time.Millisecond
	tconf.ClientTimeout = time.Second
	tconf.ShutdownTimeout = 2 * tconf.ClientTimeout
	tconf.MaxRetryPeriod = time.Second
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
	case <-time.After(1 * time.Minute):
		dumpGoroutineProfile()
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
func (s *integrationTestSuite) waitForReload(serviceC chan *service.TeleportProcess, old *service.TeleportProcess) (*service.TeleportProcess, error) {
	var svc *service.TeleportProcess
	select {
	case svc = <-serviceC:
	case <-time.After(1 * time.Minute):
		dumpGoroutineProfile()
		return nil, trace.BadParameter("timeout waiting for service to start")
	}

	eventC := make(chan service.Event, 1)
	svc.WaitForEvent(context.TODO(), service.TeleportReadyEvent, eventC)
	select {
	case <-eventC:

	case <-time.After(20 * time.Second):
		dumpGoroutineProfile()
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
		case <-time.After(1 * time.Minute):
			dumpGoroutineProfile()
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
func testWindowChange(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	teleport := suite.newTeleport(t, nil, true)
	defer teleport.StopAll()

	site := teleport.GetSiteAPI(Site)
	require.NotNil(t, site)

	personA := NewTerminal(250)
	personB := NewTerminal(250)

	// openSession will open a new session on a server.
	openSession := func() {
		cl, err := teleport.NewClient(t, ClientConfig{
			Login:   suite.me.Username,
			Cluster: Site,
			Host:    Host,
			Port:    teleport.GetPortSSHInt(),
		})
		require.NoError(t, err)

		cl.Stdout = personA
		cl.Stdin = personA

		err = cl.SSH(context.TODO(), []string{}, false)
		if !isSSHError(err) {
			require.NoError(t, err)
		}
	}

	// joinSession will join the existing session on a server.
	joinSession := func() {
		// Find the existing session in the backend.
		timeoutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		sessions, err := waitForSessionToBeEstablished(timeoutCtx, apidefaults.Namespace, site)
		require.NoError(t, err)
		sessionID := string(sessions[0].ID)

		cl, err := teleport.NewClient(t, ClientConfig{
			Login:   suite.me.Username,
			Cluster: Site,
			Host:    Host,
			Port:    teleport.GetPortSSHInt(),
		})
		require.NoError(t, err)

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
			err = cl.Join(context.TODO(), apidefaults.Namespace, session.ID(sessionID), personB)
			if err == nil || isSSHError(err) {
				err = nil
				break
			}
		}

		require.NoError(t, err)
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
				dumpGoroutineProfile()
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
	require.NoError(t, err)

	// As soon as person B joins the session, the terminal is resized to 160x48.
	// Have another user join the session. As soon as the second shell is
	// created, the window is resized to 160x48 (see joinSession implementation).
	go joinSession()

	// Use the "printf" command to print the window size again and make sure it's
	// 160x48.
	personA.Type("\atput cols; tput lines\n\r\a")
	err = waitForOutput(personA, "160\r\n48", "160\n\r48", "160\n48")
	require.NoError(t, err)

	// Close the session.
	personA.Type("\aexit\r\n\a")
}

// testList checks that the list of servers returned is identity aware.
func testList(t *testing.T, suite *integrationTestSuite) {
	ctx := context.Background()

	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	// Create and start a Teleport cluster with auth, proxy, and node.
	makeConfig := func() (*testing.T, []string, []*InstanceSecrets, *service.Config) {
		recConfig, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
			Mode: types.RecordOff,
		})
		require.NoError(t, err)

		tconf := suite.defaultServiceConfig()
		tconf.Hostname = "server-01"
		tconf.Auth.Enabled = true
		tconf.Auth.SessionRecordingConfig = recConfig
		tconf.Proxy.Enabled = true
		tconf.Proxy.DisableWebService = true
		tconf.Proxy.DisableWebInterface = true
		tconf.SSH.Enabled = true
		tconf.SSH.Labels = map[string]string{
			"role": "worker",
		}

		return t, nil, nil, tconf
	}
	teleport := suite.newTeleportWithConfig(makeConfig())
	defer teleport.StopAll()

	// Create and start a Teleport node.
	nodeSSHPort := ports.PopInt()
	nodeConfig := func() *service.Config {
		tconf := suite.defaultServiceConfig()
		tconf.Hostname = "server-02"
		tconf.SSH.Enabled = true
		tconf.SSH.Addr.Addr = net.JoinHostPort(teleport.Hostname, fmt.Sprintf("%v", nodeSSHPort))
		tconf.SSH.Labels = map[string]string{
			"role": "database",
		}

		return tconf
	}
	_, err := teleport.StartNode(nodeConfig())
	require.NoError(t, err)

	// Get an auth client to the cluster.
	clt := teleport.GetSiteAPI(Site)
	require.NotNil(t, clt)

	// Wait 10 seconds for both nodes to show up to make sure they both have
	// registered themselves.
	waitForNodes := func(clt auth.ClientI, count int) error {
		tickCh := time.Tick(500 * time.Millisecond)
		stopCh := time.After(10 * time.Second)
		for {
			select {
			case <-tickCh:
				nodesInCluster, err := clt.GetNodes(ctx, apidefaults.Namespace)
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
	require.NoError(t, err)

	tests := []struct {
		inRoleName string
		inLabels   types.Labels
		inLogin    string
		outNodes   []string
	}{
		// 0 - Role has label "role:worker", only server-01 is returned.
		{
			inRoleName: "worker-only",
			inLogin:    "foo",
			inLabels:   types.Labels{"role": []string{"worker"}},
			outNodes:   []string{"server-01"},
		},
		// 1 - Role has label "role:database", only server-02 is returned.
		{
			inRoleName: "database-only",
			inLogin:    "bar",
			inLabels:   types.Labels{"role": []string{"database"}},
			outNodes:   []string{"server-02"},
		},
		// 2 - Role has wildcard label, all nodes are returned server-01 and server-2.
		{
			inRoleName: "worker-and-database",
			inLogin:    "baz",
			inLabels:   types.Labels{types.Wildcard: []string{types.Wildcard}},
			outNodes:   []string{"server-01", "server-02"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.inRoleName, func(t *testing.T) {
			// Create role with logins and labels for this test.
			role, err := types.NewRole(tt.inRoleName, types.RoleSpecV4{
				Allow: types.RoleConditions{
					Logins:     []string{tt.inLogin},
					NodeLabels: tt.inLabels,
				},
			})
			require.NoError(t, err)

			// Create user, role, and generate credentials.
			err = SetupUser(teleport.Process, tt.inLogin, []types.Role{role})
			require.NoError(t, err)
			initialCreds, err := GenerateUserCreds(UserCredsRequest{Process: teleport.Process, Username: tt.inLogin})
			require.NoError(t, err)

			// Create a Teleport client.
			cfg := ClientConfig{
				Login:   tt.inLogin,
				Cluster: Site,
				Port:    teleport.GetPortSSHInt(),
			}
			userClt, err := teleport.NewClientWithCreds(cfg, *initialCreds)
			require.NoError(t, err)

			// Get list of nodes and check that the returned nodes match the
			// expected nodes.
			nodes, err := userClt.ListNodes(context.Background())
			require.NoError(t, err)
			for _, node := range nodes {
				ok := apiutils.SliceContainsStr(tt.outNodes, node.GetHostname())
				if !ok {
					t.Fatalf("Got nodes: %v, want: %v.", nodes, tt.outNodes)
				}
			}
		})
	}
}

// TestCmdLabels verifies the behavior of running commands via labels
// with a mixture of regular and reversetunnel nodes.
func testCmdLabels(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	// InsecureDevMode needed for IoT node handshake
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	// Create and start a Teleport cluster with auth, proxy, and node.
	makeConfig := func() *service.Config {
		recConfig, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
			Mode: types.RecordOff,
		})
		require.NoError(t, err)

		tconf := suite.defaultServiceConfig()
		tconf.Hostname = "server-01"
		tconf.Auth.Enabled = true
		tconf.Auth.SessionRecordingConfig = recConfig
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
	teleport := suite.newTeleportWithConfig(t, nil, nil, makeConfig())
	defer teleport.StopAll()

	// Create and start a reversetunnel node.
	nodeConfig := func() *service.Config {
		tconf := suite.defaultServiceConfig()
		tconf.Hostname = "server-02"
		tconf.SSH.Enabled = true
		tconf.SSH.Labels = map[string]string{
			"role": "database",
			"spam": "eggs",
		}

		return tconf
	}
	_, err := teleport.StartReverseTunnelNode(nodeConfig())
	require.NoError(t, err)

	// test label patterns that match both nodes, and each
	// node individually.
	tts := []struct {
		desc    string
		command []string
		labels  map[string]string
		expect  string
	}{
		{
			desc:    "Both",
			command: []string{"echo", "two"},
			labels:  map[string]string{"spam": "eggs"},
			expect:  "two\ntwo\n",
		},
		{
			desc:    "Worker only",
			command: []string{"echo", "worker"},
			labels:  map[string]string{"role": "worker"},
			expect:  "worker\n",
		},
		{
			desc:    "Database only",
			command: []string{"echo", "database"},
			labels:  map[string]string{"role": "database"},
			expect:  "database\n",
		},
	}

	for _, tt := range tts {
		t.Run(tt.desc, func(t *testing.T) {
			cfg := ClientConfig{
				Login:   suite.me.Username,
				Cluster: Site,
				Labels:  tt.labels,
			}

			output, err := runCommand(t, teleport, tt.command, cfg, 1)
			require.NoError(t, err)
			require.Equal(t, tt.expect, output)
		})
	}
}

// TestDataTransfer makes sure that a "session.data" event is emitted at the
// end of a session that matches the amount of data that was transferred.
func testDataTransfer(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	KB := 1024
	MB := 1048576

	// Create a Teleport cluster.
	main := suite.newTeleport(t, nil, true)
	defer main.StopAll()

	// Create a client to the above Teleport cluster.
	clientConfig := ClientConfig{
		Login:   suite.me.Username,
		Cluster: Site,
		Host:    Host,
		Port:    main.GetPortSSHInt(),
	}

	// Write 1 MB to stdout.
	command := []string{"dd", "if=/dev/zero", "bs=1024", "count=1024"}
	output, err := runCommand(t, main, command, clientConfig, 1)
	require.NoError(t, err)

	// Make sure exactly 1 MB was written to output.
	require.Len(t, output, MB)

	// Make sure the session.data event was emitted to the audit log.
	eventFields, err := findEventInLog(main, events.SessionDataEvent)
	require.NoError(t, err)

	// Make sure the audit event shows that 1 MB was written to the output.
	require.Greater(t, eventFields.GetInt(events.DataReceived), MB)
	require.Greater(t, eventFields.GetInt(events.DataTransmitted), KB)
}

func testBPFInteractive(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	// Check if BPF tests can be run on this host.
	err := canTestBPF()
	if err != nil {
		t.Skip(fmt.Sprintf("Tests for BPF functionality can not be run: %v.", err))
		return
	}

	lsPath, err := exec.LookPath("ls")
	require.NoError(t, err)

	tests := []struct {
		desc               string
		inSessionRecording string
		inBPFEnabled       bool
		outFound           bool
	}{
		// For session recorded at the node, enhanced events should be found.
		{
			desc:               "Enabled and Recorded At Node",
			inSessionRecording: types.RecordAtNode,
			inBPFEnabled:       true,
			outFound:           true,
		},
		// For session recorded at the node, but BPF is turned off, no events
		// should be found.
		{
			desc:               "Disabled and Recorded At Node",
			inSessionRecording: types.RecordAtNode,
			inBPFEnabled:       false,
			outFound:           false,
		},
		// For session recorded at the proxy, enhanced events should not be found.
		// BPF turned off simulates an OpenSSH node.
		{
			desc:               "Disabled and Recorded At Proxy",
			inSessionRecording: types.RecordAtProxy,
			inBPFEnabled:       false,
			outFound:           false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// Create temporary directory where cgroup2 hierarchy will be mounted.
			dir := t.TempDir()

			// Create and start a Teleport cluster.
			makeConfig := func() (*testing.T, []string, []*InstanceSecrets, *service.Config) {
				recConfig, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
					Mode: tt.inSessionRecording,
				})
				require.NoError(t, err)

				// Create default config.
				tconf := suite.defaultServiceConfig()

				// Configure Auth.
				tconf.Auth.Preference.SetSecondFactor("off")
				tconf.Auth.Enabled = true
				tconf.Auth.SessionRecordingConfig = recConfig

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
				return t, nil, nil, tconf
			}
			main := suite.newTeleportWithConfig(makeConfig())
			defer main.StopAll()

			// Create a client terminal and context to signal when the client is done
			// with the terminal.
			term := NewTerminal(250)
			doneContext, doneCancel := context.WithCancel(context.Background())

			func() {
				client, err := main.NewClient(t, ClientConfig{
					Login:   suite.me.Username,
					Cluster: Site,
					Host:    Host,
					Port:    main.GetPortSSHInt(),
				})
				require.NoError(t, err)

				// Connect terminal to std{in,out} of client.
				client.Stdout = term
				client.Stdin = term

				// "Type" a command into the terminal.
				term.Type(fmt.Sprintf("\a%v\n\r\aexit\n\r\a", lsPath))
				err = client.SSH(context.TODO(), []string{}, false)
				require.NoError(t, err)

				// Signal that the client has finished the interactive session.
				doneCancel()
			}()

			// Wait 10 seconds for the client to finish up the interactive session.
			select {
			case <-time.After(10 * time.Second):
				t.Fatalf("Timed out waiting for client to finish interactive session.")
			case <-doneContext.Done():
			}

			// Enhanced events should show up for session recorded at the node but not
			// at the proxy.
			if tt.outFound {
				_, err = findCommandEventInLog(main, events.SessionCommandEvent, lsPath)
				require.NoError(t, err)
			} else {
				_, err = findCommandEventInLog(main, events.SessionCommandEvent, lsPath)
				require.Error(t, err)
			}
		})
	}
}

func testBPFExec(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	// Check if BPF tests can be run on this host.
	err := canTestBPF()
	if err != nil {
		t.Skip(fmt.Sprintf("Tests for BPF functionality can not be run: %v.", err))
		return
	}

	lsPath, err := exec.LookPath("ls")
	require.NoError(t, err)

	tests := []struct {
		desc               string
		inSessionRecording string
		inBPFEnabled       bool
		outFound           bool
	}{
		// For session recorded at the node, enhanced events should be found.
		{
			desc:               "Enabled and recorded at node",
			inSessionRecording: types.RecordAtNode,
			inBPFEnabled:       true,
			outFound:           true,
		},
		// For session recorded at the node, but BPF is turned off, no events
		// should be found.
		{
			desc:               "Disabled and recorded at node",
			inSessionRecording: types.RecordAtNode,
			inBPFEnabled:       false,
			outFound:           false,
		},
		// For session recorded at the proxy, enhanced events should not be found.
		// BPF turned off simulates an OpenSSH node.
		{
			desc:               "Disabled and recorded at proxy",
			inSessionRecording: types.RecordAtProxy,
			inBPFEnabled:       false,
			outFound:           false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// Create temporary directory where cgroup2 hierarchy will be mounted.
			dir := t.TempDir()

			// Create and start a Teleport cluster.
			makeConfig := func() (*testing.T, []string, []*InstanceSecrets, *service.Config) {
				recConfig, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
					Mode: tt.inSessionRecording,
				})
				require.NoError(t, err)

				// Create default config.
				tconf := suite.defaultServiceConfig()

				// Configure Auth.
				tconf.Auth.Preference.SetSecondFactor("off")
				tconf.Auth.Enabled = true
				tconf.Auth.SessionRecordingConfig = recConfig

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
				return t, nil, nil, tconf
			}
			main := suite.newTeleportWithConfig(makeConfig())
			defer main.StopAll()

			// Create a client to the above Teleport cluster.
			clientConfig := ClientConfig{
				Login:   suite.me.Username,
				Cluster: Site,
				Host:    Host,
				Port:    main.GetPortSSHInt(),
			}

			// Run exec command.
			_, err = runCommand(t, main, []string{lsPath}, clientConfig, 1)
			require.NoError(t, err)

			// Enhanced events should show up for session recorded at the node but not
			// at the proxy.
			if tt.outFound {
				_, err = findCommandEventInLog(main, events.SessionCommandEvent, lsPath)
				require.NoError(t, err)
			} else {
				_, err = findCommandEventInLog(main, events.SessionCommandEvent, lsPath)
				require.Error(t, err)
			}
		})
	}
}

func testSSHExitCode(t *testing.T, suite *integrationTestSuite) {
	lsPath, err := exec.LookPath("ls")
	require.NoError(t, err)

	var tests = []struct {
		desc           string
		command        []string
		input          string
		interactive    bool
		errorAssertion require.ErrorAssertionFunc
		statusCode     int
	}{
		// A successful noninteractive session should have a zero status code
		{
			desc:           "Run Command and Exit Successfully",
			command:        []string{lsPath},
			interactive:    false,
			errorAssertion: require.NoError,
		},
		// A failed noninteractive session should have a non-zero status code
		{
			desc:           "Run Command and Fail With Code 2",
			command:        []string{"exit 2"},
			interactive:    false,
			errorAssertion: require.Error,
			statusCode:     2,
		},
		// A failed interactive session should have a non-zero status code
		{
			desc:           "Run Command Interactively and Fail With Code 2",
			command:        []string{"exit 2"},
			interactive:    true,
			errorAssertion: require.Error,
			statusCode:     2,
		},
		// A failed interactive session should have a non-zero status code
		{
			desc:           "Interactively Fail With Code 3",
			input:          "exit 3\n\r",
			interactive:    true,
			errorAssertion: require.Error,
			statusCode:     3,
		},
		// A failed interactive session should have a non-zero status code
		{
			desc:           "Interactively Fail With Code 3",
			input:          fmt.Sprintf("%v\n\rexit 3\n\r", lsPath),
			interactive:    true,
			errorAssertion: require.Error,
			statusCode:     3,
		},
		// A successful interactive session should have a zero status code
		{
			desc:           "Interactively Run Command and Exit Successfully",
			input:          fmt.Sprintf("%v\n\rexit\n\r", lsPath),
			interactive:    true,
			errorAssertion: require.NoError,
		},
		// A successful interactive session should have a zero status code
		{
			desc:           "Interactively Exit",
			input:          "exit\n\r",
			interactive:    true,
			errorAssertion: require.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			// Create and start a Teleport cluster.
			makeConfig := func() (*testing.T, []string, []*InstanceSecrets, *service.Config) {
				// Create default config.
				tconf := suite.defaultServiceConfig()

				// Configure Auth.
				tconf.Auth.Preference.SetSecondFactor("off")
				tconf.Auth.Enabled = true
				tconf.Auth.NoAudit = true

				// Configure Proxy.
				tconf.Proxy.Enabled = true
				tconf.Proxy.DisableWebService = false
				tconf.Proxy.DisableWebInterface = true

				// Configure Node.
				tconf.SSH.Enabled = true
				return t, nil, nil, tconf
			}
			main := suite.newTeleportWithConfig(makeConfig())
			t.Cleanup(func() { main.StopAll() })

			// context to signal when the client is done with the terminal.
			doneContext, doneCancel := context.WithTimeout(context.Background(), time.Second*10)
			defer doneCancel()

			cli, err := main.NewClient(t, ClientConfig{
				Login:       suite.me.Username,
				Cluster:     Site,
				Host:        Host,
				Port:        main.GetPortSSHInt(),
				Interactive: tt.interactive,
			})
			require.NoError(t, err)

			if tt.interactive {
				// Create a new terminal and connect it to std{in,out} of client.
				term := NewTerminal(250)
				cli.Stdout = term
				cli.Stdin = term
				term.Type(tt.input)
			}

			// run the ssh command
			err = cli.SSH(doneContext, tt.command, false)
			tt.errorAssertion(t, err)

			// check that the exit code of the session matches the expected one
			if err != nil {
				var exitError *ssh.ExitError
				require.ErrorAs(t, trace.Unwrap(err), &exitError)
				require.Equal(t, tt.statusCode, exitError.ExitStatus())
			}
		})
	}
}

// testBPFSessionDifferentiation verifies that the bpf package can
// differentiate events from two different sessions. This test in turn also
// verifies the cgroup package.
func testBPFSessionDifferentiation(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	// Check if BPF tests can be run on this host.
	err := canTestBPF()
	if err != nil {
		t.Skip(fmt.Sprintf("Tests for BPF functionality can not be run: %v.", err))
		return
	}

	lsPath, err := exec.LookPath("ls")
	require.NoError(t, err)

	// Create temporary directory where cgroup2 hierarchy will be mounted.
	dir := t.TempDir()

	// Create and start a Teleport cluster.
	makeConfig := func() (*testing.T, []string, []*InstanceSecrets, *service.Config) {
		recConfig, err := types.NewSessionRecordingConfigFromConfigFile(types.SessionRecordingConfigSpecV2{
			Mode: types.RecordAtNode,
		})
		require.NoError(t, err)

		// Create default config.
		tconf := suite.defaultServiceConfig()

		// Configure Auth.
		tconf.Auth.Preference.SetSecondFactor("off")
		tconf.Auth.Enabled = true
		tconf.Auth.SessionRecordingConfig = recConfig

		// Configure Proxy.
		tconf.Proxy.Enabled = true
		tconf.Proxy.DisableWebService = false
		tconf.Proxy.DisableWebInterface = true

		// Configure Node. If session are being recorded at the proxy, don't enable
		// BPF to simulate an OpenSSH node.
		tconf.SSH.Enabled = true
		tconf.SSH.BPF.Enabled = true
		tconf.SSH.BPF.CgroupPath = dir
		return t, nil, nil, tconf
	}
	main := suite.newTeleportWithConfig(makeConfig())
	defer main.StopAll()

	// Create two client terminals and channel to signal when the clients are
	// done with the terminals.
	termA := NewTerminal(250)
	termB := NewTerminal(250)
	doneCh := make(chan bool, 2)

	// Open a terminal and type "ls" into both and exit.
	writeTerm := func(term *Terminal) {
		client, err := main.NewClient(t, ClientConfig{
			Login:   suite.me.Username,
			Cluster: Site,
			Host:    Host,
			Port:    main.GetPortSSHInt(),
		})
		require.NoError(t, err)

		// Connect terminal to std{in,out} of client.
		client.Stdout = term
		client.Stdin = term

		// "Type" a command into the terminal.
		term.Type(fmt.Sprintf("\a%v\n\r\aexit\n\r\a", lsPath))
		err = client.SSH(context.Background(), []string{}, false)
		require.NoError(t, err)

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
			dumpGoroutineProfile()
			require.FailNow(t, "Timed out waiting for client to finish interactive session.")
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
	require.Fail(t, "Failed to find command events from two different sessions.")
}

// testExecEvents tests if exec events were emitted with and without PTY allocated
func testExecEvents(t *testing.T, suite *integrationTestSuite) {
	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	lsPath, err := exec.LookPath("ls")
	require.NoError(t, err)

	// Creates new teleport cluster
	main := suite.newTeleport(t, nil, true)
	defer main.StopAll()

	execTests := []struct {
		name          string
		isInteractive bool
		outCommand    string
	}{
		{
			name:          "PTY allocated",
			isInteractive: true,
			outCommand:    lsPath,
		},
		{
			name:          "PTY not allocated",
			isInteractive: false,
			outCommand:    lsPath,
		},
	}

	for _, tt := range execTests {
		t.Run(tt.name, func(t *testing.T) {
			// Create client for each test in grid tests
			clientConfig := ClientConfig{
				Login:       suite.me.Username,
				Cluster:     Site,
				Host:        Host,
				Port:        main.GetPortSSHInt(),
				Interactive: tt.isInteractive,
			}
			_, err := runCommand(t, main, []string{lsPath}, clientConfig, 1)
			require.NoError(t, err)

			// Make sure the exec event was emitted to the audit log.
			eventFields, err := findEventInLog(main, events.ExecEvent)
			require.NoError(t, err)
			require.Equal(t, events.ExecCode, eventFields.GetCode())
			require.Equal(t, tt.outCommand, eventFields.GetString(events.ExecEventCommand))
		})
	}
}

func testSessionStartContainsAccessRequest(t *testing.T, suite *integrationTestSuite) {
	accessRequestsKey := "access_requests"
	requestedRoleName := "requested-role"
	userRoleName := "user-role"

	tr := utils.NewTracer(utils.ThisFunction()).Start()
	defer tr.Stop()

	lsPath, err := exec.LookPath("ls")
	require.NoError(t, err)

	// Creates new teleport cluster
	main := suite.newTeleport(t, nil, true)
	defer main.StopAll()

	ctx := context.Background()
	// Get auth server
	authServer := main.Process.GetAuthServer()

	// Create new request role
	requestedRole, err := types.NewRole(requestedRoleName, types.RoleSpecV4{
		Options: types.RoleOptions{},
		Allow:   types.RoleConditions{},
	})
	require.NoError(t, err)

	err = authServer.UpsertRole(ctx, requestedRole)
	require.NoError(t, err)

	// Create user role with ability to request role
	userRole, err := types.NewRole(userRoleName, types.RoleSpecV4{
		Options: types.RoleOptions{},
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				Roles: []string{requestedRoleName},
			},
		},
	})
	require.NoError(t, err)

	err = authServer.UpsertRole(ctx, userRole)
	require.NoError(t, err)

	user, err := types.NewUser(suite.me.Username)
	user.AddRole(userRole.GetName())
	require.NoError(t, err)

	watcher, err := authServer.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{
			{Kind: types.KindUser},
			{Kind: types.KindAccessRequest},
		},
	})
	require.NoError(t, err)
	defer watcher.Close()

	select {
	case <-time.After(time.Second * 30):
		t.Fatalf("Timeout waiting for event.")
	case event := <-watcher.Events():
		if event.Type != types.OpInit {
			t.Fatalf("Unexpected event type.")
		}
		require.Equal(t, event.Type, types.OpInit)
	case <-watcher.Done():
		t.Fatal(watcher.Error())
	}

	// Update user
	err = authServer.UpsertUser(user)
	require.NoError(t, err)

	WaitForResource(t, watcher, user.GetKind(), user.GetName())

	req, err := services.NewAccessRequest(suite.me.Username, requestedRole.GetMetadata().Name)
	require.NoError(t, err)

	accessRequestID := req.GetName()

	err = authServer.CreateAccessRequest(context.TODO(), req)
	require.NoError(t, err)

	err = authServer.SetAccessRequestState(context.TODO(), types.AccessRequestUpdate{
		RequestID: accessRequestID,
		State:     types.RequestState_APPROVED,
	})
	require.NoError(t, err)

	WaitForResource(t, watcher, req.GetKind(), req.GetName())

	clientConfig := ClientConfig{
		Login:       suite.me.Username,
		Cluster:     Site,
		Host:        Host,
		Port:        main.GetPortSSHInt(),
		Interactive: false,
	}
	clientReissueParams := client.ReissueParams{
		AccessRequests: []string{accessRequestID},
	}
	err = runCommandWithCertReissue(t, main, []string{lsPath}, clientReissueParams, client.CertCacheDrop, clientConfig)
	require.NoError(t, err)

	// Get session start event
	sessionStart, err := findEventInLog(main, events.SessionStartEvent)
	require.NoError(t, err)
	require.Equal(t, sessionStart.GetCode(), events.SessionStartCode)
	require.Equal(t, sessionStart.HasField(accessRequestsKey), true)

	val, found := sessionStart[accessRequestsKey]
	require.Equal(t, found, true)

	result := strings.Contains(fmt.Sprintf("%v", val), accessRequestID)
	require.Equal(t, result, true)
}

func WaitForResource(t *testing.T, watcher types.Watcher, kind, name string) {
	timeout := time.After(time.Second * 15)
	for {
		select {
		case <-timeout:
			t.Fatalf("Timeout waiting for event.")
		case event := <-watcher.Events():
			if event.Type != types.OpPut {
				continue
			}
			if event.Resource.GetKind() == kind && event.Resource.GetMetadata().Name == name {
				return
			}
		case <-watcher.Done():
			t.Fatalf("Watcher error %s.", watcher.Error())
		}
	}
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

// runCommandWithCertReissue runs an SSH command and generates certificates for the user
func runCommandWithCertReissue(t *testing.T, instance *TeleInstance, cmd []string, reissueParams client.ReissueParams, cachePolicy client.CertCachePolicy, cfg ClientConfig) error {
	tc, err := instance.NewClient(t, cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	err = tc.ReissueUserCerts(context.Background(), cachePolicy, reissueParams)
	if err != nil {
		return trace.Wrap(err)
	}

	out := &bytes.Buffer{}
	tc.Stdout = out

	err = tc.SSH(context.TODO(), cmd, false)
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// runCommand is a shortcut for running SSH command, it creates a client
// connected to proxy of the passed in instance, runs the command, and returns
// the result. If multiple attempts are requested, a 250 millisecond delay is
// added between them before giving up.
func runCommand(t *testing.T, instance *TeleInstance, cmd []string, cfg ClientConfig, attempts int) (string, error) {
	tc, err := instance.NewClient(t, cfg)
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

func (s *integrationTestSuite) newTeleportInstance() *TeleInstance {
	return NewInstance(s.defaultInstanceConfig())
}

func (s *integrationTestSuite) defaultInstanceConfig() InstanceConfig {
	return InstanceConfig{
		ClusterName: Site,
		HostID:      HostID,
		NodeName:    Host,
		Priv:        s.priv,
		Pub:         s.pub,
		log:         s.log,
		Ports:       standardPortSetup(),
	}
}

type InstanceConfigOption func(config *InstanceConfig)

func (s *integrationTestSuite) newNamedTeleportInstance(t *testing.T, clusterName string, opts ...InstanceConfigOption) *TeleInstance {
	cfg := InstanceConfig{
		ClusterName: clusterName,
		HostID:      HostID,
		NodeName:    Host,
		Priv:        s.priv,
		Pub:         s.pub,
		log:         utils.WrapLogger(s.log.WithField("cluster", clusterName)),
		Ports:       standardPortSetup(),
	}
	for _, opt := range opts {
		opt(&cfg)
	}
	return NewInstance(cfg)
}

func (s *integrationTestSuite) defaultServiceConfig() *service.Config {
	cfg := service.MakeDefaultConfig()
	cfg.Console = nil
	cfg.Log = s.log
	return cfg
}

// waitFor helper waits on a channel for up to the given timeout
func waitFor(c chan interface{}, timeout time.Duration) error {
	tick := time.Tick(timeout)
	select {
	case <-c:
		return nil
	case <-tick:
		return trace.LimitExceeded("timeout waiting for event")
	}
}

// waitForError helper waits on an error channel for up to the given timeout
func waitForError(c chan error, timeout time.Duration) error {
	tick := time.Tick(timeout)
	select {
	case err := <-c:
		return err
	case <-tick:
		return trace.LimitExceeded("timeout waiting for event")
	}
}

// hasPAMPolicy checks if the three policy files needed for tests exists. If
// they do it returns true, otherwise returns false.
func hasPAMPolicy() bool {
	pamPolicyFiles := []string{
		"/etc/pam.d/teleport-acct-failure",
		"/etc/pam.d/teleport-session-failure",
		"/etc/pam.d/teleport-success",
		"/etc/pam.d/teleport-custom-env",
	}

	for _, fileName := range pamPolicyFiles {
		_, err := os.Stat(fileName)
		if os.IsNotExist(err) {
			return false
		}
	}

	return true
}

// isRoot returns a boolean if the test is being run as root or not.
func isRoot() bool {
	return os.Geteuid() == 0
}

// canTestBPF runs checks to determine whether BPF tests will run or not.
// Tests for this package must be run as root.
func canTestBPF() error {
	if !isRoot() {
		return trace.BadParameter("not root")
	}

	err := bpf.IsHostCompatible()
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func dumpGoroutineProfile() {
	pprof.Lookup("goroutine").WriteTo(os.Stderr, 2)
}

// TestWebProxyInsecure makes sure that proxy endpoint works when TLS is disabled.
func TestWebProxyInsecure(t *testing.T) {
	privateKey, publicKey, err := testauthority.New().GenerateKeyPair("")
	require.NoError(t, err)

	rc := NewInstance(InstanceConfig{
		ClusterName: "example.com",
		HostID:      uuid.New().String(),
		NodeName:    Host,
		Priv:        privateKey,
		Pub:         publicKey,
		log:         utils.NewLoggerForTests(),
	})

	rcConf := service.MakeDefaultConfig()
	rcConf.DataDir = t.TempDir()
	rcConf.Auth.Enabled = true
	rcConf.Auth.Preference.SetSecondFactor("off")
	rcConf.Proxy.Enabled = true
	rcConf.Proxy.DisableWebInterface = true
	// DisableTLS flag should turn off TLS termination and multiplexing.
	rcConf.Proxy.DisableTLS = true

	err = rc.CreateEx(t, nil, rcConf)
	require.NoError(t, err)

	err = rc.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		rc.StopAll()
	})

	// Web proxy endpoint should just respond with 200 when called over http://,
	// content doesn't matter.
	resp, err := http.Get(fmt.Sprintf("http://%v/webapi/ping", net.JoinHostPort(Loopback, rc.GetPortWeb())))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NoError(t, resp.Body.Close())
}

// TestTraitsPropagation makes sure that user traits are applied properly to
// roles in root and leaf clusters.
func TestTraitsPropagation(t *testing.T) {
	log := utils.NewLoggerForTests()

	privateKey, publicKey, err := testauthority.New().GenerateKeyPair("")
	require.NoError(t, err)

	// Create root cluster.
	rc := NewInstance(InstanceConfig{
		ClusterName: "root.example.com",
		HostID:      uuid.New().String(),
		NodeName:    Host,
		Priv:        privateKey,
		Pub:         publicKey,
		log:         log,
	})

	// Create leaf cluster.
	lc := NewInstance(InstanceConfig{
		ClusterName: "leaf.example.com",
		HostID:      uuid.New().String(),
		NodeName:    Host,
		Priv:        privateKey,
		Pub:         publicKey,
		log:         log,
	})

	// Make root cluster config.
	rcConf := service.MakeDefaultConfig()
	rcConf.DataDir = t.TempDir()
	rcConf.Auth.Enabled = true
	rcConf.Auth.Preference.SetSecondFactor("off")
	rcConf.Proxy.Enabled = true
	rcConf.Proxy.DisableWebService = true
	rcConf.Proxy.DisableWebInterface = true
	rcConf.SSH.Enabled = true
	rcConf.SSH.Addr.Addr = net.JoinHostPort(rc.Hostname, rc.GetPortSSH())
	rcConf.SSH.Labels = map[string]string{"env": "integration"}

	// Make leaf cluster config.
	lcConf := service.MakeDefaultConfig()
	lcConf.DataDir = t.TempDir()
	lcConf.Auth.Enabled = true
	lcConf.Auth.Preference.SetSecondFactor("off")
	lcConf.Proxy.Enabled = true
	lcConf.Proxy.DisableWebInterface = true
	lcConf.SSH.Enabled = true
	lcConf.SSH.Addr.Addr = net.JoinHostPort(lc.Hostname, lc.GetPortSSH())
	lcConf.SSH.Labels = map[string]string{"env": "integration"}

	// Create identical user/role in both clusters.
	me, err := user.Current()
	require.NoError(t, err)

	role := services.NewImplicitRole()
	role.SetName("test")
	role.SetLogins(types.Allow, []string{me.Username})
	// Users created by CreateEx have "testing: integration" trait.
	role.SetNodeLabels(types.Allow, map[string]apiutils.Strings{"env": []string{"{{external.testing}}"}})

	rc.AddUserWithRole(me.Username, role)
	lc.AddUserWithRole(me.Username, role)

	// Establish trust b/w root and leaf.
	err = rc.CreateEx(t, lc.Secrets.AsSlice(), rcConf)
	require.NoError(t, err)
	err = lc.CreateEx(t, rc.Secrets.AsSlice(), lcConf)
	require.NoError(t, err)

	// Start both clusters.
	require.NoError(t, rc.Start())
	t.Cleanup(func() {
		rc.StopAll()
	})
	require.NoError(t, lc.Start())
	t.Cleanup(func() {
		lc.StopAll()
	})

	// Update root's certificate authority on leaf to configure role mapping.
	ca, err := lc.Process.GetAuthServer().GetCertAuthority(types.CertAuthID{
		Type:       types.UserCA,
		DomainName: rc.Secrets.SiteName,
	}, false)
	require.NoError(t, err)
	ca.SetRoles(nil) // Reset roles, otherwise they will take precedence.
	ca.SetRoleMap(types.RoleMap{{Remote: role.GetName(), Local: []string{role.GetName()}}})
	err = lc.Process.GetAuthServer().UpsertCertAuthority(ca)
	require.NoError(t, err)

	// Run command in root.
	outputRoot, err := runCommand(t, rc, []string{"echo", "hello root"}, ClientConfig{
		Login:   me.Username,
		Cluster: "root.example.com",
		Host:    Loopback,
		Port:    rc.GetPortSSHInt(),
	}, 1)
	require.NoError(t, err)
	require.Equal(t, "hello root", strings.TrimSpace(outputRoot))

	// Run command in leaf.
	outputLeaf, err := runCommand(t, rc, []string{"echo", "hello leaf"}, ClientConfig{
		Login:   me.Username,
		Cluster: "leaf.example.com",
		Host:    Loopback,
		Port:    lc.GetPortSSHInt(),
	}, 1)
	require.NoError(t, err)
	require.Equal(t, "hello leaf", strings.TrimSpace(outputLeaf))
}

// testSessionStreaming tests streaming events from session recordings.
func testSessionStreaming(t *testing.T, suite *integrationTestSuite) {
	ctx := context.Background()
	sessionID := session.ID(uuid.New().String())
	teleport := suite.newTeleport(t, nil, true)
	defer teleport.StopAll()

	api := teleport.GetSiteAPI(Site)
	uploadStream, err := api.CreateAuditStream(ctx, sessionID)
	require.Nil(t, err)

	generatedSession := events.GenerateTestSession(events.SessionParams{
		PrintEvents: 100,
		SessionID:   string(sessionID),
		ServerID:    "00000000-0000-0000-0000-000000000000",
	})

	for _, event := range generatedSession {
		err := uploadStream.EmitAuditEvent(ctx, event)
		require.NoError(t, err)
	}

	err = uploadStream.Complete(ctx)
	require.Nil(t, err)
	start := time.Now()

	// retry in case of error
outer:
	for time.Since(start) < time.Minute*5 {
		time.Sleep(time.Second * 5)

		receivedSession := make([]apievents.AuditEvent, 0)
		sessionPlayback, e := api.StreamSessionEvents(ctx, sessionID, 0)

	inner:
		for {
			select {
			case event, more := <-sessionPlayback:
				if !more {
					break inner
				}

				receivedSession = append(receivedSession, event)
			case <-ctx.Done():
				require.Nil(t, ctx.Err())
			case err := <-e:
				require.Nil(t, err)
			case <-time.After(time.Minute * 5):
				t.FailNow()
			}
		}

		for i := range generatedSession {
			receivedSession[i].SetClusterName("")
			if !reflect.DeepEqual(generatedSession[i], receivedSession[i]) {
				continue outer
			}
		}

		return
	}

	t.FailNow()
}
