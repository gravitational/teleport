package command

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gravitational/teleport/lib/auth"
	authority "github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/backend/encryptedbk"
	"github.com/gravitational/teleport/lib/events/boltlog"
	"github.com/gravitational/teleport/lib/recorder"
	"github.com/gravitational/teleport/lib/recorder/boltrec"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/session"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/log"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/secret"

	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

const OK = ".*OK.*"

func TestTeleportCLI(t *testing.T) { TestingT(t) }

type CmdSuite struct {
	srv  *httptest.Server
	asrv *auth.AuthServer
	clt  *auth.Client
	cmd  *Command
	out  *bytes.Buffer
	bk   *encryptedbk.ReplicatedBackend
	bl   *boltlog.BoltLog
	scrt secret.SecretService
	rec  recorder.Recorder
	addr utils.NetAddr
	dir  string

	CAS           *services.CAService
	LockS         *services.LockService
	PresenceS     *services.PresenceService
	ProvisioningS *services.ProvisioningService
	UserS         *services.UserService
	WebS          *services.WebService
}

var _ = Suite(&CmdSuite{})

func (s *CmdSuite) SetUpSuite(c *C) {
	key, err := secret.NewKey()
	c.Assert(err, IsNil)
	srv, err := secret.New(&secret.Config{KeyBytes: key})
	c.Assert(err, IsNil)
	s.scrt = srv

	log.Initialize("console", "WARN")
}

func (s *CmdSuite) SetUpTest(c *C) {
	s.dir = c.MkDir()

	baseBk, err := boltbk.New(filepath.Join(s.dir, "db"))
	c.Assert(err, IsNil)
	s.bk, err = encryptedbk.NewReplicatedBackend(baseBk, filepath.Join(s.dir, "keys"), nil)
	c.Assert(err, IsNil)

	s.bl, err = boltlog.New(filepath.Join(s.dir, "eventsdb"))
	c.Assert(err, IsNil)

	s.rec, err = boltrec.New(s.dir)
	c.Assert(err, IsNil)

	s.asrv = auth.NewAuthServer(s.bk, authority.New(), s.scrt)
	s.srv = httptest.NewServer(auth.NewAPIServer(s.asrv, s.bl,
		session.New(s.bk), s.rec))

	u, err := url.Parse(s.srv.URL)
	c.Assert(err, IsNil)

	s.addr = utils.NetAddr{Network: "tcp", Addr: u.Host}

	clt, err := auth.NewClientFromNetAddr(s.addr)
	c.Assert(err, IsNil)
	s.clt = clt

	s.out = &bytes.Buffer{}
	s.cmd = &Command{out: s.out}

	s.CAS = services.NewCAService(s.bk)
	s.LockS = services.NewLockService(s.bk)
	s.PresenceS = services.NewPresenceService(s.bk)
	s.ProvisioningS = services.NewProvisioningService(s.bk)
	s.UserS = services.NewUserService(s.bk)
	s.WebS = services.NewWebService(s.bk)
}

func (s *CmdSuite) TearDownTest(c *C) {
	s.srv.Close()
}

func (s *CmdSuite) runString(in string) string {
	return s.run(strings.Split(in, " ")...)
}

func (s *CmdSuite) run(params ...string) string {
	args := []string{"tctl"}
	args = append(args, params...)
	args = append(args, fmt.Sprintf("--auth=%v", &s.addr))
	s.out = &bytes.Buffer{}
	s.cmd = &Command{out: s.out}
	s.cmd.Run(args)
	return strings.Replace(s.out.String(), "\n", " ", -1)
}

func (s *CmdSuite) TestHostCACRUD(c *C) {
	c.Assert(
		s.run("host-ca", "reset", "--confirm"),
		Matches, fmt.Sprintf(".*%v.*", "regenerated"))

	hostCA, err := s.CAS.GetHostCA()
	c.Assert(err, IsNil)
	c.Assert(hostCA, NotNil)

	c.Assert(
		s.run("host-ca", "pub-key"),
		Matches, fmt.Sprintf(".*%v.*", hostCA.Pub))
}

func (s *CmdSuite) TestUserCACRUD(c *C) {
	c.Assert(
		s.run("user-ca", "reset", "--confirm"),
		Matches, fmt.Sprintf(".*%v.*", "regenerated"))

	userCA, err := s.CAS.GetUserCA()
	c.Assert(err, IsNil)
	c.Assert(userCA, NotNil)
	c.Assert(userCA, NotNil)

	c.Assert(
		s.run("user-ca", "pub-key"),
		Matches, fmt.Sprintf(".*%v.*", userCA.Pub))
}

