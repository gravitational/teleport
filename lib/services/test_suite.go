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
package services

import (
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"

	"github.com/gokyle/hotp"
	"github.com/mailgun/lemma/random"
	"golang.org/x/crypto/ssh"

	. "gopkg.in/check.v1"
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
	ca := LocalCertificateAuthority{
		CertificateAuthority: CertificateAuthority{
			PublicKey:  []byte("capub"),
			ID:         "id1",
			Type:       UserCert,
			DomainName: "host1",
		},
		PrivateKey: []byte("capriv"),
	}
	c.Assert(s.CAS.UpsertUserCertificateAuthority(ca), IsNil)

	out, err := s.CAS.GetUserPrivateCertificateAuthority()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, &ca)

	outp, err := s.CAS.GetUserCertificateAuthority()
	c.Assert(err, IsNil)
	c.Assert(outp, DeepEquals, &ca.CertificateAuthority)
}

func (s ServicesTestSuite) HostCACRUD(c *C) {
	ca := LocalCertificateAuthority{
		CertificateAuthority: CertificateAuthority{
			PublicKey:  []byte("capub"),
			ID:         "id2",
			Type:       HostCert,
			DomainName: "host2",
		},
		PrivateKey: []byte("capriv"),
	}
	c.Assert(s.CAS.UpsertHostCertificateAuthority(ca), IsNil)

	out, err := s.CAS.GetHostPrivateCertificateAuthority()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, &ca)

	outp, err := s.CAS.GetHostCertificateAuthority()
	c.Assert(err, IsNil)
	c.Assert(outp, DeepEquals, &ca.CertificateAuthority)
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

	c.Assert(s.ProvisioningS.UpsertToken("token", "a.example.com", "RoleExample", 0), IsNil)

	token, err := s.ProvisioningS.GetToken("token")
	c.Assert(token.DomainName, Equals, "a.example.com")
	c.Assert(token.Role, Equals, "RoleExample")
	c.Assert(err, IsNil)

	c.Assert(s.ProvisioningS.DeleteToken("token"), IsNil)

	_, err = s.ProvisioningS.GetToken("token")
	c.Assert(err, FitsTypeOf, &teleport.NotFoundError{})

	outputToken, err := JoinTokenRole("token1", "Auth")
	c.Assert(err, IsNil)

	tok, role, err := SplitTokenRole(outputToken)
	c.Assert(err, IsNil)
	c.Assert(tok, Equals, "token1")
	c.Assert(role, Equals, "Auth")
}

func (s *ServicesTestSuite) RemoteCertCRUD(c *C) {
	out, err := s.CAS.GetRemoteCertificates(HostCert, "")
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []CertificateAuthority{})

	ca := CertificateAuthority{
		Type:       HostCert,
		ID:         "c1",
		DomainName: "example.com",
		PublicKey:  []byte("hello"),
	}
	c.Assert(s.CAS.UpsertRemoteCertificate(ca, 0), IsNil)

	out, err = s.CAS.GetRemoteCertificates(HostCert, ca.DomainName)
	c.Assert(err, IsNil)
	c.Assert(out[0], DeepEquals, ca)

	ca2 := CertificateAuthority{
		Type:       HostCert,
		ID:         "c2",
		DomainName: "example.org",
		PublicKey:  []byte("hello2"),
	}
	c.Assert(s.CAS.UpsertRemoteCertificate(ca2, 0), IsNil)

	out, err = s.CAS.GetRemoteCertificates(HostCert, ca2.DomainName)
	c.Assert(err, IsNil)
	c.Assert(out[0], DeepEquals, ca2)

	out, err = s.CAS.GetRemoteCertificates(HostCert, "")
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 2)

	certs := make(map[string]CertificateAuthority)
	for _, c := range out {
		certs[c.DomainName+c.ID] = c
	}
	c.Assert(certs[ca.DomainName+ca.ID], DeepEquals, ca)
	c.Assert(certs[ca2.DomainName+ca2.ID], DeepEquals, ca2)

	// Update ca
	ca.PublicKey = []byte("hello updated")
	c.Assert(s.CAS.UpsertRemoteCertificate(ca, 0), IsNil)

	out, err = s.CAS.GetRemoteCertificates(HostCert, ca.DomainName)
	c.Assert(err, IsNil)
	c.Assert(out[0], DeepEquals, ca)

	err = s.CAS.DeleteRemoteCertificate(HostCert, ca.DomainName, ca.ID)
	c.Assert(err, IsNil)

	err = s.CAS.DeleteRemoteCertificate(HostCert, ca.DomainName, ca.ID)
	c.Assert(err, NotNil)
}

