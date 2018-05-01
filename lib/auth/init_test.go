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

package auth

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/boltbk"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/trace"

	. "gopkg.in/check.v1"
)

type AuthInitSuite struct {
	tempDir string
}

var _ = Suite(&AuthInitSuite{})
var _ = fmt.Printf

func (s *AuthInitSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
}

func (s *AuthInitSuite) TearDownSuite(c *C) {
}

func (s *AuthInitSuite) SetUpTest(c *C) {
	var err error
	s.tempDir, err = ioutil.TempDir("", "auth-init-test-")
	c.Assert(err, IsNil)
}

func (s *AuthInitSuite) TearDownTest(c *C) {
	var err error
	err = os.RemoveAll(s.tempDir)
	c.Assert(err, IsNil)
}

// TestReadIdentity makes parses identity from private key and certificate
// and checks that all parameters are valid
func (s *AuthInitSuite) TestReadIdentity(c *C) {
	t := testauthority.New()
	priv, pub, err := t.GenerateKeyPair("")
	c.Assert(err, IsNil)

	cert, err := t.GenerateHostCert(services.HostCertParams{
		PrivateCASigningKey: priv,
		PublicHostKey:       pub,
		HostID:              "id1",
		NodeName:            "node-name",
		ClusterName:         "example.com",
		Roles:               teleport.Roles{teleport.RoleNode},
		TTL:                 0,
	})
	c.Assert(err, IsNil)

	id, err := ReadSSHIdentityFromKeyPair(priv, cert)
	c.Assert(err, IsNil)
	c.Assert(id.ClusterName, Equals, "example.com")
	c.Assert(id.ID, DeepEquals, IdentityID{HostUUID: "id1.example.com", Role: teleport.RoleNode})
	c.Assert(id.CertBytes, DeepEquals, cert)
	c.Assert(id.KeyBytes, DeepEquals, priv)

	// test TTL by converting the generated cert to text -> back and making sure ExpireAfter is valid
	ttl := time.Second * 10
	expiryDate := time.Now().Add(ttl)
	bytes, err := t.GenerateHostCert(services.HostCertParams{
		PrivateCASigningKey: priv,
		PublicHostKey:       pub,
		HostID:              "id1",
		NodeName:            "node-name",
		ClusterName:         "example.com",
		Roles:               teleport.Roles{teleport.RoleNode},
		TTL:                 ttl,
	})
	c.Assert(err, IsNil)
	pk, _, _, _, err := ssh.ParseAuthorizedKey(bytes)
	c.Assert(err, IsNil)
	copy, ok := pk.(*ssh.Certificate)
	c.Assert(ok, Equals, true)
	c.Assert(uint64(expiryDate.Unix()), Equals, copy.ValidBefore)
}

func (s *AuthInitSuite) TestBadIdentity(c *C) {
	t := testauthority.New()
	priv, pub, err := t.GenerateKeyPair("")
	c.Assert(err, IsNil)

	// bad cert type
	_, err = ReadSSHIdentityFromKeyPair(priv, pub)
	c.Assert(trace.IsBadParameter(err), Equals, true, Commentf("%#v", err))

	// missing authority domain
	cert, err := t.GenerateHostCert(services.HostCertParams{
		PrivateCASigningKey: priv,
		PublicHostKey:       pub,
		HostID:              "id2",
		NodeName:            "",
		ClusterName:         "",
		Roles:               teleport.Roles{teleport.RoleNode},
		TTL:                 0,
	})
	c.Assert(err, IsNil)

	_, err = ReadSSHIdentityFromKeyPair(priv, cert)
	c.Assert(trace.IsBadParameter(err), Equals, true, Commentf("%#v", err))

	// missing host uuid
	cert, err = t.GenerateHostCert(services.HostCertParams{
		PrivateCASigningKey: priv,
		PublicHostKey:       pub,
		HostID:              "example.com",
		NodeName:            "",
		ClusterName:         "",
		Roles:               teleport.Roles{teleport.RoleNode},
		TTL:                 0,
	})
	c.Assert(err, IsNil)

	_, err = ReadSSHIdentityFromKeyPair(priv, cert)
	c.Assert(trace.IsBadParameter(err), Equals, true, Commentf("%#v", err))

	// unrecognized role
	cert, err = t.GenerateHostCert(services.HostCertParams{
		PrivateCASigningKey: priv,
		PublicHostKey:       pub,
		HostID:              "example.com",
		NodeName:            "",
		ClusterName:         "id1",
		Roles:               teleport.Roles{teleport.Role("bad role")},
		TTL:                 0,
	})
	c.Assert(err, IsNil)

	_, err = ReadSSHIdentityFromKeyPair(priv, cert)
	c.Assert(trace.IsBadParameter(err), Equals, true, Commentf("%#v", err))
}

