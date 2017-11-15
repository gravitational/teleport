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

package regular

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/dir"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/services/suite"
	sess "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/state"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
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
	testUser      string
	domainName    string
	freePorts     utils.PortList
	access        services.Access
	identity      services.Identity
	trust         services.Trust
}

// teleportTestUser is additional user used for tests
const teleportTestUser = "teleport-test"

var _ = Suite(&SrvSuite{})

func (s *SrvSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
}

const hostID = "00000000-0000-0000-0000-000000000000"

func (s *SrvSuite) SetUpTest(c *C) {
	var err error
	s.dir = c.MkDir()

	s.alog, err = events.NewAuditLog(s.dir, true)
	c.Assert(err, IsNil)

	u, err := user.Current()
	c.Assert(err, IsNil)
	s.user = u.Username

	s.freePorts, err = utils.GetFreeTCPPorts(10)
	c.Assert(err, IsNil)

	s.bk, err = dir.New(backend.Params{"path": s.dir})
	c.Assert(err, IsNil)

	s.access = local.NewAccessService(s.bk)
	s.identity = local.NewIdentityService(s.bk)
	s.trust = local.NewCAService(s.bk)

	s.domainName = "localhost"
	s.a = auth.NewAuthServer(&auth.InitConfig{
		Backend:   s.bk,
		Authority: authority.New(),
		Identity:  s.identity,
		Access:    s.access,
	})

	// set cluster name
	clusterName, err := services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: s.domainName,
	})
	c.Assert(err, IsNil)
	err = s.a.SetClusterName(clusterName)
	c.Assert(err, IsNil)

	// set static tokens
	staticTokens, err := services.NewStaticTokens(services.StaticTokensSpecV2{
		StaticTokens: []services.ProvisionToken{},
	})
	c.Assert(err, IsNil)
	err = s.a.SetStaticTokens(staticTokens)
	c.Assert(err, IsNil)

	sessionServer, err := sess.New(s.bk)
	s.sessionServer = sessionServer
	c.Assert(err, IsNil)

	authorizer, err := auth.NewAuthorizer(s.access, s.identity, s.trust)
	c.Assert(err, IsNil)

	c.Assert(s.a.UpsertCertAuthority(suite.NewTestCA(services.UserCA, s.domainName)), IsNil)
	c.Assert(s.a.UpsertCertAuthority(suite.NewTestCA(services.HostCA, s.domainName)), IsNil)

	// set up SSH client using the user private key for signing
	up, err := newUpack(s.user, []string{s.user}, s.a)
	c.Assert(err, IsNil)

	ctx := context.WithValue(context.TODO(), auth.ContextUser, auth.LocalUser{Username: s.user})
	authContext, err := authorizer.Authorize(ctx)
	c.Assert(err, IsNil)

	s.roleAuth = auth.NewAuthWithRoles(s.a, authContext.Checker, authContext.User, sessionServer, nil)

	// set up host private key and certificate
	hpriv, hpub, err := s.a.GenerateKeyPair("")
	c.Assert(err, IsNil)
	hcert, err := s.a.GenerateHostCert(hpub, hostID, s.domainName, s.domainName, teleport.Roles{teleport.RoleAdmin}, 0)
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
		utils.NetAddr{},
		SetNamespace(defaults.Namespace),
		SetAuditLog(s.alog),
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

// TestAgentForwardPermission makes sure if RBAC rules don't allow agent
// forwarding, we don't start an agent even if requested.
func (s *SrvSuite) TestAgentForwardPermission(c *C) {
	se, err := s.clt.NewSession()
	c.Assert(err, IsNil)
	defer se.Close()

	// make sure the role does not allow agent forwarding
	roleName := services.RoleNameForUser(s.user)
	role, err := s.a.GetRole(roleName)
	c.Assert(err, IsNil)
	roleOptions := role.GetOptions()
	roleOptions.Set(services.ForwardAgent, false)
	role.SetOptions(roleOptions)
	err = s.a.UpsertRole(role, backend.Forever)
	c.Assert(err, IsNil)

	// to interoperate with OpenSSH, requests for agent forwarding always succeed.
	// however that does not mean the users agent will actually be forwarded.
	err = agent.RequestAgentForwarding(se)
	c.Assert(err, IsNil)

	// the output of env, we should not see SSH_AUTH_SOCK in the output
	output, err := se.Output("env")
	c.Assert(err, IsNil)
	c.Assert(strings.Contains(string(output), "SSH_AUTH_SOCK"), Equals, false)
}

