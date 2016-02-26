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
package client

import (
	"bufio"
	"bytes"
	"io/ioutil"
	"net/http"
	"os/user"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"
	"github.com/gravitational/teleport/lib/events/boltlog"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/recorder/boltrec"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	sess "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"

	"github.com/gokyle/hotp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	. "gopkg.in/check.v1"

	log "github.com/Sirupsen/logrus"
)

func TestClient(t *testing.T) { TestingT(t) }

type ClientSuite struct {
	srv          *srv.Server
	srv2         *srv.Server
	proxy        *srv.Server
	srvAddress   string
	srv2Address  string
	proxyAddress string
	webAddress   string
	clt          *ssh.Client
	bk           *encryptedbk.ReplicatedBackend
	a            *auth.AuthServer
	roleAuth     *auth.AuthWithRoles
	signer       ssh.Signer
	teleagent    *teleagent.TeleAgent
	dir          string
	dir2         string
	otp          *hotp.HOTP
	user         string
	pass         []byte
	freePorts    []string
}

var _ = Suite(&ClientSuite{})

func (s *ClientSuite) SetUpSuite(c *C) {
	utils.InitLoggerCLI()
	KeysDir = c.MkDir()
	s.dir = c.MkDir()
	s.dir2 = c.MkDir()

	allowAllLimiter, err := limiter.NewLimiter(limiter.LimiterConfig{})
	c.Assert(err, IsNil)

	s.freePorts, err = utils.GetFreeTCPPorts(10)
	c.Assert(err, IsNil)

	baseBk, err := boltbk.New(filepath.Join(s.dir, "db"))
	c.Assert(err, IsNil)
	s.bk, err = encryptedbk.NewReplicatedBackend(baseBk,
		filepath.Join(s.dir, "keys"), nil,
		encryptor.GetTestKey)
	c.Assert(err, IsNil)

	s.a = auth.NewAuthServer(s.bk, authority.New(), "localhost")

	// set up host private key and certificate
	c.Assert(s.a.UpsertCertAuthority(
		*services.NewTestCA(services.HostCA, "localhost"), backend.Forever), IsNil)

	hpriv, hpub, err := s.a.GenerateKeyPair("")
	c.Assert(err, IsNil)
	hcert, err := s.a.GenerateHostCert(hpub, "localhost", "localhost", teleport.RoleAdmin, 0)
	c.Assert(err, IsNil)

	// set up user CA and set up a user that has access to the server
	c.Assert(s.a.UpsertCertAuthority(
		*services.NewTestCA(services.UserCA, "localhost"), backend.Forever), IsNil)

	s.signer, err = sshutils.NewSigner(hpriv, hcert)
	c.Assert(err, IsNil)

	bl, err := boltlog.New(filepath.Join(s.dir, "eventsdb"))
	c.Assert(err, IsNil)

	rec, err := boltrec.New(s.dir)
	c.Assert(err, IsNil)
	s.roleAuth = auth.NewAuthWithRoles(s.a,
		auth.NewStandardPermissions(),
		bl,
		sess.New(baseBk),
		teleport.RoleAdmin,
		nil)

	// Starting node1
	s.srvAddress = "127.0.0.1:" + s.freePorts[len(s.freePorts)-1]
	s.freePorts = s.freePorts[:len(s.freePorts)-1]
	s.srv, err = srv.New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: s.srvAddress},
		"alice",
		[]ssh.Signer{s.signer},
		s.roleAuth,
		allowAllLimiter,
		s.dir,
		srv.SetShell("/bin/sh"),
		srv.SetLabels(
			map[string]string{"label1": "value1", "label2": "value2"},
			services.CommandLabels{
				"cmdLabel1": services.CommandLabel{
					Period:  time.Second,
					Command: []string{"expr", "1", "+", "3"}},
			},
		),
	)
	c.Assert(err, IsNil)
	c.Assert(s.srv.Start(), IsNil)

	// Starting node2
	s.srv2Address = "127.0.0.1:" + s.freePorts[len(s.freePorts)-1]
	s.freePorts = s.freePorts[:len(s.freePorts)-1]
	s.srv2, err = srv.New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: s.srv2Address},
		"bob",
		[]ssh.Signer{s.signer},
		s.roleAuth,
		allowAllLimiter,
		s.dir2,
		srv.SetShell("/bin/sh"),
		srv.SetLabels(
			map[string]string{"label1": "value1"},
			services.CommandLabels{
				"cmdLabel1": services.CommandLabel{
					Period:  time.Second,
					Command: []string{"expr", "1", "+", "4"},
				},
				"cmdLabel2": services.CommandLabel{
					Period:  time.Second,
					Command: []string{"expr", "1", "+", "5"},
				},
			},
		),
	)
	c.Assert(err, IsNil)
	c.Assert(s.srv2.Start(), IsNil)

	// Starting proxy
	reverseTunnelPort := s.freePorts[len(s.freePorts)-1]
	s.freePorts = s.freePorts[:len(s.freePorts)-1]
	c.Assert(err, IsNil)
	reverseTunnelAddress := utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:" + reverseTunnelPort}
	reverseTunnelServer, err := reversetunnel.NewServer(
		reverseTunnelAddress,
		[]ssh.Signer{s.signer},
		s.roleAuth, allowAllLimiter)
	c.Assert(err, IsNil)
	c.Assert(reverseTunnelServer.Start(), IsNil)

	s.proxyAddress = "localhost:" + s.freePorts[len(s.freePorts)-1]
	s.freePorts = s.freePorts[:len(s.freePorts)-1]
	s.proxy, err = srv.New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: s.proxyAddress},
		"localhost",
		[]ssh.Signer{s.signer},
		s.roleAuth,
		allowAllLimiter,
		s.dir,
		srv.SetProxyMode(reverseTunnelServer),
	)
	c.Assert(err, IsNil)
	c.Assert(s.proxy.Start(), IsNil)

	apiSrv := auth.NewAPIWithRoles(s.a, bl, sess.New(s.bk), rec,
		auth.NewAllowAllPermissions(),
		auth.StandardRoles,
	)
	go apiSrv.Serve()

	tsrv, err := auth.NewTunServer(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:0"},
		[]ssh.Signer{s.signer},
		apiSrv, s.a, allowAllLimiter)
	c.Assert(err, IsNil)
	c.Assert(tsrv.Start(), IsNil)

	u, err := user.Current()
	c.Assert(err, IsNil)
	s.user = u.Username
	s.pass = []byte("utndkrn")

	c.Assert(s.a.UpsertUser(services.User{Name: s.user, AllowedLogins: []string{s.user}}), IsNil)

	hotpURL, _, err := s.a.UpsertPassword(s.user, s.pass)
	c.Assert(err, IsNil)
	s.otp, _, err = hotp.FromURL(hotpURL)
	c.Assert(err, IsNil)
	s.otp.Increment()

	authMethod, err := auth.NewWebPasswordAuth(s.user, s.pass, s.otp.OTP())
	c.Assert(err, IsNil)

	tunClt, err := auth.NewTunClient(
		utils.NetAddr{AddrNetwork: "tcp", Addr: tsrv.Addr()}, s.user, authMethod)
	c.Assert(err, IsNil)

	rsAgent, err := reversetunnel.NewAgent(
		reverseTunnelAddress,
		"localhost",
		[]ssh.Signer{s.signer}, tunClt)
	c.Assert(err, IsNil)
	c.Assert(rsAgent.Start(), IsNil)

	webHandler, err := web.NewHandler(
		web.Config{
			Proxy:       reverseTunnelServer,
			AssetsDir:   "../../assets/web",
			AuthServers: utils.NetAddr{AddrNetwork: "tcp", Addr: tsrv.Addr()},
			DomainName:  "localhost",
		},
	)
	c.Assert(err, IsNil)

	s.webAddress = "localhost:31386"

	go func() {
		err := http.ListenAndServe(s.webAddress, webHandler)
		if err != nil {
			log.Errorf(err.Error())
		}
	}()

	s.teleagent = teleagent.NewTeleAgent()
	err = s.teleagent.Login("http://"+s.webAddress, s.user, string(s.pass), s.otp.OTP(), time.Minute)
	c.Assert(err, IsNil)

	_, err = ConnectToProxy(s.proxyAddress,
		[]ssh.AuthMethod{s.teleagent.AuthMethod()}, CheckHostSignerFromCache, s.user)
	c.Assert(err, NotNil)

	passwordCallback := func() (string, string, error) {
		return string(s.pass), s.otp.OTP(), nil
	}

	_, hostChecker := NewWebAuth(
		agent.NewKeyring(),
		s.user,
		passwordCallback,
		"http://"+s.webAddress,
		time.Hour,
	)

	_, err = ConnectToProxy(s.proxyAddress,
		[]ssh.AuthMethod{s.teleagent.AuthMethod()}, hostChecker, s.user)
	c.Assert(err, IsNil)

	_, err = ConnectToProxy(s.proxyAddress,
		[]ssh.AuthMethod{s.teleagent.AuthMethod()}, CheckHostSignerFromCache, s.user)
	c.Assert(err, IsNil)

	// "Command labels will be calculated only on the second heartbeat"
	time.Sleep(time.Millisecond * 3100)
}

