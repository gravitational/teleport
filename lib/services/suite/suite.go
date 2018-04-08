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
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"sort"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/tstranex/u2f"
	"golang.org/x/crypto/ssh"

	. "gopkg.in/check.v1"
)

// NewTestCA returns new test authority with a test key as a public and
// signing key
func NewTestCA(caType services.CertAuthType, clusterName string, privateKeys ...[]byte) *services.CertAuthorityV2 {
	// privateKeys is to specify another RSA private key
	if len(privateKeys) == 0 {
		privateKeys = [][]byte{fixtures.PEMBytes["rsa"]}
	}
	keyBytes := privateKeys[0]
	rsaKey, err := ssh.ParseRawPrivateKey(keyBytes)
	if err != nil {
		panic(err)
	}

	signer, err := ssh.NewSignerFromKey(rsaKey)
	if err != nil {
		panic(err)
	}

	key, cert, err := tlsca.GenerateSelfSignedCAWithPrivateKey(rsaKey.(*rsa.PrivateKey), pkix.Name{
		CommonName:   clusterName,
		Organization: []string{clusterName},
	}, nil, defaults.CATTL)
	if err != nil {
		panic(err)
	}

	return &services.CertAuthorityV2{
		Kind:    services.KindCertAuthority,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      clusterName,
			Namespace: defaults.Namespace,
		},
		Spec: services.CertAuthoritySpecV2{
			Type:         caType,
			ClusterName:  clusterName,
			CheckingKeys: [][]byte{ssh.MarshalAuthorizedKey(signer.PublicKey())},
			SigningKeys:  [][]byte{keyBytes},
			TLSKeyPairs:  []services.TLSKeyPair{{Cert: cert, Key: key}},
		},
	}
}

type ServicesTestSuite struct {
	Access        services.Access
	CAS           services.Trust
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
			c.Fatalf("Timeout occurred waiting for events")
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

func newUser(name string, roles []string) services.User {
	return &services.UserV2{
		Kind:    services.KindUser,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      name,
			Namespace: defaults.Namespace,
		},
		Spec: services.UserSpecV2{
			Roles: roles,
		},
	}
}

func (s *ServicesTestSuite) UsersCRUD(c *C) {
	u, err := s.WebS.GetUsers()
	c.Assert(err, IsNil)
	c.Assert(len(u), Equals, 0)

	c.Assert(s.WebS.UpsertPasswordHash("user1", []byte("hash")), IsNil)
	c.Assert(s.WebS.UpsertPasswordHash("user2", []byte("hash2")), IsNil)

	u, err = s.WebS.GetUsers()
	c.Assert(err, IsNil)
	userSlicesEqual(c, u, []services.User{newUser("user1", nil), newUser("user2", nil)})

	out, err := s.WebS.GetUser("user1")
	usersEqual(c, out, u[0])

	user := newUser("user1", []string{"admin", "user"})
	c.Assert(s.WebS.UpsertUser(user), IsNil)

	out, err = s.WebS.GetUser("user1")
	c.Assert(err, IsNil)
	usersEqual(c, out, user)

	out, err = s.WebS.GetUser("user1")
	c.Assert(err, IsNil)
	usersEqual(c, out, user)

	c.Assert(s.WebS.DeleteUser("user1"), IsNil)

	u, err = s.WebS.GetUsers()
	c.Assert(err, IsNil)
	userSlicesEqual(c, u, []services.User{newUser("user2", nil)})

	err = s.WebS.DeleteUser("user1")
	fixtures.ExpectNotFound(c, err)

	// bad username
	err = s.WebS.UpsertUser(newUser("", nil))
	fixtures.ExpectBadParameter(c, err)
}

func (s *ServicesTestSuite) LoginAttempts(c *C) {
	user := newUser("user1", []string{"admin", "user"})
	c.Assert(s.WebS.UpsertUser(user), IsNil)

	attempts, err := s.WebS.GetUserLoginAttempts(user.GetName())
	c.Assert(err, IsNil)
	c.Assert(len(attempts), Equals, 0)

	clock := clockwork.NewFakeClock()
	attempt1 := services.LoginAttempt{Time: clock.Now().UTC(), Success: false}
	err = s.WebS.AddUserLoginAttempt(user.GetName(), attempt1, defaults.AttemptTTL)
	c.Assert(err, IsNil)

	attempt2 := services.LoginAttempt{Time: clock.Now().UTC(), Success: false}
	err = s.WebS.AddUserLoginAttempt(user.GetName(), attempt2, defaults.AttemptTTL)
	c.Assert(err, IsNil)

	attempts, err = s.WebS.GetUserLoginAttempts(user.GetName())
	c.Assert(err, IsNil)
	c.Assert(attempts, DeepEquals, []services.LoginAttempt{attempt1, attempt2})
	c.Assert(services.LastFailed(3, attempts), Equals, false)
	c.Assert(services.LastFailed(2, attempts), Equals, true)
}

