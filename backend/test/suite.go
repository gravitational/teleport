// package test contains a backend acceptance test suite that is backend implementation independant
// each backend will use the suite to test itself
package test

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/random"
	"github.com/gravitational/teleport/backend"

	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

func TestBackend(t *testing.T) { TestingT(t) }

type BackendSuite struct {
	B        backend.Backend
	ChangesC chan interface{}
}

func (s *BackendSuite) collectChanges(c *C, expected int) []interface{} {
	changes := make([]interface{}, expected)
	for i, _ := range changes {
		select {
		case changes[i] = <-s.ChangesC:
			// successfully collected changes
		case <-time.After(2 * time.Second):
			c.Fatalf("Timeout occured waiting for events")
		}
	}
	return changes
}

func (s *BackendSuite) expectChanges(c *C, expected ...interface{}) {
	changes := s.collectChanges(c, len(expected))
	for i, ch := range changes {
		c.Assert(ch, DeepEquals, expected[i])
	}
}

func (s *BackendSuite) UserKeyCRUD(c *C) {
	k := backend.AuthorizedKey{ID: "id1", Value: []byte("val1")}

	c.Assert(s.B.UpsertUserKey("user1", k, 0), IsNil)

	keys, err := s.B.GetUserKeys("user1")
	c.Assert(err, IsNil)
	c.Assert(keys, DeepEquals, []backend.AuthorizedKey{k})

	c.Assert(s.B.DeleteUserKey("user1", k.ID), IsNil)

	keys, err = s.B.GetUserKeys("user1")
	c.Assert(err, IsNil)
	c.Assert(keys, DeepEquals, []backend.AuthorizedKey{})
}

func (s *BackendSuite) UsersCRUD(c *C) {
	u, err := s.B.GetUsers()
	c.Assert(err, IsNil)
	c.Assert(len(u), Equals, 0)

	k := backend.AuthorizedKey{ID: "id1", Value: []byte("val1")}

	c.Assert(s.B.UpsertUserKey("user1", k, 0), IsNil)
	c.Assert(s.B.UpsertUserKey("user2", k, 0), IsNil)

	u, err = s.B.GetUsers()
	c.Assert(err, IsNil)
	c.Assert(toSet(u), DeepEquals, map[string]struct{}{"user1": struct{}{}, "user2": struct{}{}})

	c.Assert(s.B.DeleteUser("user1"), IsNil)

	u, err = s.B.GetUsers()
	c.Assert(err, IsNil)
	c.Assert(toSet(u), DeepEquals, map[string]struct{}{"user2": struct{}{}})

	c.Assert(s.B.DeleteUser("user1"), FitsTypeOf, &backend.NotFoundError{})
}

func (s *BackendSuite) UserCACRUD(c *C) {
	ca := backend.CA{
		Pub:  []byte("capub"),
		Priv: []byte("capriv"),
	}
	c.Assert(s.B.UpsertUserCA(ca), IsNil)

	out, err := s.B.GetUserCA()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, &ca)

	outp, err := s.B.GetUserCAPub()
	c.Assert(err, IsNil)
	c.Assert(outp, DeepEquals, ca.Pub)
}

func (s *BackendSuite) HostCACRUD(c *C) {
	ca := backend.CA{
		Pub:  []byte("capub"),
		Priv: []byte("capriv"),
	}
	c.Assert(s.B.UpsertHostCA(ca), IsNil)

	out, err := s.B.GetHostCA()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, &ca)

	outp, err := s.B.GetHostCAPub()
	c.Assert(err, IsNil)
	c.Assert(outp, DeepEquals, ca.Pub)
}

func (s *BackendSuite) ServerCRUD(c *C) {
	out, err := s.B.GetServers()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	srv := backend.Server{ID: "srv1", Addr: "localhost:2022"}
	c.Assert(s.B.UpsertServer(srv, 0), IsNil)

	out, err = s.B.GetServers()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []backend.Server{srv})
}