func (s *ClientSuite) TestRunCommand(c *C) {
	nodeClient, err := ConnectToNode(nil, s.srvAddress,
		[]ssh.AuthMethod{s.teleagent.AuthMethod()}, CheckHostSignerFromCache, s.user)
	c.Assert(err, IsNil)

	buf := bytes.Buffer{}
	err = nodeClient.Run("expr 3 + 5", &buf)
	c.Assert(err, IsNil)
	c.Assert(buf.String(), Equals, "8\n")
}

func (s *ClientSuite) TestConnectViaProxy(c *C) {
	proxyClient, err := ConnectToProxy(s.proxyAddress,
		[]ssh.AuthMethod{s.teleagent.AuthMethod()}, CheckHostSignerFromCache, s.user)
	c.Assert(err, IsNil)

	nodeClient, err := proxyClient.ConnectToNode(s.srvAddress,
		[]ssh.AuthMethod{s.teleagent.AuthMethod()}, CheckHostSignerFromCache, s.user)
	c.Assert(err, IsNil)

	buf := bytes.Buffer{}
	err = nodeClient.Run("expr 3 + 6", &buf)
	c.Assert(err, IsNil)
	c.Assert(buf.String(), Equals, "9\n")
}

func (s *ClientSuite) TestConnectUsingSeveralAgents(c *C) {
	agent1 := agent.NewKeyring()
	agent2 := agent.NewKeyring()

	passwordCallback := func() (string, string, error) {
		return string(s.pass), s.otp.OTP(), nil
	}

	_, err := ConnectToProxy(
		s.proxyAddress,
		[]ssh.AuthMethod{
			AuthMethodFromAgent(agent1),
			AuthMethodFromAgent(agent2),
		}, CheckHostSignerFromCache, s.user)
	c.Assert(err, NotNil)

	webAuth, hostChecker := NewWebAuth(
		agent2,
		s.user,
		passwordCallback,
		"http://"+s.webAddress,
		time.Hour,
	)
	proxyClient, err := ConnectToProxy(
		s.proxyAddress,
		[]ssh.AuthMethod{
			AuthMethodFromAgent(agent1),
			AuthMethodFromAgent(agent2),
			webAuth,
		},
		hostChecker, s.user)
	c.Assert(err, IsNil)

	nodeClient, err := proxyClient.ConnectToNode(
		s.srvAddress,
		[]ssh.AuthMethod{
			AuthMethodFromAgent(agent1),
			AuthMethodFromAgent(agent2),
		},
		CheckHostSignerFromCache, s.user)
	c.Assert(err, IsNil)

	buf := bytes.Buffer{}
	err = nodeClient.Run("expr 3 + 6", &buf)
	c.Assert(err, IsNil)
	c.Assert(buf.String(), Equals, "9\n")

	nodeClient, err = ConnectToNode(
		nil,
		s.srvAddress,
		[]ssh.AuthMethod{
			AuthMethodFromAgent(agent1),
			AuthMethodFromAgent(agent2),
		},
		CheckHostSignerFromCache, s.user)
	c.Assert(err, IsNil)

	buf = bytes.Buffer{}
	err = nodeClient.Run("expr 3 + 6", &buf)
	c.Assert(err, IsNil)
	c.Assert(buf.String(), Equals, "9\n")
}

