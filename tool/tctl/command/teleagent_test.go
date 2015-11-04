package command

import (
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/gravitational/teleport/lib/auth"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"
	"github.com/gravitational/teleport/lib/events/boltlog"
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
	srv                 *srv.Server
	proxy               *srv.Server
	clt                 *ssh.Client
	bk                  *encryptedbk.ReplicatedBackend
	a                   *auth.AuthServer
	reverseTunnelServer reversetunnel.Server
	scrt                secret.SecretService
	signer              ssh.Signer
	dir                 string
}

var _ = Suite(&TeleagentSuite{})

func (s *TeleagentSuite) SetUpSuite(c *C) {
	log.Initialize("console", "INFO")
	key, err := secret.NewKey()
	c.Assert(err, IsNil)
	srv, err := secret.New(&secret.Config{KeyBytes: key})
	c.Assert(err, IsNil)
	s.scrt = srv
}

// TestExec executes a command on a remote server
func (s *TeleagentSuite) TestTeleagent(c *C) {
	s.dir = c.MkDir()

	baseBk, err := boltbk.New(filepath.Join(s.dir, "db"))
	c.Assert(err, IsNil)
	s.bk, err = encryptedbk.NewReplicatedBackend(baseBk,
		filepath.Join(s.dir, "keys"), nil,
		encryptor.GetTestKey)
	c.Assert(err, IsNil)

	s.a = auth.NewAuthServer(s.bk, authority.New(), s.scrt)

	// set up host private key and certificate
	c.Assert(s.a.ResetHostCA(""), IsNil)
	hpriv, hpub, err := s.a.GenerateKeyPair("")
	c.Assert(err, IsNil)
	hcert, err := s.a.GenerateHostCert(hpub, "localhost", "localhost", 0)
	c.Assert(err, IsNil)

	// set up user CA and set up a user that has access to the server
	c.Assert(s.a.ResetUserCA(""), IsNil)

	s.signer, err = sshutils.NewSigner(hpriv, hcert)
	c.Assert(err, IsNil)

	ap := auth.NewBackendAccessPoint(s.bk)

	reverseTunnelServer, err := reversetunnel.NewServer(
		utils.NetAddr{Network: "tcp", Addr: "localhost:33056"},
		[]ssh.Signer{s.signer},
		ap)
	c.Assert(err, IsNil)
	c.Assert(reverseTunnelServer.Start(), IsNil)

	bl, err := boltlog.New(filepath.Join(s.dir, "eventsdb"))
	c.Assert(err, IsNil)

	rec, err := boltrec.New(s.dir)
	c.Assert(err, IsNil)

	apiSrv := httptest.NewServer(
		auth.NewAPIServer(s.a, bl, sess.New(s.bk), rec))

	u, err := url.Parse(apiSrv.URL)
	c.Assert(err, IsNil)

	tsrv, err := auth.NewTunServer(
		utils.NetAddr{Network: "tcp", Addr: "localhost:31497"},
		[]ssh.Signer{s.signer},
		utils.NetAddr{Network: "tcp", Addr: u.Host}, s.a)
	c.Assert(err, IsNil)
	c.Assert(tsrv.Start(), IsNil)

	user := "user1"
	pass := []byte("utndkrn")

	hotpURL, _, err := s.a.UpsertPassword(user, pass)
	c.Assert(err, IsNil)

	otp, _, err := hotp.FromURL(hotpURL)
	c.Assert(err, IsNil)
	otp.Increment()

	authMethod, err := auth.NewWebPasswordAuth(user, pass, otp.OTP())
	c.Assert(err, IsNil)

	tunClt, err := auth.NewTunClient(
		utils.NetAddr{Network: "tcp", Addr: tsrv.Addr()}, user, authMethod)
	c.Assert(err, IsNil)
	defer tunClt.Close()

	rsAgent, err := reversetunnel.NewAgent(
		utils.NetAddr{Network: "tcp", Addr: "localhost:33056"},
		"localhost",
		[]ssh.Signer{s.signer}, tunClt)
	c.Assert(err, IsNil)
	c.Assert(rsAgent.Start(), IsNil)

	srv, err := srv.New(
		utils.NetAddr{Network: "tcp", Addr: "localhost:30185"},
		[]ssh.Signer{s.signer},
		ap,
		srv.SetShell("/bin/sh"),
	)
	c.Assert(err, IsNil)
	s.srv = srv

	c.Assert(s.srv.Start(), IsNil)

	webHandler, err := web.NewMultiSiteHandler(
		web.MultiSiteConfig{
			Tun:       reverseTunnelServer,
			AssetsDir: "../../../assets/web",
			AuthAddr:  utils.NetAddr{Network: "tcp", Addr: tsrv.Addr()},
			FQDN:      "localhost",
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

	agentAddr := "unix://" + filepath.Join(s.dir, "agent.sock")
	agentAPIAddr := "unix://" + filepath.Join(s.dir, "api.sock")

	agent := teleagent.TeleAgent{}
	apiServer := teleagent.NewAgentAPIServer(&agent)
	c.Assert(agent.Start(agentAddr), IsNil)

	go func() {
		if err := apiServer.Start(agentAPIAddr); err != nil {
			log.Errorf(err.Error())
			return
		}
	}()

	// Login agent

	err = teleagent.Login(agentAPIAddr, "http://"+webAddr, string(user), string(pass),
		otp.OTP(), time.Hour)
	c.Assert(err, IsNil)

	agentAddress, err := utils.ParseAddr(agentAddr)
	c.Assert(err, IsNil)
	sshAgent, err := connectToSSHAgent(agentAddress.Network, agentAddress.Addr)
	c.Assert(err, IsNil)

	/*// set up SSH client using the user private key for signing
	up, err := newUpack("test", s.a)
	c.Assert(err, IsNil)

	// set up an agent server and a client that uses agent for forwarding
	keyring := agent.NewKeyring()
	addedKey := agent.AddedKey{
		PrivateKey:  up.pkey,
		Certificate: up.pcert,
	}
	c.Assert(keyring.Add(addedKey), IsNil)
	s.up = up
	*/
	sshConfig := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{ssh.PublicKeysCallback(sshAgent.Signers)},
	}

	client, err := ssh.Dial("tcp", s.srv.Addr(), sshConfig)
	c.Assert(err, IsNil)
	//c.Assert(sshAgent.ForwardToAgent(client, keyring), IsNil)
	s.clt = client

	se, err := s.clt.NewSession()
	c.Assert(err, IsNil)
	defer se.Close()

	out, err := se.Output("expr 2 + 3")
	c.Assert(err, IsNil)
	c.Assert(strings.Trim(string(out), " \n"), Equals, "5")

	c.Assert(s.clt.Close(), IsNil)
	c.Assert(s.srv.Close(), IsNil)

}

func connectToSSHAgent(network, address string) (agent.Agent, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return agent.NewClient(conn), nil

}
