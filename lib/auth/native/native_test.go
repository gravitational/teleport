/*
Copyright 2017 Gravitational, Inc.

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
package native

import (
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth/test"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	. "gopkg.in/check.v1"
)

func TestNative(t *testing.T) { TestingT(t) }

type NativeSuite struct {
	suite *test.AuthSuite
}

var _ = Suite(&NativeSuite{})

func (s *NativeSuite) SetUpSuite(c *C) {
	utils.InitLoggerForTests()
	PrecalculatedKeysNum = 1
	s.suite = &test.AuthSuite{A: New()}
}

func (s *NativeSuite) TestGenerateKeypairEmptyPass(c *C) {
	s.suite.GenerateKeypairEmptyPass(c)
}

func (s *NativeSuite) TestGenerateKeypairPass(c *C) {
	s.suite.GenerateKeypairPass(c)
}

func (s *NativeSuite) TestGenerateHostCert(c *C) {
	s.suite.GenerateHostCert(c)
}

func (s *NativeSuite) TestGenerateUserCert(c *C) {
	s.suite.GenerateUserCert(c)
}

func (s *NativeSuite) TestBuildPrincipals(c *C) {
	caPrivateKey, _, err := s.suite.A.GenerateKeyPair("")
	c.Assert(err, IsNil)

	_, hostPublicKey, err := s.suite.A.GenerateKeyPair("")
	c.Assert(err, IsNil)

	tests := []struct {
		inHostID           string
		inNodeName         string
		inClusterName      string
		inRoles            teleport.Roles
		outValidPrincipals []string
	}{
		// 0 - admin role
		{
			"00000000-0000-0000-0000-000000000000",
			"auth",
			"example.com",
			teleport.Roles{teleport.RoleAdmin},
			[]string{"00000000-0000-0000-0000-000000000000"},
		},
		// 1 - backward compatibility
		{
			"11111111-1111-1111-1111-111111111111",
			"",
			"example.com",
			teleport.Roles{teleport.RoleNode},
			[]string{"11111111-1111-1111-1111-111111111111.example.com"},
		},
		// 2 - dual principals
		{
			"22222222-2222-2222-2222-222222222222",
			"proxy",
			"example.com",
			teleport.Roles{teleport.RoleProxy},
			[]string{"22222222-2222-2222-2222-222222222222.example.com", "proxy.example.com"},
		},
	}

	// run tests
	for _, tt := range tests {
		hostCertificateBytes, err := s.suite.A.GenerateHostCert(
			services.CertParams{
				PrivateCASigningKey: caPrivateKey,
				PublicHostKey:       hostPublicKey,
				HostID:              tt.inHostID,
				NodeName:            tt.inNodeName,
				ClusterName:         tt.inClusterName,
				Roles:               tt.inRoles,
				TTL:                 time.Hour,
			})
		c.Assert(err, IsNil)

		publicKey, _, _, _, err := ssh.ParseAuthorizedKey(hostCertificateBytes)
		c.Assert(err, IsNil)

		hostCertificate, ok := publicKey.(*ssh.Certificate)
		c.Assert(ok, Equals, true)

		c.Assert(hostCertificate.ValidPrincipals, HasLen, len(tt.outValidPrincipals))
		c.Assert(hostCertificate.ValidPrincipals, DeepEquals, tt.outValidPrincipals)
	}
}
