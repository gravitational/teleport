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
	"fmt"
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
var _ = fmt.Printf

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

// TestBuildPrincipals makes sure that the list of principals for a host
// certificate is correctly built.
//   * If the node has role admin, then only the host ID should be listed
//     in the principals field.
//   * If only a host ID is provided, don't include a empty node name
//     this is for backward compatibility.
//   * If both host ID and node name are given, then both should be included
//     on the certificate.
//   * If the host ID and node name are the same, only list one.
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
			[]string{"22222222-2222-2222-2222-222222222222.example.com", "proxy.example.com", "proxy"},
		},
		// 3 - deduplicate principals
		{
			"33333333-3333-3333-3333-333333333333",
			"33333333-3333-3333-3333-333333333333",
			"example.com",
			teleport.Roles{teleport.RoleProxy},
			[]string{"33333333-3333-3333-3333-333333333333.example.com", "33333333-3333-3333-3333-333333333333"},
		},
	}

	// run tests
	for _, tt := range tests {
		hostCertificateBytes, err := s.suite.A.GenerateHostCert(
			services.HostCertParams{
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

		c.Assert(hostCertificate.ValidPrincipals, DeepEquals, tt.outValidPrincipals)
	}
}

// TestUserCertCompatibility makes sure the compatibility flag can be used to
// add to remove roles from certificate extensions.
func (s *NativeSuite) TestUserCertCompatibility(c *C) {
	priv, pub, err := s.suite.A.GenerateKeyPair("")
	c.Assert(err, IsNil)

	tests := []struct {
		inCompatibility string
		outHasRoles     bool
	}{
		// 0 - no compatibility, has roles
		{
			"",
			true,
		},
		// 1 - no compatibility, has roles
		{
			"invalid",
			true,
		},
		// 2 - compatibility, has roles
		{
			"oldssh",
			false,
		},
	}

	// run tests
	for i, tt := range tests {
		comment := Commentf("Test %v", i)

		userCertificateBytes, err := s.suite.A.GenerateUserCert(services.UserCertParams{
			PrivateCASigningKey:   priv,
			PublicUserKey:         pub,
			Username:              "user",
			AllowedLogins:         []string{"centos", "root"},
			TTL:                   time.Hour,
			Roles:                 []string{"foo"},
			Compatibility:         tt.inCompatibility,
			PermitAgentForwarding: true,
		})
		c.Assert(err, IsNil, comment)

		publicKey, _, _, _, err := ssh.ParseAuthorizedKey(userCertificateBytes)
		c.Assert(err, IsNil, comment)

		userCertificate, ok := publicKey.(*ssh.Certificate)
		c.Assert(ok, Equals, true, comment)

		// check if we added the roles extension
		_, ok = userCertificate.Extensions[teleport.CertExtensionTeleportRoles]
		c.Assert(ok, Equals, tt.outHasRoles, comment)
	}
}