func (s *ServicesTestSuite) CertAuthCRUD(c *C) {
	ca := NewTestCA(services.UserCA, "example.com")
	c.Assert(s.CAS.UpsertCertAuthority(ca), IsNil)

	out, err := s.CAS.GetCertAuthority(ca.GetID(), true)
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, ca)

	cas, err := s.CAS.GetCertAuthorities(services.UserCA, false)
	c.Assert(err, IsNil)
	ca2 := *ca
	ca2.Spec.SigningKeys = nil
	ca2.Spec.TLSKeyPairs = []services.TLSKeyPair{{Cert: ca2.Spec.TLSKeyPairs[0].Cert}}
	fixtures.DeepCompare(c, cas[0], &ca2)

	cas, err = s.CAS.GetCertAuthorities(services.UserCA, true)
	c.Assert(err, IsNil)
	fixtures.DeepCompare(c, cas[0], ca)

	err = s.CAS.DeleteCertAuthority(*ca.ID())
	c.Assert(err, IsNil)

	// test compare and swap
	ca = NewTestCA(services.UserCA, "example.com")
	c.Assert(s.CAS.CreateCertAuthority(ca), IsNil)

	clock := clockwork.NewFakeClock()
	newCA := *ca
	rotation := services.Rotation{
		State:       services.RotationStateInProgress,
		CurrentID:   "id1",
		GracePeriod: services.NewDuration(time.Hour),
		Started:     clock.Now(),
	}
	newCA.SetRotation(rotation)

	err = s.CAS.CompareAndSwapCertAuthority(&newCA, ca)

	out, err = s.CAS.GetCertAuthority(ca.GetID(), true)
	c.Assert(err, IsNil)
	fixtures.DeepCompare(c, &newCA, out)
}

func newServer(kind, name, addr, namespace string) *services.ServerV2 {
	return &services.ServerV2{
		Kind:    kind,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      name,
			Namespace: namespace,
		},
		Spec: services.ServerSpecV2{
			Addr:       addr,
			PublicAddr: addr,
		},
	}
}

func (s *ServicesTestSuite) ServerCRUD(c *C) {
	out, err := s.PresenceS.GetNodes(defaults.Namespace)
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	srv := newServer(services.KindNode, "srv1", "127.0.0.1:2022", defaults.Namespace)
	c.Assert(s.PresenceS.UpsertNode(srv), IsNil)

	out, err = s.PresenceS.GetNodes(srv.Metadata.Namespace)
	c.Assert(err, IsNil)
	fixtures.DeepCompare(c, out, []services.Server{srv})

	out, err = s.PresenceS.GetProxies()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	proxy := newServer(services.KindProxy, "proxy1", "127.0.0.1:2023", defaults.Namespace)
	c.Assert(s.PresenceS.UpsertProxy(proxy), IsNil)

	out, err = s.PresenceS.GetProxies()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []services.Server{proxy})

	out, err = s.PresenceS.GetAuthServers()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	auth := newServer(services.KindAuthServer, "auth1", "127.0.0.1:2025", defaults.Namespace)
	c.Assert(s.PresenceS.UpsertAuthServer(auth), IsNil)

	out, err = s.PresenceS.GetAuthServers()
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, []services.Server{auth})
}

func newReverseTunnel(clusterName string, dialAddrs []string) *services.ReverseTunnelV2 {
	return &services.ReverseTunnelV2{
		Kind:    services.KindReverseTunnel,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      clusterName,
			Namespace: defaults.Namespace,
		},
		Spec: services.ReverseTunnelSpecV2{
			ClusterName: clusterName,
			DialAddrs:   dialAddrs,
		},
	}
}

