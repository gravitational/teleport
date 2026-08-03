// Teleport
// Copyright (C) 2025 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package kinit

import (
	"context"
	"crypto/tls"
	"log/slog"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/tlsca"
)

func generateDatabaseCert(_ context.Context, req *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error) {
	csr, err := tlsca.ParseCertificateRequestPEM(req.CSR)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCACert, err := tls.X509KeyPair([]byte(fixtures.TLSCACertPEM), []byte(fixtures.TLSCAKeyPEM))
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsCA, err := tlsca.FromTLSCertificate(tlsCACert)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	certReq := tlsca.CertificateRequest{
		PublicKey: csr.PublicKey,
		Subject:   csr.Subject,
		NotAfter:  time.Now().Add(req.TTL.Get()),
		DNSNames:  req.ServerNames,
	}
	cert, err := tlsCA.GenerateCertificate(certReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &proto.DatabaseCertResponse{
		Cert: cert,
		CACerts: [][]byte{
			[]byte(fixtures.TLSCACertPEM),
		},
	}, nil
}

type mockAuthClient struct {
	generateDatabaseCert func(ctx context.Context, request *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error)
}

func (m *mockAuthClient) GenerateDatabaseCert(ctx context.Context, request *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error) {
	if m.generateDatabaseCert == nil {
		return nil, trace.BadParameter("generateDatabaseCert callback function not set")
	}
	return m.generateDatabaseCert(ctx, request)
}

func (m *mockAuthClient) GenerateWindowsDesktopCert(ctx context.Context, request *proto.WindowsDesktopCertRequest) (*proto.WindowsDesktopCertResponse, error) {
	return nil, trace.NotImplemented("GenerateWindowsDesktopCert not implemented")
}

func (m *mockAuthClient) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error) {
	return nil, trace.NotImplemented("GetCertAuthority not implemented")
}

func (m *mockAuthClient) GetClusterName(ctx context.Context) (types.ClusterName, error) {
	return types.NewClusterName(types.ClusterNameSpecV2{ClusterName: "test-cluster", ClusterID: "test-cluster-id"})
}

// TestNewLDAPConnectorEndpoint verifies how the LDAP endpoint is derived from
// the AD configuration. By default the LDAP server is assumed to be reachable
// under the KDC host name; ldap_host and ldap_tls_server_name override that.
func TestNewLDAPConnectorEndpoint(t *testing.T) {
	for _, tt := range []struct {
		name              string
		kdcHostName       string
		ldapHost          string
		ldapTLSServerName string
		wantAddress       string
		wantTLSServerName string
		wantErrMessage    string
	}{
		{
			name:              "LDAP endpoint defaults to KDC host name",
			kdcHostName:       "kdc.example.com",
			wantAddress:       "kdc.example.com",
			wantTLSServerName: "kdc.example.com",
		},
		{
			name:              "ldap_host overrides address and TLS server name",
			kdcHostName:       "kdc.example.com",
			ldapHost:          "ldap.example.com",
			wantAddress:       "ldap.example.com",
			wantTLSServerName: "ldap.example.com",
		},
		{
			name:              "ldap_tls_server_name overrides TLS server name only",
			kdcHostName:       "kdc.example.com",
			ldapTLSServerName: "ldap.example.com",
			wantAddress:       "kdc.example.com",
			wantTLSServerName: "ldap.example.com",
		},
		{
			name:              "ldap_host and ldap_tls_server_name are independent",
			kdcHostName:       "kdc.example.com",
			ldapHost:          "10.0.0.1:636",
			ldapTLSServerName: "ldap.example.com",
			wantAddress:       "10.0.0.1:636",
			wantTLSServerName: "ldap.example.com",
		},
		{
			name:              "port is trimmed from the derived TLS server name",
			kdcHostName:       "kdc.example.com",
			ldapHost:          "ldap.example.com:3269",
			wantAddress:       "ldap.example.com:3269",
			wantTLSServerName: "ldap.example.com",
		},
		{
			name:              "port is trimmed from the derived TLS server name of the KDC",
			kdcHostName:       "kdc.example.com:636",
			wantAddress:       "kdc.example.com:636",
			wantTLSServerName: "kdc.example.com",
		},
		{
			name:           "KDC host name is required even when ldap_host is set",
			ldapHost:       "ldap.example.com",
			wantErrMessage: "missing KDC host name",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			adConfig := types.AD{
				Domain:                 "example.com",
				LDAPCert:               fixtures.TLSCACertPEM,
				KDCHostName:            tt.kdcHostName,
				LDAPHost:               tt.ldapHost,
				LDAPTLSServerName:      tt.ldapTLSServerName,
				LDAPServiceAccountName: "DOMAIN\\test-user",
				LDAPServiceAccountSID:  "S-1-5-21-2191801808-3167526388-2669316733-1104",
			}

			connector, err := newLDAPConnector(slog.Default(), &mockAuthClient{}, adConfig)
			if tt.wantErrMessage != "" {
				require.ErrorContains(t, err, tt.wantErrMessage)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantAddress, connector.ldapConfig.address)
			require.Equal(t, tt.wantTLSServerName, connector.ldapConfig.tlsServerName)
		})
	}
}

func TestTLSConfigForLDAP(t *testing.T) {
	for _, tt := range []struct {
		name          string
		domain        string
		pkiDomain     string
		wantCRLDomain string
	}{
		{
			name:          "CRL domain defaults to domain",
			domain:        "example.com",
			wantCRLDomain: "example.com",
		},
		{
			name:          "pki_domain overrides CRL domain",
			domain:        "child.example.com",
			pkiDomain:     "example.com",
			wantCRLDomain: "example.com",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			auth := &mockAuthClient{
				generateDatabaseCert: func(ctx context.Context, request *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error) {
					require.Equal(t, tt.wantCRLDomain, request.CRLDomain)

					csr, err := tlsca.ParseCertificateRequestPEM(request.CSR)
					if err != nil {
						return nil, trace.Wrap(err)
					}
					require.Equal(t, "CN=test-user", csr.Subject.String())
					require.Len(t, csr.Extensions, 3)
					return generateDatabaseCert(ctx, request)
				},
			}

			adConfig := types.AD{
				Domain:                 tt.domain,
				PKIDomain:              tt.pkiDomain,
				LDAPCert:               fixtures.TLSCACertPEM,
				KDCHostName:            "ldap.example.com",
				LDAPServiceAccountName: "DOMAIN\\test-user",
				LDAPServiceAccountSID:  "S-1-5-21-2191801808-3167526388-2669316733-1104",
			}

			connector, err := newLDAPConnector(slog.Default(), auth, adConfig)
			require.NoError(t, err)

			ctx := context.Background()
			tlsConfig, err := connector.tlsConfigForLDAP(ctx, "test-cluster")
			require.NoError(t, err)
			require.NotNil(t, tlsConfig)
			require.Equal(t, "ldap.example.com", tlsConfig.ServerName)
			require.NotEmpty(t, tlsConfig.Certificates)
			require.NotNil(t, tlsConfig.RootCAs)
		})
	}
}
