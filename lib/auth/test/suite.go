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

// package test contains CA authority acceptance test suite.
package test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshca"
)

type AuthSuite struct {
	A      sshca.Authority
	Keygen func() ([]byte, []byte, error)
	Clock  clockwork.Clock
}

func (s *AuthSuite) GenerateKeypairEmptyPass(t *testing.T) {
	priv, pub, err := s.Keygen()
	require.NoError(t, err)

	// make sure we can parse the private and public key
	_, err = ssh.ParsePrivateKey(priv)
	require.NoError(t, err)

	_, _, _, _, err = ssh.ParseAuthorizedKey(pub)
	require.NoError(t, err)
}

func (s *AuthSuite) GenerateHostCert(t *testing.T) {
	priv, pub, err := s.Keygen()
	require.NoError(t, err)

	caSigner, err := ssh.ParsePrivateKey(priv)
	require.NoError(t, err)

	cert, err := s.A.GenerateHostCert(
		services.HostCertParams{
			CASigner:      caSigner,
			PublicHostKey: pub,
			HostID:        "00000000-0000-0000-0000-000000000000",
			NodeName:      "auth.example.com",
			ClusterName:   "example.com",
			Role:          types.RoleAdmin,
			TTL:           time.Hour,
		})
	require.NoError(t, err)

	certificate, err := sshutils.ParseCertificate(cert)
	require.NoError(t, err)

	// Check the valid time is not more than 1 minute before the current time.
	validAfter := time.Unix(int64(certificate.ValidAfter), 0)
	require.Equal(t, validAfter.Unix(), s.Clock.Now().UTC().Add(-1*time.Minute).Unix())

	// Check the valid time is not more than 1 hour after the current time.
	validBefore := time.Unix(int64(certificate.ValidBefore), 0)
	require.Equal(t, validBefore.Unix(), s.Clock.Now().UTC().Add(1*time.Hour).Unix())
}

