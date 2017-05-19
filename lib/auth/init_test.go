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

	id, err := ReadIdentityFromKeyPair(priv, cert)
	c.Assert(err, IsNil)
	c.Assert(id.AuthorityDomain, Equals, "example.com")
	c.Assert(id.ID, DeepEquals, IdentityID{HostUUID: "id1", Role: teleport.RoleNode})
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
	_, err = ReadIdentityFromKeyPair(priv, pub)
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

	_, err = ReadIdentityFromKeyPair(priv, cert)
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

	_, err = ReadIdentityFromKeyPair(priv, cert)
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

	_, err = ReadIdentityFromKeyPair(priv, cert)
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
	})
	c.Assert(err, IsNil)

	ac := InitConfig{
		DataDir:        s.tempDir,
		HostUUID:       "00000000-0000-0000-0000-000000000000",
		NodeName:       "foo",
		Backend:        bk,
		Authority:      testauthority.New(),
		DomainName:     "me.localhost",
		AuthPreference: ap,
	}
	as, _, err := Init(ac, false)
	c.Assert(err, IsNil)

	cap, err := as.GetClusterAuthPreference()
	c.Assert(err, IsNil)
	c.Assert(cap.GetType(), Equals, "local")
	c.Assert(cap.GetSecondFactor(), Equals, "u2f")
}

// TestDynamicConfigTrue makes sure that if dynamicConfig is true, changing the
// configuration does not result in the changes being uploaded to the backend.
func (s *AuthInitSuite) TestDynamicConfigTrue(c *C) {
	bk, err := boltbk.New(backend.Params{"path": s.tempDir})
	c.Assert(err, IsNil)

	ap, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         "local",
		SecondFactor: "u2f",
	})
	c.Assert(err, IsNil)

	ac := InitConfig{
		DataDir:        s.tempDir,
		HostUUID:       "00000000-0000-0000-0000-000000000000",
		NodeName:       "foo",
		Backend:        bk,
		Authority:      testauthority.New(),
		DomainName:     "me.localhost",
		AuthPreference: ap,
	}
	as, _, err := Init(ac, true)
	c.Assert(err, IsNil)

	// the first time resources should be uploaded and we should see
	// local/u2f as the authentication preferences
	cap, err := as.GetClusterAuthPreference()
	c.Assert(err, IsNil)
	c.Assert(cap.GetType(), Equals, "local")
	c.Assert(cap.GetSecondFactor(), Equals, "u2f")

	// create new authentication preferences, even though we define
	// these, because we are in dynamic config mode they should not
	// be uploaded to the backend
	nap, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         "local",
		SecondFactor: "otp",
	})
	c.Assert(err, IsNil)

	ac = InitConfig{
		DataDir:        s.tempDir,
		HostUUID:       "00000000-0000-0000-0000-000000000000",
		NodeName:       "foo",
		Backend:        bk,
		Authority:      testauthority.New(),
		DomainName:     "me.localhost",
		AuthPreference: nap,
	}
	as, _, err = Init(ac, true)
	c.Assert(err, IsNil)

	// check and make sure we still have original settings
	cap, err = as.GetClusterAuthPreference()
	c.Assert(err, IsNil)
	c.Assert(cap.GetType(), Equals, "local")
	c.Assert(cap.GetSecondFactor(), Equals, "u2f")
}

// TestDynamicConfigFalse makes sure that if dynamicConfig is false, changing the
// configuration results in the changes being uploaded to the backend.
func (s *AuthInitSuite) TestDynamicConfigFalse(c *C) {
	bk, err := boltbk.New(backend.Params{"path": s.tempDir})
	c.Assert(err, IsNil)

	// create the resource we will be checking, we start with
	// authentication preferences being local/u2f
	ap, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         "local",
		SecondFactor: "u2f",
	})
	c.Assert(err, IsNil)

	ac := InitConfig{
		DataDir:        s.tempDir,
		HostUUID:       "00000000-0000-0000-0000-000000000000",
		NodeName:       "foo",
		Backend:        bk,
		Authority:      testauthority.New(),
		DomainName:     "me.localhost",
		AuthPreference: ap,
	}
	as, _, err := Init(ac, false)
	c.Assert(err, IsNil)

	// check the backend for the authentication preferences.
	cap, err := as.GetClusterAuthPreference()
	c.Assert(err, IsNil)
	c.Assert(cap.GetType(), Equals, "local")
	c.Assert(cap.GetSecondFactor(), Equals, "u2f")

	// create new authentication preferences, since dynamic config
	// is off, we will upload these to the backend as well
	nap, err := services.NewAuthPreference(services.AuthPreferenceSpecV2{
		Type:         "local",
		SecondFactor: "otp",
	})
	c.Assert(err, IsNil)

	ac = InitConfig{
		DataDir:        s.tempDir,
		HostUUID:       "00000000-0000-0000-0000-000000000000",
		NodeName:       "foo",
		Backend:        bk,
		Authority:      testauthority.New(),
		DomainName:     "me.localhost",
		AuthPreference: nap,
	}
	as, _, err = Init(ac, false)
	c.Assert(err, IsNil)

	// check that the new settings were pushed to the backend
	cap, err = as.GetClusterAuthPreference()
	c.Assert(err, IsNil)
	c.Assert(cap.GetType(), Equals, "local")
	c.Assert(cap.GetSecondFactor(), Equals, "otp")
}