func (s *ClientSuite) TestShell(c *C) {
	proxyClient, err := ConnectToProxy(s.proxyAddress,
		[]ssh.AuthMethod{s.teleagent.AuthMethod()}, CheckHostSignerFromCache, s.user)
	c.Assert(err, IsNil)

	nodeClient, err := proxyClient.ConnectToNode(s.srvAddress,
		[]ssh.AuthMethod{s.teleagent.AuthMethod()}, CheckHostSignerFromCache, s.user)
	c.Assert(err, IsNil)

	shell, err := nodeClient.Shell(100, 100, "")
	c.Assert(err, IsNil)

	shellReader := bufio.NewReader(shell)

	out, err := shellReader.ReadString('$')
	c.Assert(err, IsNil)
	c.Assert(out, Equals, "$")
	// run first command
	_, err = shell.Write([]byte("expr 11 + 22\n"))
	c.Assert(err, IsNil)

	out, err = shellReader.ReadString('$')
	c.Assert(err, IsNil)
	c.Assert(out, Equals, " expr 11 + 22\r\n33\r\n$")

	// run second command
	_, err = shell.Write([]byte("expr 2 + 3\n"))
	c.Assert(err, IsNil)

	out, err = shellReader.ReadString('$')
	c.Assert(err, IsNil)
	c.Assert(out, Equals, " expr 2 + 3\r\n5\r\n$")

	c.Assert(shell.Close(), IsNil)
}