// TestAgentForward tests agent forwarding via unix sockets
func (s *SrvSuite) TestAgentForward(c *C) {
	se, err := s.clt.NewSession()
	c.Assert(err, IsNil)
	defer se.Close()

	roleName := services.RoleNameForUser(s.user)
	role, err := s.a.GetRole(roleName)
	c.Assert(err, IsNil)
	roleOptions := role.GetOptions()
	roleOptions.Set(services.ForwardAgent, true)
	role.SetOptions(roleOptions)
	err = s.a.UpsertRole(role, backend.Forever)
	c.Assert(err, IsNil)

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
	socketPath := strings.TrimSpace(string(output))

	// try dialing the ssh agent socket:
	file, err := net.Dial("unix", socketPath)
	c.Assert(err, IsNil)
	clientAgent := agent.NewClient(file)

	signers, err := clientAgent.Signers()
	c.Assert(err, IsNil)

	sshConfig := &ssh.ClientConfig{
		User: s.user,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signers...)},
	}

	client, err := ssh.Dial("tcp", s.srv.Addr(), sshConfig)
	c.Assert(err, IsNil)
	err = client.Close()
	c.Assert(err, IsNil)

	// make sure the socket is gone after we closed the session
	se.Close()
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
	up, err := newUpack(s.user, []string{s.user}, s.a)
	c.Assert(err, IsNil)

	// login with first user
	sshConfig := &ssh.ClientConfig{
		User: s.user,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
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
	up2, err := newUpack(teleportTestUser, []string{teleportTestUser}, s.a)
	c.Assert(err, IsNil)

	sshConfig2 := &ssh.ClientConfig{
		User: teleportTestUser,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(up2.certSigner)},
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

func (s *SrvSuite) TestProxyReverseTunnel(c *C) {
	log.Infof("[TEST START] TestProxyReverseTunnel")

	reverseTunnelPort := s.freePorts[len(s.freePorts)-1]
	s.freePorts = s.freePorts[:len(s.freePorts)-1]
	reverseTunnelAddress := utils.NetAddr{AddrNetwork: "tcp", Addr: fmt.Sprintf("%v:%v", s.domainName, reverseTunnelPort)}
	reverseTunnelServer, err := reversetunnel.NewServer(reversetunnel.Config{
		ID:                    s.domainName,
		ListenAddr:            reverseTunnelAddress,
		HostSigners:           []ssh.Signer{s.signer},
		AccessPoint:           s.roleAuth,
		NewCachingAccessPoint: state.NoCache,
	})
	c.Assert(err, IsNil)
	c.Assert(reverseTunnelServer.Start(), IsNil)

	proxy, err := New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		s.domainName,
		[]ssh.Signer{s.signer},
		s.roleAuth,
		s.dir,
		nil,
		utils.NetAddr{},
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
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: tsrv.Addr()}}, hostID, []ssh.AuthMethod{ssh.PublicKeys(s.signer)})
	c.Assert(err, IsNil)
	defer tunClt.Close()

	agentPool, err := reversetunnel.NewAgentPool(reversetunnel.AgentPoolConfig{
		Client:      tunClt,
		HostSigners: []ssh.Signer{s.signer},
		HostUUID:    hostID,
		AccessPoint: tunClt,
	})
	c.Assert(err, IsNil)

	err = tunClt.UpsertReverseTunnel(
		services.NewReverseTunnel(s.domainName, []string{reverseTunnelAddress.String()}))
	c.Assert(err, IsNil)

	err = agentPool.FetchAndSyncAgents()
	c.Assert(err, IsNil)

	eventsC := make(chan string, 1)
	rsAgent, err := reversetunnel.NewAgent(reversetunnel.AgentConfig{
		Context:       context.TODO(),
		Addr:          reverseTunnelAddress,
		RemoteCluster: "remote",
		Username:      hostID,
		Signers:       []ssh.Signer{s.signer},
		Client:        tunClt,
		AccessPoint:   tunClt,
		EventsC:       eventsC,
	})
	c.Assert(err, IsNil)
	rsAgent.Start()

	timeout := time.After(time.Second)
	select {
	case event := <-eventsC:
		c.Assert(event, Equals, reversetunnel.ConnectedEvent)
	case <-timeout:
		c.Fatalf("timeout waiting for clusters to connect")
	}

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
		utils.NetAddr{},
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
	reverseTunnelServer, err := reversetunnel.NewServer(reversetunnel.Config{
		ID:                    s.domainName,
		ListenAddr:            reverseTunnelAddress,
		HostSigners:           []ssh.Signer{s.signer},
		AccessPoint:           s.roleAuth,
		NewCachingAccessPoint: state.NoCache,
	})
	c.Assert(err, IsNil)

	c.Assert(reverseTunnelServer.Start(), IsNil)

	proxy, err := New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		s.domainName,
		[]ssh.Signer{s.signer},
		s.roleAuth,
		s.dir,
		nil,
		utils.NetAddr{},
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
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: tsrv.Addr()}}, hostID, []ssh.AuthMethod{ssh.PublicKeys(s.signer)})
	c.Assert(err, IsNil)
	defer tunClt.Close()

	// start agent and load balance requests
	eventsC := make(chan string, 2)
	rsAgent, err := reversetunnel.NewAgent(reversetunnel.AgentConfig{
		Context:       context.TODO(),
		Addr:          reverseTunnelAddress,
		RemoteCluster: "remote",
		Username:      hostID,
		Signers:       []ssh.Signer{s.signer},
		Client:        tunClt,
		AccessPoint:   tunClt,
		EventsC:       eventsC,
	})
	c.Assert(err, IsNil)
	rsAgent.Start()

	rsAgent2, err := reversetunnel.NewAgent(reversetunnel.AgentConfig{
		Context:       context.TODO(),
		Addr:          reverseTunnelAddress,
		RemoteCluster: "remote",
		Username:      hostID,
		Signers:       []ssh.Signer{s.signer},
		Client:        tunClt,
		AccessPoint:   tunClt,
		EventsC:       eventsC,
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
	reverseTunnelServer, err := reversetunnel.NewServer(reversetunnel.Config{
		ID:                    s.domainName,
		ListenAddr:            reverseTunnelAddress,
		HostSigners:           []ssh.Signer{s.signer},
		AccessPoint:           s.roleAuth,
		NewCachingAccessPoint: state.NoCache,
		DirectClusters:        []reversetunnel.DirectCluster{{Name: s.domainName, Client: s.roleAuth}},
	})
	c.Assert(err, IsNil)

	proxy, err := New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		s.domainName,
		[]ssh.Signer{s.signer},
		s.roleAuth,
		s.dir,
		nil,
		utils.NetAddr{},
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
		[]utils.NetAddr{{AddrNetwork: "tcp", Addr: tsrv.Addr()}}, hostID, []ssh.AuthMethod{ssh.PublicKeys(s.signer)})
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
		utils.NetAddr{},
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

// TestServerAliveInterval simulates ServerAliveInterval and OpenSSH
// interoperability by sending a keepalive@openssh.com global request to the
// server and expecting a response in return.
func (s *SrvSuite) TestServerAliveInterval(c *C) {
	ok, _, err := s.clt.SendRequest(teleport.KeepAliveReqType, true, nil)
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, true)
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
	rules := role.GetRules(services.Allow)
	rules = append(rules, services.NewRule(services.Wildcard, services.RW()))
	role.SetRules(services.Allow, rules)
	role.SetLogins(services.Allow, allowedLogins)
	err = a.UpsertRole(role, backend.Forever)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user.AddRole(role.GetName())
	err = a.UpsertUser(user)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	ucert, err := a.GenerateUserCert(upub, user, allowedLogins, 0, true, teleport.CompatibilityNone)
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
	authorizer, err := auth.NewAuthorizer(s.access, s.identity, s.trust)
	c.Assert(err, IsNil)

	tsrv, err := auth.NewTunnel(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		s.signer,
		&auth.APIConfig{
			AuthServer:     s.a,
			AuditLog:       s.alog,
			SessionService: s.sessionServer,
			Authorizer:     authorizer,
		})
	c.Assert(err, IsNil)
	return tsrv
}
