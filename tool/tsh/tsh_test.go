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
package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
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
	. "gopkg.in/check.v1"

	log "github.com/Sirupsen/logrus"
)

func TestTsh(t *testing.T) { TestingT(t) }

type TshSuite struct {
	srv          *srv.Server
	srv2         *srv.Server
	srv3         *exec.Cmd
	srv4         *exec.Cmd
	proxy        *srv.Server
	srvAddress   string
	srvHost      string
	srvPort      string
	srv2Address  string
	srv3Address  string
	srv3Dir      string
	srv4Address  string
	srv4Dir      string
	proxyAddress string
	webAddress   string
	agentAddr    string
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
	envVars      []string
	userDir      string
}

var _ = Suite(&TshSuite{})

func (s *TshSuite) SetUpSuite(c *C) {
	utils.InitLoggerCLI()
	client.KeysDir = c.MkDir()

	s.dir = c.MkDir()
	s.dir2 = c.MkDir()

	allowAllLimiter, err := limiter.NewLimiter(limiter.LimiterConfig{})

	baseBk, err := boltbk.New(filepath.Join(s.dir, "db"))
	c.Assert(err, IsNil)
	s.bk, err = encryptedbk.NewReplicatedBackend(baseBk,
		filepath.Join(s.dir, "keys"), nil,
		encryptor.GetTestKey)
	c.Assert(err, IsNil)

	s.a = auth.NewAuthServer(s.bk, authority.New(), "localhost")

	eventsLog, err := boltlog.New(filepath.Join(s.dir, "boltlog"))
	c.Assert(err, IsNil)

	s.roleAuth = auth.NewAuthWithRoles(s.a,
		auth.NewStandardPermissions(),
		eventsLog,
		sess.New(baseBk),
		teleport.RoleAdmin,
		nil)

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

	// Starting node1
	s.srvAddress = "127.0.0.1:30136"
	s.srvHost = "127.0.0.1"
	s.srvPort = "30136"

	s.srv, err = srv.New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: s.srvAddress},
		"localhost",
		[]ssh.Signer{s.signer},
		s.roleAuth,
		allowAllLimiter,
		s.dir,
		srv.SetShell("/bin/sh"),
		srv.SetLabels(
			map[string]string{
				"label1": "value1",
				"label2": "value2",
				"label3": "value3",
			},
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
		s.roleAuth,
		allowAllLimiter,
		s.dir2,
		srv.SetShell("/bin/sh"),
		srv.SetLabels(
			map[string]string{"label1": "value1", "label3": "value4"},
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
		s.roleAuth, allowAllLimiter)
	c.Assert(err, IsNil)
	c.Assert(reverseTunnelServer.Start(), IsNil)

	s.proxyAddress = "localhost:34284"

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

	bl, err := boltlog.New(filepath.Join(s.dir, "eventsdb"))
	c.Assert(err, IsNil)

	rec, err := boltrec.New(s.dir)
	c.Assert(err, IsNil)

	apiSrv := auth.NewAPIWithRoles(s.a, bl, sess.New(s.bk), rec,
		auth.NewAllowAllPermissions(),
		auth.StandardRoles,
	)
	go apiSrv.Serve()

	tsrv, err := auth.NewTunServer(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:31948"},
		[]ssh.Signer{s.signer},
		apiSrv, s.a, allowAllLimiter)
	c.Assert(err, IsNil)
	c.Assert(tsrv.Start(), IsNil)

	u, err := user.Current()
	c.Assert(err, IsNil)
	s.user = u.Username
	s.userDir = u.HomeDir

	c.Assert(s.a.UpsertUser(services.User{Name: s.user, AllowedLogins: []string{s.user}}), IsNil)

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

	currentDir, err := os.Getwd()
	c.Assert(err, IsNil)
	token1, err := s.a.GenerateToken("localhost", "Node", time.Minute)
	c.Assert(err, IsNil)
	s.srv3Dir = c.MkDir()
	s.srv3Address = "127.0.0.1:30984"
	s.srv3 = exec.Command("teleport", "--env")
	s.srv3.Dir = s.srv3Dir
	s.srv3.Env = append(
		[]string{
			"TELEPORT_HOSTNAME=localhost",
			"TELEPORT_SSH_TOKEN=" + token1,
			"TELEPORT_SSH_ENABLED=true",
			"TELEPORT_SSH_ADDR=tcp://" + s.srv3Address,
			`TELEPORT_AUTH_SERVERS=["tcp://` + tsrv.Addr() + `"]`,
			"TELEPORT_DATA_DIR=" + s.srv3Dir,
			`TELEPORT_SSH_LABELS={"label4":"value4", "label5":"value5"}`,
			"PWD=" + s.srv3Dir}, os.Environ()...,
	)
	s.srv3.Start()

	token2, err := s.a.GenerateToken("localhost", "Node", time.Minute)
	c.Assert(err, IsNil)
	s.srv4Dir = c.MkDir()
	os.Chdir(s.srv4Dir)
	s.srv4Address = "127.0.0.1:30985"
	s.srv4 = exec.Command("teleport", "--env")
	s.srv4.Env = append(
		[]string{
			"TELEPORT_HOSTNAME=localhost",
			"TELEPORT_SSH_TOKEN=" + token2,
			"TELEPORT_SSH_ENABLED=true",
			"TELEPORT_SSH_ADDR=tcp://" + s.srv4Address,
			`TELEPORT_AUTH_SERVERS=["tcp://` + tsrv.Addr() + `"]`,
			"TELEPORT_DATA_DIR=" + s.srv4Dir,
			`TELEPORT_SSH_LABELS={"label4":"value4", "label5":"value6"}`,
			"PWD=" + s.srv4Dir}, os.Environ()...,
	)
	s.srv4.Start()

	os.Chdir(currentDir)

	s.teleagent = teleagent.NewTeleAgent()
	s.agentAddr = filepath.Join(s.dir, "agent.sock")
	c.Assert(s.teleagent.Start("unix://"+s.agentAddr), IsNil)
	err = s.teleagent.Login("http://"+s.webAddress, s.user, string(s.pass), s.otp.OTP(), time.Minute)
	c.Assert(err, IsNil)

	s.envVars = append([]string{"SSH_AUTH_SOCK=" + s.agentAddr}, os.Environ()...)
	// "Command labels will be calculated only on the second heartbeat"
	time.Sleep(time.Millisecond * 3100)
}

