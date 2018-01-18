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
// package test contains CA authority acceptance test suite
package test

import (
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshca"

	"golang.org/x/crypto/ssh"
	. "gopkg.in/check.v1"
)

func TestAuth(t *testing.T) { TestingT(t) }

type AuthSuite struct {
	A sshca.Authority
}

func (s *AuthSuite) GenerateKeypairEmptyPass(c *C) {
	priv, pub, err := s.A.GenerateKeyPair("")
	c.Assert(err, IsNil)

	// make sure we can parse the private and public key
	_, err = ssh.ParsePrivateKey(priv)
	c.Assert(err, IsNil)

	_, _, _, _, err = ssh.ParseAuthorizedKey(pub)
	c.Assert(err, IsNil)
}

func (s *AuthSuite) GenerateKeypairPass(c *C) {
	_, pub, err := s.A.GenerateKeyPair("pass1")
	c.Assert(err, IsNil)

	// make sure we can parse the private and public key
	// TODO(klizhentas) test the private key actually
	_, _, _, _, err = ssh.ParseAuthorizedKey(pub)
	c.Assert(err, IsNil)
}

func (s *AuthSuite) GenerateHostCert(c *C) {
	priv, pub, err := s.A.GenerateKeyPair("")
	c.Assert(err, IsNil)

	cert, err := s.A.GenerateHostCert(
		services.HostCertParams{
			PrivateCASigningKey: priv,
			PublicHostKey:       pub,
			HostID:              "00000000-0000-0000-0000-000000000000",
			NodeName:            "auth.example.com",
			ClusterName:         "example.com",
			Roles:               teleport.Roles{teleport.RoleAdmin},
			TTL:                 time.Hour,
		})
	c.Assert(err, IsNil)

	_, _, _, _, err = ssh.ParseAuthorizedKey(cert)
	c.Assert(err, IsNil)
}

func (s *AuthSuite) GenerateUserCert(c *C) {
	priv, pub, err := s.A.GenerateKeyPair("")
	c.Assert(err, IsNil)

	cert, err := s.A.GenerateUserCert(services.UserCertParams{
		PrivateCASigningKey: priv,
		PublicUserKey:       pub,
		Username:            "user",
		AllowedLogins:       []string{"centos", "root"},
		TTL:                 time.Hour,
		PermitAgentForwarding: true,
		PermitPortForwarding:  true,
		CertificateFormat:     teleport.CertificateFormatStandard,
	})
	c.Assert(err, IsNil)

	_, _, _, _, err = ssh.ParseAuthorizedKey(cert)
	c.Assert(err, IsNil)

	_, err = s.A.GenerateUserCert(services.UserCertParams{
		PrivateCASigningKey: priv,
		PublicUserKey:       pub,
		Username:            "user",
		AllowedLogins:       []string{"root"},
		TTL:                 -20,
		PermitAgentForwarding: true,
		PermitPortForwarding:  true,
		CertificateFormat:     teleport.CertificateFormatStandard,
	})
	c.Assert(err, NotNil)

	_, err = s.A.GenerateUserCert(services.UserCertParams{
		PrivateCASigningKey: priv,
		PublicUserKey:       pub,
		Username:            "user",
		AllowedLogins:       []string{"root"},
		TTL:                 0,
		PermitAgentForwarding: true,
		PermitPortForwarding:  true,
		CertificateFormat:     teleport.CertificateFormatStandard,
	})
	c.Assert(err, NotNil)

	_, err = s.A.GenerateUserCert(services.UserCertParams{
		PrivateCASigningKey: priv,
		PublicUserKey:       pub,
		Username:            "user",
		AllowedLogins:       []string{"root"},
		TTL:                 time.Hour,
		PermitAgentForwarding: true,
		PermitPortForwarding:  true,
		CertificateFormat:     teleport.CertificateFormatStandard,
	})
	c.Assert(err, IsNil)

	inRoles := []string{"role-1", "role-2"}
	cert, err = s.A.GenerateUserCert(services.UserCertParams{
		PrivateCASigningKey: priv,
		PublicUserKey:       pub,
		Username:            "user",
		AllowedLogins:       []string{"root"},
		TTL:                 time.Hour,
		PermitAgentForwarding: true,
		PermitPortForwarding:  true,
		CertificateFormat:     teleport.CertificateFormatStandard,
		Roles:                 inRoles,
	})
	c.Assert(err, IsNil)
	parsedKey, _, _, _, err := ssh.ParseAuthorizedKey(cert)
	c.Assert(err, IsNil)
	parsedCert, ok := parsedKey.(*ssh.Certificate)
	c.Assert(ok, Equals, true)
	outRoles, err := services.UnmarshalCertRoles(parsedCert.Extensions[teleport.CertExtensionTeleportRoles])
	c.Assert(err, IsNil)
	c.Assert(outRoles, DeepEquals, inRoles)
}