func (s *ServicesTestSuite) ReverseTunnelsCRUD(c *C) {
	out, err := s.PresenceS.GetReverseTunnels()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	tunnel := newReverseTunnel("example.com", []string{"example.com:2023"})
	c.Assert(s.PresenceS.UpsertReverseTunnel(tunnel), IsNil)

	out, err = s.PresenceS.GetReverseTunnels()
	c.Assert(err, IsNil)
	fixtures.DeepCompare(c, out, []services.ReverseTunnel{tunnel})

	err = s.PresenceS.DeleteReverseTunnel(tunnel.Spec.ClusterName)
	c.Assert(err, IsNil)

	out, err = s.PresenceS.GetReverseTunnels()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	err = s.PresenceS.UpsertReverseTunnel(newReverseTunnel("", []string{"127.0.0.1:1234"}))
	c.Assert(trace.IsBadParameter(err), Equals, true, Commentf("%#v", err))

	err = s.PresenceS.UpsertReverseTunnel(newReverseTunnel("example.com", []string{"bad address"}))
	c.Assert(trace.IsBadParameter(err), Equals, true, Commentf("%#v", err))

	err = s.PresenceS.UpsertReverseTunnel(newReverseTunnel("example.com", []string{}))
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
	ws := services.NewWebSession("sid1", services.WebSessionSpecV2{
		Pub:     []byte("pub123"),
		Priv:    []byte("priv123"),
		Expires: dt,
	})
	err = s.WebS.UpsertWebSession("user1", "sid1", ws)
	c.Assert(err, IsNil)

	out, err := s.WebS.GetWebSession("user1", "sid1")
	c.Assert(err, IsNil)
	c.Assert(out, DeepEquals, ws)

	ws1 := services.NewWebSession(
		"sid1", services.WebSessionSpecV2{Pub: []byte("pub321"), Priv: []byte("priv321"), Expires: dt})
	err = s.WebS.UpsertWebSession("user1", "sid1", ws1)
	c.Assert(err, IsNil)

	out2, err := s.WebS.GetWebSession("user1", "sid1")
	c.Assert(err, IsNil)
	c.Assert(out2, DeepEquals, ws1)

	c.Assert(s.WebS.DeleteWebSession("user1", "sid1"), IsNil)

	_, err = s.WebS.GetWebSession("user1", "sid1")
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))
}

func (s *ServicesTestSuite) TokenCRUD(c *C) {
	_, err := s.ProvisioningS.GetToken("token")
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))

	c.Assert(s.ProvisioningS.UpsertToken("token", teleport.Roles{teleport.RoleAuth, teleport.RoleNode}, 0), IsNil)

	token, err := s.ProvisioningS.GetToken("token")
	c.Assert(err, IsNil)
	c.Assert(token.Roles.Include(teleport.RoleAuth), Equals, true)
	c.Assert(token.Roles.Include(teleport.RoleNode), Equals, true)
	c.Assert(token.Roles.Include(teleport.RoleProxy), Equals, false)
	diff := time.Now().UTC().Add(defaults.ProvisioningTokenTTL).Second() - token.Expires.Second()
	if diff > 1 {
		c.Fatalf("expected diff to be within one second, got %v instead", diff)
	}

	c.Assert(s.ProvisioningS.DeleteToken("token"), IsNil)

	_, err = s.ProvisioningS.GetToken("token")
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%#v", err))
}

func (s *ServicesTestSuite) RolesCRUD(c *C) {
	out, err := s.Access.GetRoles()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	role := services.RoleV3{
		Kind:    services.KindRole,
		Version: services.V3,
		Metadata: services.Metadata{
			Name:      "role1",
			Namespace: defaults.Namespace,
		},
		Spec: services.RoleSpecV3{
			Options: services.RoleOptions{
				services.MaxSessionTTL: services.Duration{Duration: time.Hour},
			},
			Allow: services.RoleConditions{
				Logins:     []string{"root", "bob"},
				NodeLabels: map[string]string{services.Wildcard: services.Wildcard},
				Namespaces: []string{defaults.Namespace},
				Rules: []services.Rule{
					services.NewRule(services.KindRole, services.RO()),
				},
			},
			Deny: services.RoleConditions{
				Namespaces: []string{defaults.Namespace},
			},
		},
	}
	err = s.Access.UpsertRole(&role, backend.Forever)
	c.Assert(err, IsNil)
	rout, err := s.Access.GetRole(role.Metadata.Name)
	c.Assert(err, IsNil)
	c.Assert(rout, DeepEquals, &role)

	role.Spec.Allow.Logins = []string{"bob"}
	err = s.Access.UpsertRole(&role, backend.Forever)
	c.Assert(err, IsNil)
	rout, err = s.Access.GetRole(role.Metadata.Name)
	c.Assert(err, IsNil)
	c.Assert(rout, DeepEquals, &role)

	err = s.Access.DeleteRole(role.Metadata.Name)
	c.Assert(err, IsNil)

	_, err = s.Access.GetRole(role.Metadata.Name)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%T", err))
}

