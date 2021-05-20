/*
Copyright 2015-2020 Gravitational, Inc.

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

package regular

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/user"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/bpf"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/pam"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	sess "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	"github.com/jonboulle/clockwork"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
	. "gopkg.in/check.v1"
)

func TestRegular(t *testing.T) { TestingT(t) }

type SrvSuite struct {
	srv         *Server
	srvAddress  string
	srvPort     string
	srvHostPort string
	clt         *ssh.Client
	cltConfig   *ssh.ClientConfig
	up          *upack
	signer      ssh.Signer
	user        string
	proxyClient *auth.Client
	proxyID     string
	nodeClient  *auth.Client
	nodeID      string
	adminClient *auth.Client
	clock       clockwork.FakeClock
	server      *auth.TestServer
}

// teleportTestUser is additional user used for tests
const teleportTestUser = "teleport-test"

// wildcardAllow is used in tests to allow access to all labels.
var wildcardAllow = types.Labels{
	types.Wildcard: []string{types.Wildcard},
}

var _ = Suite(&SrvSuite{})

// TestMain will re-execute Teleport to run a command if "exec" is passed to
// it as an argument. Otherwise it will run tests as normal.
func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	if len(os.Args) == 2 &&
		(os.Args[1] == teleport.ExecSubCommand || os.Args[1] == teleport.ForwardSubCommand) {
		srv.RunAndExit(os.Args[1])
		return
	}

	code := m.Run()
	os.Exit(code)
}

const hostID = "00000000-0000-0000-0000-000000000000"

func (s *SrvSuite) SetUpTest(c *C) {
	u, err := user.Current()
	c.Assert(err, IsNil)
	s.user = u.Username

	s.clock = clockwork.NewFakeClock()

	s.server, err = auth.NewTestServer(auth.TestServerConfig{
		Auth: auth.TestAuthServerConfig{
			ClusterName: "localhost",
			Dir:         c.MkDir(),
			Clock:       s.clock,
		},
	})
	c.Assert(err, IsNil)

	// create proxy client used in some tests
	s.proxyID = uuid.New()
	s.proxyClient, err = s.server.NewClient(auth.TestIdentity{
		I: auth.BuiltinRole{
			Role:     teleport.RoleProxy,
			Username: s.proxyID,
		},
	})
	c.Assert(err, IsNil)

	// admin client is for admin actions, e.g. creating new users
	s.adminClient, err = s.server.NewClient(auth.TestBuiltin(teleport.RoleAdmin))
	c.Assert(err, IsNil)

	// set up SSH client using the user private key for signing
	up, err := s.newUpack(s.user, []string{s.user}, wildcardAllow)
	c.Assert(err, IsNil)

	// set up host private key and certificate
	certs, err := s.server.Auth().GenerateServerKeys(auth.GenerateServerKeysRequest{
		HostID:   hostID,
		NodeName: s.server.ClusterName(),
		Roles:    teleport.Roles{teleport.RoleNode},
	})
	c.Assert(err, IsNil)

	// set up user CA and set up a user that has access to the server
	s.signer, err = sshutils.NewSigner(certs.Key, certs.Cert)
	c.Assert(err, IsNil)

	s.nodeID = uuid.New()
	s.nodeClient, err = s.server.NewClient(auth.TestIdentity{
		I: auth.BuiltinRole{
			Role:     teleport.RoleNode,
			Username: s.nodeID,
		},
	})
	c.Assert(err, IsNil)

	nodeDir := c.MkDir()
	srv, err := New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"},
		s.server.ClusterName(),
		[]ssh.Signer{s.signer},
		s.nodeClient,
		nodeDir,
		"",
		utils.NetAddr{},
		SetUUID(s.nodeID),
		SetNamespace(defaults.Namespace),
		SetEmitter(s.nodeClient),
		SetShell("/bin/sh"),
		SetSessionServer(s.nodeClient),
		SetPAMConfig(&pam.Config{Enabled: false}),
		SetLabels(
			map[string]string{"foo": "bar"},
			services.CommandLabels{
				"baz": &types.CommandLabelV2{
					Period:  types.NewDuration(time.Millisecond),
					Command: []string{"expr", "1", "+", "3"}},
			},
		),
		SetBPF(&bpf.NOP{}),
		SetClock(s.clock),
	)
	c.Assert(err, IsNil)
	s.srv = srv
	s.srv.isTestStub = true
	c.Assert(auth.CreateUploaderDir(nodeDir), IsNil)
	c.Assert(s.srv.Start(), IsNil)
	c.Assert(s.srv.heartbeat.ForceSend(time.Second), IsNil)

	s.srvAddress = s.srv.Addr()
	_, s.srvPort, err = net.SplitHostPort(s.srvAddress)
	c.Assert(err, IsNil)
	s.srvHostPort = fmt.Sprintf("%v:%v", s.server.ClusterName(), s.srvPort)

	// set up an agent server and a client that uses agent for forwarding
	keyring := agent.NewKeyring()
	addedKey := agent.AddedKey{
		PrivateKey:  up.pkey,
		Certificate: up.pcert,
	}
	c.Assert(keyring.Add(addedKey), IsNil)
	s.up = up
	s.cltConfig = &ssh.ClientConfig{
		User:            s.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(s.signer.PublicKey()),
	}

	client, err := ssh.Dial("tcp", s.srv.Addr(), s.cltConfig)
	c.Assert(err, IsNil)
	c.Assert(agent.ForwardToAgent(client, keyring), IsNil)
	s.clt = client
}

func (s *SrvSuite) TearDownTest(c *C) {
	if s.clt != nil {
		c.Assert(s.clt.Close(), IsNil)
	}
	if s.srv != nil {
		c.Assert(s.srv.Close(), IsNil)
	}
	if s.server != nil {
		c.Assert(s.server.Shutdown(context.Background()), IsNil)
	}
}

// TestDirectTCPIP ensures that the server can create a "direct-tcpip"
// channel to the target address. The "direct-tcpip" channel is what port
// forwarding is built upon.
func (s *SrvSuite) TestDirectTCPIP(c *C) {
	// Startup a test server that will reply with "hello, world\n"
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "hello, world")
	}))
	defer ts.Close()

	// Extract the host:port the test HTTP server is running on.
	u, err := url.Parse(ts.URL)
	c.Assert(err, IsNil)

	// Build a http.Client that will dial through the server to establish the
	// connection. That's why a custom dialer is used and the dialer uses
	// s.clt.Dial (which performs the "direct-tcpip" request).
	httpClient := http.Client{
		Transport: &http.Transport{
			Dial: func(network string, addr string) (net.Conn, error) {
				return s.clt.Dial("tcp", u.Host)
			},
		},
	}

	// Perform a HTTP GET to the test HTTP server through a "direct-tcpip" request.
	resp, err := httpClient.Get(ts.URL)
	c.Assert(err, IsNil)
	defer resp.Body.Close()

	// Make sure the response is what was expected.
	body, err := ioutil.ReadAll(resp.Body)
	c.Assert(err, IsNil)
	c.Assert(body, DeepEquals, []byte("hello, world\n"))
}

func (s *SrvSuite) TestAdvertiseAddr(c *C) {
	// No advertiseAddr was set in SetUpTest, should default to srvAddress.
	c.Assert(s.srv.AdvertiseAddr(), Equals, s.srvAddress)

	var (
		advIP      = utils.MustParseAddr("10.10.10.1")
		advIPPort  = utils.MustParseAddr("10.10.10.1:1234")
		advBadAddr = &utils.NetAddr{Addr: "localhost:badport", AddrNetwork: "tcp"}
	)
	// IP-only advertiseAddr should use the port from srvAddress.
	s.srv.setAdvertiseAddr(advIP)
	c.Assert(s.srv.AdvertiseAddr(), Equals, fmt.Sprintf("%s:%s", advIP, s.srvPort))

	// IP and port advertiseAddr should fully override srvAddress.
	s.srv.setAdvertiseAddr(advIPPort)
	c.Assert(s.srv.AdvertiseAddr(), Equals, advIPPort.String())

	// nil advertiseAddr should default to srvAddress.
	s.srv.setAdvertiseAddr(nil)
	c.Assert(s.srv.AdvertiseAddr(), Equals, s.srvAddress)

	// Invalid advertiseAddr should fall back to srvAddress.
	s.srv.setAdvertiseAddr(advBadAddr)
	c.Assert(s.srv.AdvertiseAddr(), Equals, s.srvAddress)
}

// TestAgentForwardPermission makes sure if RBAC rules don't allow agent
// forwarding, we don't start an agent even if requested.
func (s *SrvSuite) TestAgentForwardPermission(c *C) {
	ctx := context.Background()
	// make sure the role does not allow agent forwarding
	roleName := services.RoleNameForUser(s.user)
	role, err := s.server.Auth().GetRole(ctx, roleName)
	c.Assert(err, IsNil)
	roleOptions := role.GetOptions()
	roleOptions.ForwardAgent = types.NewBool(false)
	role.SetOptions(roleOptions)
	err = s.server.Auth().UpsertRole(ctx, role)
	c.Assert(err, IsNil)

	se, err := s.clt.NewSession()
	c.Assert(err, IsNil)
	defer se.Close()

	// to interoperate with OpenSSH, requests for agent forwarding always succeed.
	// however that does not mean the users agent will actually be forwarded.
	err = agent.RequestAgentForwarding(se)
	c.Assert(err, IsNil)

	// the output of env, we should not see SSH_AUTH_SOCK in the output
	output, err := se.Output("env")
	c.Assert(err, IsNil)
	c.Assert(strings.Contains(string(output), "SSH_AUTH_SOCK"), Equals, false)
}

// TestMaxSesssions makes sure that MaxSessions RBAC rules prevent
// too many concurrent sessions.
func (s *SrvSuite) TestMaxSessions(c *C) {
	const maxSessions int64 = 2
	ctx := context.Background()
	// make sure the role does not allow agent forwarding
	roleName := services.RoleNameForUser(s.user)
	role, err := s.server.Auth().GetRole(ctx, roleName)
	c.Assert(err, IsNil)
	roleOptions := role.GetOptions()
	roleOptions.MaxSessions = maxSessions
	role.SetOptions(roleOptions)
	err = s.server.Auth().UpsertRole(ctx, role)
	c.Assert(err, IsNil)

	for i := int64(0); i < maxSessions; i++ {
		se, err := s.clt.NewSession()
		c.Assert(err, IsNil)
		defer se.Close()
	}

	_, err = s.clt.NewSession()
	c.Assert(err, NotNil)
	c.Assert(strings.Contains(err.Error(), "too many session channels"), Equals, true)

	// verfiy that max sessions does not affect max connections.
	for i := int64(0); i <= maxSessions; i++ {
		clt, err := ssh.Dial("tcp", s.srv.Addr(), s.cltConfig)
		c.Assert(err, IsNil)
		c.Assert(clt.Close(), IsNil)
	}
}

// TestOpenExecSessionSetsSession tests that OpenExecSession()
// sets ServerContext session.
func (s *SrvSuite) TestOpenExecSessionSetsSession(c *C) {
	se, err := s.clt.NewSession()
	c.Assert(err, IsNil)
	defer se.Close()

	// This will trigger an exec request, which will start a non-interactive session,
	// which then triggers setting env for SSH_SESSION_ID.
	output, err := se.Output("env")
	c.Assert(err, IsNil)
	c.Assert(strings.Contains(string(output), teleport.SSHSessionID), Equals, true)
}

// TestAgentForward tests agent forwarding via unix sockets
func (s *SrvSuite) TestAgentForward(c *C) {
	ctx := context.Background()
	roleName := services.RoleNameForUser(s.user)
	role, err := s.server.Auth().GetRole(ctx, roleName)
	c.Assert(err, IsNil)
	roleOptions := role.GetOptions()
	roleOptions.ForwardAgent = types.NewBool(true)
	role.SetOptions(roleOptions)
	err = s.server.Auth().UpsertRole(ctx, role)
	c.Assert(err, IsNil)

	se, err := s.clt.NewSession()
	c.Assert(err, IsNil)
	defer se.Close()

	err = agent.RequestAgentForwarding(se)
	c.Assert(err, IsNil)

	// prepare to send virtual "keyboard input" into the shell:
	keyboard, err := se.StdinPipe()
	c.Assert(err, IsNil)

	// start interactive SSH session (new shell):
	err = se.Shell()
	c.Assert(err, IsNil)

	// create a temp file to collect the shell output into:
	tmpFile, err := ioutil.TempFile(os.TempDir(), "teleport-agent-forward-test")
	c.Assert(err, IsNil)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	// type 'printenv SSH_AUTH_SOCK > /path/to/tmp/file' into the session (dumping the value of SSH_AUTH_STOCK into the temp file)
	_, err = keyboard.Write([]byte(fmt.Sprintf("printenv %v > %s\n\r", teleport.SSHAuthSock, tmpFile.Name())))
	c.Assert(err, IsNil)

	// wait for the output
	var output []byte
	for i := 0; i < 100 && len(output) == 0; i++ {
		time.Sleep(10 * time.Millisecond)
		output, _ = ioutil.ReadFile(tmpFile.Name())
	}
	c.Assert(output, Not(Equals), "")
	socketPath := strings.TrimSpace(string(output))

	// try dialing the ssh agent socket:
	file, err := net.Dial("unix", socketPath)
	c.Assert(err, IsNil)
	clientAgent := agent.NewClient(file)

	signers, err := clientAgent.Signers()
	c.Assert(err, IsNil)

	sshConfig := &ssh.ClientConfig{
		User:            s.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(signers...)},
		HostKeyCallback: ssh.FixedHostKey(s.signer.PublicKey()),
	}

	client, err := ssh.Dial("tcp", s.srv.Addr(), sshConfig)
	c.Assert(err, IsNil)
	err = client.Close()
	c.Assert(err, IsNil)

	// make sure the socket persists after the session is closed.
	// (agents are started from specific sessions, but apply to all
	// sessions on the connection).
	err = se.Close()
	c.Assert(err, IsNil)
	// Pause to allow closure to propagate.
	time.Sleep(150 * time.Millisecond)
	_, err = net.Dial("unix", socketPath)
	c.Assert(err, IsNil)

	// make sure the socket is gone after we closed the connection.
	err = s.clt.Close()
	c.Assert(err, IsNil)
	// clt must be nullified to prevent double-close during test cleanup
	s.clt = nil
	for i := 0; i < 4; i++ {
		_, err = net.Dial("unix", socketPath)
		if err != nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	c.Fatalf("expected socket to be closed, still could dial after 150 ms")
}

func (s *SrvSuite) TestAllowedUsers(c *C) {
	up, err := s.newUpack(s.user, []string{s.user}, wildcardAllow)
	c.Assert(err, IsNil)

	sshConfig := &ssh.ClientConfig{
		User:            s.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(s.signer.PublicKey()),
	}

	client, err := ssh.Dial("tcp", s.srv.Addr(), sshConfig)
	c.Assert(err, IsNil)
	c.Assert(client.Close(), IsNil)

	// now remove OS user from valid principals
	up, err = s.newUpack(s.user, []string{"otheruser"}, wildcardAllow)
	c.Assert(err, IsNil)

	sshConfig = &ssh.ClientConfig{
		User:            s.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(s.signer.PublicKey()),
	}

	_, err = ssh.Dial("tcp", s.srv.Addr(), sshConfig)
	c.Assert(err, NotNil)
}

func (s *SrvSuite) TestAllowedLabels(c *C) {
	var tests = []struct {
		inLabelMap types.Labels
		outError   bool
	}{
		// Valid static label.
		{
			inLabelMap: types.Labels{"foo": []string{"bar"}},
			outError:   false,
		},
		// Invalid static label.
		{
			inLabelMap: types.Labels{"foo": []string{"baz"}},
			outError:   true,
		},
		// Valid dynamic label.
		{
			inLabelMap: types.Labels{"baz": []string{"4"}},
			outError:   false,
		},
		// Invalid dynamic label.
		{
			inLabelMap: types.Labels{"baz": []string{"5"}},
			outError:   true,
		},
	}

	for _, tt := range tests {
		up, err := s.newUpack(s.user, []string{s.user}, tt.inLabelMap)
		c.Assert(err, IsNil)

		sshConfig := &ssh.ClientConfig{
			User:            s.user,
			Auth:            []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
			HostKeyCallback: ssh.FixedHostKey(s.signer.PublicKey()),
		}

		_, err = ssh.Dial("tcp", s.srv.Addr(), sshConfig)
		if tt.outError {
			c.Assert(err, NotNil)
		} else {
			c.Assert(err, IsNil)
		}
	}
}

// TestKeyAlgorithms makes sure Teleport does not accept invalid user
// certificates. The main check is the certificate algorithms.
func (s *SrvSuite) TestKeyAlgorithms(c *C) {
	_, ellipticSigner, err := utils.CreateEllipticCertificate("foo", ssh.UserCert)
	c.Assert(err, IsNil)

	sshConfig := &ssh.ClientConfig{
		User:            s.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(ellipticSigner)},
		HostKeyCallback: ssh.FixedHostKey(s.signer.PublicKey()),
	}

	_, err = ssh.Dial("tcp", s.srv.Addr(), sshConfig)
	c.Assert(err, NotNil)
}

func (s *SrvSuite) TestInvalidSessionID(c *C) {
	session, err := s.clt.NewSession()
	c.Assert(err, IsNil)

	err = session.Setenv(sshutils.SessionEnvVar, "foo")
	c.Assert(err, IsNil)

	err = session.Shell()
	c.Assert(err, NotNil)
}

func (s *SrvSuite) TestSessionHijack(c *C) {
	_, err := user.Lookup(teleportTestUser)
	if err != nil {
		c.Skip(fmt.Sprintf("user %v is not found, skipping test", teleportTestUser))
	}

	// user 1 has access to the server
	up, err := s.newUpack(s.user, []string{s.user}, wildcardAllow)
	c.Assert(err, IsNil)

	// login with first user
	sshConfig := &ssh.ClientConfig{
		User:            s.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(s.signer.PublicKey()),
	}

	client, err := ssh.Dial("tcp", s.srv.Addr(), sshConfig)
	c.Assert(err, IsNil)
	defer func() {
		err := client.Close()
		c.Assert(err, IsNil)
	}()

	se, err := client.NewSession()
	c.Assert(err, IsNil)
	defer se.Close()

	firstSessionID := string(sess.NewID())
	err = se.Setenv(sshutils.SessionEnvVar, firstSessionID)
	c.Assert(err, IsNil)

	err = se.Shell()
	c.Assert(err, IsNil)

	// user 2 does not have s.user as a listed principal
	up2, err := s.newUpack(teleportTestUser, []string{teleportTestUser}, wildcardAllow)
	c.Assert(err, IsNil)

	sshConfig2 := &ssh.ClientConfig{
		User:            teleportTestUser,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(up2.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(s.signer.PublicKey()),
	}

	client2, err := ssh.Dial("tcp", s.srv.Addr(), sshConfig2)
	c.Assert(err, IsNil)
	defer func() {
		err := client2.Close()
		c.Assert(err, IsNil)
	}()

	se2, err := client2.NewSession()
	c.Assert(err, IsNil)
	defer se2.Close()

	err = se2.Setenv(sshutils.SessionEnvVar, firstSessionID)
	c.Assert(err, IsNil)

	// attempt to hijack, should return error
	err = se2.Shell()
	c.Assert(err, NotNil)
}

// testClient dials targetAddr via proxyAddr and executes 2+3 command
func (s *SrvSuite) testClient(c *C, proxyAddr, targetAddr, remoteAddr string, sshConfig *ssh.ClientConfig) {
	// Connect to node using registered address
	client, err := ssh.Dial("tcp", proxyAddr, sshConfig)
	c.Assert(err, IsNil)
	defer client.Close()

	se, err := client.NewSession()
	c.Assert(err, IsNil)
	defer se.Close()

	writer, err := se.StdinPipe()
	c.Assert(err, IsNil)

	reader, err := se.StdoutPipe()
	c.Assert(err, IsNil)

	// Request opening TCP connection to the remote host
	c.Assert(se.RequestSubsystem(fmt.Sprintf("proxy:%v", targetAddr)), IsNil)

	local, err := utils.ParseAddr("tcp://" + proxyAddr)
	c.Assert(err, IsNil)
	remote, err := utils.ParseAddr("tcp://" + remoteAddr)
	c.Assert(err, IsNil)

	pipeNetConn := utils.NewPipeNetConn(
		reader,
		writer,
		se,
		local,
		remote,
	)

	defer pipeNetConn.Close()

	// Open SSH connection via TCP
	conn, chans, reqs, err := ssh.NewClientConn(pipeNetConn,
		s.srv.Addr(), sshConfig)
	c.Assert(err, IsNil)
	defer conn.Close()

	// using this connection as regular SSH
	client2 := ssh.NewClient(conn, chans, reqs)
	c.Assert(err, IsNil)
	defer client2.Close()

	se2, err := client2.NewSession()
	c.Assert(err, IsNil)
	defer se2.Close()

	out, err := se2.Output("echo hello")
	c.Assert(err, IsNil)

	c.Assert(string(out), Equals, "hello\n")
}

func (s *SrvSuite) mustListen(c *C) (net.Listener, utils.NetAddr) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	c.Assert(err, IsNil)
	addr := utils.NetAddr{AddrNetwork: "tcp", Addr: l.Addr().String()}
	return l, addr
}

func (s *SrvSuite) TestProxyReverseTunnel(c *C) {
	log.Infof("[TEST START] TestProxyReverseTunnel")

	// Create host key and certificate for proxy.
	proxyKeys, err := s.server.Auth().GenerateServerKeys(auth.GenerateServerKeysRequest{
		HostID:   hostID,
		NodeName: s.server.ClusterName(),
		Roles:    teleport.Roles{teleport.RoleProxy},
	})
	c.Assert(err, IsNil)
	proxySigner, err := sshutils.NewSigner(proxyKeys.Key, proxyKeys.Cert)
	c.Assert(err, IsNil)

	logger := logrus.WithField("test", "TestProxyReverseTunnel")
	listener, reverseTunnelAddress := s.mustListen(c)
	defer listener.Close()
	reverseTunnelServer, err := reversetunnel.NewServer(reversetunnel.Config{
		ClientTLS:                     s.proxyClient.TLSConfig(),
		ID:                            hostID,
		ClusterName:                   s.server.ClusterName(),
		Listener:                      listener,
		HostSigners:                   []ssh.Signer{proxySigner},
		LocalAuthClient:               s.proxyClient,
		LocalAccessPoint:              s.proxyClient,
		NewCachingAccessPoint:         auth.NoCache,
		NewCachingAccessPointOldProxy: auth.NoCache,
		DirectClusters:                []reversetunnel.DirectCluster{{Name: s.server.ClusterName(), Client: s.proxyClient}},
		DataDir:                       c.MkDir(),
		Component:                     teleport.ComponentProxy,
		Emitter:                       s.proxyClient,
		Log:                           logger,
	})
	c.Assert(err, IsNil)
	c.Assert(reverseTunnelServer.Start(), IsNil)
	defer reverseTunnelServer.Close()

	proxy, err := New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		s.server.ClusterName(),
		[]ssh.Signer{s.signer},
		s.proxyClient,
		c.MkDir(),
		"",
		utils.NetAddr{},
		SetUUID(s.proxyID),
		SetProxyMode(reverseTunnelServer),
		SetSessionServer(s.proxyClient),
		SetEmitter(s.nodeClient),
		SetNamespace(defaults.Namespace),
		SetPAMConfig(&pam.Config{Enabled: false}),
		SetBPF(&bpf.NOP{}),
		SetClock(s.clock),
	)
	c.Assert(err, IsNil)
	c.Assert(proxy.Start(), IsNil)

	// set up SSH client using the user private key for signing
	up, err := s.newUpack(s.user, []string{s.user}, wildcardAllow)
	c.Assert(err, IsNil)

	rcWatcher, err := reversetunnel.NewRemoteClusterTunnelManager(reversetunnel.RemoteClusterTunnelManagerConfig{
		AuthClient:          s.proxyClient,
		HostSigner:          proxySigner,
		HostUUID:            fmt.Sprintf("%v.%v", hostID, s.server.ClusterName()),
		AccessPoint:         s.proxyClient,
		ReverseTunnelServer: reverseTunnelServer,
		LocalCluster:        s.server.ClusterName(),
	})
	c.Assert(err, IsNil)

	ctx := context.Background()
	go rcWatcher.Run(ctx)
	defer rcWatcher.Close()

	// Create a reverse tunnel and remote cluster simulating what the trusted
	// cluster exchange does.
	err = s.server.Auth().UpsertReverseTunnel(
		types.NewReverseTunnel(s.server.ClusterName(), []string{reverseTunnelAddress.String()}))
	c.Assert(err, IsNil)
	remoteCluster, err := types.NewRemoteCluster("localhost")
	c.Assert(err, IsNil)
	err = s.server.Auth().CreateRemoteCluster(remoteCluster)
	c.Assert(err, IsNil)

	err = rcWatcher.Sync(ctx)
	c.Assert(err, IsNil)

	// Wait for both sites to show up.
	err = waitForSites(reverseTunnelServer, 2)
	c.Assert(err, IsNil)

	sshConfig := &ssh.ClientConfig{
		User:            s.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(s.signer.PublicKey()),
	}

	_, err = s.newUpack("user1", []string{s.user}, wildcardAllow)
	c.Assert(err, IsNil)

	s.testClient(c, proxy.Addr(), s.srvAddress, s.srv.Addr(), sshConfig)
	s.testClient(c, proxy.Addr(), s.srvHostPort, s.srv.Addr(), sshConfig)

	// adding new node
	srv2, err := New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"},
		"bob",
		[]ssh.Signer{s.signer},
		s.nodeClient,
		c.MkDir(),
		"",
		utils.NetAddr{},
		SetShell("/bin/sh"),
		SetLabels(
			map[string]string{"label1": "value1"},
			services.CommandLabels{
				"cmdLabel1": &types.CommandLabelV2{
					Period:  types.NewDuration(time.Millisecond),
					Command: []string{"expr", "1", "+", "3"}},
				"cmdLabel2": &types.CommandLabelV2{
					Period:  types.NewDuration(time.Second * 2),
					Command: []string{"expr", "2", "+", "3"}},
			},
		),
		SetSessionServer(s.nodeClient),
		SetNamespace(defaults.Namespace),
		SetPAMConfig(&pam.Config{Enabled: false}),
		SetBPF(&bpf.NOP{}),
		SetEmitter(s.nodeClient),
		SetClock(s.clock),
	)
	c.Assert(err, IsNil)
	c.Assert(srv2.Start(), IsNil)
	c.Assert(srv2.heartbeat.ForceSend(time.Second), IsNil)
	defer srv2.Close()
	// test proxysites
	client, err := ssh.Dial("tcp", proxy.Addr(), sshConfig)
	c.Assert(err, IsNil)

	se3, err := client.NewSession()
	c.Assert(err, IsNil)
	defer se3.Close()

	stdout := &bytes.Buffer{}
	reader, err := se3.StdoutPipe()
	c.Assert(err, IsNil)
	done := make(chan struct{})
	go func() {
		_, err := io.Copy(stdout, reader)
		c.Assert(err, IsNil)
		close(done)
	}()

	// to make sure  labels have the right output
	s.srv.syncUpdateLabels()
	srv2.syncUpdateLabels()
	c.Assert(s.srv.heartbeat.ForceSend(time.Second), IsNil)
	c.Assert(srv2.heartbeat.ForceSend(time.Second), IsNil)
	// request "list of sites":
	c.Assert(se3.RequestSubsystem("proxysites"), IsNil)
	<-done
	var sites []types.Site
	c.Assert(json.Unmarshal(stdout.Bytes(), &sites), IsNil)
	c.Assert(sites, NotNil)
	c.Assert(sites, HasLen, 2)
	c.Assert(sites[0].Name, Equals, "localhost")
	c.Assert(sites[0].Status, Equals, "online")
	c.Assert(sites[1].Name, Equals, "localhost")
	c.Assert(sites[1].Status, Equals, "online")

	c.Assert(time.Since(sites[0].LastConnected).Seconds() < 5, Equals, true)
	c.Assert(time.Since(sites[1].LastConnected).Seconds() < 5, Equals, true)

	err = s.server.Auth().DeleteReverseTunnel(s.server.ClusterName())
	c.Assert(err, IsNil)

	err = rcWatcher.Sync(ctx)
	c.Assert(err, IsNil)
}

func (s *SrvSuite) TestProxyRoundRobin(c *C) {
	log.Infof("[TEST START] TestProxyRoundRobin")

	logger := logrus.WithField("test", "TestProxyRoundRobin")
	listener, reverseTunnelAddress := s.mustListen(c)
	reverseTunnelServer, err := reversetunnel.NewServer(reversetunnel.Config{
		ClusterName:                   s.server.ClusterName(),
		ClientTLS:                     s.proxyClient.TLSConfig(),
		ID:                            hostID,
		Listener:                      listener,
		HostSigners:                   []ssh.Signer{s.signer},
		LocalAuthClient:               s.proxyClient,
		LocalAccessPoint:              s.proxyClient,
		NewCachingAccessPoint:         auth.NoCache,
		NewCachingAccessPointOldProxy: auth.NoCache,
		DirectClusters:                []reversetunnel.DirectCluster{{Name: s.server.ClusterName(), Client: s.proxyClient}},
		DataDir:                       c.MkDir(),
		Emitter:                       s.proxyClient,
		Log:                           logger,
	})
	c.Assert(err, IsNil)
	logger.WithField("tun-addr", reverseTunnelAddress.String()).Info("Created reverse tunnel server.")

	c.Assert(reverseTunnelServer.Start(), IsNil)
	defer reverseTunnelServer.Close()

	proxy, err := New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		s.server.ClusterName(),
		[]ssh.Signer{s.signer},
		s.proxyClient,
		c.MkDir(),
		"",
		utils.NetAddr{},
		SetProxyMode(reverseTunnelServer),
		SetSessionServer(s.proxyClient),
		SetEmitter(s.nodeClient),
		SetNamespace(defaults.Namespace),
		SetPAMConfig(&pam.Config{Enabled: false}),
		SetBPF(&bpf.NOP{}),
		SetClock(s.clock),
	)
	c.Assert(err, IsNil)
	c.Assert(proxy.Start(), IsNil)

	// set up SSH client using the user private key for signing
	up, err := s.newUpack(s.user, []string{s.user}, wildcardAllow)
	c.Assert(err, IsNil)

	// start agent and load balance requests
	eventsC := make(chan string, 2)
	rsAgent, err := reversetunnel.NewAgent(reversetunnel.AgentConfig{
		Context:     context.TODO(),
		Addr:        reverseTunnelAddress,
		ClusterName: "remote",
		Username:    fmt.Sprintf("%v.%v", hostID, s.server.ClusterName()),
		Signer:      s.signer,
		Client:      s.proxyClient,
		AccessPoint: s.proxyClient,
		EventsC:     eventsC,
		Log:         logger,
	})
	c.Assert(err, IsNil)
	rsAgent.Start()

	rsAgent2, err := reversetunnel.NewAgent(reversetunnel.AgentConfig{
		Context:     context.TODO(),
		Addr:        reverseTunnelAddress,
		ClusterName: "remote",
		Username:    fmt.Sprintf("%v.%v", hostID, s.server.ClusterName()),
		Signer:      s.signer,
		Client:      s.proxyClient,
		AccessPoint: s.proxyClient,
		EventsC:     eventsC,
		Log:         logger,
	})
	c.Assert(err, IsNil)
	rsAgent2.Start()
	defer rsAgent2.Close()

	timeout := time.After(time.Second)
	for i := 0; i < 2; i++ {
		select {
		case event := <-eventsC:
			c.Assert(event, Equals, reversetunnel.ConnectedEvent)
		case <-timeout:
			c.Fatalf("timeout waiting for clusters to connect")
		}
	}

	sshConfig := &ssh.ClientConfig{
		User:            s.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(s.signer.PublicKey()),
	}

	_, err = s.newUpack("user1", []string{s.user}, wildcardAllow)
	c.Assert(err, IsNil)

	for i := 0; i < 3; i++ {
		s.testClient(c, proxy.Addr(), s.srvAddress, s.srv.Addr(), sshConfig)
	}
	// close first connection, and test it again
	rsAgent.Close()

	for i := 0; i < 3; i++ {
		s.testClient(c, proxy.Addr(), s.srvAddress, s.srv.Addr(), sshConfig)
	}
}

// TestProxyDirectAccess tests direct access via proxy bypassing
// reverse tunnel
func (s *SrvSuite) TestProxyDirectAccess(c *C) {
	listener, _ := s.mustListen(c)
	logger := logrus.WithField("test", "TestProxyDirectAccess")
	reverseTunnelServer, err := reversetunnel.NewServer(reversetunnel.Config{
		ClientTLS:                     s.proxyClient.TLSConfig(),
		ID:                            hostID,
		ClusterName:                   s.server.ClusterName(),
		Listener:                      listener,
		HostSigners:                   []ssh.Signer{s.signer},
		LocalAuthClient:               s.proxyClient,
		LocalAccessPoint:              s.proxyClient,
		NewCachingAccessPoint:         auth.NoCache,
		NewCachingAccessPointOldProxy: auth.NoCache,
		DirectClusters:                []reversetunnel.DirectCluster{{Name: s.server.ClusterName(), Client: s.proxyClient}},
		DataDir:                       c.MkDir(),
		Emitter:                       s.proxyClient,
		Log:                           logger,
	})
	c.Assert(err, IsNil)

	c.Assert(reverseTunnelServer.Start(), IsNil)
	defer reverseTunnelServer.Close()

	proxy, err := New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		s.server.ClusterName(),
		[]ssh.Signer{s.signer},
		s.proxyClient,
		c.MkDir(),
		"",
		utils.NetAddr{},
		SetProxyMode(reverseTunnelServer),
		SetSessionServer(s.proxyClient),
		SetEmitter(s.nodeClient),
		SetNamespace(defaults.Namespace),
		SetPAMConfig(&pam.Config{Enabled: false}),
		SetBPF(&bpf.NOP{}),
		SetClock(s.clock),
	)
	c.Assert(err, IsNil)
	c.Assert(proxy.Start(), IsNil)
	defer proxy.Close()

	// set up SSH client using the user private key for signing
	up, err := s.newUpack(s.user, []string{s.user}, wildcardAllow)
	c.Assert(err, IsNil)

	sshConfig := &ssh.ClientConfig{
		User:            s.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(s.signer.PublicKey()),
	}

	_, err = s.newUpack("user1", []string{s.user}, wildcardAllow)
	c.Assert(err, IsNil)

	s.testClient(c, proxy.Addr(), s.srvAddress, s.srv.Addr(), sshConfig)
}

// TestPTY requests PTY for an interactive session
func (s *SrvSuite) TestPTY(c *C) {
	se, err := s.clt.NewSession()
	c.Assert(err, IsNil)
	defer se.Close()

	// request PTY with valid size
	c.Assert(se.RequestPty("xterm", 30, 30, ssh.TerminalModes{}), IsNil)

	// request PTY with invalid size, should still work (selects defaults)
	c.Assert(se.RequestPty("xterm", 0, 0, ssh.TerminalModes{}), IsNil)
}

// TestEnv requests setting environment variables. (We are currently ignoring these requests)
func (s *SrvSuite) TestEnv(c *C) {
	se, err := s.clt.NewSession()
	c.Assert(err, IsNil)
	defer se.Close()

	c.Assert(se.Setenv("HOME", "/"), IsNil)
}

// TestNoAuth tries to log in with no auth methods and should be rejected
func (s *SrvSuite) TestNoAuth(c *C) {
	_, err := ssh.Dial("tcp", s.srv.Addr(), &ssh.ClientConfig{})
	c.Assert(err, NotNil)
}

// TestPasswordAuth tries to log in with empty pass and should be rejected
func (s *SrvSuite) TestPasswordAuth(c *C) {
	config := &ssh.ClientConfig{
		Auth:            []ssh.AuthMethod{ssh.Password("")},
		HostKeyCallback: ssh.FixedHostKey(s.signer.PublicKey()),
	}
	_, err := ssh.Dial("tcp", s.srv.Addr(), config)
	c.Assert(err, NotNil)
}

func (s *SrvSuite) TestClientDisconnect(c *C) {
	config := &ssh.ClientConfig{
		User:            s.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(s.up.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(s.signer.PublicKey()),
	}
	clt, err := ssh.Dial("tcp", s.srv.Addr(), config)
	c.Assert(clt, NotNil)
	c.Assert(err, IsNil)

	se, err := s.clt.NewSession()
	c.Assert(err, IsNil)
	c.Assert(se.Shell(), IsNil)
	c.Assert(clt.Close(), IsNil)
}

func (s *SrvSuite) TestLimiter(c *C) {
	limiter, err := limiter.NewLimiter(
		limiter.Config{
			MaxConnections: 2,
			Rates: []limiter.Rate{
				{
					Period:  10 * time.Second,
					Average: 1,
					Burst:   3,
				},
				{
					Period:  40 * time.Millisecond,
					Average: 10,
					Burst:   30,
				},
			},
		},
	)
	c.Assert(err, IsNil)

	nodeStateDir := c.MkDir()
	srv, err := New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "127.0.0.1:0"},
		s.server.ClusterName(),
		[]ssh.Signer{s.signer},
		s.nodeClient,
		nodeStateDir,
		"",
		utils.NetAddr{},
		SetLimiter(limiter),
		SetShell("/bin/sh"),
		SetSessionServer(s.nodeClient),
		SetEmitter(s.nodeClient),
		SetNamespace(defaults.Namespace),
		SetPAMConfig(&pam.Config{Enabled: false}),
		SetBPF(&bpf.NOP{}),
		SetClock(s.clock),
	)
	c.Assert(err, IsNil)
	c.Assert(srv.Start(), IsNil)

	c.Assert(auth.CreateUploaderDir(nodeStateDir), IsNil)
	defer srv.Close()

	// maxConnection = 3
	// current connections = 1 (one connection is opened from SetUpTest)
	config := &ssh.ClientConfig{
		User:            s.user,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(s.up.certSigner)},
		HostKeyCallback: ssh.FixedHostKey(s.signer.PublicKey()),
	}

	clt0, err := ssh.Dial("tcp", srv.Addr(), config)
	c.Assert(clt0, NotNil)
	c.Assert(err, IsNil)
	se0, err := clt0.NewSession()
	c.Assert(err, IsNil)
	c.Assert(se0.Shell(), IsNil)

	// current connections = 2
	clt, err := ssh.Dial("tcp", srv.Addr(), config)
	c.Assert(clt, NotNil)
	c.Assert(err, IsNil)
	se, err := clt.NewSession()
	c.Assert(err, IsNil)
	c.Assert(se.Shell(), IsNil)

	// current connections = 3
	_, err = ssh.Dial("tcp", srv.Addr(), config)
	c.Assert(err, NotNil)

	c.Assert(se.Close(), IsNil)
	c.Assert(clt.Close(), IsNil)
	time.Sleep(50 * time.Millisecond)

	// current connections = 2
	clt, err = ssh.Dial("tcp", srv.Addr(), config)
	c.Assert(clt, NotNil)
	c.Assert(err, IsNil)
	se, err = clt.NewSession()
	c.Assert(err, IsNil)
	c.Assert(se.Shell(), IsNil)

	// current connections = 3
	_, err = ssh.Dial("tcp", srv.Addr(), config)
	c.Assert(err, NotNil)

	c.Assert(se.Close(), IsNil)
	c.Assert(clt.Close(), IsNil)
	time.Sleep(50 * time.Millisecond)

	// current connections = 2
	// requests rate should exceed now
	clt, err = ssh.Dial("tcp", srv.Addr(), config)
	c.Assert(clt, NotNil)
	c.Assert(err, IsNil)
	_, err = clt.NewSession()
	c.Assert(err, NotNil)

	clt.Close()
}

// TestServerAliveInterval simulates ServerAliveInterval and OpenSSH
// interoperability by sending a keepalive@openssh.com global request to the
// server and expecting a response in return.
func (s *SrvSuite) TestServerAliveInterval(c *C) {
	ok, _, err := s.clt.SendRequest(teleport.KeepAliveReqType, true, nil)
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, true)
}

// TestGlobalRequestRecordingProxy simulates sending a global out-of-band
// recording-proxy@teleport.com request.
func (s *SrvSuite) TestGlobalRequestRecordingProxy(c *C) {
	ctx := context.Background()

	// set cluster config to record at the node
	recConfig, err := types.NewSessionRecordingConfig(types.SessionRecordingConfigSpecV2{
		Mode: types.RecordAtNode,
	})
	c.Assert(err, IsNil)
	err = s.server.Auth().SetSessionRecordingConfig(ctx, recConfig)
	c.Assert(err, IsNil)

	// send the request again, we have cluster config and when we parse the
	// response, it should be false because recording is occurring at the node.
	ok, responseBytes, err := s.clt.SendRequest(teleport.RecordingProxyReqType, true, nil)
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, true)
	response, err := strconv.ParseBool(string(responseBytes))
	c.Assert(err, IsNil)
	c.Assert(response, Equals, false)

	// set cluster config to record at the proxy
	recConfig, err = types.NewSessionRecordingConfig(types.SessionRecordingConfigSpecV2{
		Mode: types.RecordAtProxy,
	})
	c.Assert(err, IsNil)
	err = s.server.Auth().SetSessionRecordingConfig(ctx, recConfig)
	c.Assert(err, IsNil)

	// send request again, now that we have cluster config and it's set to record
	// at the proxy, we should return true and when we parse the payload it should
	// also be true
	ok, responseBytes, err = s.clt.SendRequest(teleport.RecordingProxyReqType, true, nil)
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, true)
	response, err = strconv.ParseBool(string(responseBytes))
	c.Assert(err, IsNil)
	c.Assert(response, Equals, true)
}

// rawNode is a basic non-teleport node which holds a
// valid teleport cert and allows any client to connect.
// useful for simulating basic behaviors of openssh nodes.
type rawNode struct {
	listener net.Listener
	cfg      ssh.ServerConfig
	addr     string
}

// accept an incoming connection and perform a basic ssh handshake
func (r *rawNode) accept() (*ssh.ServerConn, <-chan ssh.NewChannel, <-chan *ssh.Request, error) {
	netConn, err := r.listener.Accept()
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}
	srvConn, chs, reqs, err := ssh.NewServerConn(netConn, &r.cfg)
	if err != nil {
		netConn.Close()
		return nil, nil, nil, trace.Wrap(err)
	}
	return srvConn, chs, reqs, nil
}

func (r *rawNode) Close() error {
	return trace.Wrap(r.listener.Close())
}

// newRawNode constructs a new raw node instance.
func (s *SrvSuite) newRawNode(c *C) *rawNode {
	hostname, err := os.Hostname()
	c.Assert(err, IsNil)

	// Create host key and certificate for node.
	keys, err := s.server.Auth().GenerateServerKeys(auth.GenerateServerKeysRequest{
		HostID:               "raw-node",
		NodeName:             "raw-node",
		Roles:                teleport.Roles{teleport.RoleNode},
		AdditionalPrincipals: []string{hostname},
		DNSNames:             []string{hostname},
	})
	c.Assert(err, IsNil)

	signer, err := sshutils.NewSigner(keys.Key, keys.Cert)
	c.Assert(err, IsNil)

	// configure a server which allows any client to connect
	cfg := ssh.ServerConfig{
		NoClientAuth: true,
		PasswordCallback: func(conn ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			return &ssh.Permissions{}, nil
		},
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			return &ssh.Permissions{}, nil
		},
	}
	cfg.AddHostKey(signer)

	listener, err := net.Listen("tcp", ":0")
	c.Assert(err, IsNil)

	_, port, err := net.SplitHostPort(listener.Addr().String())
	c.Assert(err, IsNil)

	addr := net.JoinHostPort(hostname, port)

	return &rawNode{
		listener: listener,
		cfg:      cfg,
		addr:     addr,
	}
}

// startX11EchoServer starts a fake node which, for each incomging SSH connection, accepts an
// X11 forwarding request and then dials a single X11 channel which echoes all bytes written
// to it.  Used to verify the behavior of X11 forwarding in recording proxies.
func (s *SrvSuite) startX11EchoServer(ctx context.Context, c *C) *rawNode {
	node := s.newRawNode(c)
	go func() {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		for {
			conn, chs, _, err := node.accept()
			if err != nil {
				log.Warnf("X11 echo server closing: %v", err)
				return
			}
			go func() {
				defer conn.Close()

				// expect client to open a session channel
				var nch ssh.NewChannel
				select {
				case nch = <-chs:
				case <-time.After(time.Second * 3):
					c.Fatalf("Timeout waiting for session channel")
				case <-ctx.Done():
					return
				}
				c.Assert(nch.ChannelType(), Equals, teleport.ChanSession)

				sch, creqs, err := nch.Accept()
				c.Assert(err, IsNil)
				defer sch.Close()

				// expect client to send an X11 forwarding request
				var req *ssh.Request
				select {
				case req = <-creqs:
				case <-time.After(time.Second * 3):
					c.Fatalf("Timeout waiting for x11 forwarding request")
				case <-ctx.Done():
					return
				}

				c.Assert(req.Type, Equals, sshutils.X11ForwardRequest)
				c.Assert(req.Reply(true, nil), IsNil)

				// start a fake X11 channel
				xch, _, err := conn.OpenChannel(sshutils.X11ChannelRequest, nil)
				c.Assert(err, IsNil)

				defer xch.Close()
				// echo all bytes back across the X11 channel
				_, err = io.Copy(xch, xch)
				if err == nil {
					xch.CloseWrite()
				} else {
					log.Errorf("X11 channel error: %v", err)
				}
			}()
		}
	}()
	return node
}

// TestX11ProxySupport verifies that recording proxies correctly forward
// X11 request/channels.
func (s *SrvSuite) TestX11ProxySupport(c *C) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// set cluster config to record at the proxy
	recConfig, err := types.NewSessionRecordingConfig(types.SessionRecordingConfigSpecV2{
		Mode: types.RecordAtProxy,
	})
	c.Assert(err, IsNil)
	err = s.server.Auth().SetSessionRecordingConfig(ctx, recConfig)
	c.Assert(err, IsNil)

	// verify that the proxy is in recording mode
	ok, responseBytes, err := s.clt.SendRequest(teleport.RecordingProxyReqType, true, nil)
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, true)
	response, err := strconv.ParseBool(string(responseBytes))
	c.Assert(err, IsNil)
	c.Assert(response, Equals, true)

	// setup our fake X11 echo server
	node := s.startX11EchoServer(ctx, c)

	// Create a direct TCP/IP connection from proxy to our X11 test server.
	netConn, err := s.clt.Dial("tcp", node.addr)
	c.Assert(err, IsNil)
	defer netConn.Close()

	// make an insecure version of our client config (this test is only about X11 forwarding,
	// so we don't bother to verify recording proxy key generation here).
	cltConfig := *s.cltConfig
	cltConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()

	// Perform ssh handshake and setup client for X11 test server.
	cltConn, chs, reqs, err := ssh.NewClientConn(netConn, node.addr, &cltConfig)
	c.Assert(err, IsNil)
	clt := ssh.NewClient(cltConn, chs, reqs)

	sess, err := clt.NewSession()
	c.Assert(err, IsNil)

	// register X11 channel handler before requesting forwarding to avoid races
	xchs := clt.HandleChannelOpen(sshutils.X11ChannelRequest)
	c.Assert(xchs, NotNil)

	// Send an X11 forwarding request to the server
	ok, err = sess.SendRequest(sshutils.X11ForwardRequest, true, nil)
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, true)

	// wait for server to start an X11 channel
	var xnc ssh.NewChannel
	select {
	case xnc = <-xchs:
	case <-time.After(time.Second * 3):
		c.Fatalf("Timeout waiting for X11 channel open from %v", node.addr)
	}
	c.Assert(xnc, NotNil)
	c.Assert(xnc.ChannelType(), Equals, sshutils.X11ChannelRequest)

	xch, _, err := xnc.Accept()
	c.Assert(err, IsNil)

	defer xch.Close()

	// write some data to the channel
	msg := []byte("testing!")
	_, err = xch.Write(msg)
	c.Assert(err, IsNil)

	// send EOF
	c.Assert(xch.CloseWrite(), IsNil)

	// expect node to successfully echo the data
	rsp := make([]byte, len(msg))
	_, err = io.ReadFull(xch, rsp)
	c.Assert(err, IsNil)
	c.Assert(string(msg), Equals, string(rsp))
}

// upack holds all ssh signing artefacts needed for signing and checking user keys
type upack struct {
	// key is a raw private user key
	key []byte

	// pkey is parsed private SSH key
	pkey interface{}

	// pub is a public user key
	pub []byte

	//cert is a certificate signed by user CA
	cert []byte
	// pcert is a parsed ssh Certificae
	pcert *ssh.Certificate

	// signer is a signer that answers signing challenges using private key
	signer ssh.Signer

	// certSigner is a signer that answers signing challenges using private
	// key and a certificate issued by user certificate authority
	certSigner ssh.Signer
}

func (s *SrvSuite) newUpack(username string, allowedLogins []string, allowedLabels types.Labels) (*upack, error) {
	ctx := context.Background()
	auth := s.server.Auth()
	upriv, upub, err := auth.GenerateKeyPair("")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := types.NewUser(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	role := services.RoleForUser(user)
	rules := role.GetRules(services.Allow)
	rules = append(rules, types.NewRule(types.Wildcard, services.RW()))
	role.SetRules(services.Allow, rules)
	opts := role.GetOptions()
	opts.PermitX11Forwarding = types.NewBool(true)
	role.SetOptions(opts)
	role.SetLogins(services.Allow, allowedLogins)
	role.SetNodeLabels(services.Allow, allowedLabels)
	err = auth.UpsertRole(ctx, role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user.AddRole(role.GetName())
	err = auth.UpsertUser(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	ucert, err := s.server.AuthServer.GenerateUserCert(upub, user.GetName(), 5*time.Minute, teleport.CertificateFormatStandard)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	upkey, err := ssh.ParseRawPrivateKey(upriv)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	usigner, err := ssh.NewSignerFromKey(upkey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	pcert, _, _, _, err := ssh.ParseAuthorizedKey(ucert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ucertSigner, err := ssh.NewCertSigner(pcert.(*ssh.Certificate), usigner)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &upack{
		key:        upriv,
		pkey:       upkey,
		pub:        upub,
		cert:       ucert,
		pcert:      pcert.(*ssh.Certificate),
		signer:     usigner,
		certSigner: ucertSigner,
	}, nil
}

func waitForSites(s reversetunnel.Tunnel, count int) error {
	timeout := time.NewTimer(10 * time.Second)
	defer timeout.Stop()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			clusters, err := s.GetSites()
			if err != nil {
				return trace.Wrap(err)
			}
			if len(clusters) == count {
				return nil
			}
		case <-timeout.C:
			return trace.BadParameter("timed out waiting for clusters")
		}
	}
}
