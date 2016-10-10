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

package suite

import (
	"sort"
	"sync/atomic"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"

	"github.com/gokyle/hotp"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	. "gopkg.in/check.v1"
)

// NewTestCA returns new test authority with a test key as a public and
// signing key
func NewTestCA(caType services.CertAuthType, domainName string) *services.CertAuthority {
	keyBytes := PEMBytes["rsa"]
	key, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		panic(err)
	}
	pubKey := key.PublicKey()

	return &services.CertAuthority{
		Type:         caType,
		DomainName:   domainName,
		CheckingKeys: [][]byte{ssh.MarshalAuthorizedKey(pubKey)},
		SigningKeys:  [][]byte{keyBytes},
	}
}

type ServicesTestSuite struct {
	CAS           services.Trust
	LockS         services.Lock
	PresenceS     services.Presence
	ProvisioningS services.Provisioner
	WebS          services.Identity
	ChangesC      chan interface{}
}

func (s *ServicesTestSuite) collectChanges(c *C, expected int) []interface{} {
	changes := make([]interface{}, expected)
	for i := range changes {
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

func userSlicesEqual(c *C, a []services.User, b []services.User) {
	comment := Commentf("a: %#v b: %#v", a, b)
	c.Assert(len(a), Equals, len(b), comment)
	sort.Sort(services.Users(a))
	sort.Sort(services.Users(b))
	for i := range a {
		usersEqual(c, a[i], b[i])
	}
}

func usersEqual(c *C, a services.User, b services.User) {
	comment := Commentf("a: %#v b: %#v", a, b)
	c.Assert(a.Equals(b), Equals, true, comment)
}

func (s *ServicesTestSuite) UsersCRUD(c *C) {
	u, err := s.WebS.GetUsers()
	c.Assert(err, IsNil)
	c.Assert(len(u), Equals, 0)

	c.Assert(s.WebS.UpsertPasswordHash("user1", []byte("hash")), IsNil)
	c.Assert(s.WebS.UpsertPasswordHash("user2", []byte("hash2")), IsNil)

	u, err = s.WebS.GetUsers()
	c.Assert(err, IsNil)
	userSlicesEqual(c, u, []services.User{
		&services.TeleportUser{Name: "user1"}, &services.TeleportUser{Name: "user2"}})

	out, err := s.WebS.GetUser("user1")
	c.Assert(err, IsNil)
	usersEqual(c, out, &services.TeleportUser{Name: "user1"})

	user := &services.TeleportUser{Name: "user1", AllowedLogins: []string{"admin", "root"}}
	c.Assert(s.WebS.UpsertUser(user), IsNil)

	out, err = s.WebS.GetUser("user1")
	c.Assert(err, IsNil)
	usersEqual(c, out, user)

	user.AllowedLogins = nil
	c.Assert(s.WebS.UpsertUser(user), IsNil)

	out, err = s.WebS.GetUser("user1")
	c.Assert(err, IsNil)
	usersEqual(c, out, user)

	c.Assert(s.WebS.DeleteUser("user1"), IsNil)

	u, err = s.WebS.GetUsers()
	c.Assert(err, IsNil)
	userSlicesEqual(c, u, []services.User{&services.TeleportUser{Name: "user2"}})

	err = s.WebS.DeleteUser("user1")
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("unexpected %T %#v", err, err))

	// bad username
	err = s.WebS.UpsertUser(&services.TeleportUser{Name: ""})
	c.Assert(trace.IsBadParameter(err), Equals, true, Commentf("expected bad parameter error, got %T", err))

	// bad allowed login
	err = s.WebS.UpsertUser(&services.TeleportUser{Name: "bob", AllowedLogins: []string{"oops  typo!"}})
	c.Assert(trace.IsBadParameter(err), Equals, true, Commentf("expected bad parameter error, got %T", err))
}

func (s *ServicesTestSuite) CertAuthCRUD(c *C) {
	ca := NewTestCA(services.UserCA, "example.com")
	c.Assert(s.CAS.UpsertCertAuthority(
		*ca, backend.Forever), IsNil)

	out, err := s.CAS.GetCertAuthority(*ca.ID(), true)
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, ca)

	cas, err := s.CAS.GetCertAuthorities(services.UserCA, false)
	c.Assert(err, IsNil)
	ca2 := *ca
	ca2.SigningKeys = nil
	c.Assert(cas[0], DeepEquals, &ca2)

	cas, err = s.CAS.GetCertAuthorities(services.UserCA, true)
	c.Assert(err, IsNil)
	c.Assert(cas[0], DeepEquals, ca)

	err = s.CAS.DeleteCertAuthority(*ca.ID())
	c.Assert(err, IsNil)
}