func (s *ServicesTestSuite) NamespacesCRUD(c *C) {
	out, err := s.PresenceS.GetNamespaces()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	ns := services.Namespace{
		Kind:    services.KindNamespace,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      defaults.Namespace,
			Namespace: defaults.Namespace,
		},
	}
	err = s.PresenceS.UpsertNamespace(ns)
	c.Assert(err, IsNil)
	nsout, err := s.PresenceS.GetNamespace(ns.Metadata.Name)
	c.Assert(err, IsNil)
	c.Assert(nsout, DeepEquals, &ns)

	err = s.PresenceS.DeleteNamespace(ns.Metadata.Name)
	c.Assert(err, IsNil)

	_, err = s.PresenceS.GetNamespace(ns.Metadata.Name)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("%T", err))
}

func (s *ServicesTestSuite) U2FCRUD(c *C) {
	token := "tok1"
	appId := "https://localhost"
	user1 := "user1"

	challenge, err := u2f.NewChallenge(appId, []string{appId})
	c.Assert(err, IsNil)

	err = s.WebS.UpsertU2FRegisterChallenge(token, challenge)
	c.Assert(err, IsNil)

	challengeOut, err := s.WebS.GetU2FRegisterChallenge(token)
	c.Assert(err, IsNil)
	c.Assert(challenge.Challenge, DeepEquals, challengeOut.Challenge)
	c.Assert(challenge.Timestamp.Unix(), Equals, challengeOut.Timestamp.Unix())
	c.Assert(challenge.AppID, Equals, challengeOut.AppID)
	c.Assert(challenge.TrustedFacets, DeepEquals, challengeOut.TrustedFacets)

	err = s.WebS.UpsertU2FSignChallenge(user1, challenge)
	c.Assert(err, IsNil)

	challengeOut, err = s.WebS.GetU2FSignChallenge(user1)
	c.Assert(err, IsNil)
	c.Assert(challenge.Challenge, DeepEquals, challengeOut.Challenge)
	c.Assert(challenge.Timestamp.Unix(), Equals, challengeOut.Timestamp.Unix())
	c.Assert(challenge.AppID, Equals, challengeOut.AppID)
	c.Assert(challenge.TrustedFacets, DeepEquals, challengeOut.TrustedFacets)

	derKey, err := base64.StdEncoding.DecodeString("MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEGOi54Eun0r3Xrj8PjyOGYzJObENYI/t/Lr9g9PsHTHnp1qI2ysIhsdMPd7x/vpsL6cr+2EPVik7921OSsVjEMw==")
	c.Assert(err, IsNil)
	pubkeyInterface, err := x509.ParsePKIXPublicKey(derKey)
	c.Assert(err, IsNil)

	pubkey, ok := pubkeyInterface.(*ecdsa.PublicKey)
	c.Assert(ok, Equals, true)

	registration := u2f.Registration{
		Raw:       []byte("BQQY6LngS6fSvdeuPw+PI4ZjMk5sQ1gj+38uv2D0+wdMeenWojbKwiGx0w93vH++mwvpyv7YQ9WKTv3bU5KxWMQzQIJ+PVFsYjEa0Xgnx+siQaxdlku+U+J2W55U5NrN1iGIc0Amh+0HwhbV2W90G79cxIYS2SVIFAdqTTDXvPXJbeAwggE8MIHkoAMCAQICChWIR0AwlYJZQHcwCgYIKoZIzj0EAwIwFzEVMBMGA1UEAxMMRlQgRklETyAwMTAwMB4XDTE0MDgxNDE4MjkzMloXDTI0MDgxNDE4MjkzMlowMTEvMC0GA1UEAxMmUGlsb3RHbnViYnktMC40LjEtMTU4ODQ3NDAzMDk1ODI1OTQwNzcwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAQY6LngS6fSvdeuPw+PI4ZjMk5sQ1gj+38uv2D0+wdMeenWojbKwiGx0w93vH++mwvpyv7YQ9WKTv3bU5KxWMQzMAoGCCqGSM49BAMCA0cAMEQCIIbmYKu6I2L4pgZCBms9NIo9yo5EO9f2irp0ahvLlZudAiC8RN/N+WHAFdq8Z+CBBOMsRBFDDJy3l5EDR83B5GAfrjBEAiBl6R6gAmlbudVpW2jSn3gfjmA8EcWq0JsGZX9oFM/RJwIgb9b01avBY5jBeVIqw5KzClLzbRDMY4K+Ds6uprHyA1Y="),
		KeyHandle: []byte("gn49UWxiMRrReCfH6yJBrF2WS75T4nZbnlTk2s3WIYhzQCaH7QfCFtXZb3Qbv1zEhhLZJUgUB2pNMNe89clt4A=="),
		PubKey:    *pubkey,
	}
	err = s.WebS.UpsertU2FRegistration(user1, &registration)
	c.Assert(err, IsNil)

	registrationOut, err := s.WebS.GetU2FRegistration(user1)
	c.Assert(err, IsNil)
	c.Assert(&registration, DeepEquals, registrationOut)
}

