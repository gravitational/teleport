// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package windows

import (
	"context"
	"crypto/x509"
	"encoding/asn1"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
)

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

	client, err := tlsServer.NewClient(auth.TestServerID(types.RoleWindowsDesktop, "test-host-id"))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, client.Close())
	})

	ldapConfig := LDAPConfig{
		Domain: domain,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	testSid := "S-1-5-21-1329593140-2634913955-1900852804-500"

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
		t.Run(test.name, func(t *testing.T) {
			certb, keyb, err := GenerateWindowsDesktopCredentials(ctx, &GenerateCredentialsRequest{
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
					require.Equal(t, san.OtherName.OID, UPNOtherNameOID)
					require.Equal(t, san.OtherName.Value.Value, user+"@"+domain)
				case extension.Id.Equal(ADUserMappingExtensionOID):
					foundAdUserMapping = true
					var adUserMapping SubjectAltName[adSid]
					_, err = asn1.Unmarshal(extension.Value, &adUserMapping)
					require.NoError(t, err)
					require.Equal(t, adUserMapping.OtherName.OID, ADUserMappingInternalOID)
					require.Equal(t, adUserMapping.OtherName.Value.Value, []byte(testSid))

				}
			}
			require.True(t, foundKeyUsage)
			require.True(t, foundAltName)
			require.Equal(t, test.activeDirectorySID != "", foundAdUserMapping)
		})
	}
}

func TestCRLDN(t *testing.T) {
	for _, test := range []struct {
		clusterName string
		crlDN       string
	}{
		{
			clusterName: "test",
			crlDN:       "CN=test,CN=Teleport,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration,DC=test,DC=goteleport,DC=com",
		},
		{
			clusterName: "cluster.goteleport.com",
			crlDN:       "CN=cluster.goteleport.com,CN=Teleport,CN=CDP,CN=Public Key Services,CN=Services,CN=Configuration,DC=test,DC=goteleport,DC=com",
		},
	} {
		t.Run(test.clusterName, func(t *testing.T) {
			cfg := LDAPConfig{
				Domain: "test.goteleport.com",
			}
			require.Equal(t, test.crlDN, crlDN(test.clusterName, cfg))
		})
	}
}
