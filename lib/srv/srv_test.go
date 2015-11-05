package srv

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gravitational/teleport/lib/auth"
	authority "github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk/encryptor"
	"github.com/gravitational/teleport/lib/events/boltlog"
	"github.com/gravitational/teleport/lib/recorder/boltrec"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/services"
	sess "github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gokyle/hotp"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/secret"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh"
	"github.com/gravitational/teleport/Godeps/_workspace/src/golang.org/x/crypto/ssh/agent"
	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestSrv(t *testing.T) { TestingT(t) }

type SrvSuite struct {
	srv        *Server
	srvAddress string
	clt        *ssh.Client
	bk         *encryptedbk.ReplicatedBackend
	a          *auth.AuthServer
	up         *upack
	scrt       secret.SecretService
	signer     ssh.Signer
	dir        string
}

var _ = Suite(&SrvSuite{})

func (s *SrvSuite) SetUpSuite(c *C) {
	key, err := secret.NewKey()
	c.Assert(err, IsNil)
	srv, err := secret.New(&secret.Config{KeyBytes: key})
	c.Assert(err, IsNil)
	s.scrt = srv
}

func (s *SrvSuite) SetUpTest(c *C) {
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
	s.srvAddress = "localhost:30185"
	srv, err := New(
		utils.NetAddr{Network: "tcp", Addr: s.srvAddress},
		[]ssh.Signer{s.signer},
		ap,
		SetShell("/bin/sh"),
	)
	c.Assert(err, IsNil)
	s.srv = srv

	c.Assert(s.srv.Start(), IsNil)

	// set up SSH client using the user private key for signing
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

	sshConfig := &ssh.ClientConfig{
		User: "test",
		Auth: []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
	}

	client, err := ssh.Dial("tcp", s.srv.Addr(), sshConfig)
	c.Assert(err, IsNil)
	c.Assert(agent.ForwardToAgent(client, keyring), IsNil)
	s.clt = client
}

func (s *SrvSuite) TearDownTest(c *C) {
	c.Assert(s.clt.Close(), IsNil)
	c.Assert(s.srv.Close(), IsNil)
}

// TestExec executes a command on a remote server
func (s *SrvSuite) TestExec(c *C) {
	se, err := s.clt.NewSession()
	c.Assert(err, IsNil)
	defer se.Close()

	out, err := se.Output("expr 2 + 3")
	c.Assert(err, IsNil)
	c.Assert(strings.Trim(string(out), " \n"), Equals, "5")
}

// TestShell launches interactive shell session and executes a command
func (s *SrvSuite) TestShell(c *C) {
	se, err := s.clt.NewSession()
	c.Assert(err, IsNil)

	w, err := se.StdinPipe()
	c.Assert(err, IsNil)

	stdout := &bytes.Buffer{}
	se.Stdout = stdout
	c.Assert(se.Shell(), IsNil)
	_, err = io.WriteString(w, "expr 7 + 70;exit\r\n")
	c.Assert(err, IsNil)
	c.Assert(se.Wait(), IsNil)
	c.Assert(removeNL(stdout.String()), Matches, ".*77.*")
}

// TestMux tests multiplexing command with agent forwarding
func (s *SrvSuite) TestMux(c *C) {
	se, err := s.clt.NewSession()
	c.Assert(err, IsNil)
	defer se.Close()
	c.Assert(agent.RequestAgentForwarding(se), IsNil)

	stdout := &bytes.Buffer{}
	reader, err := se.StdoutPipe()
	done := make(chan struct{})
	go func() {
		io.Copy(stdout, reader)
		close(done)
	}()

	c.Assert(se.RequestSubsystem(fmt.Sprintf("mux:%v/expr 22 + 55", s.srv.Addr())), IsNil)
	<-done
	c.Assert(removeNL(stdout.String()), Matches, ".*77.*")
}

// TestTun tests tunneling command with agent forwarding
func (s *SrvSuite) TestTun(c *C) {
	se, err := s.clt.NewSession()
	c.Assert(err, IsNil)
	defer se.Close()
	c.Assert(agent.RequestAgentForwarding(se), IsNil)

	writer, err := se.StdinPipe()
	c.Assert(err, IsNil)

	stdout := &bytes.Buffer{}
	reader, err := se.StdoutPipe()
	done := make(chan struct{})
	go func() {
		io.Copy(stdout, reader)
		close(done)
	}()

	c.Assert(se.RequestSubsystem(fmt.Sprintf("tun:%v", s.srv.Addr())), IsNil)

	_, err = io.WriteString(writer, "expr 7 + 70;exit\r\n")
	c.Assert(err, IsNil)

	<-done
	c.Assert(removeNL(stdout.String()), Matches, ".*77.*")
}