func (s *ServicesTestSuite) SAMLCRUD(c *C) {
	connector := &services.SAMLConnectorV2{
		Kind:    services.KindSAML,
		Version: services.V2,
		Metadata: services.Metadata{
			Name:      "saml1",
			Namespace: defaults.Namespace,
		},
		Spec: services.SAMLConnectorSpecV2{
			Issuer: "http://example.com",
			SSO:    "https://example.com/saml/sso",
			AssertionConsumerService: "https://localhost/acs",
			Audience:                 "https://localhost/aud",
			ServiceProviderIssuer:    "https://localhost/iss",
			AttributesToRoles: []services.AttributeMapping{
				{Name: "groups", Value: "admin", Roles: []string{"admin"}},
			},
			Cert: fixtures.SigningCertPEM,
			SigningKeyPair: &services.SigningKeyPair{
				PrivateKey: fixtures.SigningKeyPEM,
				Cert:       fixtures.SigningCertPEM,
			},
		},
	}
	err := connector.CheckAndSetDefaults()
	c.Assert(err, IsNil)
	err = s.WebS.UpsertSAMLConnector(connector)
	c.Assert(err, IsNil)
	out, err := s.WebS.GetSAMLConnector(connector.GetName(), true)
	c.Assert(err, IsNil)
	fixtures.DeepCompare(c, out, connector)

	connectors, err := s.WebS.GetSAMLConnectors(true)
	c.Assert(err, IsNil)
	fixtures.DeepCompare(c, []services.SAMLConnector{connector}, connectors)

	out2, err := s.WebS.GetSAMLConnector(connector.GetName(), false)
	c.Assert(err, IsNil)
	connectorNoSecrets := *connector
	connectorNoSecrets.Spec.SigningKeyPair.PrivateKey = ""
	fixtures.DeepCompare(c, out2, &connectorNoSecrets)

	connectorsNoSecrets, err := s.WebS.GetSAMLConnectors(false)
	c.Assert(err, IsNil)
	fixtures.DeepCompare(c, []services.SAMLConnector{&connectorNoSecrets}, connectorsNoSecrets)

	err = s.WebS.DeleteSAMLConnector(connector.GetName())
	c.Assert(err, IsNil)

	err = s.WebS.DeleteSAMLConnector(connector.GetName())
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("expected not found, got %T", err))

	_, err = s.WebS.GetSAMLConnector(connector.GetName(), true)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("expected not found, got %T", err))
}

func (s *ServicesTestSuite) TunnelConnectionsCRUD(c *C) {
	clusterName := "example.com"
	out, err := s.PresenceS.GetTunnelConnections(clusterName)
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	dt := time.Date(2015, 6, 5, 4, 3, 2, 1, time.UTC).UTC()
	conn, err := services.NewTunnelConnection("conn1", services.TunnelConnectionSpecV2{
		ClusterName:   clusterName,
		ProxyName:     "p1",
		LastHeartbeat: dt,
	})
	c.Assert(err, IsNil)

	err = s.PresenceS.UpsertTunnelConnection(conn)
	c.Assert(err, IsNil)

	out, err = s.PresenceS.GetTunnelConnections(clusterName)
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 1)
	fixtures.DeepCompare(c, out[0], conn)

	out, err = s.PresenceS.GetAllTunnelConnections()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 1)
	fixtures.DeepCompare(c, out[0], conn)

	dt = dt.Add(time.Hour)
	conn.SetLastHeartbeat(dt)

	err = s.PresenceS.UpsertTunnelConnection(conn)
	c.Assert(err, IsNil)

	out, err = s.PresenceS.GetTunnelConnections(clusterName)
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 1)
	fixtures.DeepCompare(c, out[0], conn)

	err = s.PresenceS.DeleteAllTunnelConnections()
	c.Assert(err, IsNil)

	out, err = s.PresenceS.GetTunnelConnections(clusterName)
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	err = s.PresenceS.DeleteAllTunnelConnections()
	c.Assert(err, IsNil)

	// test delete individual connection
	err = s.PresenceS.UpsertTunnelConnection(conn)
	c.Assert(err, IsNil)

	out, err = s.PresenceS.GetTunnelConnections(clusterName)
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 1)
	fixtures.DeepCompare(c, out[0], conn)

	err = s.PresenceS.DeleteTunnelConnection(clusterName, conn.GetName())
	c.Assert(err, IsNil)

	out, err = s.PresenceS.GetTunnelConnections(clusterName)
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)
}