func (s *BackendSuite) PasswordHashCRUD(c *C) {
	_, err := s.B.GetPasswordHash("user1")
	c.Assert(err, FitsTypeOf, &backend.NotFoundError{})

	err = s.B.UpsertPasswordHash("user1", []byte("hello123"))
	c.Assert(err, IsNil)

	hash, err := s.B.GetPasswordHash("user1")
	c.Assert(err, IsNil)
	c.Assert(hash, DeepEquals, []byte("hello123"))

	err = s.B.UpsertPasswordHash("user1", []byte("hello321"))
	c.Assert(err, IsNil)

	hash, err = s.B.GetPasswordHash("user1")
	c.Assert(err, IsNil)
	c.Assert(hash, DeepEquals, []byte("hello321"))
}

func (s *BackendSuite) WebSessionCRUD(c *C) {
	_, err := s.B.GetWebSession("user1", "sid1")
	c.Assert(err, FitsTypeOf, &backend.NotFoundError{})

	ws := backend.WebSession{Pub: []byte("pub123"), Priv: []byte("priv123")}
	err = s.B.UpsertWebSession("user1", "sid1", ws, 0)
	c.Assert(err, IsNil)

	out, err := s.B.GetWebSession("user1", "sid1")
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, &ws)

	ws1 := backend.WebSession{Pub: []byte("pub321"), Priv: []byte("priv321")}
	err = s.B.UpsertWebSession("user1", "sid1", ws1, 0)
	c.Assert(err, IsNil)

	out2, err := s.B.GetWebSession("user1", "sid1")
	c.Assert(err, IsNil)
	c.Assert(out2, DeepEquals, &ws1)

	keys, err := s.B.GetWebSessionsKeys("user1")
	c.Assert(err, IsNil)
	c.Assert(keys[0].Value, DeepEquals, out2.Pub)

	c.Assert(s.B.DeleteWebSession("user1", "sid1"), IsNil)

	_, err = s.B.GetWebSession("user1", "sid1")
	c.Assert(err, FitsTypeOf, &backend.NotFoundError{})
}

func (s *BackendSuite) WebTunCRUD(c *C) {
	_, err := s.B.GetWebTun("p1")
	c.Assert(err, FitsTypeOf, &backend.NotFoundError{})

	t := backend.WebTun{
		Prefix:     "p1",
		TargetAddr: "http://localhost:5000",
		ProxyAddr:  "node1.gravitational.io",
	}
	c.Assert(s.B.UpsertWebTun(t, 0), IsNil)

	out, err := s.B.GetWebTun("p1")
	c.Assert(out, DeepEquals, &t)

	tuns, err := s.B.GetWebTuns()
	c.Assert(err, IsNil)
	c.Assert(tuns, DeepEquals, []backend.WebTun{t})

	t1 := backend.WebTun{
		Prefix:     "p1",
		TargetAddr: "http://localhost:5001",
		ProxyAddr:  "node1.gravitational2.io",
	}
	c.Assert(s.B.UpsertWebTun(t1, 0), IsNil)

	out, err = s.B.GetWebTun("p1")
	c.Assert(out, DeepEquals, &t1)

	c.Assert(s.B.DeleteWebTun("p1"), IsNil)

	_, err = s.B.GetWebTun("p1")
	c.Assert(err, FitsTypeOf, &backend.NotFoundError{})
}

func (s *BackendSuite) Locking(c *C) {
	tok := randomToken()
	ttl := 30 * time.Second

	c.Assert(s.B.AcquireLock(tok, ttl), IsNil)
	c.Assert(s.B.AcquireLock(tok, ttl),
		FitsTypeOf, &backend.AlreadyExistsError{})

	c.Assert(s.B.ReleaseLock(tok), IsNil)
	c.Assert(s.B.ReleaseLock(tok), FitsTypeOf, &backend.NotFoundError{})

	c.Assert(s.B.AcquireLock(tok, 30*time.Second), IsNil)
}

func (s *BackendSuite) TokenCRUD(c *C) {
	_, err := s.B.GetToken("token")
	c.Assert(err, FitsTypeOf, &backend.NotFoundError{})

	c.Assert(s.B.UpsertToken("token", "a.example.com", 0), IsNil)

	out, err := s.B.GetToken("token")
	c.Assert(out, Equals, "a.example.com")
	c.Assert(err, IsNil)

	c.Assert(s.B.DeleteToken("token"), IsNil)

	_, err = s.B.GetToken("token")
	c.Assert(err, FitsTypeOf, &backend.NotFoundError{})
}

