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
package tsh

import (
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/auth"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
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

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gokyle/hotp"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/secret"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
)

func TestTsh(t *testing.T) { TestingT(t) }

type TshSuite struct {
	srv          *srv.Server
	srv2         *srv.Server
	proxy        *srv.Server
	srvAddress   string
	srv2Address  string
	proxyAddress string
	webAddress   string
	agentAddr    string
	agentAddrEnv string
	clt          *ssh.Client
	bk           *encryptedbk.ReplicatedBackend
	a            *auth.AuthServer
	scrt         secret.SecretService
	signer       ssh.Signer
	teleagent    *teleagent.TeleAgent
	dir          string
	dir2         string
	otp          *hotp.HOTP
	user         string
	pass         []byte
	envVars      []string
}

var _ = Suite(&TshSuite{})

func (s *TshSuite) SetUpSuite(c *C) {
	key, err := secret.NewKey()
	c.Assert(err, IsNil)
	scrt, err := secret.New(&secret.Config{KeyBytes: key})
	c.Assert(err, IsNil)
	s.scrt = scrt

	s.dir = c.MkDir()
	s.dir2 = c.MkDir()

	allowAllLimiter, err := limiter.NewLimiter(limiter.LimiterConfig{})

	baseBk, err := boltbk.New(filepath.Join(s.dir, "db"))
	c.Assert(err, IsNil)
	s.bk, err = encryptedbk.NewReplicatedBackend(baseBk,
		filepath.Join(s.dir, "keys"), nil,
		encryptor.GetTestKey)
	c.Assert(err, IsNil)

	s.a = auth.NewAuthServer(s.bk, authority.New(), s.scrt, "host5")

	// set up host private key and certificate
	c.Assert(s.a.ResetHostCertificateAuthority(""), IsNil)
	hpriv, hpub, err := s.a.GenerateKeyPair("")
	c.Assert(err, IsNil)
	hcert, err := s.a.GenerateHostCert(hpub, "localhost", "localhost", auth.RoleAdmin, 0)
	c.Assert(err, IsNil)

	// set up user CA and set up a user that has access to the server
	c.Assert(s.a.ResetUserCertificateAuthority(""), IsNil)

	s.signer, err = sshutils.NewSigner(hpriv, hcert)
	c.Assert(err, IsNil)

	ap := auth.NewBackendAccessPoint(s.bk)

	// Starting node1
	s.srvAddress = "127.0.0.1:30136"
	s.srv, err = srv.New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: s.srvAddress},
		"localhost",
		[]ssh.Signer{s.signer},
		ap,
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
	s.srv2Address = "127.0.0.1:30983"
	s.srv2, err = srv.New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: s.srv2Address},
		"localhost",
		[]ssh.Signer{s.signer},
		ap,
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
	reverseTunnelAddress := utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:33736"}
	reverseTunnelServer, err := reversetunnel.NewServer(
		reverseTunnelAddress,
		[]ssh.Signer{s.signer},
		ap, allowAllLimiter)
	c.Assert(err, IsNil)
	c.Assert(reverseTunnelServer.Start(), IsNil)

	s.proxyAddress = "localhost:34284"

	s.proxy, err = srv.New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: s.proxyAddress},
		"localhost",
		[]ssh.Signer{s.signer},
		ap,
		allowAllLimiter,
		s.dir,
		srv.SetProxyMode(reverseTunnelServer),
	)
	c.Assert(err, IsNil)
	c.Assert(s.proxy.Start(), IsNil)

	bl, err := boltlog.New(filepath.Join(s.dir, "eventsdb"))
	c.Assert(err, IsNil)

	rec, err := boltrec.New(s.dir)
	c.Assert(err, IsNil)

	apiSrv := auth.NewAPIWithRoles(s.a, bl, sess.New(s.bk), rec,
		auth.NewAllowAllPermissions(),
		auth.StandardRoles,
	)
	apiSrv.Serve()

	tsrv, err := auth.NewTunServer(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:31948"},
		[]ssh.Signer{s.signer},
		apiSrv, s.a, allowAllLimiter)
	c.Assert(err, IsNil)
	c.Assert(tsrv.Start(), IsNil)

	s.user = "user1"
	s.pass = []byte("utndkrn")

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
			AssetsDir:  "../../../assets/web",
			AuthAddr:   utils.NetAddr{AddrNetwork: "tcp", Addr: tsrv.Addr()},
			DomainName: "localhost",
		},
	)
	c.Assert(err, IsNil)

	s.webAddress = "localhost:31236"

	go func() {
		err := http.ListenAndServe(s.webAddress, webHandler)
		if err != nil {
			log.Errorf(err.Error())
		}
	}()

	s.teleagent = teleagent.NewTeleAgent()
	s.agentAddr = filepath.Join(s.dir, "agent.sock")
	c.Assert(s.teleagent.Start("unix://"+s.agentAddr), IsNil)
	err = s.teleagent.Login("http://"+s.webAddress, s.user, string(s.pass), s.otp.OTP(), time.Minute)
	c.Assert(err, IsNil)

	s.envVars = append([]string{"SSH_AUTH_SOCK=" + s.agentAddr}, os.Environ()...)
	s.agentAddrEnv = "SSH_AUTH_SOCK=" + s.agentAddrEnv + "; export SSH_AUTH_SOCK;"
	// "Command labels will be calculated only on the second heartbeat"
	time.Sleep(time.Millisecond * 3100)
}

