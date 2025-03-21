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

package windows

import (
	"context"
	"crypto/x509"
	"encoding/asn1"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/modules"
)

func TestMain(m *testing.M) {
	modules.SetInsecureTestMode(true)
	os.Exit(m.Run())
}

// TestGenerateCredentials verifies that the smartcard certificates generated
// by Teleport meet the requirements for Windows logon.
func TestGenerateCredentials(t *testing.T) {
	const (
		clusterName = "test"
		user        = "test-user"
		domain      = "test.example.com"
	)

	authServer, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		ClusterName: clusterName,
		Dir:         t.TempDir(),
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, authServer.Close())
	})

	tlsServer, err := authServer.NewTestTLSServer()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, tlsServer.Close())
	})

	ldapConfig := LDAPConfig{
		Domain: domain,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	testSid := "S-1-5-21-1329593140-2634913955-1900852804-500"

	for _, credType := range []struct {
		name                    string
		role                    types.SystemRole
		generateCredentialsFunc func(t *testing.T, ctx context.Context, req *GenerateCredentialsRequest) (certPEM, keyPEM []byte)
	}{
		{
			name: "desktop",
			role: types.RoleWindowsDesktop,
			generateCredentialsFunc: func(t *testing.T, ctx context.Context, req *GenerateCredentialsRequest) (certPEM, keyPEM []byte) {
				cert, key, err := GenerateWindowsDesktopCredentials(ctx, req)
				require.NoError(t, err)
				return cert, key
			},
		},
		{
			name: "database",
			role: types.RoleDatabase,
			generateCredentialsFunc: func(t *testing.T, ctx context.Context, req *GenerateCredentialsRequest) (certPEM, keyPEM []byte) {
				cert, key, _, err := generateDatabaseCredentials(ctx, req)
				require.NoError(t, err)
				return cert, key
			},
		},
	} {
		t.Run(credType.name, func(t *testing.T) {
			for _, test := range []struct {
				name               string
				activeDirectorySID string
			}{
				{
					name:               "no ad sid",
					activeDirectorySID: "",
				},
				{
					name:               "with ad sid",
					activeDirectorySID: testSid,
				},
			} {
				client, err := tlsServer.NewClient(auth.TestServerID(credType.role, "test-host-id"))
				require.NoError(t, err)
				t.Cleanup(func() {
					require.NoError(t, client.Close())
				})

				t.Run(test.name, func(t *testing.T) {
					certb, keyb := credType.generateCredentialsFunc(t, ctx, &GenerateCredentialsRequest{
						Username:           user,
						Domain:             domain,
						TTL:                CertTTL,
						ClusterName:        clusterName,
						ActiveDirectorySID: test.activeDirectorySID,
						LDAPConfig:         ldapConfig,
						AuthClient:         client,
					})
					require.NoError(t, err)
					require.NotNil(t, certb)
					require.NotNil(t, keyb)

					cert, err := x509.ParseCertificate(certb)
					require.NoError(t, err)
					require.NotNil(t, cert)

					require.Equal(t, user, cert.Subject.CommonName)
					require.Contains(t, cert.CRLDistributionPoints,
						`ldap:///CN=test,CN=Teleport,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration,DC=test,DC=example,DC=com?certificateRevocationList?base?objectClass=cRLDistributionPoint`)

					foundKeyUsage := false
					foundAltName := false
					foundAdUserMapping := false
					for _, extension := range cert.Extensions {
						switch {
						case extension.Id.Equal(EnhancedKeyUsageExtensionOID):
							foundKeyUsage = true
							var oids []asn1.ObjectIdentifier
							_, err = asn1.Unmarshal(extension.Value, &oids)
							require.NoError(t, err)
							require.Len(t, oids, 2)
							require.Contains(t, oids, ClientAuthenticationOID)
							require.Contains(t, oids, SmartcardLogonOID)
						case extension.Id.Equal(SubjectAltNameExtensionOID):
							foundAltName = true
							var san SubjectAltName[upn]
							_, err = asn1.Unmarshal(extension.Value, &san)
							require.NoError(t, err)
							require.Equal(t, UPNOtherNameOID, san.OtherName.OID)
							require.Equal(t, user+"@"+domain, san.OtherName.Value.Value)
						case extension.Id.Equal(ADUserMappingExtensionOID):
							foundAdUserMapping = true
							var adUserMapping SubjectAltName[adSid]
							_, err = asn1.Unmarshal(extension.Value, &adUserMapping)
							require.NoError(t, err)
							require.Equal(t, ADUserMappingInternalOID, adUserMapping.OtherName.OID)
							require.Equal(t, []byte(testSid), adUserMapping.OtherName.Value.Value)

						}
					}
					require.True(t, foundKeyUsage)
					require.True(t, foundAltName)
					require.Equal(t, test.activeDirectorySID != "", foundAdUserMapping)
				})
			}

		})
	}
}

func TestCRLDN(t *testing.T) {
	for _, test := range []struct {
		name        string
		clusterName string
		crlDN       string
		caType      types.CertAuthType
	}{
		{
			name:        "test cluster name",
			clusterName: "test",
			crlDN:       "CN=test,CN=Teleport,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration,DC=test,DC=goteleport,DC=com",
		},
		{
			name:        "full cluster name",
			clusterName: "cluster.goteleport.com",
			crlDN:       "CN=cluster.goteleport.com,CN=Teleport,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration,DC=test,DC=goteleport,DC=com",
		},
		{
			name:        "database CA",
			clusterName: "cluster.goteleport.com",
			caType:      types.DatabaseClientCA,
			crlDN:       "CN=cluster.goteleport.com,CN=TeleportDB,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration,DC=test,DC=goteleport,DC=com",
		},
		{
			name:        "user CA",
			clusterName: "cluster.goteleport.com",
			caType:      types.UserCA,
			crlDN:       "CN=cluster.goteleport.com,CN=Teleport,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration,DC=test,DC=goteleport,DC=com",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := LDAPConfig{
				Domain: "test.goteleport.com",
			}
			require.Equal(t, test.crlDN, crlDN(test.clusterName, cfg, test.caType))
		})
	}
}