func (s *ServicesTestSuite) TrustedCertificates(c *C) {
	a := testauthority.New()

	priv1, pub1, err := a.GenerateKeyPair("")
	c.Assert(err, IsNil)
	key1, _, _, _, err := ssh.ParseAuthorizedKey(pub1)
	c.Assert(err, IsNil)

	userCA := LocalCertificateAuthority{
		CertificateAuthority: CertificateAuthority{
			PublicKey:  pub1,
			ID:         "id1",
			Type:       UserCert,
			DomainName: "host1",
		},
		PrivateKey: priv1,
	}
	c.Assert(s.CAS.UpsertUserCertificateAuthority(userCA), IsNil)
	userPubCA, err := s.CAS.GetUserCertificateAuthority()
	c.Assert(err, IsNil)

	hostCA := LocalCertificateAuthority{
		CertificateAuthority: CertificateAuthority{
			PublicKey:  []byte("capub"),
			ID:         "id2",
			Type:       UserCert,
			DomainName: "host1",
		},
		PrivateKey: []byte("capriv"),
	}
	c.Assert(s.CAS.UpsertHostCertificateAuthority(hostCA), IsNil)
	hostPubCA, err := s.CAS.GetHostCertificateAuthority()
	c.Assert(err, IsNil)

	ca1 := CertificateAuthority{
		Type:       UserCert,
		ID:         "c1",
		DomainName: "example.com",
		PublicKey:  []byte("hello1"),
	}
	c.Assert(s.CAS.UpsertRemoteCertificate(ca1, 0), IsNil)

	ca2 := CertificateAuthority{
		Type:       UserCert,
		ID:         "c2",
		DomainName: "example.org",
		PublicKey:  []byte("hello2"),
	}
	c.Assert(s.CAS.UpsertRemoteCertificate(ca2, 0), IsNil)

	remoteUserCAs, err := s.CAS.GetRemoteCertificates(UserCert, "")
	c.Assert(err, IsNil)
	remoteHostCAs, err := s.CAS.GetRemoteCertificates(HostCert, "")
	c.Assert(err, IsNil)

	trustedUserCertificates, err := s.CAS.GetTrustedCertificates(UserCert)
	c.Assert(err, IsNil)
	trustedHostCertificates, err := s.CAS.GetTrustedCertificates(HostCert)
	c.Assert(err, IsNil)

	c.Assert(trustedUserCertificates, DeepEquals, append(remoteUserCAs, *userPubCA))
	c.Assert(trustedHostCertificates, DeepEquals, append(remoteHostCAs, *hostPubCA))

	id1, found, err := s.CAS.GetCertificateID(UserCert, key1)
	c.Assert(err, IsNil)
	c.Assert(found, Equals, true)
	c.Assert(id1, Equals, "id1")
}

func (s *ServicesTestSuite) UserMapping(c *C) {
	c.Assert(s.CAS.UpsertUserMapping("a1", "b1", "c1", 0), IsNil)
	c.Assert(s.CAS.UpsertUserMapping("a2", "b2", "c2", 0), IsNil)
	c.Assert(s.CAS.UpsertUserMapping("a3", "b3", "c3", 0), IsNil)
	c.Assert(s.CAS.UpsertUserMapping("a4", "b4", "c4", time.Millisecond*200), IsNil)

	ok, err := s.CAS.UserMappingExists("a1", "b1", "c1")
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, true)
	ok, err = s.CAS.UserMappingExists("a2", "b2", "c2")
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, true)
	ok, err = s.CAS.UserMappingExists("a3", "b3", "c3")
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, true)
	ok, err = s.CAS.UserMappingExists("a4", "b4", "c4")
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, true)
	ok, err = s.CAS.UserMappingExists("a5", "b5", "c5")
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, false)

	c.Assert(s.CAS.DeleteUserMapping("a2", "b2", "c2"), IsNil)

	time.Sleep(time.Millisecond * 300)

	ok, err = s.CAS.UserMappingExists("a1", "b1", "c1")
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, true)
	ok, err = s.CAS.UserMappingExists("a2", "b2", "c2")
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, false)
	ok, err = s.CAS.UserMappingExists("a3", "b3", "c3")
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, true)
	ok, err = s.CAS.UserMappingExists("a4", "b4", "c4")
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, false)

	hashes, err := s.CAS.GetAllUserMappings()
	c.Assert(err, IsNil)

	c.Assert(s.CAS.DeleteUserMapping("a1", "b1", "c1"), IsNil)

	ok, err = s.CAS.UserMappingExists("a1", "b1", "c1")
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, false)
	ok, err = s.CAS.UserMappingExists("a2", "b2", "c2")
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, false)
	ok, err = s.CAS.UserMappingExists("a3", "b3", "c3")
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, true)
	ok, err = s.CAS.UserMappingExists("a4", "b4", "c4")
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, false)

	c.Assert(s.CAS.UpdateUserMappings(hashes, time.Minute), IsNil)

	ok, err = s.CAS.UserMappingExists("a1", "b1", "c1")
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, true)
	ok, err = s.CAS.UserMappingExists("a2", "b2", "c2")
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, false)
	ok, err = s.CAS.UserMappingExists("a3", "b3", "c3")
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, true)
	ok, err = s.CAS.UserMappingExists("a4", "b4", "c4")
	c.Assert(err, IsNil)
	c.Assert(ok, Equals, false)
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

	_ = otp.OTP()
	_ = otp.OTP()
	_ = otp.OTP()
	token6 := otp.OTP()
	token7 := otp.OTP()
	c.Assert(s.WebS.CheckPassword("user1", pass, token7), FitsTypeOf, &teleport.BadParameterError{})
	c.Assert(s.WebS.CheckPassword("user1", pass, token6), IsNil)
	c.Assert(s.WebS.CheckPassword("user1", pass, "123456"), FitsTypeOf, &teleport.BadParameterError{})
	c.Assert(s.WebS.CheckPassword("user1", pass, token7), IsNil)

	_ = otp.OTP()
	token9 := otp.OTP()
	c.Assert(s.WebS.CheckPassword("user1", pass, token9), IsNil)

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
