package kinit

import (
	"context"
	"crypto/tls"
	"crypto/x509"
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
	return nil, trace.NotImplemented("GenerateWindowsDesktopCert")
}

func (m *mockAuthClient) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadKeys bool) (types.CertAuthority, error) {
	return nil, trace.NotImplemented("GetCertAuthority")
}

func (m *mockAuthClient) GetClusterName(ctx context.Context) (types.ClusterName, error) {
	return nil, trace.NotImplemented("GetClusterName")
}

func TestTLSConfigForLDAP(t *testing.T) {
	mockCACert := &x509.Certificate{}

	connector := newLDAPConnector(ldapConnectorConfig{
		authClient: &mockAuthClient{
			generateDatabaseCert: func(ctx context.Context, request *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error) {
				csr, err := tlsca.ParseCertificateRequestPEM(request.CSR)
				if err != nil {
					return nil, trace.Wrap(err)
				}
				require.Equal(t, "CN=test-user", csr.Subject.String())
				require.Len(t, csr.Extensions, 3)
				return generateDatabaseCert(ctx, request)
			},
		},
		ldapConfig: ldapConfig{
			ServiceAccount:    "DOMAIN\\test-user",
			ServiceAccountSID: "S-1-5-21-2191801808-3167526388-2669316733-1104",
			Domain:            "example.com",
			TLSServerName:     "ldap.example.com",
			TLSCACert:         mockCACert,
		},
		clusterName: "test-cluster",
	})

	ctx := context.Background()
	tlsConfig, err := connector.tlsConfigForLDAP(ctx)
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
	mockCACert := &x509.Certificate{}

	connector := newLDAPConnector(ldapConnectorConfig{
		authClient: &mockAuthClient{},
		ldapConfig: ldapConfig{
			ServiceAccount:    "DOMAIN\\test-service-account",
			ServiceAccountSID: "S-1-5-21-2191801808-3167526388-2669316733-1104",
			Domain:            "example.com",
			TLSServerName:     "ldap.example.com",
			TLSCACert:         mockCACert,
		},
		clusterName: "test-cluster",
	})

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