func (s *ClientSuite) TestSharedShellSession(c *C) {
	proxyClient1, err := ConnectToProxy(s.proxyAddress,
		[]ssh.AuthMethod{s.teleagent.AuthMethod()}, CheckHostSignerFromCache, s.user)
	c.Assert(err, IsNil)

	nodeClient1, err := proxyClient1.ConnectToNode(s.srvAddress,
		[]ssh.AuthMethod{s.teleagent.AuthMethod()}, CheckHostSignerFromCache, s.user)
	c.Assert(err, IsNil)

	shell1, err := nodeClient1.Shell(100, 100, "shell123")
	c.Assert(err, IsNil)
	shell1Reader := bufio.NewReader(shell1)

	out, err := shell1Reader.ReadString('$')
	c.Assert(err, IsNil)
	c.Assert(out, Equals, "$")
	// run first command
	_, err = shell1.Write([]byte("expr 11 + 22\n"))
	c.Assert(err, IsNil)

	out, err = shell1Reader.ReadString('$')
	c.Assert(err, IsNil)
	c.Assert(out, Equals, " expr 11 + 22\r\n33\r\n$")

	proxyClient2, err := ConnectToProxy(s.proxyAddress,
		[]ssh.AuthMethod{s.teleagent.AuthMethod()}, CheckHostSignerFromCache, s.user)
	c.Assert(err, IsNil)

	nodeClient2, err := proxyClient2.ConnectToNode(s.srvAddress,
		[]ssh.AuthMethod{s.teleagent.AuthMethod()}, CheckHostSignerFromCache, s.user)
	c.Assert(err, IsNil)

	shell2, err := nodeClient2.Shell(100, 100, "shell123")
	c.Assert(err, IsNil)
	shell2Reader := bufio.NewReader(shell2)
	// run second command
	_, err = shell1.Write([]byte("expr 2 + 3\n"))
	c.Assert(err, IsNil)

	out, err = shell1Reader.ReadString('$')
	c.Assert(err, IsNil)
	c.Assert(out, Equals, " expr 2 + 3\r\n5\r\n$")

	out, err = shell2Reader.ReadString('$')
	c.Assert(err, IsNil)
	c.Assert(out, Equals, "expr 2 + 3\r\n5\r\n$")

	// run third command
	_, err = shell2.Write([]byte("expr 6 + 2\n"))
	c.Assert(err, IsNil)

	out, err = shell1Reader.ReadString('$')
	c.Assert(err, IsNil)
	c.Assert(out, Equals, " expr 6 + 2\r\n8\r\n$")

	out, err = shell2Reader.ReadString('$')
	c.Assert(err, IsNil)
	c.Assert(out, Equals, " expr 6 + 2\r\n8\r\n$")

	c.Assert(shell1.Close(), IsNil)
	c.Assert(shell2.Close(), IsNil)
}