func (s *TshSuite) TearDownSuite(c *C) {
	s.srv3.Process.Kill()
	s.srv4.Process.Kill()
}

func (s *TshSuite) TestRunCommand(c *C) {
	cmd := exec.Command("tsh",
		"ssh", s.user+"@"+s.srvAddress,
		"--proxy-user="+s.user,
		`""expr 30 + 5""`)
	cmd.Env = s.envVars
	out, err := cmd.Output()
	c.Assert(err, IsNil)
	c.Assert(string(out), Equals, fmt.Sprintf("Running command on %v\n-----------------------------\n35\n-----------------------------\n\n", s.srvAddress))

	cmd = exec.Command("tsh",
		"ssh",
		"-p", s.srvPort,
		s.user+"@"+s.srvHost,
		"--proxy-user="+s.user,
		`""expr 30 + 5""`)
	cmd.Env = s.envVars
	out, err = cmd.Output()
	c.Assert(string(out), Equals, fmt.Sprintf("Running command on %v\n-----------------------------\n35\n-----------------------------\n\n", s.srvAddress))
	c.Assert(err, IsNil)

	cmd = exec.Command("tsh",
		"ssh",
		"-p", "123",
		s.user+"@"+s.srvAddress,
		"--proxy-user="+s.user,
		`""expr 30 + 5""`)
	cmd.Env = s.envVars
	out, err = cmd.Output()
	c.Assert(err, IsNil)
	c.Assert(string(out), Equals, fmt.Sprintf("Running command on %v\n-----------------------------\n35\n-----------------------------\n\n", s.srvAddress))

	cmd = exec.Command("tsh",
		"ssh", s.user+"@"+s.srvHost,
		`""expr 30 + 5""`)
	cmd.Env = s.envVars
	out, err = cmd.Output()
	c.Assert(err, NotNil)
}

func (s *TshSuite) TestRunCommandOn2Servers(c *C) {
	cmd := exec.Command("tsh",
		"ssh", s.user+"@_label4:value4",
		"--proxy="+s.proxyAddress,
		"--proxy-user="+s.user,
		`""pwd""`)
	cmd.Env = s.envVars
	out, err := cmd.Output()
	c.Assert(err, IsNil)

	c.Assert(true, Equals, strings.Contains(string(out), fmt.Sprintf(
		"Running command on %v\n-----------------------------\n%v\n-----------------------------\n\n",
		s.srv3Address, s.userDir)))

	c.Assert(true, Equals, strings.Contains(string(out), fmt.Sprintf(
		"Running command on %v\n-----------------------------\n%v\n-----------------------------\n\n",
		s.srv4Address, s.userDir)))
}

func (s *TshSuite) TestRunCommandWithProxy(c *C) {
	cmd := exec.Command("tsh", "ssh",
		s.user+"@"+s.srvAddress,
		"--proxy="+s.proxyAddress,
		"--proxy-user="+s.user,
		`""expr 3 + 50""`)
	cmd.Env = s.envVars
	out, err := cmd.Output()
	c.Assert(err, IsNil)

	c.Assert(string(out), Equals, fmt.Sprintf("Running command on %v\n-----------------------------\n53\n-----------------------------\n\n", s.srvAddress))
}

