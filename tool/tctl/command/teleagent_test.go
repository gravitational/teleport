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
package command

import (
	"net"
	"net/http"
	"path/filepath"
	"strings"
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
	sess "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/web"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gokyle/hotp"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/trace"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/secret"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh/agent"
	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

type TeleagentSuite struct {
}

var _ = Suite(&TeleagentSuite{})

func (s *TeleagentSuite) TestTeleagent(c *C) {
	key, err := secret.NewKey()
	c.Assert(err, IsNil)
	scrt, err := secret.New(&secret.Config{KeyBytes: key})
	c.Assert(err, IsNil)

	dir := c.MkDir()

	baseBk, err := boltbk.New(filepath.Join(dir, "db"))
	c.Assert(err, IsNil)
	bk, err := encryptedbk.NewReplicatedBackend(baseBk,
		filepath.Join(dir, "keys"), nil,
		encryptor.GetTestKey)
	c.Assert(err, IsNil)

	a := auth.NewAuthServer(bk, authority.New(), scrt, "host3")

	// set up host private key and certificate
	c.Assert(a.ResetHostCertificateAuthority(""), IsNil)
	hpriv, hpub, err := a.GenerateKeyPair("")
	c.Assert(err, IsNil)
	hcert, err := a.GenerateHostCert(hpub, "localhost", "localhost", "RoleAdmin", 0)
	c.Assert(err, IsNil)

	// set up user CertificateAuthority and set up a user that has access to the server
	c.Assert(a.ResetUserCertificateAuthority(""), IsNil)

	signer, err := sshutils.NewSigner(hpriv, hcert)
	c.Assert(err, IsNil)

	ap := auth.NewBackendAccessPoint(bk)

	limiter, err := limiter.NewLimiter(limiter.LimiterConfig{})
	c.Assert(err, IsNil)

	reverseTunnelAddress := utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:33058"}
	reverseTunnelServer, err := reversetunnel.NewServer(
		reverseTunnelAddress,
		[]ssh.Signer{signer},
		ap, limiter)
	c.Assert(err, IsNil)
	c.Assert(reverseTunnelServer.Start(), IsNil)

	bl, err := boltlog.New(filepath.Join(dir, "eventsdb"))
	c.Assert(err, IsNil)

	rec, err := boltrec.New(dir)
	c.Assert(err, IsNil)
	apiSrv := auth.NewAPIWithRoles(a, bl, sess.New(bk), rec,
		auth.NewAllowAllPermissions(),
		auth.StandardRoles,
	)
	apiSrv.Serve()

	tsrv, err := auth.NewTunServer(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:31497"},
		[]ssh.Signer{signer},
		apiSrv, a, limiter)
	c.Assert(err, IsNil)
	c.Assert(tsrv.Start(), IsNil)

	// Creating new user
	user := "user1"
	pass := []byte("utndkrn")
	hotpURL, _, err := a.UpsertPassword(user, pass)
	c.Assert(err, IsNil)

	otp, _, err := hotp.FromURL(hotpURL)
	c.Assert(err, IsNil)
	otp.Increment()

	authMethod, err := auth.NewWebPasswordAuth(user, pass, otp.OTP())
	c.Assert(err, IsNil)

	tunClt, err := auth.NewTunClient(
		utils.NetAddr{AddrNetwork: "tcp", Addr: tsrv.Addr()}, user, authMethod)
	c.Assert(err, IsNil)
	defer tunClt.Close()

	rsAgent, err := reversetunnel.NewAgent(
		reverseTunnelAddress,
		"localhost",
		[]ssh.Signer{signer}, tunClt)
	c.Assert(err, IsNil)
	c.Assert(rsAgent.Start(), IsNil)

	srv, err := srv.New(
		utils.NetAddr{AddrNetwork: "tcp", Addr: "localhost:30185"},
		"localhost",
		[]ssh.Signer{signer},
		ap,
		limiter,
		dir,
		srv.SetShell("/bin/sh"),
	)
	c.Assert(err, IsNil)
	srv = srv

	c.Assert(srv.Start(), IsNil)

	webHandler, err := web.NewMultiSiteHandler(
		web.MultiSiteConfig{
			Tun:        reverseTunnelServer,
			AssetsDir:  "../../../assets/web",
			AuthAddr:   utils.NetAddr{AddrNetwork: "tcp", Addr: tsrv.Addr()},
			DomainName: "localhost",
		},
	)
	c.Assert(err, IsNil)

	webAddr := "localhost:31386"

	go func() {
		err := http.ListenAndServe(webAddr, webHandler)
		if err != nil {
			log.Errorf(err.Error())
		}
	}()

	// Starting SSH agent

	agentAddr := "unix://" + filepath.Join(dir, "agent.sock")
	agentAPIAddr := "unix://" + filepath.Join(dir, "api.sock")

	agent := teleagent.NewTeleAgent()
	apiServer := teleagent.NewAgentAPIServer(agent)
	c.Assert(agent.Start(agentAddr), IsNil)

	go func() {
		if err := apiServer.Start(agentAPIAddr); err != nil {
			log.Errorf(err.Error())
			return
		}
	}()

	// Trying to create ssh connection without any keys in the agent
	agentAddress, err := utils.ParseAddr(agentAddr)
	c.Assert(err, IsNil)

	sshAgent, err := connectToSSHAgent(agentAddress.AddrNetwork, agentAddress.Addr)
	c.Assert(err, IsNil)

	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.PublicKeysCallback(sshAgent.Signers)},
	}

	clt, err := ssh.Dial("tcp", srv.Addr(), sshConfig)
	c.Assert(err, NotNil)

	// Login agent
	err = teleagent.Login(agentAPIAddr, "http://"+webAddr, string(user), string(pass),
		otp.OTP(), time.Hour)
	c.Assert(err, IsNil)

	// Creating ssh connection
	sshAgent, err = connectToSSHAgent(agentAddress.AddrNetwork, agentAddress.Addr)
	c.Assert(err, IsNil)

	sshConfig = &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.PublicKeysCallback(sshAgent.Signers)},
	}

	clt, err = ssh.Dial("tcp", srv.Addr(), sshConfig)
	c.Assert(err, IsNil)

	se, err := clt.NewSession()
	c.Assert(err, IsNil)
	defer se.Close()

	out, err := se.Output("expr 2 + 3")
	c.Assert(err, IsNil)
	c.Assert(strings.Trim(string(out), " \n"), Equals, "5")

	c.Assert(clt.Close(), IsNil)
	c.Assert(srv.Close(), IsNil)

}

func connectToSSHAgent(network, address string) (agent.Agent, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return agent.NewClient(conn), nil

}