func (s *ClientSuite) TestGetServer(c *C) {
	proxyClient, err := ConnectToProxy(s.proxyAddress,
		[]ssh.AuthMethod{s.teleagent.AuthMethod()}, CheckHostSignerFromCache, s.user)
	c.Assert(err, IsNil)

	server1Info := services.Server{
		ID:       s.srvAddress,
		Addr:     s.srvAddress,
		Hostname: "alice",
		Labels: map[string]string{
			"label1": "value1",
			"label2": "value2",
		},
		CmdLabels: map[string]services.CommandLabel{
			"cmdLabel1": services.CommandLabel{
				Period:  time.Second,
				Command: []string{"expr", "1", "+", "3"},
				Result:  "4",
			},
		},
	}

	server2Info := services.Server{
		ID:       s.srv2Address,
		Addr:     s.srv2Address,
		Hostname: "bob",
		Labels: map[string]string{
			"label1": "value1",
		},
		CmdLabels: map[string]services.CommandLabel{
			"cmdLabel1": services.CommandLabel{
				Period:  time.Second,
				Command: []string{"expr", "1", "+", "4"},
				Result:  "5",
			},
			"cmdLabel2": services.CommandLabel{
				Period:  time.Second,
				Command: []string{"expr", "1", "+", "5"},
				Result:  "6",
			},
		},
	}

	servers, err := proxyClient.GetServers()
	c.Assert(err, IsNil)
	if servers[0].Addr == server1Info.Addr {
		c.Assert(servers, DeepEquals, []services.Server{
			server1Info,
			server2Info,
		})
	} else {
		c.Assert(servers, DeepEquals, []services.Server{
			server2Info,
			server1Info,
		})
	}

	servers, err = proxyClient.FindServers("label1", "value1")
	c.Assert(err, IsNil)
	if servers[0].Addr == server1Info.Addr {
		c.Assert(servers, DeepEquals, []services.Server{
			server1Info,
			server2Info,
		})
	} else {
		c.Assert(servers, DeepEquals, []services.Server{
			server2Info,
			server1Info,
		})
	}

	servers, err = proxyClient.FindServers("label1", "val.*")
	c.Assert(err, IsNil)
	if servers[0].Addr == server1Info.Addr {
		c.Assert(servers, DeepEquals, []services.Server{
			server1Info,
			server2Info,
		})
	} else {
		c.Assert(servers, DeepEquals, []services.Server{
			server2Info,
			server1Info,
		})
	}

	servers, err = proxyClient.FindServers("label2", ".*ue2")
	c.Assert(err, IsNil)
	c.Assert(servers, DeepEquals, []services.Server{
		server1Info,
	})

	servers, err = proxyClient.FindServers("cmdLabel1", "4")
	c.Assert(err, IsNil)
	c.Assert(servers, DeepEquals, []services.Server{
		server1Info,
	})

	servers, err = proxyClient.FindServers("cmdLabel1", "5")
	c.Assert(err, IsNil)
	c.Assert(servers, DeepEquals, []services.Server{
		server2Info,
	})

	servers, err = proxyClient.FindServers("cmdLabel2", "6")
	c.Assert(err, IsNil)
	c.Assert(servers, DeepEquals, []services.Server{
		server2Info,
	})

}