func (s *TshSuite) TestRunCommand(c *C) {
	cmd := exec.Command("tsh",
		"connect", s.srvAddress, "--user="+s.user,
		`--command=""expr 3 + 5""`)
	cmd.Env = s.envVars
	out, err := cmd.Output()
	c.Assert(err, IsNil)

	c.Assert(string(out), Equals, "8\n")
}

func (s *TshSuite) TestRunCommandWithProxy(c *C) {
	cmd := exec.Command("tsh",
		"connect", s.srvAddress, "--user="+s.user,
		"--proxy="+s.proxyAddress,
		`--command=""expr 3 + 5""`)
	cmd.Env = s.envVars
	out, err := cmd.Output()
	c.Assert(err, IsNil)

	c.Assert(string(out), Equals, "8\n")
}

func (s *TshSuite) TestUploadFile(c *C) {
	dir := c.MkDir()
	sourceFileName := filepath.Join(dir, "file1")
	contents := []byte("hello world!")

	err := ioutil.WriteFile(sourceFileName, contents, 0666)
	c.Assert(err, IsNil)

	destinationFileName := filepath.Join(dir, "file2")

	cmd := exec.Command("tsh",
		"upload", s.srvAddress, "--user="+s.user,
		"--proxy="+s.proxyAddress,
		"--source="+sourceFileName,
		"--dest="+destinationFileName)
	cmd.Env = s.envVars
	out, err := cmd.Output()
	if err != nil {
		c.Assert(string(out), Equals, "")
		c.Assert(err, IsNil)
	}

	bytes, err := ioutil.ReadFile(destinationFileName)
	c.Assert(err, IsNil)
	c.Assert(string(bytes), Equals, string(contents))
}

func (s *TshSuite) TestDownloadFile(c *C) {
	dir := c.MkDir()
	sourceFileName := filepath.Join(dir, "file3")
	contents := []byte("world hello")

	err := ioutil.WriteFile(sourceFileName, contents, 0666)
	c.Assert(err, IsNil)

	destinationFileName := filepath.Join(dir, "file4")
	cmd := exec.Command("tsh",
		"upload", s.srvAddress, "--user="+s.user,
		"--proxy="+s.proxyAddress,
		"--source="+sourceFileName,
		"--dest="+destinationFileName)
	cmd.Env = s.envVars
	out, err := cmd.Output()
	if err != nil {
		c.Assert(string(out), Equals, "")
		c.Assert(err, IsNil)
	}

	bytes, err := ioutil.ReadFile(destinationFileName)
	c.Assert(err, IsNil)
	c.Assert(string(bytes), Equals, string(contents))
}

func (s *TshSuite) TestUploadDir(c *C) {
	dir1 := c.MkDir()
	dir2 := c.MkDir()
	sourceFileName1 := filepath.Join(dir1, "file1")
	sourceFileName2 := filepath.Join(dir1, "file2")
	contents1 := []byte("this is content 1")
	contents2 := []byte("this is content 2")

	err := ioutil.WriteFile(sourceFileName1, contents1, 0666)
	c.Assert(err, IsNil)
	err = ioutil.WriteFile(sourceFileName2, contents2, 0666)
	c.Assert(err, IsNil)

	destinationFileName1 := filepath.Join(dir2, "subdir", "file1")
	destinationFileName2 := filepath.Join(dir2, "subdir", "file2")

	cmd := exec.Command("tsh",
		"upload", s.srvAddress, "--user="+s.user,
		"--proxy="+s.proxyAddress,
		"--source="+dir1,
		"--dest="+dir2+"/subdir")
	cmd.Env = s.envVars
	out, err := cmd.Output()
	if err != nil {
		c.Assert(string(out), Equals, "")
		c.Assert(err, IsNil)
	}

	bytes, err := ioutil.ReadFile(destinationFileName1)
	c.Assert(err, IsNil)
	c.Assert(string(bytes), Equals, string(contents1))
	bytes, err = ioutil.ReadFile(destinationFileName2)
	c.Assert(err, IsNil)
	c.Assert(string(bytes), Equals, string(contents2))
}

func (s *TshSuite) TestDownloadDir(c *C) {
	dir1 := c.MkDir()
	dir2 := c.MkDir()
	sourceFileName1 := filepath.Join(dir1, "file1")
	sourceFileName2 := filepath.Join(dir1, "file2")
	contents1 := []byte("this is content 1")
	contents2 := []byte("this is content 2")

	err := ioutil.WriteFile(sourceFileName1, contents1, 0666)
	c.Assert(err, IsNil)
	err = ioutil.WriteFile(sourceFileName2, contents2, 0666)
	c.Assert(err, IsNil)

	destinationFileName1 := filepath.Join(dir2, "subdir", "file1")
	destinationFileName2 := filepath.Join(dir2, "subdir", "file2")

	cmd := exec.Command("tsh",
		"download", s.srvAddress, "--user="+s.user,
		"--proxy="+s.proxyAddress,
		"--source="+dir1,
		"--dest="+dir2+"/subdir",
		"--r")
	cmd.Env = s.envVars
	out, err := cmd.Output()
	if err != nil {
		c.Assert(string(out), Equals, "")
		c.Assert(err, IsNil)
	}

	bytes, err := ioutil.ReadFile(destinationFileName1)
	c.Assert(err, IsNil)
	c.Assert(string(bytes), Equals, string(contents1))
	bytes, err = ioutil.ReadFile(destinationFileName2)
	c.Assert(err, IsNil)
	c.Assert(string(bytes), Equals, string(contents2))
}
