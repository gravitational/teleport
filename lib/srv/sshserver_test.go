/*
Copyright 2015 Gravitational, Inc.

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

package srv

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os/user"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/services/suite"
	sess "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	. "gopkg.in/check.v1"
)

func TestSrv(t *testing.T) { TestingT(t) }

type SrvSuite struct {
	srv           *Server
	srvAddress    string
	srvPort       string
	srvHostPort   string
	sessionServer sess.Service
	clt           *ssh.Client
	bk            backend.Backend
	a             *auth.AuthServer
	roleAuth      *auth.AuthWithRoles
	alog          events.IAuditLog
	up            *upack
	signer        ssh.Signer
	dir           string
	user          string
	domainName    string
	freePorts     utils.PortList
	access        services.Access
	identity      services.Identity
}

var _ = Suite(&SrvSuite{})

func (s *SrvSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
}

func (s *SrvSuite) SetUpTest(c *C) {
	var err error
	s.dir = c.MkDir()

	s.alog, err = events.NewAuditLog(s.dir)
	c.Assert(err, IsNil)

	u, err := user.Current()
	c.Assert(err, IsNil)
	s.user = u.Username

	s.freePorts, err = utils.GetFreeTCPPorts(10)
	c.Assert(err, IsNil)

	s.bk, err = boltbk.New(backend.Params{"path": s.dir})
	c.Assert(err, IsNil)

	s.access = local.NewAccessService(s.bk)
	s.identity = local.NewIdentityService(s.bk)

	s.domainName = "localhost"
	s.a = auth.NewAuthServer(&auth.InitConfig{
		Backend:    s.bk,
		Authority:  authority.New(),
		DomainName: s.domainName,
		Identity:   s.identity,
		Access:     s.access,
	})

	sessionServer, err := sess.New(s.bk)
	s.sessionServer = sessionServer
	c.Assert(err, IsNil)

	newChecker, err := auth.NewAccessChecker(s.access, s.identity)
	c.Assert(err, IsNil)

	c.Assert(s.a.UpsertCertAuthority(suite.NewTestCA(services.UserCA, s.domainName), backend.Forever), IsNil)
	c.Assert(s.a.UpsertCertAuthority(suite.NewTestCA(services.HostCA, s.domainName), backend.Forever), IsNil)

	// set up SSH client using the user private key for signing
	up, err := newUpack(s.user, []string{s.user}, s.a)
	c.Assert(err, IsNil)

	checker, err := newChecker(s.user)
	c.Assert(err, IsNil)

	s.roleAuth = auth.NewAuthWithRoles(s.a, checker, s.user, sessionServer, nil)

	// set up host private key and certificate
	hpriv, hpub, err := s.a.GenerateKeyPair("")
	c.Assert(err, IsNil)
	hcert, err := s.a.GenerateHostCert(hpub, s.domainName, s.domainName, teleport.Roles{teleport.RoleAdmin}, 0)
	c.Assert(err, IsNil)

	// set up user CA and set up a user that has access to the server
	s.signer, err = sshutils.NewSigner(hpriv, hcert)
	c.Assert(err, IsNil)

	s.srvPort = s.freePorts[len(s.freePorts)-1]
	s.freePorts = s.freePorts[:len(s.freePorts)-1]
	s.srvAddress = "127.0.0.1:" + s.srvPort

	s.srvHostPort = fmt.Sprintf("%v:%v", s.domainName, s.srvPort)
	srv, err := New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: s.srvAddress},
		s.domainName,
		[]ssh.Signer{s.signer},
		s.roleAuth,
		s.dir,
		nil,
		SetShell("/bin/sh"),
		SetSessionServer(sessionServer),
	)
	c.Assert(err, IsNil)
	s.srv = srv
	s.srv.isTestStub = true

	c.Assert(s.srv.Start(), IsNil)
	c.Assert(s.srv.registerServer(), IsNil)

	// set up an agent server and a client that uses agent for forwarding
	keyring := agent.NewKeyring()
	addedKey := agent.AddedKey{
		PrivateKey:  up.pkey,
		Certificate: up.pcert,
	}
	c.Assert(keyring.Add(addedKey), IsNil)
	s.up = up
	sshConfig := &ssh.ClientConfig{
		User: s.user,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
	}

	client, err := ssh.Dial("tcp", s.srv.Addr(), sshConfig)
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
}

func (s *SrvSuite) TestAdvertiseAddr(c *C) {
	c.Assert(strings.Index(s.srv.AdvertiseAddr(), "127.0.0.1:"), Equals, 0)
	s.srv.setAdvertiseIP(net.ParseIP("10.10.10.1"))
	c.Assert(strings.Index(s.srv.AdvertiseAddr(), "10.10.10.1:"), Equals, 0)
	s.srv.setAdvertiseIP(nil)
}

// TestExec executes a command on a remote server
func (s *SrvSuite) TestExec(c *C) {
	c.Skip("disabled")

	se, err := s.clt.NewSession()
	c.Assert(err, IsNil)
	defer se.Close()

	out, err := se.Output("echo $((2 + 3))")
	c.Assert(err, IsNil)
	c.Assert(strings.Trim(string(out), " \n"), Equals, "5")
}

// TestShell launches interactive shell session and executes a command
func (s *SrvSuite) TestShell(c *C) {
	c.Skip("disabled")
	se, err := s.clt.NewSession()
	c.Assert(err, IsNil)

	writer, err := se.StdinPipe()
	c.Assert(err, IsNil)

	stdoutPipe, err := se.StdoutPipe()
	c.Assert(err, IsNil)
	reader := bufio.NewReader(stdoutPipe)

	c.Assert(se.Shell(), IsNil)

	buf := make([]byte, 256)
	_, err = reader.Read(buf)
	c.Assert(err, IsNil)
	c.Assert(len(buf) > 0, Equals, true)

	// send a few "keyboard inputs" into the session:
	_, err = io.WriteString(writer, "echo $((50+100))\n\r")
	c.Assert(err, IsNil)

	// read the output and make sure that "150" (output of $((50+100)) is there
	// NOTE: this test may fail if you have errors in your .bashrc or .profile
	// leading to tons of output when opening new bash session
	foundOutput := false
	for i := 0; i < 50 && !foundOutput; i++ {
		time.Sleep(time.Millisecond)
		_, err = reader.Read(buf)
		c.Assert(err, IsNil)
		foundOutput = strings.Contains(string(buf), "150")
	}
	c.Assert(foundOutput, Equals, true)
	c.Assert(se.Close(), IsNil)
}

func (s *SrvSuite) TestAllowedUsers(c *C) {
	up, err := newUpack(s.user, []string{s.user}, s.a)
	c.Assert(err, IsNil)

	sshConfig := &ssh.ClientConfig{
		User: s.user,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
	}

	client, err := ssh.Dial("tcp", s.srv.Addr(), sshConfig)
	c.Assert(err, IsNil)

	client, err = ssh.Dial("tcp", s.srv.Addr(), sshConfig)
	c.Assert(err, IsNil)
	c.Assert(client.Close(), IsNil)

	// now remove OS user from valid principals
	up, err = newUpack(s.user, []string{"otheruser"}, s.a)
	c.Assert(err, IsNil)

	sshConfig = &ssh.ClientConfig{
		User: s.user,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
	}

	client, err = ssh.Dial("tcp", s.srv.Addr(), sshConfig)
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

	out, err := se2.Output("echo $((2+3))")
	c.Assert(err, IsNil)

	c.Assert(strings.Trim(string(out), " \n"), Equals, "5")
}

func (s *SrvSuite) TestProxyReverseTunnel(c *C) {
	log.Infof("[TEST START] TestProxyReverseTunnel")

	reverseTunnelPort := s.freePorts[len(s.freePorts)-1]
	s.freePorts = s.freePorts[:len(s.freePorts)-1]
	reverseTunnelAddress := utils.NetAddr{AddrNetwork: "tcp", Addr: fmt.Sprintf("%v:%v", s.domainName, reverseTunnelPort)}
	reverseTunnelServer, err := reversetunnel.NewServer(
		reverseTunnelAddress,
		[]ssh.Signer{s.signer},
		s.roleAuth,
	)
	c.Assert(err, IsNil)
	c.Assert(reverseTunnelServer.Start(), IsNil)

	proxy, err := New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		s.domainName,
		[]ssh.Signer{s.signer},
		s.roleAuth,
		s.dir,
		nil,
		SetProxyMode(reverseTunnelServer),
		SetSessionServer(s.sessionServer),
	)
	c.Assert(err, IsNil)
	c.Assert(proxy.Start(), IsNil)

	// set up SSH client using the user private key for signing
	up, err := newUpack(s.user, []string{s.user}, s.a)
	c.Assert(err, IsNil)

	tsrv := s.makeTunnel(c)
	c.Assert(tsrv.Start(), IsNil)

	tunClt, err := auth.NewTunClient("test",
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: tsrv.Addr()}}, s.domainName, []ssh.AuthMethod{ssh.PublicKeys(s.signer)})
	c.Assert(err, IsNil)
	defer tunClt.Close()

	agentPool, err := reversetunnel.NewAgentPool(reversetunnel.AgentPoolConfig{
		Client:      tunClt,
		HostSigners: []ssh.Signer{s.signer},
		HostUUID:    s.domainName,
	})
	c.Assert(err, IsNil)

	err = tunClt.UpsertReverseTunnel(
		services.NewReverseTunnel(s.domainName, []string{reverseTunnelAddress.String()}), 0)
	c.Assert(err, IsNil)

	err = agentPool.FetchAndSyncAgents()
	c.Assert(err, IsNil)

	rsAgent, err := reversetunnel.NewAgent(
		reverseTunnelAddress,
		"remote",
		"localhost",
		[]ssh.Signer{s.signer}, tunClt)
	c.Assert(err, IsNil)
	c.Assert(rsAgent.Start(), IsNil)

	sshConfig := &ssh.ClientConfig{
		User: s.user,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
	}

	_, err = newUpack("user1", []string{s.user}, s.a)
	c.Assert(err, IsNil)

	s.testClient(c, proxy.Addr(), s.srvAddress, s.srv.Addr(), sshConfig)
	s.testClient(c, proxy.Addr(), s.srvHostPort, s.srv.Addr(), sshConfig)

	// adding new node
	bobAddr := "127.0.0.1:" + s.freePorts[len(s.freePorts)-1]
	s.freePorts = s.freePorts[:len(s.freePorts)-1]
	srv2, err := New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: bobAddr},
		"bob",
		[]ssh.Signer{s.signer},
		s.roleAuth,
		c.MkDir(),
		nil,
		SetShell("/bin/sh"),
		SetLabels(
			map[string]string{"label1": "value1"},
			services.CommandLabels{
				"cmdLabel1": &services.CommandLabelV2{
					Period:  services.NewDuration(time.Millisecond),
					Command: []string{"expr", "1", "+", "3"}},
				"cmdLabel2": &services.CommandLabelV2{
					Period:  services.NewDuration(time.Second * 2),
					Command: []string{"expr", "2", "+", "3"}},
			},
		),
		SetSessionServer(s.sessionServer),
	)
	srv2.uuid = bobAddr
	c.Assert(err, IsNil)
	c.Assert(srv2.Start(), IsNil)
	c.Assert(srv2.registerServer(), IsNil)
	defer srv2.Close()

	srv2.registerServer()

	// test proxysites
	client, err := ssh.Dial("tcp", proxy.Addr(), sshConfig)
	c.Assert(err, IsNil)

	se3, err := client.NewSession()
	c.Assert(err, IsNil)
	defer se3.Close()

	stdout := &bytes.Buffer{}
	reader, err := se3.StdoutPipe()
	done := make(chan struct{})
	go func() {
		io.Copy(stdout, reader)
		close(done)
	}()

	// to make sure  labels have the right output
	s.srv.syncUpdateLabels()
	srv2.syncUpdateLabels()
	s.srv.registerServer()
	srv2.registerServer()

	// request "list of sites":
	c.Assert(se3.RequestSubsystem("proxysites"), IsNil)
	<-done
	var sites []services.Site
	c.Assert(json.Unmarshal(stdout.Bytes(), &sites), IsNil)
	c.Assert(sites, NotNil)
	c.Assert(sites, HasLen, 1)
	c.Assert(sites[0].Name, Equals, "localhost")
	c.Assert(sites[0].Status, Equals, "online")
	c.Assert(time.Since(sites[0].LastConnected).Seconds() < 5, Equals, true)

	err = tunClt.DeleteReverseTunnel(s.domainName)
	c.Assert(err, IsNil)

	err = agentPool.FetchAndSyncAgents()
	c.Assert(err, IsNil)
}

func (s *SrvSuite) TestProxyRoundRobin(c *C) {
	log.Infof("[TEST START] TestProxyRoundRobin")

	reverseTunnelPort := s.freePorts[len(s.freePorts)-1]
	s.freePorts = s.freePorts[:len(s.freePorts)-1]
	reverseTunnelAddress := utils.NetAddr{
		AddrNetwork: "tcp",
		Addr:        fmt.Sprintf("%v:%v", s.domainName, reverseTunnelPort),
	}
	reverseTunnelServer, err := reversetunnel.NewServer(
		reverseTunnelAddress,
		[]ssh.Signer{s.signer},
		s.roleAuth,
	)
	c.Assert(err, IsNil)
	c.Assert(reverseTunnelServer.Start(), IsNil)

	proxy, err := New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		s.domainName,
		[]ssh.Signer{s.signer},
		s.roleAuth,
		s.dir,
		nil,
		SetProxyMode(reverseTunnelServer),
		SetSessionServer(s.sessionServer),
	)
	c.Assert(err, IsNil)
	c.Assert(proxy.Start(), IsNil)

	// set up SSH client using the user private key for signing
	up, err := newUpack(s.user, []string{s.user}, s.a)
	c.Assert(err, IsNil)

	tsrv := s.makeTunnel(c)
	c.Assert(tsrv.Start(), IsNil)

	tunClt, err := auth.NewTunClient("test",
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: tsrv.Addr()}}, s.domainName, []ssh.AuthMethod{ssh.PublicKeys(s.signer)})
	c.Assert(err, IsNil)
	defer tunClt.Close()

	// start agent and load balance requests
	rsAgent, err := reversetunnel.NewAgent(
		reverseTunnelAddress,
		"remote",
		"localhost",
		[]ssh.Signer{s.signer}, tunClt)
	c.Assert(err, IsNil)
	c.Assert(rsAgent.Start(), IsNil)

	rsAgent2, err := reversetunnel.NewAgent(
		reverseTunnelAddress,
		"remote",
		"localhost",
		[]ssh.Signer{s.signer}, tunClt)
	c.Assert(err, IsNil)
	c.Assert(rsAgent2.Start(), IsNil)
	defer rsAgent2.Close()

	sshConfig := &ssh.ClientConfig{
		User: s.user,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
	}

	_, err = newUpack("user1", []string{s.user}, s.a)
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
	reverseTunnelAddress := utils.NetAddr{
		AddrNetwork: "tcp",
		Addr:        fmt.Sprintf("%v:0", s.domainName),
	}
	reverseTunnelServer, err := reversetunnel.NewServer(
		reverseTunnelAddress,
		[]ssh.Signer{s.signer},
		s.roleAuth,
		reversetunnel.DirectSite(s.domainName, s.roleAuth),
	)
	c.Assert(err, IsNil)

	proxy, err := New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		s.domainName,
		[]ssh.Signer{s.signer},
		s.roleAuth,
		s.dir,
		nil,
		SetProxyMode(reverseTunnelServer),
		SetSessionServer(s.sessionServer),
	)
	c.Assert(err, IsNil)
	c.Assert(proxy.Start(), IsNil)

	// set up SSH client using the user private key for signing
	up, err := newUpack(s.user, []string{s.user}, s.a)
	c.Assert(err, IsNil)

	tsrv := s.makeTunnel(c)
	c.Assert(tsrv.Start(), IsNil)

	tunClt, err := auth.NewTunClient("test",
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: tsrv.Addr()}}, s.domainName, []ssh.AuthMethod{ssh.PublicKeys(s.signer)})
	c.Assert(err, IsNil)
	defer tunClt.Close()

	sshConfig := &ssh.ClientConfig{
		User: s.user,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
	}

	_, err = newUpack("user1", []string{s.user}, s.a)
	c.Assert(err, IsNil)

	s.testClient(c, proxy.Addr(), s.srvAddress, s.srv.Addr(), sshConfig)
}

// TestPTY requests PTY for an interactive session
func (s *SrvSuite) TestPTY(c *C) {
	se, err := s.clt.NewSession()
	c.Assert(err, IsNil)
	defer se.Close()

	// request PTY
	c.Assert(se.RequestPty("xterm", 30, 30, ssh.TerminalModes{}), IsNil)
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
	config := &ssh.ClientConfig{Auth: []ssh.AuthMethod{ssh.Password("")}}
	_, err := ssh.Dial("tcp", s.srv.Addr(), config)
	c.Assert(err, NotNil)
}

func (s *SrvSuite) TestClientDisconnect(c *C) {
	config := &ssh.ClientConfig{
		User: s.user,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(s.up.certSigner)},
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
		limiter.LimiterConfig{
			MaxConnections: 2,
			Rates: []limiter.Rate{
				limiter.Rate{
					Period:  10 * time.Second,
					Average: 1,
					Burst:   3,
				},
				limiter.Rate{
					Period:  40 * time.Millisecond,
					Average: 10,
					Burst:   30,
				},
			},
		},
	)
	c.Assert(err, IsNil)

	srvAddress := "127.0.0.1:" + s.freePorts[len(s.freePorts)-1]
	s.freePorts = s.freePorts[:len(s.freePorts)-1]
	srv, err := New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: srvAddress},
		s.domainName,
		[]ssh.Signer{s.signer},
		s.roleAuth,
		s.dir,
		nil,
		SetLimiter(limiter),
		SetShell("/bin/sh"),
		SetSessionServer(s.sessionServer),
	)
	c.Assert(err, IsNil)
	c.Assert(srv.Start(), IsNil)
	defer srv.Close()

	// maxConnection = 3
	// current connections = 1 (one connection is opened from SetUpTest)
	config := &ssh.ClientConfig{
		User: s.user,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(s.up.certSigner)},
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

func newUpack(username string, allowedLogins []string, a *auth.AuthServer) (*upack, error) {
	upriv, upub, err := a.GenerateKeyPair("")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user, err := services.NewUser(username)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	role := services.RoleForUser(user)
	role.SetResource(services.Wildcard, services.RW())
	role.SetLogins(allowedLogins)
	err = a.UpsertRole(role)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user.AddRole(role.GetName())
	err = a.UpsertUser(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ucert, err := a.GenerateUserCert(upub, username, allowedLogins, 0)
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

func removeNL(v string) string {
	v = strings.Replace(v, "\r", "", -1)
	v = strings.Replace(v, "\n", "", -1)
	return v
}

func (s *SrvSuite) makeTunnel(c *C) *auth.AuthTunnel {
	newChecker, err := auth.NewAccessChecker(s.access, s.identity)
	c.Assert(err, IsNil)

	tsrv, err := auth.NewTunnel(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		s.signer,
		&auth.APIConfig{
			AuthServer:     s.a,
			AuditLog:       s.alog,
			SessionService: s.sessionServer,
			NewChecker:     newChecker,
		})
	c.Assert(err, IsNil)
	return tsrv
}