func (s *ClientSuite) TestUploadFile(c *C) {
	proxyClient, err := ConnectToProxy(s.proxyAddress,
		[]ssh.AuthMethod{s.teleagent.AuthMethod()}, CheckHostSignerFromCache, s.user)
	c.Assert(err, IsNil)

	nodeClient, err := proxyClient.ConnectToNode(s.srvAddress,
		[]ssh.AuthMethod{s.teleagent.AuthMethod()}, CheckHostSignerFromCache, s.user)
	c.Assert(err, IsNil)

	dir := c.MkDir()
	sourceFileName := filepath.Join(dir, "file1")
	contents := []byte("hello world!")

	err = ioutil.WriteFile(sourceFileName, contents, 0666)
	c.Assert(err, IsNil)

	destinationFileName := filepath.Join(dir, "file2")
	c.Assert(nodeClient.Upload(sourceFileName, destinationFileName), IsNil)

	bytes, err := ioutil.ReadFile(destinationFileName)
	c.Assert(err, IsNil)
	c.Assert(string(bytes), Equals, string(contents))
}

func (s *ClientSuite) TestDownloadFile(c *C) {
	proxyClient, err := ConnectToProxy(s.proxyAddress,
		[]ssh.AuthMethod{s.teleagent.AuthMethod()}, CheckHostSignerFromCache, s.user)
	c.Assert(err, IsNil)

	nodeClient, err := proxyClient.ConnectToNode(s.srvAddress,
		[]ssh.AuthMethod{s.teleagent.AuthMethod()}, CheckHostSignerFromCache, s.user)
	c.Assert(err, IsNil)

	dir := c.MkDir()
	sourceFileName := filepath.Join(dir, "file3")
	contents := []byte("world hello")

	err = ioutil.WriteFile(sourceFileName, contents, 0666)
	c.Assert(err, IsNil)

	destinationFileName := filepath.Join(dir, "file4")
	c.Assert(nodeClient.Download(sourceFileName, destinationFileName, false), IsNil)

	bytes, err := ioutil.ReadFile(destinationFileName)
	c.Assert(err, IsNil)
	c.Assert(string(bytes), Equals, string(contents))
}

func (s *ClientSuite) TestUploadDir(c *C) {
	nodeClient, err := ConnectToNode(nil, s.srvAddress,
		[]ssh.AuthMethod{s.teleagent.AuthMethod()}, CheckHostSignerFromCache, s.user)
	c.Assert(err, IsNil)

	dir1 := c.MkDir()
	dir2 := c.MkDir()
	sourceFileName1 := filepath.Join(dir1, "file1")
	sourceFileName2 := filepath.Join(dir1, "file2")
	contents1 := []byte("this is content 1")
	contents2 := []byte("this is content 2")

	err = ioutil.WriteFile(sourceFileName1, contents1, 0666)
	c.Assert(err, IsNil)
	err = ioutil.WriteFile(sourceFileName2, contents2, 0666)
	c.Assert(err, IsNil)

	destinationFileName1 := filepath.Join(dir2, "subdir", "file1")
	destinationFileName2 := filepath.Join(dir2, "subdir", "file2")

	c.Assert(nodeClient.Upload(dir1, dir2+"/subdir"), IsNil)

	bytes, err := ioutil.ReadFile(destinationFileName1)
	c.Assert(err, IsNil)
	c.Assert(string(bytes), Equals, string(contents1))
	bytes, err = ioutil.ReadFile(destinationFileName2)
	c.Assert(err, IsNil)
	c.Assert(string(bytes), Equals, string(contents2))
}

func (s *ClientSuite) TestDownloadDir(c *C) {
	nodeClient, err := ConnectToNode(nil, s.srvAddress,
		[]ssh.AuthMethod{s.teleagent.AuthMethod()}, CheckHostSignerFromCache, s.user)
	c.Assert(err, IsNil)

	dir1 := c.MkDir()
	dir2 := c.MkDir()
	sourceFileName1 := filepath.Join(dir1, "file1")
	sourceFileName2 := filepath.Join(dir1, "file2")
	contents1 := []byte("this is content 1")
	contents2 := []byte("this is content 2")

	err = ioutil.WriteFile(sourceFileName1, contents1, 0666)
	c.Assert(err, IsNil)
	err = ioutil.WriteFile(sourceFileName2, contents2, 0666)
	c.Assert(err, IsNil)

	destinationFileName1 := filepath.Join(dir2, "subdir", "file1")
	destinationFileName2 := filepath.Join(dir2, "subdir", "file2")

	c.Assert(nodeClient.Download(dir1, dir2+"/subdir", true), IsNil)

	bytes, err := ioutil.ReadFile(destinationFileName1)
	c.Assert(err, IsNil)
	c.Assert(string(bytes), Equals, string(contents1))
	bytes, err = ioutil.ReadFile(destinationFileName2)
	c.Assert(err, IsNil)
	c.Assert(string(bytes), Equals, string(contents2))
}