func (s *BackendSuite) RemoteCertCRUD(c *C) {
	out, err := s.B.GetRemoteCerts(backend.HostCert, "")
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []backend.RemoteCert{})

	ca := backend.RemoteCert{
		Type:  backend.HostCert,
		ID:    "c1",
		FQDN:  "example.com",
		Value: []byte("hello"),
	}
	c.Assert(s.B.UpsertRemoteCert(ca, 0), IsNil)

	out, err = s.B.GetRemoteCerts(backend.HostCert, ca.FQDN)
	c.Assert(err, IsNil)
	c.Assert(out[0], DeepEquals, ca)

	ca2 := backend.RemoteCert{
		Type:  backend.HostCert,
		ID:    "c2",
		FQDN:  "example.org",
		Value: []byte("hello2"),
	}
	c.Assert(s.B.UpsertRemoteCert(ca2, 0), IsNil)

	out, err = s.B.GetRemoteCerts(backend.HostCert, ca2.FQDN)
	c.Assert(err, IsNil)
	c.Assert(out[0], DeepEquals, ca2)

	out, err = s.B.GetRemoteCerts(backend.HostCert, "")
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 2)

	certs := make(map[string]backend.RemoteCert)
	for _, c := range out {
		certs[c.FQDN+c.ID] = c
	}
	c.Assert(certs[ca.FQDN+ca.ID], DeepEquals, ca)
	c.Assert(certs[ca2.FQDN+ca2.ID], DeepEquals, ca2)

	// Update ca
	ca.Value = []byte("hello updated")
	c.Assert(s.B.UpsertRemoteCert(ca, 0), IsNil)

	out, err = s.B.GetRemoteCerts(backend.HostCert, ca.FQDN)
	c.Assert(err, IsNil)
	c.Assert(out[0], DeepEquals, ca)

	err = s.B.DeleteRemoteCert(backend.HostCert, ca.FQDN, ca.ID)
	c.Assert(err, IsNil)

	err = s.B.DeleteRemoteCert(backend.HostCert, ca.FQDN, ca.ID)
	c.Assert(err, FitsTypeOf, &backend.NotFoundError{})
}

func (s *BackendSuite) BasicCRUD(c *C) {
	keys, err := s.B.GetKeys([]string{"keys"})
	c.Assert(err, IsNil)
	c.Assert(keys, DeepEquals, []string{})

	c.Assert(s.B.UpsertVal([]string{"a", "b"}, "bkey", []byte("val1"), 0), IsNil)
	c.Assert(s.B.UpsertVal([]string{"a", "b"}, "akey", []byte("val2"), 0), IsNil)

	keys, err = s.B.GetKeys([]string{"a", "b"})
	c.Assert(err, IsNil)
	c.Assert(keys, DeepEquals, []string{"akey", "bkey"})

	out, err := s.B.GetVal([]string{"a", "b"}, "bkey")
	c.Assert(err, IsNil)
	c.Assert(string(out), Equals, "val1")

	c.Assert(s.B.UpsertVal([]string{"a", "b"}, "bkey", []byte("val-updated"), 0), IsNil)
	out, err = s.B.GetVal([]string{"a", "b"}, "bkey")
	c.Assert(err, IsNil)
	c.Assert(string(out), Equals, "val-updated")

	c.Assert(s.B.DeleteKey([]string{"a", "b"}, "bkey"), IsNil)
	c.Assert(s.B.DeleteKey([]string{"a", "b"}, "bkey"), FitsTypeOf, &backend.NotFoundError{})
}

func (s *BackendSuite) Expiration(c *C) {
	c.Assert(s.B.UpsertVal([]string{"a", "b"}, "bkey", []byte("val1"), time.Second), IsNil)
	c.Assert(s.B.UpsertVal([]string{"a", "b"}, "akey", []byte("val2"), 0), IsNil)

	time.Sleep(2 * time.Second)

	keys, err := s.B.GetKeys([]string{"a", "b"})
	c.Assert(err, IsNil)
	c.Assert(keys, DeepEquals, []string{"akey"})
}

func toSet(vals []string) map[string]struct{} {
	out := make(map[string]struct{}, len(vals))
	for _, v := range vals {
		out[v] = struct{}{}
	}
	return out
}

func randomToken() string {
	token, err := (&random.CSPRNG{}).HexDigest(32)
	if err != nil {
		panic(err)
	}
	return token
}
