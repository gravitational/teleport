package services

import (
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/mailgun/lemma/random"
	"github.com/gravitational/teleport/lib/backend"

	"github.com/gravitational/teleport/Godeps/_workspace/src/github.com/gokyle/hotp"

	. "github.com/gravitational/teleport/Godeps/_workspace/src/gopkg.in/check.v1"
)

type ServicesTestSuite struct {
	CAS           *CAService
	LockS         *LockService
	PresenceS     *PresenceService
	ProvisioningS *ProvisioningService
	UserS         *UserService
	WebS          *WebService
	ChangesC      chan interface{}
}

func NewServicesTestSuite(backend backend.Backend) *ServicesTestSuite {
	s := ServicesTestSuite{}
	s.CAS = NewCAService(backend)
	s.LockS = NewLockService(backend)
	s.PresenceS = NewPresenceService(backend)
	s.ProvisioningS = NewProvisioningService(backend)
	s.UserS = NewUserService(backend)
	s.WebS = NewWebService(backend)
	s.ChangesC = make(chan interface{})
	return &s
}

func (s *ServicesTestSuite) collectChanges(c *C, expected int) []interface{} {
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

func (s *ServicesTestSuite) expectChanges(c *C, expected ...interface{}) {
	changes := s.collectChanges(c, len(expected))
	for i, ch := range changes {
		c.Assert(ch, DeepEquals, expected[i])
	}
}

func (s *ServicesTestSuite) UserKeyCRUD(c *C) {
	k := AuthorizedKey{ID: "id1", Value: []byte("val1")}

	c.Assert(s.UserS.UpsertUserKey("user1", k, 0), IsNil)

	keys, err := s.UserS.GetUserKeys("user1")
	c.Assert(err, IsNil)
	c.Assert(keys, DeepEquals, []AuthorizedKey{k})

	c.Assert(s.UserS.DeleteUserKey("user1", k.ID), IsNil)

	keys, err = s.UserS.GetUserKeys("user1")
	c.Assert(err, IsNil)
	c.Assert(keys, DeepEquals, []AuthorizedKey{})
}

func (s *ServicesTestSuite) UsersCRUD(c *C) {
	u, err := s.UserS.GetUsers()
	c.Assert(err, IsNil)
	c.Assert(len(u), Equals, 0)

	k := AuthorizedKey{ID: "id1", Value: []byte("val1")}

	c.Assert(s.UserS.UpsertUserKey("user1", k, 0), IsNil)
	c.Assert(s.UserS.UpsertUserKey("user2", k, 0), IsNil)

	u, err = s.UserS.GetUsers()
	c.Assert(err, IsNil)
	c.Assert(toSet(u), DeepEquals, map[string]struct{}{"user1": struct{}{}, "user2": struct{}{}})

	c.Assert(s.UserS.DeleteUser("user1"), IsNil)

	u, err = s.UserS.GetUsers()
	c.Assert(err, IsNil)
	c.Assert(toSet(u), DeepEquals, map[string]struct{}{"user2": struct{}{}})

	c.Assert(s.UserS.DeleteUser("user1"), FitsTypeOf, &teleport.NotFoundError{})
}

func (s *ServicesTestSuite) UserCACRUD(c *C) {
	ca := CA{
		Pub:  []byte("capub"),
		Priv: []byte("capriv"),
	}
	c.Assert(s.CAS.UpsertUserCA(ca), IsNil)

	out, err := s.CAS.GetUserCA()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, &ca)

	outp, err := s.CAS.GetUserCAPub()
	c.Assert(err, IsNil)
	c.Assert(outp, DeepEquals, ca.Pub)
}

func (s *ServicesTestSuite) HostCACRUD(c *C) {
	ca := CA{
		Pub:  []byte("capub"),
		Priv: []byte("capriv"),
	}
	c.Assert(s.CAS.UpsertHostCA(ca), IsNil)

	out, err := s.CAS.GetHostCA()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, &ca)

	outp, err := s.CAS.GetHostCAPub()
	c.Assert(err, IsNil)
	c.Assert(outp, DeepEquals, ca.Pub)
}

func (s *ServicesTestSuite) ServerCRUD(c *C) {
	out, err := s.PresenceS.GetServers()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	srv := Server{ID: "srv1", Addr: "localhost:2022"}
	c.Assert(s.PresenceS.UpsertServer(srv, 0), IsNil)

	out, err = s.PresenceS.GetServers()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []Server{srv})
}

func (s *ServicesTestSuite) PasswordHashCRUD(c *C) {
	_, err := s.WebS.GetPasswordHash("user1")
	c.Assert(err, FitsTypeOf, &teleport.NotFoundError{})

	err = s.WebS.UpsertPasswordHash("user1", []byte("hello123"))
	c.Assert(err, IsNil)

	hash, err := s.WebS.GetPasswordHash("user1")
	c.Assert(err, IsNil)
	c.Assert(hash, DeepEquals, []byte("hello123"))

	err = s.WebS.UpsertPasswordHash("user1", []byte("hello321"))
	c.Assert(err, IsNil)

	hash, err = s.WebS.GetPasswordHash("user1")
	c.Assert(err, IsNil)
	c.Assert(hash, DeepEquals, []byte("hello321"))
}