func (s *SrvSuite) TestProxy(c *C) {
	ap := auth.NewBackendAccessPoint(s.bk)
	reverseTunnelAddress := utils.NetAddr{Network: "tcp", Addr: "localhost:33056"}
	reverseTunnelServer, err := reversetunnel.NewServer(
		reverseTunnelAddress,
		[]ssh.Signer{s.signer},
		ap)
	c.Assert(err, IsNil)
	c.Assert(reverseTunnelServer.Start(), IsNil)

	proxy, err := New(
		utils.NetAddr{Network: "tcp", Addr: "localhost:0"},
		[]ssh.Signer{s.signer},
		ap,
		SetProxyMode(reverseTunnelServer),
	)
	c.Assert(err, IsNil)
	c.Assert(proxy.Start(), IsNil)

	// set up SSH client using the user private key for signing
	up, err := newUpack("test", s.a)
	c.Assert(err, IsNil)

	keyring := agent.NewKeyring()
	addedKey := agent.AddedKey{
		PrivateKey:  up.pkey,
		Certificate: up.pcert,
	}
	c.Assert(keyring.Add(addedKey), IsNil)

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
		reverseTunnelAddress,
		"localhost",
		[]ssh.Signer{s.signer}, tunClt)
	c.Assert(err, IsNil)
	c.Assert(rsAgent.Start(), IsNil)

	sshConfig := &ssh.ClientConfig{
		User: "test",
		Auth: []ssh.AuthMethod{ssh.PublicKeys(up.certSigner)},
	}

	// Trying to connect to unregistered ssh node

	client, err := ssh.Dial("tcp", proxy.Addr(), sshConfig)
	c.Assert(err, IsNil)
	c.Assert(agent.ForwardToAgent(client, keyring), IsNil)

	se0, err := client.NewSession()
	c.Assert(err, IsNil)
	defer se0.Close()

	writer, err := se0.StdinPipe()
	c.Assert(err, IsNil)

	reader, err := se0.StdoutPipe()
	c.Assert(err, IsNil)

	// Request opening TCP connection to the remote host
	unregisteredAddress := s.srv.Addr() // proper ssh node address but with 127.0.0.1 instead of localhost
	c.Assert(se0.RequestSubsystem(fmt.Sprintf("proxy:%v", unregisteredAddress)), IsNil)

	local, err := net.ResolveTCPAddr("tcp", proxy.Addr())
	c.Assert(err, IsNil)
	remote, err := net.ResolveTCPAddr("tcp", s.srv.Addr())
	c.Assert(err, IsNil)

	pipeNetConn := utils.NewPipeNetConn(
		reader,
		writer,
		se0,
		local,
		remote,
	)

	// Open SSH connection via TCP
	conn, chans, reqs, err := ssh.NewClientConn(pipeNetConn,
		s.srv.Addr(), sshConfig)
	c.Assert(err, NotNil)

	// Connect to node using registered address
	client, err = ssh.Dial("tcp", proxy.Addr(), sshConfig)
	c.Assert(err, IsNil)
	c.Assert(agent.ForwardToAgent(client, keyring), IsNil)

	se, err := client.NewSession()
	c.Assert(err, IsNil)
	defer se.Close()

	writer, err = se.StdinPipe()
	c.Assert(err, IsNil)

	reader, err = se.StdoutPipe()
	c.Assert(err, IsNil)

	// Request opening TCP connection to the remote host
	c.Assert(se.RequestSubsystem(fmt.Sprintf("proxy:%v", s.srvAddress)), IsNil)

	local, err = net.ResolveTCPAddr("tcp", proxy.Addr())
	c.Assert(err, IsNil)
	remote, err = net.ResolveTCPAddr("tcp", s.srv.Addr())
	c.Assert(err, IsNil)

	pipeNetConn = utils.NewPipeNetConn(
		reader,
		writer,
		se,
		local,
		remote,
	)

	// Open SSH connection via TCP
	conn, chans, reqs, err = ssh.NewClientConn(pipeNetConn,
		s.srv.Addr(), sshConfig)
	c.Assert(err, IsNil)

	// using this connection as regular SSH
	client2 := ssh.NewClient(conn, chans, reqs)
	c.Assert(err, IsNil)

	c.Assert(agent.ForwardToAgent(client2, keyring), IsNil)

	se2, err := client2.NewSession()
	c.Assert(err, IsNil)
	defer se2.Close()

	out, err := se2.Output("expr 2 + 3")
	c.Assert(err, IsNil)

	c.Assert(strings.Trim(string(out), " \n"), Equals, "5")

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

// TODO(klizhentas): figure out the way to check that resources are properly deallocated
// on client disconnects
func (s *SrvSuite) TestClientDisconnect(c *C) {
	config := &ssh.ClientConfig{
		User: "test",
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

func newUpack(user string, a *auth.AuthServer) (*upack, error) {
	upriv, upub, err := a.GenerateKeyPair("")
	if err != nil {
		return nil, err
	}

	ucert, err := a.UpsertUserKey(user, services.AuthorizedKey{ID: user, Value: upub}, 0)
	if err != nil {
		return nil, err
	}

	upkey, err := ssh.ParseRawPrivateKey(upriv)
	if err != nil {
		return nil, err
	}

	usigner, err := ssh.NewSignerFromKey(upkey)
	if err != nil {
		return nil, err
	}

	pcert, _, _, _, err := ssh.ParseAuthorizedKey(ucert)
	if err != nil {
		return nil, err
	}

	ucertSigner, err := ssh.NewCertSigner(pcert.(*ssh.Certificate), usigner)
	if err != nil {
		return nil, err
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