// TestAuthPreference ensures that the act of creating an AuthServer sets
// the AuthPreference (type and second factor) on the backend.
func (s *AuthInitSuite) TestAuthPreference(c *C) {
	bk, err := boltbk.New(backend.Params{"path": s.tempDir})
	c.Assert(err, IsNil)

	ap, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         "local",
		SecondFactor: "u2f",
		U2F: &services.U2F{
			AppID:  "foo",
			Facets: []string{"bar", "baz"},
		},
	})
	c.Assert(err, IsNil)

	clusterName, err := services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	c.Assert(err, IsNil)
	staticTokens, err := services.NewStaticTokens(services.StaticTokensSpecV2{
		StaticTokens: []services.ProvisionToken{},
	})
	c.Assert(err, IsNil)

	ac := InitConfig{
		DataDir:        s.tempDir,
		HostUUID:       "00000000-0000-0000-0000-000000000000",
		NodeName:       "foo",
		Backend:        bk,
		Authority:      testauthority.New(),
		ClusterConfig:  services.DefaultClusterConfig(),
		ClusterName:    clusterName,
		StaticTokens:   staticTokens,
		AuthPreference: ap,
	}
	as, err := Init(ac)
	c.Assert(err, IsNil)

	cap, err := as.GetAuthPreference()
	c.Assert(err, IsNil)
	c.Assert(cap.GetType(), Equals, "local")
	c.Assert(cap.GetSecondFactor(), Equals, "u2f")
	u, err := cap.GetU2F()
	c.Assert(err, IsNil)
	c.Assert(u.AppID, Equals, "foo")
	c.Assert(u.Facets, DeepEquals, []string{"bar", "baz"})
}

func (s *AuthInitSuite) TestClusterID(c *C) {
	bk, err := boltbk.New(backend.Params{"path": c.MkDir()})
	c.Assert(err, IsNil)

	clusterName, err := services.NewClusterName(services.ClusterNameSpecV2{
		ClusterName: "me.localhost",
	})
	c.Assert(err, IsNil)

	authServer, err := Init(InitConfig{
		DataDir:       c.MkDir(),
		HostUUID:      "00000000-0000-0000-0000-000000000000",
		NodeName:      "foo",
		Backend:       bk,
		Authority:     testauthority.New(),
		ClusterConfig: services.DefaultClusterConfig(),
		ClusterName:   clusterName,
	})
	c.Assert(err, IsNil)

	cc, err := authServer.GetClusterConfig()
	c.Assert(err, IsNil)
	clusterID := cc.GetClusterID()
	c.Assert(clusterID, Not(Equals), "")

	// do it again and make sure cluster ID hasn't changed
	authServer, err = Init(InitConfig{
		DataDir:       c.MkDir(),
		HostUUID:      "00000000-0000-0000-0000-000000000000",
		NodeName:      "foo",
		Backend:       bk,
		Authority:     testauthority.New(),
		ClusterConfig: services.DefaultClusterConfig(),
		ClusterName:   clusterName,
	})
	c.Assert(err, IsNil)

	cc, err = authServer.GetClusterConfig()
	c.Assert(err, IsNil)
	c.Assert(cc.GetClusterID(), Equals, clusterID)
}