func (s *CmdSuite) TestUserCRUD(c *C) {
	c.Assert(s.asrv.ResetUserCA(""), IsNil)

	_, pub, err := s.asrv.GenerateKeyPair("")
	c.Assert(err, IsNil)

	fkey, err := ioutil.TempFile("", "teleport")
	c.Assert(err, IsNil)
	defer fkey.Close()
	fkey.Write(pub)

	out := s.run("user", "upsert-key", "--user", "alex", "--key-id", "key1", "--key", fkey.Name())
	c.Assert(out, Matches, fmt.Sprintf(".*%v.*", pub))

	var keys []services.AuthorizedKey
	keys, err = s.UserS.GetUserKeys("alex")
	c.Assert(err, IsNil)
	c.Assert(trim(keys[0].ID), Equals, "key1")
	c.Assert(trim(string(keys[0].Value)), Equals, trim(out))

	c.Assert(
		s.run("user", "ls"),
		Matches, fmt.Sprintf(".*%v.*", "alex"))

	c.Assert(s.run("user", "ls-keys", "--user", "alex"), Matches, fmt.Sprintf(".*%v.*", "key1"))

	c.Assert(
		s.run("user", "delete", "--user", "alex"),
		Matches, fmt.Sprintf(".*%v.*", "alex"))
}

func (s *CmdSuite) TestGenerateToken(c *C) {
	token := s.run(
		"token", "generate", "--fqdn", "a.example.com", "--ttl", "100s")
	c.Assert(s.asrv.ValidateToken(token, "a.example.com"), IsNil)
}

func (s *CmdSuite) TestRemoteCertCRUD(c *C) {
	c.Assert(s.asrv.ResetUserCA(""), IsNil)

	_, pub, err := s.asrv.GenerateKeyPair("")
	c.Assert(err, IsNil)

	fkey, err := ioutil.TempFile("", "teleport")
	c.Assert(err, IsNil)
	defer fkey.Close()
	fkey.Write(pub)

	out := s.run("remote-ca", "upsert", "--id", "id1", "--type", "user", "--fqdn", "example.com", "--path", fkey.Name())
	c.Assert(out, Matches, fmt.Sprintf(".*%v.*", "upserted"))

	var remoteCerts []services.RemoteCert
	remoteCerts, err = s.CAS.GetRemoteCerts("user", "example.com")
	c.Assert(err, IsNil)
	c.Assert(trim(string(remoteCerts[0].Value)), Equals, trim(string(pub)))

	out = s.run("remote-ca", "ls", "--type", "user")
	c.Assert(out, Matches, fmt.Sprintf(".*%v.*", "example.com"))

	out = s.run("remote-ca", "rm", "--type", "user", "--fqdn", "example.com", "--id", "id1")
	c.Assert(out, Matches, fmt.Sprintf(".*%v.*", "deleted"))

	remoteCerts, err = s.CAS.GetRemoteCerts("user", "")
	c.Assert(len(remoteCerts), Equals, 0)
}

func (s *CmdSuite) TestBackendKeys(c *C) {
	// running TestRemoteCertCRUD while changing some keys

	out := s.run("backend-keys", "generate", "--name", "key45")
	c.Assert(out, Matches, fmt.Sprintf(".*was generated.*"))

	keys, err := s.asrv.GetSealKeys()
	c.Assert(err, IsNil)
	c.Assert(len(keys), Equals, 2)

	out = s.run("backend-keys", "ls")
	for _, key := range keys {
		c.Assert(out, Matches, fmt.Sprintf(".*%v.*", key.ID))
		c.Assert(out, Matches, fmt.Sprintf(".*%v.*", key.Name))
	}

	s.run("backend-keys", "export", "--id", keys[0].ID, "--dir", s.dir)

	c.Assert(s.asrv.ResetUserCA(""), IsNil)
	_, pub, err := s.asrv.GenerateKeyPair("")
	c.Assert(err, IsNil)
	fkey, err := ioutil.TempFile("", "teleport")
	c.Assert(err, IsNil)
	defer fkey.Close()
	fkey.Write(pub)
	out = s.run("remote-ca", "upsert", "--id", "id1", "--type", "user", "--fqdn", "example.com", "--path", fkey.Name())
	c.Assert(out, Matches, fmt.Sprintf(".*%v.*", "upserted"))

	s.run("backend-keys", "delete", "--id", keys[0].ID)

	return

	var remoteCerts []services.RemoteCert
	remoteCerts, err = s.CAS.GetRemoteCerts("user", "example.com")
	c.Assert(err, IsNil)
	c.Assert(trim(string(remoteCerts[0].Value)), Equals, trim(string(pub)))

	s.run("backend-keys", "import", "--file", path.Join(s.dir, keys[0].ID+".bkey"))
	s.run("backend-keys", "delete", "--id", keys[1].ID)
	out = s.run("backend-keys", "ls")
	c.Assert(out, DeepEquals, "This server has these keys: key45 ")
	out = s.run("backend-keys", "ls")
	c.Assert(out, Matches, fmt.Sprintf(".*%v.*", keys[0].ID))
	c.Assert(out, Matches, fmt.Sprintf(".*%v.*", keys[0].Name))

	out = s.run("remote-ca", "ls", "--type", "user")
	c.Assert(out, Matches, fmt.Sprintf(".*%v.*", "example.com"))
	out = s.run("remote-ca", "rm", "--type", "user", "--fqdn", "example.com", "--id", "id1")
	c.Assert(out, Matches, fmt.Sprintf(".*%v.*", "deleted"))
	remoteCerts, err = s.CAS.GetRemoteCerts("user", "")
	c.Assert(len(remoteCerts), Equals, 0)

}

func trim(val string) string {
	return strings.Trim(val, " \t\n")
}