func (s *ClientSuite) TestHOTPMock(c *C) {
	hotpMock, err := CreateHOTPMock(s.otp.URL(""))
	c.Assert(err, IsNil)

	teleagent := teleagent.NewTeleAgent()
	err = teleagent.Login("http://"+s.webAddress, s.user, string(s.pass), "123456", time.Minute)
	c.Assert(err, NotNil)

	err = teleagent.Login("http://"+s.webAddress, s.user, string(s.pass), hotpMock.OTP(), time.Minute)
	c.Assert(err, IsNil)

	path := filepath.Join(s.dir, "hotpmock")
	c.Assert(hotpMock.SaveToFile(path), IsNil)

	token, err := GetTokenFromHOTPMockFile(path)
	c.Assert(err, IsNil)
	err = teleagent.Login("http://"+s.webAddress, s.user, string(s.pass), token, time.Minute)
	c.Assert(err, IsNil)

	token, err = GetTokenFromHOTPMockFile(path)
	c.Assert(err, IsNil)
	err = teleagent.Login("http://"+s.webAddress, s.user, string(s.pass), token, time.Minute)
	c.Assert(err, IsNil)

	hotpMock, err = LoadHOTPMockFromFile(path)
	c.Assert(err, IsNil)
	err = teleagent.Login("http://"+s.webAddress, s.user, string(s.pass), hotpMock.OTP(), time.Minute)
	c.Assert(err, IsNil)

	hotpMock, err = LoadHOTPMockFromFile(path)
	c.Assert(err, IsNil)
	err = teleagent.Login("http://"+s.webAddress, s.user, string(s.pass), hotpMock.OTP(), time.Minute)
	c.Assert(err, NotNil)

}

func (s *ClientSuite) TestParseTargetObject(c *C) {
	addresses, err := ParseTargetServers(s.srv2Address, s.user, s.proxyAddress, []ssh.AuthMethod{s.teleagent.AuthMethod()}, CheckHostSignerFromCache)
	c.Assert(err, IsNil)
	c.Assert(addresses, DeepEquals, []string{s.srv2Address})

	addresses, err = ParseTargetServers("_label1:val.*", s.user, s.proxyAddress, []ssh.AuthMethod{s.teleagent.AuthMethod()}, CheckHostSignerFromCache)
	c.Assert(err, IsNil)
	c.Assert(len(addresses), Equals, 2)
	if addresses[0] == s.srvAddress {
		c.Assert(addresses, DeepEquals, []string{s.srvAddress, s.srv2Address})
	} else {
		c.Assert(addresses, DeepEquals, []string{s.srv2Address, s.srvAddress})
	}

	addresses, err = ParseTargetServers("_label2:value2*", s.user, s.proxyAddress, []ssh.AuthMethod{s.teleagent.AuthMethod()}, CheckHostSignerFromCache)
	c.Assert(err, IsNil)
	c.Assert(addresses, DeepEquals, []string{s.srvAddress})

}

func (s *ClientSuite) TestSplitUserAndAddress(c *C) {
	user, addr := SplitUserAndAddress("user@address")
	c.Assert(user, Equals, "user")
	c.Assert(addr, Equals, "address")

	user, addr = SplitUserAndAddress("abcd:1234")
	c.Assert(user, Equals, "")
	c.Assert(addr, Equals, "abcd:1234")

	user, addr = SplitUserAndAddress("user@gmail.com@address")
	c.Assert(user, Equals, "user@gmail.com")
	c.Assert(addr, Equals, "address")
}
