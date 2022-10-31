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

	certb, keyb, err := GenerateCredentials(ctx, user, domain, CertTTL, clusterName, ldapConfig, client)
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
			var san SubjectAltName
			_, err = asn1.Unmarshal(extension.Value, &san)
			require.NoError(t, err)

			require.Equal(t, san.OtherName.OID, UPNOtherNameOID)
			require.Equal(t, san.OtherName.Value.Value, user+"@"+domain)
		}
	}
	require.True(t, foundKeyUsage)
	require.True(t, foundAltName)
}
