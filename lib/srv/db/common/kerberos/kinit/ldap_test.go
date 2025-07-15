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

	"github.com/go-ldap/ldap/v3"
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

func TestTLSConfigForLDAP(t *testing.T) {
	auth := &mockAuthClient{
		generateDatabaseCert: func(ctx context.Context, request *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error) {
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
		Domain:                 "example.com",
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
}

type mockLDAPClient struct {
	searchWithPaging func(searchRequest *ldap.SearchRequest, pagingSize uint32) (*ldap.SearchResult, error)
	ldap.Client
}

func (m *mockLDAPClient) SearchWithPaging(searchRequest *ldap.SearchRequest, pagingSize uint32) (*ldap.SearchResult, error) {
	if m.searchWithPaging == nil {
		return nil, trace.BadParameter("callback function searchWithPaging not set")
	}
	return m.searchWithPaging(searchRequest, pagingSize)
}

func TestGetActiveDirectorySID(t *testing.T) {
	adConfig := types.AD{
		KeytabFile:             "",
		Krb5File:               "",
		SPN:                    "",
		Domain:                 "example.com",
		LDAPCert:               fixtures.TLSCACertPEM,
		KDCHostName:            "ldap.example.com",
		LDAPServiceAccountName: "DOMAIN\\test-service-account",
		LDAPServiceAccountSID:  "S-1-5-21-2191801808-3167526388-2669316733-1104",
	}

	connector, err := newLDAPConnector(slog.Default(), &mockAuthClient{}, adConfig)
	require.NoError(t, err)

	connector.dialLDAPServerFunc = func(ctx context.Context) (ldap.Client, error) {
		return &mockLDAPClient{searchWithPaging: func(searchRequest *ldap.SearchRequest, pagingSize uint32) (*ldap.SearchResult, error) {
			if searchRequest.BaseDN != "DC=example,DC=com" {
				return nil, trace.BadParameter("unexpected value of base_dn")
			}
			if searchRequest.Filter != "(\u0026(sAMAccountType=805306368)(sAMAccountName=DOMAIN\\test-user))" {
				return nil, trace.BadParameter("unexpected value of filter")
			}
			if len(searchRequest.Attributes) != 1 {
				return nil, trace.BadParameter("unexpected number of search attributes")
			}
			if searchRequest.Attributes[0] != "objectSid" {
				return nil, trace.BadParameter("unexpected value of search attribute")
			}

			const sidValue = "\u0001\u0005\u0000\u0000\u0000\u0000\u0000\u0005\u0015\u0000\u0000\u0000\ufffd=\ufffd\ufffd\ufffd\ufffd̼}\ufffd\u001a\ufffd\ufffd\u0001\u0000\u0000"

			attr := ldap.NewEntryAttribute("objectSid", []string{sidValue})

			return &ldap.SearchResult{
				Entries: []*ldap.Entry{
					{
						DN:         "CN=test-user,CN=Users,DC=example,DC=com",
						Attributes: []*ldap.EntryAttribute{attr},
					},
				},
			}, nil
		}}, nil
	}

	sid, err := connector.GetActiveDirectorySID(context.Background(), "DOMAIN\\test-user")
	require.NoError(t, err)
	require.Equal(t, "S-1-5-21-1035845615-4022190063-3220159935-3183472573", sid)
}