func (s *ServicesTestSuite) ServerCRUD(c *C) {
	out, err := s.PresenceS.GetNodes()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	srv := services.Server{ID: "srv1", Addr: "localhost:2022"}
	c.Assert(s.PresenceS.UpsertNode(srv, 0), IsNil)

	out, err = s.PresenceS.GetNodes()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []services.Server{srv})

	out, err = s.PresenceS.GetProxies()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	proxy := services.Server{ID: "proxy1", Addr: "localhost:2023"}
	c.Assert(s.PresenceS.UpsertProxy(proxy, 0), IsNil)

	out, err = s.PresenceS.GetProxies()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []services.Server{proxy})

	out, err = s.PresenceS.GetAuthServers()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	auth := services.Server{ID: "auth1", Addr: "localhost:2025"}
	c.Assert(s.PresenceS.UpsertAuthServer(auth, 0), IsNil)

	out, err = s.PresenceS.GetAuthServers()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []services.Server{auth})
}

func (s *ServicesTestSuite) ReverseTunnelsCRUD(c *C) {
	out, err := s.PresenceS.GetReverseTunnels()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	tunnel := services.ReverseTunnel{DomainName: "example.com", DialAddrs: []string{"example.com:2023"}}
	c.Assert(s.PresenceS.UpsertReverseTunnel(tunnel, 0), IsNil)

	out, err = s.PresenceS.GetReverseTunnels()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []services.ReverseTunnel{tunnel})

	err = s.PresenceS.DeleteReverseTunnel(tunnel.DomainName)
	c.Assert(err, IsNil)

	out, err = s.PresenceS.GetReverseTunnels()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	err = s.PresenceS.UpsertReverseTunnel(services.ReverseTunnel{DomainName: "", DialAddrs: []string{"example.com:2023"}}, 0)
	c.Assert(trace.IsBadParameter(err), Equals, true, Commentf("%#v", err))

	err = s.PresenceS.UpsertReverseTunnel(services.ReverseTunnel{DomainName: "example.com", DialAddrs: []string{"bad address"}}, 0)
	c.Assert(trace.IsBadParameter(err), Equals, true, Commentf("%#v", err))

	err = s.PresenceS.UpsertReverseTunnel(services.ReverseTunnel{DomainName: "example.com"}, 0)
	c.Assert(trace.IsBadParameter(err), Equals, true, Commentf("%#v", err))
}

func (s *ServicesTestSuite) PasswordHashCRUD(c *C) {
	_, err := s.WebS.GetPasswordHash("user1")
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))

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
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))

	dt := time.Date(2015, 6, 5, 4, 3, 2, 1, time.UTC).UTC()
	ws := services.WebSession{
		Pub:     []byte("pub123"),
		Priv:    []byte("priv123"),
		Expires: dt,
	}
	err = s.WebS.UpsertWebSession("user1", "sid1", ws, 0)
	c.Assert(err, IsNil)

	out, err := s.WebS.GetWebSession("user1", "sid1")
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, &ws)

	ws1 := services.WebSession{Pub: []byte("pub321"), Priv: []byte("priv321"), Expires: dt}
	err = s.WebS.UpsertWebSession("user1", "sid1", ws1, 0)
	c.Assert(err, IsNil)

	out2, err := s.WebS.GetWebSession("user1", "sid1")
	c.Assert(err, IsNil)
	c.Assert(out2, DeepEquals, &ws1)

	c.Assert(s.WebS.DeleteWebSession("user1", "sid1"), IsNil)

	_, err = s.WebS.GetWebSession("user1", "sid1")
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))
}

