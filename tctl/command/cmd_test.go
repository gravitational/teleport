package command

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gravitational/teleport/auth"
	"github.com/gravitational/teleport/auth/openssh"
	"github.com/gravitational/teleport/backend/membk"
	"github.com/gravitational/teleport/utils"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gravitational/memlog"
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
	bk   *membk.MemBackend
	scrt *secret.Service
	addr utils.NetAddr
}

var _ = Suite(&CmdSuite{})

func (s *CmdSuite) SetUpSuite(c *C) {
	key, err := secret.NewKey()
	c.Assert(err, IsNil)
	srv, err := secret.New(&secret.Config{KeyBytes: key})
	c.Assert(err, IsNil)
	s.scrt = srv
}

func (s *CmdSuite) SetUpTest(c *C) {
	s.bk = membk.New()
	s.asrv = auth.NewAuthServer(s.bk, openssh.New(), s.scrt)
	s.srv = httptest.NewServer(auth.NewAPIServer(s.asrv, memlog.New()))

	u, err := url.Parse(s.srv.URL)
	c.Assert(err, IsNil)

	s.addr = utils.NetAddr{Network: "tcp", Addr: u.Host}

	clt, err := auth.NewClientFromNetAddr(s.addr)
	c.Assert(err, IsNil)
	s.clt = clt

	s.out = &bytes.Buffer{}
	s.cmd = &Command{out: s.out}
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
	args = append(args, fmt.Sprintf("--teleport=%v", &s.addr))
	s.out = &bytes.Buffer{}
	s.cmd = &Command{out: s.out}
	s.cmd.Run(args)
	return strings.Replace(s.out.String(), "\n", " ", -1)
}

func (s *CmdSuite) TestHostCACRUD(c *C) {
	c.Assert(
		s.run("hostca", "reset", "-confirm"),
		Matches, fmt.Sprintf(".*%v.*", "regenerated"))
	c.Assert(s.bk.HostCA, NotNil)

	c.Assert(
		s.run("hostca", "pubkey"),
		Matches, fmt.Sprintf(".*%v.*", s.bk.HostCA.Pub))
}

func (s *CmdSuite) TestUserCACRUD(c *C) {
	c.Assert(
		s.run("userca", "reset", "-confirm"),
		Matches, fmt.Sprintf(".*%v.*", "regenerated"))
	c.Assert(s.bk.UserCA, NotNil)

	c.Assert(
		s.run("userca", "pubkey"),
		Matches, fmt.Sprintf(".*%v.*", s.bk.UserCA.Pub))
}

func (s *CmdSuite) TestUserCRUD(c *C) {
	c.Assert(s.asrv.ResetUserCA(""), IsNil)

	_, pub, err := s.asrv.GenerateKeyPair("")
	c.Assert(err, IsNil)

	fkey, err := ioutil.TempFile("", "teleport")
	c.Assert(err, IsNil)
	defer fkey.Close()
	fkey.Write(pub)

	out := s.run("user", "upsert_key", "-user", "alex", "-keyid", "key1", "-key", fkey.Name())
	c.Assert(out, Matches, fmt.Sprintf(".*%v.*", "certificate:"))

	parts := strings.Split(out, "certificate:")
	c.Assert(len(parts), Equals, 2)

	c.Assert(trim(string(s.bk.Users["alex"].Keys["key1"].Value)), Equals, trim(parts[1]))

	c.Assert(
		s.run("user", "ls"),
		Matches, fmt.Sprintf(".*%v.*", "alex"))

	c.Assert(s.run("user", "ls_keys", "-user", "alex"), Matches, fmt.Sprintf(".*%v.*", "key1"))

	c.Assert(
		s.run("user", "delete", "-user", "alex"),
		Matches, fmt.Sprintf(".*%v.*", "alex"))
}

func (s *CmdSuite) TestGenerateToken(c *C) {
	token := s.run(
		"token", "generate", "-fqdn", "a.example.com", "-ttl", "100s")
	c.Assert(s.asrv.ValidateToken(token, "a.example.com"), IsNil)
}

func trim(val string) string {
	return strings.Trim(val, " \t\n")
}