func (s *ServicesTestSuite) WebSessionCRUD(c *C) {
	_, err := s.WebS.GetWebSession("user1", "sid1")
	c.Assert(err, FitsTypeOf, &teleport.NotFoundError{})

	ws := WebSession{Pub: []byte("pub123"), Priv: []byte("priv123")}
	err = s.WebS.UpsertWebSession("user1", "sid1", ws, 0)
	c.Assert(err, IsNil)

	out, err := s.WebS.GetWebSession("user1", "sid1")
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, &ws)

	ws1 := WebSession{Pub: []byte("pub321"), Priv: []byte("priv321")}
	err = s.WebS.UpsertWebSession("user1", "sid1", ws1, 0)
	c.Assert(err, IsNil)

	out2, err := s.WebS.GetWebSession("user1", "sid1")
	c.Assert(err, IsNil)
	c.Assert(out2, DeepEquals, &ws1)

	keys, err := s.WebS.GetWebSessionsKeys("user1")
	c.Assert(err, IsNil)
	c.Assert(keys[0].Value, DeepEquals, out2.Pub)

	c.Assert(s.WebS.DeleteWebSession("user1", "sid1"), IsNil)

	_, err = s.WebS.GetWebSession("user1", "sid1")
	c.Assert(err, FitsTypeOf, &teleport.NotFoundError{})
}

func (s *ServicesTestSuite) WebTunCRUD(c *C) {
	_, err := s.WebS.GetWebTun("p1")
	c.Assert(err, FitsTypeOf, &teleport.NotFoundError{})

	t := WebTun{
		Prefix:     "p1",
		TargetAddr: "http://localhost:5000",
		ProxyAddr:  "node1.gravitational.io",
	}
	c.Assert(s.WebS.UpsertWebTun(t, 0), IsNil)

	out, err := s.WebS.GetWebTun("p1")
	c.Assert(out, DeepEquals, &t)

	tuns, err := s.WebS.GetWebTuns()
	c.Assert(err, IsNil)
	c.Assert(tuns, DeepEquals, []WebTun{t})

	t1 := WebTun{
		Prefix:     "p1",
		TargetAddr: "http://localhost:5001",
		ProxyAddr:  "node1.gravitational2.io",
	}
	c.Assert(s.WebS.UpsertWebTun(t1, 0), IsNil)

	out, err = s.WebS.GetWebTun("p1")
	c.Assert(out, DeepEquals, &t1)

	c.Assert(s.WebS.DeleteWebTun("p1"), IsNil)

	_, err = s.WebS.GetWebTun("p1")
	c.Assert(err, FitsTypeOf, &teleport.NotFoundError{})
}

func (s *ServicesTestSuite) Locking(c *C) {
	tok1 := "token1"
	tok2 := "token2"

	c.Assert(s.LockS.ReleaseLock(tok1), FitsTypeOf, &teleport.NotFoundError{})

	c.Assert(s.LockS.AcquireLock(tok1, 30*time.Second), IsNil)
	x := 7
	go func() {
		time.Sleep(1 * time.Second)
		x = 9
		c.Assert(s.LockS.ReleaseLock(tok1), IsNil)
	}()
	c.Assert(s.LockS.AcquireLock(tok1, 0), IsNil)
	x = x * 2
	c.Assert(x, Equals, 18)
	c.Assert(s.LockS.ReleaseLock(tok1), IsNil)

	c.Assert(s.LockS.AcquireLock(tok1, 0), IsNil)
	x = 7
	go func() {
		time.Sleep(1 * time.Second)
		x = 9
		c.Assert(s.LockS.ReleaseLock(tok1), IsNil)
	}()
	c.Assert(s.LockS.AcquireLock(tok1, 0), IsNil)
	x = x * 2
	c.Assert(x, Equals, 18)
	c.Assert(s.LockS.ReleaseLock(tok1), IsNil)

	y := 0
	go func() {
		c.Assert(s.LockS.AcquireLock(tok1, 0), IsNil)
		c.Assert(s.LockS.AcquireLock(tok2, 0), IsNil)

		c.Assert(s.LockS.ReleaseLock(tok1), IsNil)
		c.Assert(s.LockS.ReleaseLock(tok2), IsNil)
		y = 15
	}()

	time.Sleep(1 * time.Second)
	c.Assert(y, Equals, 15)

	c.Assert(s.LockS.ReleaseLock(tok1), FitsTypeOf, &teleport.NotFoundError{})
}

