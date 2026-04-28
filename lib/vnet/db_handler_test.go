// Teleport
// Copyright (C) 2026 Gravitational, Inc.
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

package vnet

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestDBCertIssuerCheckCert(t *testing.T) {
	t.Parallel()

	const (
		dbName   = "my-postgres"
		protocol = "postgres"
	)
	dbInfo := &vnetv1.DatabaseInfo{
		DatabaseKey: &vnetv1.DatabaseKey{
			Profile: "proxy.example.com",
			Name:    dbName,
		},
		Protocol: protocol,
	}
	issuer := &dbCertIssuer{dbInfo: dbInfo}

	tests := []struct {
		name           string
		certServiceID  string
		certProtocol   string
		certUsername   string
		expectAccept   bool
		expectErrSubst string
	}{
		{
			name:          "matching (service + protocol) + empty username",
			certServiceID: dbName,
			certProtocol:  protocol,
			certUsername:  "",
			expectAccept:  true,
		},
		{
			name:           "mismatched service name",
			certServiceID:  "other-postgres",
			certProtocol:   protocol,
			certUsername:   "",
			expectErrSubst: "database service",
		},
		{
			name:           "mismatched protocol",
			certServiceID:  dbName,
			certProtocol:   "mysql",
			certUsername:   "",
			expectErrSubst: "database protocol",
		},
		{
			name:           "non-empty subject username is rejected (H2 invariant)",
			certServiceID:  dbName,
			certProtocol:   protocol,
			certUsername:   "alice",
			expectErrSubst: "empty subject username",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cert := mustBuildDBCert(t, tlsca.RouteToDatabase{
				ServiceName: tt.certServiceID,
				Protocol:    tt.certProtocol,
				Username:    tt.certUsername,
			})
			err := issuer.CheckCert(cert)
			if tt.expectAccept {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.expectErrSubst)
		})
	}
}

// mustBuildDBCert returns a self-signed *x509.Certificate whose subject is
// derived from the given RouteToDatabase.
func mustBuildDBCert(t *testing.T, route tlsca.RouteToDatabase) *x509.Certificate {
	t.Helper()
	id := tlsca.Identity{
		Username:        "test-user",
		Groups:          []string{"access"},
		RouteToDatabase: route,
	}
	subject, err := id.Subject()
	require.NoError(t, err)

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      subject,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &priv.PublicKey, priv)
	require.NoError(t, err)
	cert, err := x509.ParseCertificate(der)
	require.NoError(t, err)
	return cert
}