func (s *TshSuite) TestUploadFile(c *C) {
	dir := c.MkDir()
	sourceFileName := filepath.Join(dir, "file1")
	contents := []byte("hello world!")

	err := ioutil.WriteFile(sourceFileName, contents, 0666)
	c.Assert(err, IsNil)

	destinationFileName := filepath.Join(dir, "file2")

	cmd := exec.Command("tsh", "scp",
		"-P", s.srvPort,
		sourceFileName,
		s.user+"@"+s.srvHost+":"+destinationFileName,
		"--proxy-user="+s.user,
		"--proxy="+s.proxyAddress)
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
	cmd := exec.Command("tsh", "scp",
		"-P", s.srvPort,
		s.user+"@"+s.srvHost+":"+sourceFileName,
		destinationFileName,
		"--proxy-user="+s.user,
		"--proxy="+s.proxyAddress)
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

	cmd := exec.Command("tsh", "scp",
		dir1,
		s.user+"@"+s.srvAddress+":"+dir2+"/subdir",
		"--proxy-user="+s.user,
		"--proxy="+s.proxyAddress)
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

	cmd := exec.Command("tsh", "scp",
		s.user+"@"+s.srvAddress+":"+dir1,
		dir2+"/subdir",
		"--proxy="+s.proxyAddress,
		"--proxy-user="+s.user,
		"-r")
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

func (s *TshSuite) TestDownloadFileFrom2Servers(c *C) {
	sourceDir := c.MkDir()
	destDir := c.MkDir()
	sourceFileName1 := filepath.Join(sourceDir, "file3")
	contents1 := []byte("world hello from server three")

	err := ioutil.WriteFile(sourceFileName1, contents1, 0666)
	c.Assert(err, IsNil)

	cmd := exec.Command("tsh", "scp",
		s.user+"@_label4:value4:"+sourceFileName1,
		destDir,
		"--proxy="+s.proxyAddress,
		"--proxy-user="+s.user,
	)
	cmd.Env = s.envVars
	out, err := cmd.Output()
	if err != nil {
		c.Assert(string(out), Equals, "")
		c.Assert(err, IsNil)
	}

	outFile1 := filepath.Join(destDir, "file3", s.srv3Address)
	outFile2 := filepath.Join(destDir, "file3", s.srv4Address)

	bytes1, err := ioutil.ReadFile(outFile1)
	c.Assert(err, IsNil)
	c.Assert(string(bytes1), Equals, string(contents1))

	bytes2, err := ioutil.ReadFile(outFile2)
	c.Assert(err, IsNil)
	c.Assert(string(bytes2), Equals, string(contents1))
}

func (s *TshSuite) TestDownloadDirFrom2Servers(c *C) {
	destDir := c.MkDir()
	sourceDir1 := filepath.Join(c.MkDir(), "copydir")

	err := os.MkdirAll(sourceDir1, os.ModeDir|0777)
	c.Assert(err, IsNil)

	srv1File1 := filepath.Join(sourceDir1, "file11")
	srv1File2 := filepath.Join(sourceDir1, "file12")

	contents11 := []byte("world hello from file one")
	contents12 := []byte("world hello from file two")

	err = ioutil.WriteFile(srv1File1, contents11, 0666)
	c.Assert(err, IsNil)
	err = ioutil.WriteFile(srv1File2, contents12, 0666)
	c.Assert(err, IsNil)

	cmd := exec.Command("tsh", "scp",
		s.user+"@_label4:value4:"+sourceDir1,
		destDir,
		"--proxy="+s.proxyAddress,
		"--proxy-user="+s.user,
		"-r",
	)
	cmd.Env = s.envVars
	out, err := cmd.CombinedOutput()
	if err != nil {
		c.Assert(string(out), Equals, "")
		c.Assert(err, IsNil)
	}

	outFile11 := filepath.Join(destDir, "copydir", s.srv3Address, "file11")
	outFile12 := filepath.Join(destDir, "copydir", s.srv3Address, "file12")
	outFile21 := filepath.Join(destDir, "copydir", s.srv4Address, "file11")
	outFile22 := filepath.Join(destDir, "copydir", s.srv4Address, "file12")

	bytes11, err := ioutil.ReadFile(outFile11)
	c.Assert(err, IsNil)
	c.Assert(string(bytes11), Equals, string(contents11))

	bytes12, err := ioutil.ReadFile(outFile12)
	c.Assert(err, IsNil)
	c.Assert(string(bytes12), Equals, string(contents12))

	bytes21, err := ioutil.ReadFile(outFile21)
	c.Assert(err, IsNil)
	c.Assert(string(bytes21), Equals, string(contents11))

	bytes22, err := ioutil.ReadFile(outFile22)
	c.Assert(err, IsNil)
	c.Assert(string(bytes22), Equals, string(contents12))

}

func (s *TshSuite) TestUploadFileTo2Servers(c *C) {
	dir := c.MkDir()
	sourceFileName := filepath.Join(dir, "file1")
	contents := []byte("hello world!")

	err := ioutil.WriteFile(sourceFileName, contents, 0666)
	c.Assert(err, IsNil)

	destinationFileName := filepath.Join(dir, "file2")

	cmd := exec.Command("tsh", "scp",
		sourceFileName,
		s.user+"@_label5::"+destinationFileName,
		"--proxy-user="+s.user,
		"--proxy="+s.proxyAddress,
	)

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