func (s *ServicesTestSuite) Locking(c *C) {
	tok1 := "token1"
	tok2 := "token2"

	err := s.LockS.ReleaseLock(tok1)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))

	c.Assert(s.LockS.AcquireLock(tok1, 30*time.Second), IsNil)
	x := int32(7)
	go func() {
		atomic.StoreInt32(&x, 9)
		c.Assert(s.LockS.ReleaseLock(tok1), IsNil)
	}()
	c.Assert(s.LockS.AcquireLock(tok1, 0), IsNil)
	atomic.AddInt32(&x, 9)

	c.Assert(atomic.LoadInt32(&x), Equals, int32(18))
	c.Assert(s.LockS.ReleaseLock(tok1), IsNil)

	c.Assert(s.LockS.AcquireLock(tok1, 0), IsNil)
	atomic.StoreInt32(&x, 7)
	go func() {
		atomic.StoreInt32(&x, 9)
		c.Assert(s.LockS.ReleaseLock(tok1), IsNil)
	}()
	c.Assert(s.LockS.AcquireLock(tok1, 0), IsNil)
	atomic.AddInt32(&x, 9)
	c.Assert(atomic.LoadInt32(&x), Equals, int32(18))
	c.Assert(s.LockS.ReleaseLock(tok1), IsNil)

	y := int32(0)
	go func() {
		c.Assert(s.LockS.AcquireLock(tok1, 0), IsNil)
		c.Assert(s.LockS.AcquireLock(tok2, 0), IsNil)

		c.Assert(s.LockS.ReleaseLock(tok1), IsNil)
		c.Assert(s.LockS.ReleaseLock(tok2), IsNil)
		atomic.StoreInt32(&y, 15)
	}()

	time.Sleep(1 * time.Second)
	c.Assert(atomic.LoadInt32(&y), Equals, int32(15))

	err = s.LockS.ReleaseLock(tok1)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))
}

func (s *ServicesTestSuite) TokenCRUD(c *C) {
	_, err := s.ProvisioningS.GetToken("token")
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))

	c.Assert(s.ProvisioningS.UpsertToken("token", teleport.Roles{teleport.RoleAuth, teleport.RoleNode}, 0), IsNil)

	token, err := s.ProvisioningS.GetToken("token")
	c.Assert(token.Roles.Include(teleport.RoleAuth), Equals, true)
	c.Assert(token.Roles.Include(teleport.RoleNode), Equals, true)
	c.Assert(token.Roles.Include(teleport.RoleProxy), Equals, false)
	c.Assert(token.Expires.Second(), Equals, time.Now().UTC().Add(defaults.ProvisioningTokenTTL).Second())
	c.Assert(err, IsNil)

	c.Assert(s.ProvisioningS.DeleteToken("token"), IsNil)

	_, err = s.ProvisioningS.GetToken("token")
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))
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
	err = s.WebS.CheckPassword("user1", pass, "123456")
	c.Assert(trace.IsAccessDenied(err), Equals, true, Commentf("%T", err))

	c.Assert(s.WebS.CheckPassword("user1", pass, token1), IsNil)

	err = s.WebS.CheckPassword("user1", pass, token1)
	c.Assert(trace.IsAccessDenied(err), Equals, true, Commentf("%T", err))

	token2 := otp.OTP()
	err = s.WebS.CheckPassword("user1", []byte("abc123123"), token2)
	c.Assert(trace.IsAccessDenied(err), Equals, true, Commentf("%T", err))

	err = s.WebS.CheckPassword("user1", pass, "123456")
	c.Assert(trace.IsAccessDenied(err), Equals, true, Commentf("%T", err))

	c.Assert(s.WebS.CheckPassword("user1", pass, token2), IsNil)
	err = s.WebS.CheckPassword("user1", pass, token1)
	c.Assert(trace.IsAccessDenied(err), Equals, true, Commentf("%T", err))

	_ = otp.OTP()
	_ = otp.OTP()
	_ = otp.OTP()
	token6 := otp.OTP()
	token7 := otp.OTP()

	err = s.WebS.CheckPassword("user1", pass, token7)
	c.Assert(trace.IsAccessDenied(err), Equals, true, Commentf("%T", err))

	c.Assert(s.WebS.CheckPassword("user1", pass, token6), IsNil)

	err = s.WebS.CheckPassword("user1", pass, "123456")
	c.Assert(trace.IsAccessDenied(err), Equals, true, Commentf("%T", err))

	c.Assert(s.WebS.CheckPassword("user1", pass, token7), IsNil)

	_ = otp.OTP()
	token9 := otp.OTP()
	c.Assert(s.WebS.CheckPassword("user1", pass, token9), IsNil)

}

func (s *ServicesTestSuite) PasswordGarbage(c *C) {
	garbage := [][]byte{
		nil,
		make([]byte, defaults.MaxPasswordLength+1),
		make([]byte, defaults.MinPasswordLength-1),
	}
	for _, g := range garbage {
		err := s.WebS.CheckPassword("user1", g, "123456")
		c.Assert(err, NotNil)
	}
}
