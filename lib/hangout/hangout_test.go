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
package hangout

import (
	"bufio"
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
	"github.com/gravitational/teleport/lib/client"
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

func TestHangouts(t *testing.T) { TestingT(t) }

type HangoutsSuite struct {
	proxy                *srv.Server
	proxyAddress         string
	reverseTunnelAddress string
	webAddress           string
	bk                   *encryptedbk.ReplicatedBackend
	a                    *auth.AuthServer
	roleAuth             *auth.AuthWithRoles
	signer               ssh.Signer
	teleagent            *teleagent.TeleAgent
	otp                  *hotp.HOTP
	user                 string
	pass                 []byte
	dir                  string
}

var _ = Suite(&HangoutsSuite{})

func (s *HangoutsSuite) SetUpSuite(c *C) {
	utils.InitLoggerCLI()
	client.KeysDir = c.MkDir()
	s.dir = c.MkDir()
	allowAllLimiter, err := limiter.NewLimiter(limiter.LimiterConfig{})

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

	s.roleAuth = auth.NewAuthWithRoles(s.a,
		auth.NewStandardPermissions(),
		bl,
		sess.New(baseBk),
		teleport.RoleAdmin,
		nil)

	// Starting proxy
	reverseTunnelAddress := utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:34057"}
	s.reverseTunnelAddress = reverseTunnelAddress.Addr
	reverseTunnelServer, err := reversetunnel.NewServer(
		reverseTunnelAddress,
		[]ssh.Signer{s.signer},
		s.roleAuth, allowAllLimiter)
	c.Assert(err, IsNil)
	c.Assert(reverseTunnelServer.Start(), IsNil)

	s.proxyAddress = "localhost:35783"

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

	rec, err := boltrec.New(s.dir)
	c.Assert(err, IsNil)

	apiSrv := auth.NewAPIWithRoles(s.a, bl, sess.New(s.bk), rec,
		auth.NewAllowAllPermissions(),
		auth.StandardRoles,
	)
	go apiSrv.Serve()

	tsrv, err := auth.NewTunServer(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:32498"},
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

	webHandler, err := web.NewMultiSiteHandler(
		web.MultiSiteConfig{
			Tun:        reverseTunnelServer,
			AssetsDir:  "../../assets/web",
			AuthAddr:   utils.NetAddr{AddrNetwork: "tcp", Addr: tsrv.Addr()},
			DomainName: "localhost",
		},
	)
	c.Assert(err, IsNil)

	s.webAddress = "localhost:35386"

	go func() {
		err := http.ListenAndServe(s.webAddress, webHandler)
		if err != nil {
			log.Errorf(err.Error())
		}
	}()

	s.teleagent = teleagent.NewTeleAgent()
	err = s.teleagent.Login("http://"+s.webAddress, s.user, string(s.pass), s.otp.OTP(), time.Minute)
	c.Assert(err, IsNil)

	_, err = client.ConnectToProxy(s.proxyAddress,
		[]ssh.AuthMethod{s.teleagent.AuthMethod()}, client.CheckHostSignerFromCache, s.user)
	c.Assert(err, NotNil)

	passwordCallback := func() (string, string, error) {
		return string(s.pass), s.otp.OTP(), nil
	}

	_, hostChecker := client.NewWebAuth(
		agent.NewKeyring(),
		s.user,
		passwordCallback,
		"http://"+s.webAddress,
		time.Hour,
	)

	_, err = client.ConnectToProxy(s.proxyAddress,
		[]ssh.AuthMethod{s.teleagent.AuthMethod()}, hostChecker, s.user)
	c.Assert(err, IsNil)

	_, err = client.ConnectToProxy(s.proxyAddress,
		[]ssh.AuthMethod{s.teleagent.AuthMethod()}, client.CheckHostSignerFromCache, s.user)
	c.Assert(err, IsNil)

}

func (s *HangoutsSuite) TestHangout(c *C) {
	u, err := user.Current()
	c.Assert(err, IsNil)
	osUser := u.Username
	nodeListeningAddress := "localhost:41003"
	authListeningAddress := "localhost:41004"
	DefaultSSHShell = "/bin/sh"

	// Initializing tsh share
	hangoutServer, err := New(s.reverseTunnelAddress, nodeListeningAddress,
		authListeningAddress, false, []ssh.AuthMethod{s.teleagent.AuthMethod()}, client.CheckHostSignerFromCache)
	c.Assert(err, IsNil)

	_, err = client.ConnectToNode(nil, nodeListeningAddress,
		[]ssh.AuthMethod{s.teleagent.AuthMethod()}, client.CheckHostSignerFromCache, osUser)
	c.Assert(err, NotNil)

	nodeClient1, err := client.ConnectToNode(nil, nodeListeningAddress,
		[]ssh.AuthMethod{hangoutServer.ClientAuthMethod}, hangoutServer.HostKeyCallback, osUser)
	c.Assert(err, IsNil)

	shell1, err := nodeClient1.Shell(100, 100, "hangoutSession")
	c.Assert(err, IsNil)
	shell1Reader := bufio.NewReader(shell1)
	// tsh share was initialized

	out, err := shell1Reader.ReadString('$')
	c.Assert(err, IsNil)
	c.Assert(out, Equals, "$")
	// run first command
	_, err = shell1.Write([]byte("expr 11 + 22\n"))
	c.Assert(err, IsNil)

	out, err = shell1Reader.ReadString('$')
	c.Assert(err, IsNil)
	c.Assert(out, Equals, " expr 11 + 22\r\n33\r\n$")

	for i := 0; i < 3; i++ {

		// Initializing tsh join
		proxy, err := client.ConnectToProxy(s.proxyAddress,
			[]ssh.AuthMethod{s.teleagent.AuthMethod()}, client.CheckHostSignerFromCache, "anyuser")
		c.Assert(err, IsNil)

		authConn, err := proxy.ConnectToHangout(hangoutServer.HangoutID+":"+utils.HangoutAuthPortAlias, []ssh.AuthMethod{s.teleagent.AuthMethod()})
		c.Assert(err, IsNil)

		authClient, err := auth.NewClientFromSSHClient(authConn.Client)
		c.Assert(err, IsNil)

		nodeAuthMethod, err := Authorize(authClient)
		c.Assert(err, IsNil)

		c.Assert(authConn.Close(), IsNil)

		nodeConn, err := proxy.ConnectToHangout(hangoutServer.HangoutID+":"+utils.HangoutNodePortAlias, []ssh.AuthMethod{nodeAuthMethod})
		c.Assert(err, IsNil)

		shell2, err := nodeConn.Shell(100, 100, "hangoutSession")
		c.Assert(err, IsNil)
		shell2Reader := bufio.NewReader(shell2)
		// tsh join initialized

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

		c.Assert(shell2.Close(), IsNil)
		c.Assert(nodeConn.Close(), IsNil)
		c.Assert(proxy.Close(), IsNil)
	}
	c.Assert(shell1.Close(), IsNil)
}