func (s *ServicesTestSuite) TokenCRUD(c *C) {
	_, err := s.ProvisioningS.GetToken("token")
	c.Assert(err, FitsTypeOf, &teleport.NotFoundError{})

	c.Assert(s.ProvisioningS.UpsertToken("token", "a.example.com", 0), IsNil)

	out, err := s.ProvisioningS.GetToken("token")
	c.Assert(out, Equals, "a.example.com")
	c.Assert(err, IsNil)

	c.Assert(s.ProvisioningS.DeleteToken("token"), IsNil)

	_, err = s.ProvisioningS.GetToken("token")
	c.Assert(err, FitsTypeOf, &teleport.NotFoundError{})
}

func (s *ServicesTestSuite) RemoteCertCRUD(c *C) {
	out, err := s.CAS.GetRemoteCerts(HostCert, "")
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []RemoteCert{})

	ca := RemoteCert{
		Type:  HostCert,
		ID:    "c1",
		FQDN:  "example.com",
		Value: []byte("hello"),
	}
	c.Assert(s.CAS.UpsertRemoteCert(ca, 0), IsNil)

	out, err = s.CAS.GetRemoteCerts(HostCert, ca.FQDN)
	c.Assert(err, IsNil)
	c.Assert(out[0], DeepEquals, ca)

	ca2 := RemoteCert{
		Type:  HostCert,
		ID:    "c2",
		FQDN:  "example.org",
		Value: []byte("hello2"),
	}
	c.Assert(s.CAS.UpsertRemoteCert(ca2, 0), IsNil)

	out, err = s.CAS.GetRemoteCerts(HostCert, ca2.FQDN)
	c.Assert(err, IsNil)
	c.Assert(out[0], DeepEquals, ca2)

	out, err = s.CAS.GetRemoteCerts(HostCert, "")
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 2)

	certs := make(map[string]RemoteCert)
	for _, c := range out {
		certs[c.FQDN+c.ID] = c
	}
	c.Assert(certs[ca.FQDN+ca.ID], DeepEquals, ca)
	c.Assert(certs[ca2.FQDN+ca2.ID], DeepEquals, ca2)

	// Update ca
	ca.Value = []byte("hello updated")
	c.Assert(s.CAS.UpsertRemoteCert(ca, 0), IsNil)

	out, err = s.CAS.GetRemoteCerts(HostCert, ca.FQDN)
	c.Assert(err, IsNil)
	c.Assert(out[0], DeepEquals, ca)

	err = s.CAS.DeleteRemoteCert(HostCert, ca.FQDN, ca.ID)
	c.Assert(err, IsNil)

	err = s.CAS.DeleteRemoteCert(HostCert, ca.FQDN, ca.ID)
	c.Assert(err, FitsTypeOf, &teleport.NotFoundError{})
}

func (s *ServicesTestSuite) PasswordCRUD(c *C) {
	pass := []byte("abc123")

	err := s.WebS.CheckPassword("user1", pass, "123456")
	c.Assert(err, NotNil)

	hotpURL, _, err := s.WebS.UpsertPassword("user1", pass)
	c.Assert(err, IsNil)

	otp, label, err := hotp.FromURL(hotpURL)
	c.Assert(err, IsNil)
	c.Assert(label, Equals, "user1")
	otp.Increment()

	token1 := otp.OTP()
	c.Assert(s.WebS.CheckPassword("user1", pass, "123456"), FitsTypeOf, &teleport.BadParameterError{})
	c.Assert(s.WebS.CheckPassword("user1", pass, token1), IsNil)
	c.Assert(s.WebS.CheckPassword("user1", pass, token1), FitsTypeOf, &teleport.BadParameterError{})

	token2 := otp.OTP()
	c.Assert(s.WebS.CheckPassword("user1", []byte("abc123123"), token2), FitsTypeOf, &teleport.BadParameterError{})
	c.Assert(s.WebS.CheckPassword("user1", pass, "123456"), FitsTypeOf, &teleport.BadParameterError{})
	c.Assert(s.WebS.CheckPassword("user1", pass, token2), IsNil)
	c.Assert(s.WebS.CheckPassword("user1", pass, token1), FitsTypeOf, &teleport.BadParameterError{})

	token3 := otp.OTP()
	token4 := otp.OTP()
	c.Assert(s.WebS.CheckPassword("user1", pass, token4), FitsTypeOf, &teleport.BadParameterError{})
	c.Assert(s.WebS.CheckPassword("user1", pass, token3), IsNil)
	c.Assert(s.WebS.CheckPassword("user1", pass, "123456"), FitsTypeOf, &teleport.BadParameterError{})
	c.Assert(s.WebS.CheckPassword("user1", pass, token4), IsNil)
}

func (s *ServicesTestSuite) PasswordGarbage(c *C) {
	garbage := [][]byte{
		nil,
		make([]byte, MaxPasswordLength+1),
		make([]byte, MinPasswordLength-1),
	}
	for _, g := range garbage {
		err := s.WebS.CheckPassword("user1", g, "123456")
		c.Assert(err, NotNil)
	}
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
