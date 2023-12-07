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

package database

import (
	"context"
	"crypto/tls"
	"crypto/x509/pkix"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/srv/db/common"
	"github.com/gravitational/teleport/lib/srv/db/postgres"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestPostgresErrors(t *testing.T) {
	p := PostgresPinger{}

	for _, tt := range []struct {
		name     string
		pingErr  error
		errCheck require.ErrorAssertionFunc
	}{
		{
			name:    "connection refused error",
			pingErr: errors.New("failed to connect to `host=127.0.0.1 user=postgres database=postgres`: server error (: connection refused (SQLSTATE ))"),
			errCheck: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(tt, p.IsConnectionRefusedError(err))
			},
		},
		{
			name: "invalid database error",
			pingErr: &pgconn.PgError{
				Code: pgerrcode.InvalidCatalogName,
			},
			errCheck: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(tt, p.IsInvalidDatabaseNameError(err))
			},
		},
		{
			name: "invalid user error",
			pingErr: &pgconn.PgError{
				Code: pgerrcode.InvalidAuthorizationSpecification,
			},
			errCheck: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(tt, p.IsInvalidDatabaseUserError(err))
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.errCheck(t, tt.pingErr)
		})
	}
}

// mockClient is a mock that implements AuthClient interface.

type mockClient struct {
	common.AuthClientCA

	ca types.CertAuthority
}

func setupMockClient(t *testing.T) *mockClient {
	t.Helper()

	_, cert, err := tlsca.GenerateSelfSignedCA(pkix.Name{CommonName: "example.com"}, nil, time.Minute)
	require.NoError(t, err)

	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        types.HostCA,
		ClusterName: "example.com",
		ActiveKeys: types.CAKeySet{
			SSH: []*types.SSHKeyPair{{PublicKey: []byte("SSH CA cert")}},
			TLS: []*types.TLSKeyPair{{Cert: cert}},
		},
	})
	require.NoError(t, err)

	return &mockClient{
		ca: ca,
	}
}

func (c *mockClient) GenerateDatabaseCert(ctx context.Context, req *proto.DatabaseCertRequest) (*proto.DatabaseCertResponse, error) {
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

func (c *mockClient) GetCertAuthority(ctx context.Context, id types.CertAuthID, loadSigningKeys bool) (types.CertAuthority, error) {
	return c.ca, nil
}

func TestPostgresPing(t *testing.T) {
	mockClt := setupMockClient(t)

	postgresTestServer, err := postgres.NewTestServer(common.TestServerConfig{
		AuthClient: mockClt,
	})
	require.NoError(t, err)

	go func() {
		t.Logf("Postgres Fake server running at %s port", postgresTestServer.Port())
		require.NoError(t, postgresTestServer.Serve())
	}()
	t.Cleanup(func() {
		postgresTestServer.Close()
	})

	port, err := strconv.Atoi(postgresTestServer.Port())
	require.NoError(t, err)

	p := PostgresPinger{}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	err = p.Ping(ctx, PingParams{
		Host:         "localhost",
		Port:         port,
		Username:     "someuser",
		DatabaseName: "somedb",
	})

	require.NoError(t, err)
}