func (s *AuthSuite) GenerateUserCert(t *testing.T) {
	priv, pub, err := s.Keygen()
	require.NoError(t, err)

	caSigner, err := ssh.ParsePrivateKey(priv)
	require.NoError(t, err)

	cert, err := s.A.GenerateUserCert(services.UserCertParams{
		CASigner:              caSigner,
		PublicUserKey:         pub,
		Username:              "user",
		AllowedLogins:         []string{"centos", "root"},
		TTL:                   time.Hour,
		PermitAgentForwarding: true,
		PermitPortForwarding:  true,
		CertificateFormat:     constants.CertificateFormatStandard,
	})
	require.NoError(t, err)

	// Check the valid time is not more than 1 minute before and 1 hour after
	// the current time.
	err = checkCertExpiry(cert, s.Clock.Now().Add(-1*time.Minute), s.Clock.Now().Add(1*time.Hour))
	require.NoError(t, err)

	cert, err = s.A.GenerateUserCert(services.UserCertParams{
		CASigner:              caSigner,
		PublicUserKey:         pub,
		Username:              "user",
		AllowedLogins:         []string{"root"},
		TTL:                   -20,
		PermitAgentForwarding: true,
		PermitPortForwarding:  true,
		CertificateFormat:     constants.CertificateFormatStandard,
	})
	require.NoError(t, err)
	err = checkCertExpiry(cert, s.Clock.Now().Add(-1*time.Minute), s.Clock.Now().Add(apidefaults.MinCertDuration))
	require.NoError(t, err)

	_, err = s.A.GenerateUserCert(services.UserCertParams{
		CASigner:              caSigner,
		PublicUserKey:         pub,
		Username:              "user",
		AllowedLogins:         []string{"root"},
		TTL:                   0,
		PermitAgentForwarding: true,
		PermitPortForwarding:  true,
		CertificateFormat:     constants.CertificateFormatStandard,
	})
	require.NoError(t, err)
	err = checkCertExpiry(cert, s.Clock.Now().Add(-1*time.Minute), s.Clock.Now().Add(apidefaults.MinCertDuration))
	require.NoError(t, err)

	_, err = s.A.GenerateUserCert(services.UserCertParams{
		CASigner:              caSigner,
		PublicUserKey:         pub,
		Username:              "user",
		AllowedLogins:         []string{"root"},
		TTL:                   time.Hour,
		PermitAgentForwarding: true,
		PermitPortForwarding:  true,
		CertificateFormat:     constants.CertificateFormatStandard,
	})
	require.NoError(t, err)

	inRoles := []string{"role-1", "role-2"}
	impersonator := "alice"
	cert, err = s.A.GenerateUserCert(services.UserCertParams{
		CASigner:              caSigner,
		PublicUserKey:         pub,
		Username:              "user",
		Impersonator:          impersonator,
		AllowedLogins:         []string{"root"},
		TTL:                   time.Hour,
		PermitAgentForwarding: true,
		PermitPortForwarding:  true,
		CertificateFormat:     constants.CertificateFormatStandard,
		Roles:                 inRoles,
	})
	require.NoError(t, err)
	parsedCert, err := sshutils.ParseCertificate(cert)
	require.NoError(t, err)
	outRoles, err := services.UnmarshalCertRoles(parsedCert.Extensions[teleport.CertExtensionTeleportRoles])
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(outRoles, inRoles))

	outImpersonator := parsedCert.Extensions[teleport.CertExtensionImpersonator]
	require.Empty(t, cmp.Diff(outImpersonator, impersonator))

	// Check that MFAVerified and PreviousIdentityExpires are encoded into ssh cert
	clock := clockwork.NewFakeClock()
	cert, err = s.A.GenerateUserCert(services.UserCertParams{
		CASigner:                caSigner,
		PublicUserKey:           pub,
		Username:                "user",
		AllowedLogins:           []string{"root"},
		TTL:                     time.Minute,
		CertificateFormat:       constants.CertificateFormatStandard,
		MFAVerified:             "mfa-device-id",
		PreviousIdentityExpires: clock.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	parsedCert, err = sshutils.ParseCertificate(cert)
	require.NoError(t, err)
	require.Contains(t, parsedCert.Extensions, teleport.CertExtensionMFAVerified)
	require.Equal(t, "mfa-device-id", parsedCert.Extensions[teleport.CertExtensionMFAVerified])
	require.Contains(t, parsedCert.Extensions, teleport.CertExtensionPreviousIdentityExpires)
	prevIDExpires, err := time.Parse(time.RFC3339, parsedCert.Extensions[teleport.CertExtensionPreviousIdentityExpires])
	require.NoError(t, err)
	require.WithinDuration(t, clock.Now().Add(time.Hour), prevIDExpires, time.Second)

	t.Run("device extensions", func(t *testing.T) {
		const devID = "deviceid1"
		const devTag = "devicetag1"
		const devCred = "devicecred1"
		certRaw, err := s.A.GenerateUserCert(services.UserCertParams{
			CASigner:           caSigner,          // Required.
			PublicUserKey:      pub,               // Required.
			Username:           "llama",           // Required.
			AllowedLogins:      []string{"llama"}, // Required.
			DeviceID:           devID,
			DeviceAssetTag:     devTag,
			DeviceCredentialID: devCred,
		})
		require.NoError(t, err, "GenerateUserCert failed")

		sshCert, err := sshutils.ParseCertificate(certRaw)
		require.NoError(t, err, "ParseCertificate failed")
		assert.Equal(t, devID, sshCert.Extensions[teleport.CertExtensionDeviceID], "DeviceID mismatch")
		assert.Equal(t, devTag, sshCert.Extensions[teleport.CertExtensionDeviceAssetTag], "AssetTag mismatch")
		assert.Equal(t, devCred, sshCert.Extensions[teleport.CertExtensionDeviceCredentialID], "CredentialID mismatch")
	})
}

func checkCertExpiry(cert []byte, after, before time.Time) error {
	certificate, err := sshutils.ParseCertificate(cert)
	if err != nil {
		return trace.Wrap(err)
	}

	validAfter := time.Unix(int64(certificate.ValidAfter), 0)
	if !validAfter.Equal(after) {
		return trace.BadParameter("ValidAfter incorrect: got %v, want %v", validAfter, after)
	}
	validBefore := time.Unix(int64(certificate.ValidBefore), 0)
	if !validBefore.Equal(before) {
		return trace.BadParameter("ValidBefore incorrect: got %v, want %v", validBefore, before)
	}
	return nil
}