func (s *ServicesTestSuite) GithubConnectorCRUD(c *C) {
	connector := &services.GithubConnectorV3{
		Kind:    services.KindGithubConnector,
		Version: services.V3,
		Metadata: services.Metadata{
			Name:      "github",
			Namespace: defaults.Namespace,
		},
		Spec: services.GithubConnectorSpecV3{
			ClientID:     "aaa",
			ClientSecret: "bbb",
			RedirectURL:  "https://localhost:3080/v1/webapi/github/callback",
			Display:      "Github",
			TeamsToLogins: []services.TeamMapping{
				{
					Organization: "gravitational",
					Team:         "admins",
					Logins:       []string{"admin"},
				},
			},
		},
	}
	err := connector.CheckAndSetDefaults()
	c.Assert(err, IsNil)
	err = s.WebS.UpsertGithubConnector(connector)
	c.Assert(err, IsNil)
	out, err := s.WebS.GetGithubConnector(connector.GetName(), true)
	c.Assert(err, IsNil)
	fixtures.DeepCompare(c, out, connector)

	connectors, err := s.WebS.GetGithubConnectors(true)
	c.Assert(err, IsNil)
	fixtures.DeepCompare(c, []services.GithubConnector{connector}, connectors)

	out2, err := s.WebS.GetGithubConnector(connector.GetName(), false)
	c.Assert(err, IsNil)
	connectorNoSecrets := *connector
	connectorNoSecrets.Spec.ClientSecret = ""
	fixtures.DeepCompare(c, out2, &connectorNoSecrets)

	connectorsNoSecrets, err := s.WebS.GetGithubConnectors(false)
	c.Assert(err, IsNil)
	fixtures.DeepCompare(c, []services.GithubConnector{&connectorNoSecrets}, connectorsNoSecrets)

	err = s.WebS.DeleteGithubConnector(connector.GetName())
	c.Assert(err, IsNil)

	err = s.WebS.DeleteGithubConnector(connector.GetName())
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("expected not found, got %T", err))

	_, err = s.WebS.GetGithubConnector(connector.GetName(), true)
	c.Assert(trace.IsNotFound(err), Equals, true, Commentf("expected not found, got %T", err))
}

func (s *ServicesTestSuite) RemoteClustersCRUD(c *C) {
	clusterName := "example.com"
	out, err := s.PresenceS.GetRemoteClusters()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	rc, err := services.NewRemoteCluster(clusterName)
	c.Assert(err, IsNil)

	rc.SetConnectionStatus(teleport.RemoteClusterStatusOffline)

	err = s.PresenceS.CreateRemoteCluster(rc)
	c.Assert(err, IsNil)

	err = s.PresenceS.CreateRemoteCluster(rc)
	fixtures.ExpectAlreadyExists(c, err)

	out, err = s.PresenceS.GetRemoteClusters()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 1)
	fixtures.DeepCompare(c, out[0], rc)

	err = s.PresenceS.DeleteAllRemoteClusters()
	c.Assert(err, IsNil)

	out, err = s.PresenceS.GetRemoteClusters()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 0)

	// test delete individual connection
	err = s.PresenceS.CreateRemoteCluster(rc)
	c.Assert(err, IsNil)

	out, err = s.PresenceS.GetRemoteClusters()
	c.Assert(err, IsNil)
	c.Assert(len(out), Equals, 1)
	fixtures.DeepCompare(c, out[0], rc)

	err = s.PresenceS.DeleteRemoteCluster(clusterName)
	c.Assert(err, IsNil)

	err = s.PresenceS.DeleteRemoteCluster(clusterName)
	fixtures.ExpectNotFound(c, err)
}
