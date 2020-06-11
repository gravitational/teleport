/*
Copyright 2017-2018 Gravitational, Inc.

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
	"context"
	"fmt"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth/test"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/jonboulle/clockwork"
	"gopkg.in/check.v1"
)

func TestNative(t *testing.T) { check.TestingT(t) }

type NativeSuite struct {
	suite *test.AuthSuite
}

var _ = check.Suite(&NativeSuite{})
var _ = fmt.Printf

func (s *NativeSuite) SetUpSuite(c *check.C) {
	utils.InitLoggerForTests()

	fakeClock := clockwork.NewFakeClockAt(time.Date(2016, 9, 8, 7, 6, 5, 0, time.UTC))

	a, err := New(
		context.TODO(),
		PrecomputeKeys(1),
		SetClock(fakeClock),
	)
	c.Assert(err, check.IsNil)

	s.suite = &test.AuthSuite{
		A:     a,
		Clock: fakeClock,
	}
}

func (s *NativeSuite) TestGenerateKeypairEmptyPass(c *check.C) {
	s.suite.GenerateKeypairEmptyPass(c)
}

func (s *NativeSuite) TestGenerateKeypairPass(c *check.C) {
	s.suite.GenerateKeypairPass(c)
}

func (s *NativeSuite) TestGenerateHostCert(c *check.C) {
	s.suite.GenerateHostCert(c)
}

func (s *NativeSuite) TestGenerateUserCert(c *check.C) {
	s.suite.GenerateUserCert(c)
}

// TestDisablePrecompute makes sure that keygen works
// when no keys are precomputed
func (s *NativeSuite) TestDisablePrecompute(c *check.C) {
	a, err := New(context.TODO(), PrecomputeKeys(0))
	c.Assert(err, check.IsNil)

	caPrivateKey, _, err := a.GenerateKeyPair("")
	c.Assert(err, check.IsNil)
	c.Assert(caPrivateKey, check.NotNil)
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
func (s *NativeSuite) TestBuildPrincipals(c *check.C) {
	caPrivateKey, _, err := s.suite.A.GenerateKeyPair("")
	c.Assert(err, check.IsNil)

	_, hostPublicKey, err := s.suite.A.GenerateKeyPair("")
	c.Assert(err, check.IsNil)

	tests := []struct {
		desc               string
		inHostID           string
		inNodeName         string
		inClusterName      string
		inRoles            teleport.Roles
		outValidPrincipals []string
	}{
		{
			desc:               "admin role",
			inHostID:           "00000000-0000-0000-0000-000000000000",
			inNodeName:         "auth",
			inClusterName:      "example.com",
			inRoles:            teleport.Roles{teleport.RoleAdmin},
			outValidPrincipals: []string{"00000000-0000-0000-0000-000000000000"},
		},
		{
			desc:          "backward compatibility",
			inHostID:      "11111111-1111-1111-1111-111111111111",
			inNodeName:    "",
			inClusterName: "example.com",
			inRoles:       teleport.Roles{teleport.RoleNode},
			outValidPrincipals: []string{
				"11111111-1111-1111-1111-111111111111.example.com",
				"11111111-1111-1111-1111-111111111111",
				string(teleport.PrincipalLocalhost),
				string(teleport.PrincipalLoopbackV4),
				string(teleport.PrincipalLoopbackV6),
			},
		},
		{
			desc:          "dual principals",
			inHostID:      "22222222-2222-2222-2222-222222222222",
			inNodeName:    "proxy",
			inClusterName: "example.com",
			inRoles:       teleport.Roles{teleport.RoleProxy},
			outValidPrincipals: []string{
				"22222222-2222-2222-2222-222222222222.example.com",
				"22222222-2222-2222-2222-222222222222",
				"proxy.example.com",
				"proxy",
				string(teleport.PrincipalLocalhost),
				string(teleport.PrincipalLoopbackV4),
				string(teleport.PrincipalLoopbackV6),
			},
		},
		{
			desc:          "deduplicate principals",
			inHostID:      "33333333-3333-3333-3333-333333333333",
			inNodeName:    "33333333-3333-3333-3333-333333333333",
			inClusterName: "example.com",
			inRoles:       teleport.Roles{teleport.RoleProxy},
			outValidPrincipals: []string{
				"33333333-3333-3333-3333-333333333333.example.com",
				"33333333-3333-3333-3333-333333333333",
				string(teleport.PrincipalLocalhost),
				string(teleport.PrincipalLoopbackV4),
				string(teleport.PrincipalLoopbackV6),
			},
		},
	}

	// run tests
	for _, tt := range tests {
		c.Logf("Running test case: %q", tt.desc)
		hostCertificateBytes, err := s.suite.A.GenerateHostCert(
			services.HostCertParams{
				PrivateCASigningKey: caPrivateKey,
				CASigningAlg:        defaults.CASignatureAlgorithm,
				PublicHostKey:       hostPublicKey,
				HostID:              tt.inHostID,
				NodeName:            tt.inNodeName,
				ClusterName:         tt.inClusterName,
				Roles:               tt.inRoles,
				TTL:                 time.Hour,
			})
		c.Assert(err, check.IsNil)

		publicKey, _, _, _, err := ssh.ParseAuthorizedKey(hostCertificateBytes)
		c.Assert(err, check.IsNil)

		hostCertificate, ok := publicKey.(*ssh.Certificate)
		c.Assert(ok, check.Equals, true)

		c.Assert(hostCertificate.ValidPrincipals, check.DeepEquals, tt.outValidPrincipals)
	}
}

// TestUserCertCompatibility makes sure the compatibility flag can be used to
// add to remove roles from certificate extensions.
func (s *NativeSuite) TestUserCertCompatibility(c *check.C) {
	priv, pub, err := s.suite.A.GenerateKeyPair("")
	c.Assert(err, check.IsNil)

	tests := []struct {
		inCompatibility string
		outHasRoles     bool
	}{
		// 0 - standard, has roles
		{
			teleport.CertificateFormatStandard,
			true,
		},
		// 1 - oldssh, no roles
		{
			teleport.CertificateFormatOldSSH,
			false,
		},
	}

	// run tests
	for i, tt := range tests {
		comment := check.Commentf("Test %v", i)

		userCertificateBytes, err := s.suite.A.GenerateUserCert(services.UserCertParams{
			PrivateCASigningKey:   priv,
			CASigningAlg:          defaults.CASignatureAlgorithm,
			PublicUserKey:         pub,
			Username:              "user",
			AllowedLogins:         []string{"centos", "root"},
			TTL:                   time.Hour,
			Roles:                 []string{"foo"},
			CertificateFormat:     tt.inCompatibility,
			PermitAgentForwarding: true,
			PermitPortForwarding:  true,
		})
		c.Assert(err, check.IsNil, comment)

		publicKey, _, _, _, err := ssh.ParseAuthorizedKey(userCertificateBytes)
		c.Assert(err, check.IsNil, comment)

		userCertificate, ok := publicKey.(*ssh.Certificate)
		c.Assert(ok, check.Equals, true, comment)
		// Check that the signature algorithm is correct.
		c.Assert(userCertificate.Signature.Format, check.Equals, defaults.CASignatureAlgorithm)
		// check if we added the roles extension
		_, ok = userCertificate.Extensions[teleport.CertExtensionTeleportRoles]
		c.Assert(ok, check.Equals, tt.outHasRoles, comment)
	}
}
