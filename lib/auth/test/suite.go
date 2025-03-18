/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
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

	cert, err := s.A.GenerateHostCert(sshca.HostCertificateRequest{
		CASigner:      caSigner,
		PublicHostKey: pub,
		HostID:        "00000000-0000-0000-0000-000000000000",
		NodeName:      "auth.example.com",
		TTL:           time.Hour,
		Identity: sshca.Identity{
			ClusterName: "example.com",
			SystemRole:  types.RoleAdmin,
		},
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

	cert, err := s.A.GenerateUserCert(sshca.UserCertificateRequest{
		CASigner:          caSigner,
		PublicUserKey:     pub,
		TTL:               time.Hour,
		CertificateFormat: constants.CertificateFormatStandard,
		Identity: sshca.Identity{
			Username:              "user",
			Principals:            []string{"centos", "root"},
			PermitAgentForwarding: true,
			PermitPortForwarding:  true,
		},
	})
	require.NoError(t, err)

	// Check the valid time is not more than 1 minute before and 1 hour after
	// the current time.
	err = checkCertExpiry(cert, s.Clock.Now().Add(-1*time.Minute), s.Clock.Now().Add(1*time.Hour))
	require.NoError(t, err)

	cert, err = s.A.GenerateUserCert(sshca.UserCertificateRequest{
		CASigner:          caSigner,
		PublicUserKey:     pub,
		TTL:               -20,
		CertificateFormat: constants.CertificateFormatStandard,
		Identity: sshca.Identity{
			Username:              "user",
			Principals:            []string{"root"},
			PermitAgentForwarding: true,
			PermitPortForwarding:  true,
		},
	})
	require.NoError(t, err)
	err = checkCertExpiry(cert, s.Clock.Now().Add(-1*time.Minute), s.Clock.Now().Add(apidefaults.MinCertDuration))
	require.NoError(t, err)

	_, err = s.A.GenerateUserCert(sshca.UserCertificateRequest{
		CASigner:          caSigner,
		PublicUserKey:     pub,
		TTL:               0,
		CertificateFormat: constants.CertificateFormatStandard,
		Identity: sshca.Identity{
			Username:              "user",
			Principals:            []string{"root"},
			PermitAgentForwarding: true,
			PermitPortForwarding:  true,
		},
	})
	require.NoError(t, err)
	err = checkCertExpiry(cert, s.Clock.Now().Add(-1*time.Minute), s.Clock.Now().Add(apidefaults.MinCertDuration))
	require.NoError(t, err)

	_, err = s.A.GenerateUserCert(sshca.UserCertificateRequest{
		CASigner:          caSigner,
		PublicUserKey:     pub,
		TTL:               time.Hour,
		CertificateFormat: constants.CertificateFormatStandard,
		Identity: sshca.Identity{
			Username:              "user",
			Principals:            []string{"root"},
			PermitAgentForwarding: true,
			PermitPortForwarding:  true,
		},
	})
	require.NoError(t, err)

	inRoles := []string{"role-1", "role-2"}
	impersonator := "alice"
	cert, err = s.A.GenerateUserCert(sshca.UserCertificateRequest{
		CASigner:          caSigner,
		PublicUserKey:     pub,
		TTL:               time.Hour,
		CertificateFormat: constants.CertificateFormatStandard,
		Identity: sshca.Identity{
			Username:              "user",
			Impersonator:          impersonator,
			Principals:            []string{"root"},
			PermitAgentForwarding: true,
			PermitPortForwarding:  true,
			Roles:                 inRoles,
		},
	})
	require.NoError(t, err)
	parsedCert, err := sshutils.ParseCertificate(cert)
	require.NoError(t, err)

	parsedIdent, err := sshca.DecodeIdentity(parsedCert)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(parsedIdent.Roles, inRoles))

	require.Empty(t, cmp.Diff(parsedIdent.Impersonator, impersonator))

	// Check that MFAVerified and PreviousIdentityExpires are encoded into ssh cert
	clock := clockwork.NewFakeClock()
	cert, err = s.A.GenerateUserCert(sshca.UserCertificateRequest{
		CASigner:          caSigner,
		PublicUserKey:     pub,
		TTL:               time.Minute,
		CertificateFormat: constants.CertificateFormatStandard,
		Identity: sshca.Identity{
			Username:                "user",
			Principals:              []string{"root"},
			MFAVerified:             "mfa-device-id",
			PreviousIdentityExpires: clock.Now().Add(time.Hour),
		},
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
		certRaw, err := s.A.GenerateUserCert(sshca.UserCertificateRequest{
			CASigner:      caSigner, // Required.
			PublicUserKey: pub,      // Required.
			Identity: sshca.Identity{
				Username:           "llama",           // Required.
				Principals:         []string{"llama"}, // Required.
				DeviceID:           devID,
				DeviceAssetTag:     devTag,
				DeviceCredentialID: devCred,
			},
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
